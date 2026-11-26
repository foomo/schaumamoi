package distillery_test

import (
	"slices"
	"testing"
	"time"

	"github.com/foomo/schaumamoi/pkg/distillery"
	"github.com/foomo/schaumamoi/pkg/distillery/recipes/visits"
	"github.com/foomo/schaumamoi/pkg/distillery/stills/affinity"
	"github.com/foomo/schaumamoi/pkg/internal/mock/catalog"
	"github.com/foomo/schaumamoi/pkg/internal/mock/persona"
	"github.com/foomo/schaumamoi/pkg/internal/mock/shop"
	"github.com/foomo/schaumamoi/pkg/internal/mock/shopper"
	"github.com/foomo/schaumamoi/pkg/internal/mock/wire"
	"github.com/foomo/schaumamoi/pkg/personalization"
	"github.com/foomo/sesamy-go/pkg/encoding/mpv2"
	"github.com/foomo/sesamy-go/pkg/sesamy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The still under test is the affinity still the shop ranks with (pkg/distillery/stills/affinity). The
// short names below are the tests' own for its parts, so the tests read as they always
// did: what is kept, how it is infused, and what is distilled.
type (
	envelope = affinity.Envelope
	infusion = affinity.Infusion

	testEvent   = sesamy.Event[any]
	testInfused = affinity.Infused
	testInfuser = affinity.Infuser
	testCut     = affinity.Cut
	testConfig  = distillery.Config[envelope, any, infusion]
	testStill   = affinity.Still
)

var (
	addAttractors    = affinity.AddAttractors
	weighByIntent    = affinity.WeighByIntent
	affinityInfusers = affinity.Infusers
	intentWeight     = affinity.Weights
	choices          = affinity.Choices
	newAffinityStill = affinity.New
	flat             = affinity.Flat
	decayed          = affinity.Decayed
)

// itemsParams is the one thing every ecommerce event has in common. Decoding into
// catalog.Item keeps the attractor and, in memory, the category — see TestItemDecode for why
// the GA4 fields come back zeroed off the wire.
type itemsParams struct {
	Items []catalog.Item `json:"items"`
}

func only(names ...sesamy.EventName) testCut {
	return func(_ envelope, e testEvent) bool {
		for _, name := range names {
			if e.Name == name {
				return true
			}
		}

		return false
	}
}

// kateJourney is the stream the still will actually see: heterogeneous GA4 events
// type-erased to sesamy.Event[any], as sesamy-go hands them downstream.
func kateJourney() []testEvent {
	return shopper.Journey(persona.Kate).Events()
}

// nearestIn is the product of one category that ranks first for a taste: how that
// category's listing would open for it. It ranks with the shop's own scorer, catalog.Score —
// the taste weighed by the category's weights — through Persona.TopMatch. With listings
// cut and choices weighed as the shop weighs them, a flat affinity is the shop's
// profile, so this is exactly the listing the shop would show. Cosine stays for comparing a taste with a persona, where the
// magnitude must not count; for ranking it disagrees with the shop on near ties — the
// sequin flares and the velvet palazzo, for Sofia.
func nearestIn(aff personalization.Map, category string) catalog.Item {
	return persona.Persona{Affinity: aff}.TopMatch(shop.Filter{Category: category}.Apply(catalog.All()))
}

// touchedCategories are the categories of the products a shopper selected, viewed or put
// in the cart, in the order first touched. Listings do not count: a category the shopper
// only scrolled through is one it was offered, not one it chose from.
func touchedCategories(t *testing.T, events []testEvent) []string {
	t.Helper()

	var out []string

	for _, e := range events {
		if _, ok := intentWeight[e.Name]; !ok {
			continue
		}

		var p itemsParams
		require.NoError(t, e.DecodeParams(&p))

		for _, item := range p.Items {
			if !slices.Contains(out, item.Category()) {
				out = append(out, item.Category())
			}
		}
	}

	return out
}

// assertPerCategory reads a distilled taste against the persona that produced the events,
// one category at a time, in every category the shopper chose from: there, the product
// the taste ranks first must be one the persona likes — within persona.GoodEnough of its
// favourite in that category. Not the favourite itself: the shopper does not always open
// its favourite, and the still can only know what it did open. Categories the shopper
// was only offered are not asserted — the still has nothing of theirs to go on.
func assertPerCategory(t *testing.T, aff personalization.Map, want persona.Persona, events []testEvent) {
	t.Helper()

	touched := touchedCategories(t, events)
	require.NotEmpty(t, touched)

	for _, category := range touched {
		items := shop.Filter{Category: category}.Apply(catalog.All())
		got := nearestIn(aff, category)

		assert.True(t, want.Likes(got, items), "%s for %s: %s ranks first, which %s does not like", category, want.Name, got.ItemName, want.Name)
	}
}

// addPayload is how a routed batch reaches the still: its events, with the envelope
// fields that differ between batches resolved into an envelope.
func addPayload(p *testStill, payload mpv2.Payload[any]) {
	p.Feed(envelopeOf(payload), payload.Events...)
}

// addSession feeds a visit into the still as the client sent it: request by request, each
// with its own envelope.
func addSession(p *testStill, session wire.Session) {
	for _, hit := range session {
		addPayload(p, hit)
	}
}

// envelopeOf resolves the fields of an mpv2 envelope that differ between batches.
func envelopeOf(payload mpv2.Payload[any]) envelope {
	return envelope{
		At:              time.UnixMicro(payload.TimestampMicros).UTC(),
		Session:         payload.SessionID,
		Personalization: personalizationGranted(payload.Consent),
	}
}

// personalizationGranted reads personalization consent off an envelope. No consent block means no
// decision was made, which is not consent.
func personalizationGranted(c *mpv2.ConsentData) bool {
	return c != nil && c.PersonalizationStorage != nil && *c.PersonalizationStorage == mpv2.ConsentGranted
}

// funnel counts kept events by name — a domain state that says nothing about taste.
func funnel(events []testInfused) map[sesamy.EventName]int {
	out := map[sesamy.EventName]int{}
	for _, e := range events {
		out[e.Event.Name]++
	}

	return out
}

// nearestPersona is the persona whose taste the affinity is closest to, ties broken by
// name so the result is stable.
func nearestPersona(aff personalization.Map, personas []persona.Persona) persona.Persona {
	var (
		best  persona.Persona
		score personalization.Value
	)

	for i, p := range personas {
		v := aff.Cosine(p.Affinity)
		if i == 0 || v > score || (v == score && p.Name < best.Name) {
			best, score = p, v
		}
	}

	return best
}

// nearestItem is the item whose attractor the affinity is closest to, ties broken by
// item id, as in Persona.TopMatch.
func nearestItem(aff personalization.Map, items []catalog.Item) catalog.Item {
	var (
		best  catalog.Item
		score personalization.Value
	)

	for i, item := range items {
		v := aff.Cosine(item.Attractor)
		if i == 0 || v > score || (v == score && item.ItemID < best.ItemID) {
			best, score = item, v
		}
	}

	return best
}

// purchased is the item the journey put in the cart.
func purchased(t *testing.T, events []testEvent) catalog.Item {
	t.Helper()

	for _, e := range events {
		if e.Name != sesamy.EventNameAddToCart {
			continue
		}

		var p itemsParams
		require.NoError(t, e.DecodeParams(&p))
		require.Len(t, p.Items, 1)

		return p.Items[0]
	}

	require.FailNow(t, "journey has no add_to_cart")

	return catalog.Item{}
}

// TestAffinityRecoversPersona is the use case: the affinity folded out of a shopper's
// stream must sit closer to the taste that generated the stream than to any other.
func TestAffinityRecoversPersona(t *testing.T) {
	t.Parallel()

	for _, who := range persona.All() {
		t.Run(who.Name, func(t *testing.T) {
			t.Parallel()

			p := newAffinityStill(nil)
			p.Feed(envelope{}, shopper.Journey(who).Events()...)

			aff := p.Distill(flat)
			assert.Equal(t, who.Name, nearestPersona(aff, persona.All()).Name)

			own := aff.Cosine(who.Affinity)
			for _, other := range persona.All() {
				if other.Name == who.Name {
					continue
				}

				assert.Greater(t, own, aff.Cosine(other.Affinity), "closer to %s than to %s", other.Name, who.Name)
			}
		})
	}
}

// TestAffinityNearestItemIsPurchase measures the other distance, per category: in every
// category the shopper chose from — dresses, trousers, and one in home decoration — the
// product the distilled taste ranks first is one the persona likes.
func TestAffinityNearestItemIsPurchase(t *testing.T) {
	t.Parallel()

	for _, who := range persona.All() {
		t.Run(who.Name, func(t *testing.T) {
			t.Parallel()

			events := shopper.Journey(who).Events()

			p := newAffinityStill(nil)
			p.Feed(envelope{}, events...)

			got := p.Distill(flat)
			assertPerCategory(t, got, who, events)

		})
	}
}

// TestFeedAppliesCut pins the affinity still's positive cut: of a whole journey it
// keeps only the choices — no listing gets in — and those alone say who the shopper is.
// A cut handed to newAffinityStill narrows that further, never widens it.
func TestFeedAppliesCut(t *testing.T) {
	t.Parallel()

	p := newAffinityStill(nil)
	p.Feed(envelope{}, kateJourney()...)

	assert.Equal(t, 10, p.Len())
	assert.Equal(t, map[sesamy.EventName]int{
		sesamy.EventNameSelectItem: 7,
		sesamy.EventNameViewItem:   2,
		sesamy.EventNameAddToCart:  1,
	}, p.Distill(funnel))
	assert.Equal(t, persona.Kate.Name, nearestPersona(p.Distill(flat), persona.All()).Name)

	carts := newAffinityStill(only(sesamy.EventNameAddToCart, sesamy.EventNameViewItemList))
	carts.Feed(envelope{}, kateJourney()...)
	assert.Equal(t, map[sesamy.EventName]int{sesamy.EventNameAddToCart: 1}, carts.Distill(funnel), "an extra cut cannot let listings back in")
}

// choicesIn is what the affinity still keeps of events: the choices, in order.
func choicesIn(events []testEvent) []testEvent {
	var out []testEvent

	for _, e := range events {
		if choices(envelope{}, e) {
			out = append(out, e)
		}
	}

	return out
}

// TestFeedWithoutCutKeepsEverything pins the zero-value config: a still that
// only collects must not have to write a keep-everything cut, and must not panic.
func TestFeedWithoutCutKeepsEverything(t *testing.T) {
	t.Parallel()

	events := kateJourney()

	p := distillery.New(testConfig{})
	require.NotPanics(t, func() { p.Feed(envelope{}, events...) })

	assert.Equal(t, len(events), p.Len())
	assert.Empty(t, p.Distill(flat), "no infusers, no infusion")
}

// TestInfusersFoldInOrder uses the fact that the affinity infusers do not commute:
// weighting before anything has been collected scales an empty map.
func TestInfusersFoldInOrder(t *testing.T) {
	t.Parallel()

	pick := persona.Kate.TopMatch(catalog.All())
	cart := []testEvent{shop.AddToCart(pick).AnyEvent()}

	distill := func(infusers ...testInfuser) personalization.Map {
		p := distillery.New(testConfig{Infusers: infusers})
		p.Feed(envelope{}, cart...)

		return p.Distill(flat)
	}

	inOrder, reversed := distill(addAttractors, weighByIntent), distill(weighByIntent, addAttractors)

	require.NotEmpty(t, pick.Attractor)
	for dimension, v := range pick.Attractor {
		assert.InDelta(t, float64(v*intentWeight[sesamy.EventNameAddToCart]), float64(inOrder[dimension]), 1e-6, "addAttractors, weighByIntent: %s", dimension)
		assert.InDelta(t, float64(v), float64(reversed[dimension]), 1e-6, "weighByIntent, addAttractors: %s", dimension)
	}
}

// TestDistillIsRepeatable is the regression. Infusion happens once, on arrival, so
// distilling state is a pure read: the affinity must not drift with the number of distills,
// and distilling must not write through into the infusions it was handed.
func TestDistillIsRepeatable(t *testing.T) {
	t.Parallel()

	p := newAffinityStill(nil)
	p.Feed(envelope{}, kateJourney()...)

	infusions := func(events []testInfused) []personalization.Map {
		out := make([]personalization.Map, 0, len(events))
		for _, e := range events {
			out = append(out, clone(e.Infusion.Affinity))
		}

		return out
	}

	before := p.Distill(infusions)
	first := p.Distill(flat)

	require.NotEmpty(t, first)
	for i := range 4 {
		assert.Equal(t, first, p.Distill(flat), "distill %d", i+2)
	}
	assert.Equal(t, before, p.Distill(infusions), "distilling modified the still")
}

// TestFeedAfterDistillDoesNotReinfuse is the other half of the regression: a later
// batch must not drag the infusers back over events that were already folded. Intent scales,
// so an event infused twice would carry its weight squared.
func TestFeedAfterDistillDoesNotReinfuse(t *testing.T) {
	t.Parallel()

	first, second := kateJourney(), shopper.BrowseJourney(persona.Lena).Events()

	p := newAffinityStill(nil)
	p.Feed(envelope{}, first...)
	require.NotEmpty(t, p.Distill(flat))
	p.Feed(envelope{}, second...)

	got := p.Distill(func(events []testInfused) []personalization.Map {
		out := make([]personalization.Map, 0, len(events))
		for _, e := range events {
			out = append(out, e.Infusion.Affinity)
		}

		return out
	})

	// Stated per event against a still that saw only that event, so a doubled first
	// batch cannot hide behind anything in the second.
	all := choicesIn(append(append([]testEvent{}, first...), second...))
	require.Len(t, got, len(all))
	for i, e := range all {
		fresh := newAffinityStill(nil)
		fresh.Feed(envelope{}, e)

		assert.Equal(t, fresh.Distill(flat), got[i], "event %d (%s)", i, e.Name)
	}
}

// TestOneStillManyRecipes is what moving Spirit off the still buys: recipes
// of unrelated types over a single ingest.
func TestOneStillManyRecipes(t *testing.T) {
	t.Parallel()

	p := newAffinityStill(nil)
	p.Feed(envelope{}, kateJourney()...)

	assert.Equal(t, map[sesamy.EventName]int{
		sesamy.EventNameSelectItem: 7,
		sesamy.EventNameViewItem:   2,
		sesamy.EventNameAddToCart:  1,
	}, p.Distill(funnel))

	assert.Equal(t, persona.Kate.Name, p.Distill(func(events []testInfused) persona.Persona {
		return nearestPersona(flat(events), persona.All())
	}).Name)
}

// TestDistillIsDeterministic pins the float summation: the same still must give a
// bit-identical affinity every time, and so must a second still fed the same stream,
// for the reason Map.Score sorts its dimensions.
func TestDistillIsDeterministic(t *testing.T) {
	t.Parallel()

	p := newAffinityStill(nil)
	p.Feed(envelope{}, kateJourney()...)

	want := p.Distill(flat)
	for range 8 {
		assert.Equal(t, want, p.Distill(flat))

		again := newAffinityStill(nil)
		again.Feed(envelope{}, kateJourney()...)
		assert.Equal(t, want, again.Distill(flat))
	}
}

const week = 7 * 24 * time.Hour

// TestStampReadsEnvelopeTime pins where recency comes from: every event carries the time
// of the request it was sent in — a visit's requests follow each other in time, so its
// events do too — and an event added with a zero envelope carries the zero time.
func TestStampReadsEnvelopeTime(t *testing.T) {
	t.Parallel()

	earlier, later := wire.BaseTime.Add(-week), wire.BaseTime

	p := newAffinityStill(nil)
	visits := []wire.Session{
		shopper.Journey(persona.Sofia, wire.WithTimestamp(earlier)),
		shopper.BrowseJourney(persona.Kate, wire.WithTimestamp(later)),
	}
	addHistory(p, visits)
	p.Feed(envelope{}, shop.AddToCart(persona.Kate.TopMatch(catalog.All())).AnyEvent())

	stamps := p.Distill(func(events []testInfused) []time.Time {
		out := make([]time.Time, 0, len(events))
		for _, e := range events {
			out = append(out, e.Infusion.At)
		}

		return out
	})

	var want []time.Time

	for _, session := range visits {
		for _, hit := range session {
			if choices(envelope{}, hit.Events[0]) {
				want = append(want, time.UnixMicro(hit.TimestampMicros).UTC())
			}
		}
	}

	want = append(want, time.Time{})

	assert.Equal(t, want, stamps)
	assert.True(t, stamps[0].After(earlier), "a visit's first choice is sent after its landing page, not with it")
	assert.True(t, slices.IsSortedFunc(stamps[:len(stamps)-1], func(a, b time.Time) int { return a.Compare(b) }), "events follow each other in time")
}

// TestDecayHalvesAtHalfLife pins the curve itself on a single purchase: full weight now,
// half after one half-life, a quarter after two — and no boost for an event from after
// now.
func TestDecayHalvesAtHalfLife(t *testing.T) {
	t.Parallel()

	pick := persona.Kate.TopMatch(catalog.All())
	cart := shop.AddToCart(pick).AnyEvent()

	for name, tt := range map[string]struct {
		age  time.Duration
		want personalization.Value
	}{
		"now":            {age: 0, want: 1},
		"one half-life":  {age: week, want: 0.5},
		"two half-lives": {age: 2 * week, want: 0.25},
		"from after now": {age: -week, want: 1},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			p := newAffinityStill(nil)
			p.Feed(envelope{At: wire.BaseTime.Add(-tt.age)}, cart)

			full := p.Distill(flat)
			got := p.Distill(decayed(wire.BaseTime, week))

			require.NotEmpty(t, full)
			for dimension, v := range full {
				assert.InDelta(t, float64(v*tt.want), float64(got[dimension]), 1e-6, dimension)
			}
		})
	}
}

// eventsOf is every event of a history, in order.
func eventsOf(history []wire.Session) []testEvent {
	var out []testEvent
	for _, session := range history {
		out = append(out, session.Events()...)
	}

	return out
}

// addHistory feeds a shopper's visits into the still, request by request.
func addHistory(p *testStill, history []wire.Session) {
	for _, session := range history {
		addSession(p, session)
	}
}

// drifted is a month that starts with one taste and settles on another: the first three
// visits, up to two weeks ago and including the first purchase, are old's; the rest,
// ending with today's purchase, are recent's.
func drifted(old, recent persona.Persona) []wire.Session {
	visits := shopper.Month(recent)
	for i := range 3 {
		visits[i].Persona = old
	}

	return shopper.History(visits)
}

// TestMonthRecoversPersona is TestAffinityRecoversPersona over a month of visits rather
// than a single session, distilled with decay: a steady taste must still come out as
// itself, and read per category as in TestAffinityNearestItemIsPurchase.
func TestMonthRecoversPersona(t *testing.T) {
	t.Parallel()

	for _, who := range persona.All() {
		t.Run(who.Name, func(t *testing.T) {
			t.Parallel()

			history := shopper.History(shopper.Month(who))
			p := newAffinityStill(nil)
			addHistory(p, history)

			got := p.Distill(decayed(wire.BaseTime, week))
			assert.Equal(t, who.Name, nearestPersona(got, persona.All()).Name)
			assertPerCategory(t, got, who, eventsOf(history))
		})
	}
}

// TestDecayFollowsRecentTaste is what decay is for. A shopper spends the first half of
// the month on one persona's taste and the second half on another's; with a one-week
// half-life the affinity must follow whichever came last — for every ordered pair, so
// it is recency deciding and not which of the two tastes happens to be louder.
func TestDecayFollowsRecentTaste(t *testing.T) {
	t.Parallel()

	for _, old := range persona.All() {
		for _, recent := range persona.All() {
			if old.Name == recent.Name {
				continue
			}

			t.Run(old.Name+" then "+recent.Name, func(t *testing.T) {
				t.Parallel()

				history := drifted(old, recent)
				p := newAffinityStill(nil)
				addHistory(p, history)

				got := p.Distill(decayed(wire.BaseTime, week))
				assert.Equal(t, recent.Name, nearestPersona(got, persona.All()).Name)
				assertPerCategory(t, got, recent, eventsOf(history[3:])) // the recent half
			})
		}
	}
}

// TestUndecayedMisreadsDrift is the other side of TestDecayFollowsRecentTaste: summed
// flat, the same months are misread. Sofia's dresses carry the largest attractors, so
// her half of a month outweighs Kate's; and Mira's three visits outweigh Lena's four, so
// a shopper who has moved on still reads as who they were.
//
// Flat sums used to have a third failure: Lena then Kate read as Mira, an average of two
// tastes that was neither. That came from listings outweighing choices, and went once
// listings stopped counting (see choices); across all twelve drifts, a flat sum now
// reads one of the two real tastes, just too often the old one — four times in twelve.
func TestUndecayedMisreadsDrift(t *testing.T) {
	t.Parallel()

	for name, tt := range map[string]struct {
		old, recent, flat persona.Persona
	}{
		"louder old taste wins": {old: persona.Sofia, recent: persona.Kate, flat: persona.Sofia},
		"old taste lingers":     {old: persona.Mira, recent: persona.Lena, flat: persona.Mira},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			p := newAffinityStill(nil)
			addHistory(p, drifted(tt.old, tt.recent))

			assert.Equal(t, tt.flat.Name, nearestPersona(p.Distill(flat), persona.All()).Name)
		})
	}
}

// TestDecayIsRepeatable keeps decay a pure read: the same now gives a bit-identical
// state, and moving now forward only ever fades the still, never rewrites it.
func TestDecayIsRepeatable(t *testing.T) {
	t.Parallel()

	visit := shopper.Journey(persona.Kate, wire.WithTimestamp(wire.BaseTime))
	end := time.UnixMicro(visit[len(visit)-1].TimestampMicros).UTC() // every event is in the past

	p := newAffinityStill(nil)
	addSession(p, visit)

	now := p.Distill(decayed(end, week))
	for range 4 {
		assert.Equal(t, now, p.Distill(decayed(end, week)))
	}

	later := p.Distill(decayed(end.Add(week), week))
	for dimension, v := range now {
		assert.InDelta(t, float64(v/2), float64(later[dimension]), 1e-6, dimension)
	}

	assert.Equal(t, now, p.Distill(decayed(end, week)), "distilling later modified the still")
}

// TestFeedWithoutTimeDecaysAway pins what Feed documents: an event with no recording time
// is infinitely old to a time-aware recipe, and contributes nothing once decayed.
func TestFeedWithoutTimeDecaysAway(t *testing.T) {
	t.Parallel()

	p := newAffinityStill(nil)
	p.Feed(envelope{}, kateJourney()...)

	require.NotEmpty(t, p.Distill(flat))
	for dimension, v := range p.Distill(decayed(wire.BaseTime, week)) {
		assert.Zero(t, v, dimension)
	}
}

// --- taste phases -----------------------------------------------------------------
//
// A phase is a window of a history in which the shopper's taste holds steady. The
// session infuser is what makes it identifiable: it lets the recipe see where one
// visit ends and the next begins, and a visit is the unit taste is judged in — a single
// event is too small, a whole month too large.

// phasePersonas names the persona each phase is nearest to, which is how a test reads a
// list of phases back.
func phasePersonas(ps []affinity.Phase) []string {
	out := make([]string, 0, len(ps))
	for _, p := range ps {
		out = append(out, nearestPersona(p.Taste, persona.All()).Name)
	}

	return out
}

// TestStampReadsEnvelopeSession pins where the session comes from: the envelope, not
// the event. Every event of a visit carries that visit's session id, and an
// event added with a zero envelope carries none.
func TestStampReadsEnvelopeSession(t *testing.T) {
	t.Parallel()

	history := shopper.History(shopper.Month(persona.Kate))

	p := newAffinityStill(nil)
	addHistory(p, history)

	want := make([]string, 0, len(history))
	for _, session := range history {
		want = append(want, session.ID())
	}

	got := p.Distill(func(events []testInfused) []string {
		var out []string
		for _, v := range visits.Of(events) {
			out = append(out, v.Session)
		}

		return out
	})
	assert.Equal(t, want, got)

	bare := newAffinityStill(nil)
	bare.Feed(envelope{}, kateJourney()...)
	assert.Equal(t, []string{""}, bare.Distill(func(events []testInfused) []string {
		return []string{visits.Of(events)[0].Session}
	}))
}

// consented keeps only events from batches whose shopper consented to personalization.
// It is a cut by where an event came from rather than by what it is, which only the
// envelope can answer.
func consented(env envelope, _ testEvent) bool {
	return env.Personalization
}

// TestCutSeesEnvelope is why the cut gets the envelope too. Sofia shops twice
// without consent in the middle of Kate's month; a personalization still must never
// see those visits, so the month reads as Kate's in one unbroken phase — where, kept,
// the same visits would split it.
func TestCutSeesEnvelope(t *testing.T) {
	t.Parallel()

	visitsOf := shopper.Month(persona.Kate)
	visitsOf[2].Persona = persona.Sofia
	visitsOf[3].Persona = persona.Sofia

	history := shopper.History(visitsOf)
	for _, i := range []int{2, 3} {
		history[i] = history[i].WithConsent(wire.ConsentDenied())
	}

	all, consentCut := newAffinityStill(nil), newAffinityStill(consented)
	addHistory(all, history)
	addHistory(consentCut, history)

	kept := 0
	for i, session := range history {
		if i != 2 && i != 3 {
			kept += len(choicesIn(session.Events()))
		}
	}

	assert.Equal(t, kept, consentCut.Len())
	assert.Equal(t, []string{"Kate", "Sofia", "Kate"}, phasePersonas(all.Distill(affinity.Phases(affinity.DefaultPhaseThreshold))))
	assert.Equal(t, []string{"Kate"}, phasePersonas(consentCut.Distill(affinity.Phases(affinity.DefaultPhaseThreshold))))
}

func clone(m personalization.Map) personalization.Map {
	out := make(personalization.Map, len(m))
	for dimension, v := range m {
		out[dimension] = v
	}

	return out
}
