package corpus_test

import (
	"math"
	"slices"
	"testing"
	"time"

	"github.com/foomo/schaumamoi/pkg/affinity"
	"github.com/foomo/schaumamoi/pkg/corpus"
	"github.com/foomo/schaumamoi/pkg/mock"
	"github.com/foomo/schaumamoi/pkg/personalization"
	"github.com/foomo/sesamy-go/pkg/encoding/mpv2"
	"github.com/foomo/sesamy-go/pkg/sesamy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The corpus under test is the affinity corpus the shop ranks with (pkg/affinity). The
// short names below are the tests' own for its parts, so the tests read as they always
// did: what is admitted, how it is decorated, and what is derived.
type (
	envelope   = affinity.Envelope
	decoration = affinity.Decoration

	testEvent     = sesamy.Event[any]
	testDecorated = affinity.Decorated
	testDecorator = affinity.Decorator
	testFilter    = affinity.Filter
	testConfig    = corpus.Config[envelope, any, decoration]
	testCorpus    = affinity.Corpus
)

var (
	stampTime          = affinity.StampTime
	stampSession       = affinity.StampSession
	addAttractors      = affinity.AddAttractors
	weighByIntent      = affinity.WeighByIntent
	affinityDecorators = affinity.Decorators
	intentWeight       = affinity.Weights
	choices            = affinity.Choices
	newAffinityCorpus  = affinity.New
	flat               = affinity.Flat
	decayed            = affinity.Decayed
)

// itemsParams is the one thing every ecommerce event has in common. Decoding into
// mock.Item keeps the attractor and, in memory, the category — see TestItemDecode for why
// the GA4 fields come back zeroed off the wire.
type itemsParams struct {
	Items []mock.Item `json:"items"`
}

func only(names ...sesamy.EventName) testFilter {
	return func(_ envelope, e testEvent) bool {
		for _, name := range names {
			if e.Name == name {
				return true
			}
		}

		return false
	}
}

// kateJourney is the stream the corpus will actually see: heterogeneous GA4 events
// type-erased to sesamy.Event[any], as sesamy-go hands them downstream.
func kateJourney() []testEvent {
	return mock.Journey(mock.PersonaBusinessKate).Events
}

// nearestIn is the product of one category that ranks first for a taste: how that
// category's listing would open for it. It ranks with the shop's own scorer, mock.Score —
// the taste weighed by the category's weights — through Persona.TopMatch. With listings
// filtered out and choices weighed as the shop weighs them, a flat affinity is the shop's
// profile, so this is exactly the listing the shop would show. Cosine stays for comparing a taste with a persona, where the
// magnitude must not count; for ranking it disagrees with the shop on near ties — the
// sequin flares and the velvet palazzo, for Sofia.
func nearestIn(aff personalization.Map, category string) mock.Item {
	return mock.Persona{Affinity: aff}.TopMatch(mock.Filter{Category: category}.Apply(mock.Catalog()))
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

// assertPerCategory reads a derived taste against the persona that produced the events,
// one category at a time, in every category the shopper chose from: there, the product
// the taste ranks first must be one the persona likes — within mock.GoodEnough of its
// favourite in that category. Not the favourite itself: the shopper does not always open
// its favourite, and the corpus can only know what it did open. Categories the shopper
// was only offered are not asserted — the corpus has nothing of theirs to go on.
func assertPerCategory(t *testing.T, aff personalization.Map, want mock.Persona, events []testEvent) {
	t.Helper()

	touched := touchedCategories(t, events)
	require.NotEmpty(t, touched)

	for _, category := range touched {
		items := mock.Filter{Category: category}.Apply(mock.Catalog())
		got := nearestIn(aff, category)

		assert.True(t, want.Likes(got, items), "%s for %s: %s ranks first, which %s does not like", category, want.Name, got.ItemName, want.Name)
	}
}

// addPayload is how a routed batch reaches the corpus: its events, with the envelope
// fields that differ between batches resolved into an envelope.
func addPayload(p *testCorpus, payload mpv2.Payload[any]) {
	p.Add(envelopeOf(payload), payload.Events...)
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

// funnel counts admitted events by name — a domain state that says nothing about taste.
func funnel(events []testDecorated) map[sesamy.EventName]int {
	out := map[sesamy.EventName]int{}
	for _, e := range events {
		out[e.Event.Name]++
	}

	return out
}

// cosine is the similarity of two affinity maps, independent of their magnitude: a
// shopper seen for six events and one seen for sixty compare to a persona on equal terms.
func cosine(a, b personalization.Map) personalization.Value {
	norm := float64(a.Score(a)) * float64(b.Score(b))
	if norm == 0 {
		return 0
	}

	return personalization.Value(float64(a.Score(b)) / math.Sqrt(norm))
}

// nearestPersona is the persona whose taste the affinity is closest to, ties broken by
// name so the result is stable.
func nearestPersona(aff personalization.Map, personas []mock.Persona) mock.Persona {
	var (
		best  mock.Persona
		score personalization.Value
	)

	for i, p := range personas {
		v := cosine(aff, p.Affinity)
		if i == 0 || v > score || (v == score && p.Name < best.Name) {
			best, score = p, v
		}
	}

	return best
}

// nearestItem is the item whose attractor the affinity is closest to, ties broken by
// item id, as in Persona.TopMatch.
func nearestItem(aff personalization.Map, items []mock.Item) mock.Item {
	var (
		best  mock.Item
		score personalization.Value
	)

	for i, item := range items {
		v := cosine(aff, item.Attractor)
		if i == 0 || v > score || (v == score && item.ItemID < best.ItemID) {
			best, score = item, v
		}
	}

	return best
}

// purchased is the item the journey put in the cart.
func purchased(t *testing.T, events []testEvent) mock.Item {
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

	return mock.Item{}
}

// TestAffinityRecoversPersona is the use case: the affinity folded out of a shopper's
// stream must sit closer to the taste that generated the stream than to any other.
func TestAffinityRecoversPersona(t *testing.T) {
	t.Parallel()

	for _, persona := range mock.Personas() {
		t.Run(persona.Name, func(t *testing.T) {
			t.Parallel()

			p := newAffinityCorpus(nil)
			p.Add(envelope{}, mock.Journey(persona).Events...)

			aff := p.Derive(flat)
			assert.Equal(t, persona.Name, nearestPersona(aff, mock.Personas()).Name)

			own := cosine(aff, persona.Affinity)
			for _, other := range mock.Personas() {
				if other.Name == persona.Name {
					continue
				}

				assert.Greater(t, own, cosine(aff, other.Affinity), "closer to %s than to %s", other.Name, persona.Name)
			}
		})
	}
}

// TestAffinityNearestItemIsPurchase measures the other distance, per category: in every
// category the shopper chose from — dresses, trousers, and one in home decoration — the
// product the derived taste ranks first is one the persona likes.
func TestAffinityNearestItemIsPurchase(t *testing.T) {
	t.Parallel()

	for _, persona := range mock.Personas() {
		t.Run(persona.Name, func(t *testing.T) {
			t.Parallel()

			events := mock.Journey(persona).Events

			p := newAffinityCorpus(nil)
			p.Add(envelope{}, events...)

			got := p.Derive(flat)
			assertPerCategory(t, got, persona, events)

		})
	}
}

// TestAddAppliesFilter pins the affinity corpus's positive filter: of a whole journey it
// admits only the choices — no listing gets in — and those alone say who the shopper is.
// A filter handed to newAffinityCorpus narrows that further, never widens it.
func TestAddAppliesFilter(t *testing.T) {
	t.Parallel()

	p := newAffinityCorpus(nil)
	p.Add(envelope{}, kateJourney()...)

	assert.Equal(t, 10, p.Len())
	assert.Equal(t, map[sesamy.EventName]int{
		sesamy.EventNameSelectItem: 7,
		sesamy.EventNameViewItem:   2,
		sesamy.EventNameAddToCart:  1,
	}, p.Derive(funnel))
	assert.Equal(t, mock.PersonaBusinessKate.Name, nearestPersona(p.Derive(flat), mock.Personas()).Name)

	carts := newAffinityCorpus(only(sesamy.EventNameAddToCart, sesamy.EventNameViewItemList))
	carts.Add(envelope{}, kateJourney()...)
	assert.Equal(t, map[sesamy.EventName]int{sesamy.EventNameAddToCart: 1}, carts.Derive(funnel), "an extra filter cannot let listings back in")
}

// choicesIn is what the affinity corpus admits of events: the choices, in order.
func choicesIn(events []testEvent) []testEvent {
	var out []testEvent

	for _, e := range events {
		if choices(envelope{}, e) {
			out = append(out, e)
		}
	}

	return out
}

// TestAddWithoutFilterAdmitsEverything pins the zero-value config: a corpus that
// only collects must not have to write an admit-everything filter, and must not panic.
func TestAddWithoutFilterAdmitsEverything(t *testing.T) {
	t.Parallel()

	events := kateJourney()

	p := corpus.New(testConfig{})
	require.NotPanics(t, func() { p.Add(envelope{}, events...) })

	assert.Equal(t, len(events), p.Len())
	assert.Empty(t, p.Derive(flat), "no decorators, no decoration")
}

// TestDecoratorsFoldInOrder uses the fact that the affinity decorators do not commute:
// weighting before anything has been collected scales an empty map.
func TestDecoratorsFoldInOrder(t *testing.T) {
	t.Parallel()

	pick := mock.PersonaBusinessKate.TopMatch(mock.Catalog())
	cart := []testEvent{mock.AddToCart(pick).AnyEvent()}

	derive := func(decorators ...testDecorator) personalization.Map {
		p := corpus.New(testConfig{Decorators: decorators})
		p.Add(envelope{}, cart...)

		return p.Derive(flat)
	}

	inOrder, reversed := derive(addAttractors, weighByIntent), derive(weighByIntent, addAttractors)

	require.NotEmpty(t, pick.Attractor)
	for dimension, v := range pick.Attractor {
		assert.InDelta(t, float64(v*intentWeight[sesamy.EventNameAddToCart]), float64(inOrder[dimension]), 1e-6, "addAttractors, weighByIntent: %s", dimension)
		assert.InDelta(t, float64(v), float64(reversed[dimension]), 1e-6, "weighByIntent, addAttractors: %s", dimension)
	}
}

// TestDeriveIsRepeatable is the regression. Decoration happens once, on arrival, so
// deriving state is a pure read: the affinity must not drift with the number of derives,
// and deriving must not write through into the decorations it was handed.
func TestDeriveIsRepeatable(t *testing.T) {
	t.Parallel()

	p := newAffinityCorpus(nil)
	p.Add(envelope{}, kateJourney()...)

	decorations := func(events []testDecorated) []personalization.Map {
		out := make([]personalization.Map, 0, len(events))
		for _, e := range events {
			out = append(out, clone(e.Decoration.Affinity))
		}

		return out
	}

	before := p.Derive(decorations)
	first := p.Derive(flat)

	require.NotEmpty(t, first)
	for i := range 4 {
		assert.Equal(t, first, p.Derive(flat), "derive %d", i+2)
	}
	assert.Equal(t, before, p.Derive(decorations), "deriving modified the corpus")
}

// TestAddAfterDeriveDoesNotRedecorate is the other half of the regression: a later
// batch must not drag the decorators back over events that were already folded. Intent scales,
// so an event decorated twice would carry its weight squared.
func TestAddAfterDeriveDoesNotRedecorate(t *testing.T) {
	t.Parallel()

	first, second := kateJourney(), mock.BrowseJourney(mock.PersonaCasualLena).Events

	p := newAffinityCorpus(nil)
	p.Add(envelope{}, first...)
	require.NotEmpty(t, p.Derive(flat))
	p.Add(envelope{}, second...)

	got := p.Derive(func(events []testDecorated) []personalization.Map {
		out := make([]personalization.Map, 0, len(events))
		for _, e := range events {
			out = append(out, e.Decoration.Affinity)
		}

		return out
	})

	// Stated per event against a corpus that saw only that event, so a doubled first
	// batch cannot hide behind anything in the second.
	all := choicesIn(append(append([]testEvent{}, first...), second...))
	require.Len(t, got, len(all))
	for i, e := range all {
		fresh := newAffinityCorpus(nil)
		fresh.Add(envelope{}, e)

		assert.Equal(t, fresh.Derive(flat), got[i], "event %d (%s)", i, e.Name)
	}
}

// TestOneCorpusManyQuestions is what moving DomainState off the corpus buys: derivers
// of unrelated types over a single ingest.
func TestOneCorpusManyQuestions(t *testing.T) {
	t.Parallel()

	p := newAffinityCorpus(nil)
	p.Add(envelope{}, kateJourney()...)

	assert.Equal(t, map[sesamy.EventName]int{
		sesamy.EventNameSelectItem: 7,
		sesamy.EventNameViewItem:   2,
		sesamy.EventNameAddToCart:  1,
	}, p.Derive(funnel))

	assert.Equal(t, mock.PersonaBusinessKate.Name, p.Derive(func(events []testDecorated) mock.Persona {
		return nearestPersona(flat(events), mock.Personas())
	}).Name)
}

// TestDeriveIsDeterministic pins the float summation: the same corpus must give a
// bit-identical affinity every time, and so must a second corpus fed the same stream,
// for the reason Map.Score sorts its dimensions.
func TestDeriveIsDeterministic(t *testing.T) {
	t.Parallel()

	p := newAffinityCorpus(nil)
	p.Add(envelope{}, kateJourney()...)

	want := p.Derive(flat)
	for range 8 {
		assert.Equal(t, want, p.Derive(flat))

		again := newAffinityCorpus(nil)
		again.Add(envelope{}, kateJourney()...)
		assert.Equal(t, want, again.Derive(flat))
	}
}

const week = 7 * 24 * time.Hour

// TestStampTimeReadsEnvelope pins where recency comes from: every event of a batch
// carries the time of the envelope it was added with, and a zero envelope leaves the
// zero time.
func TestStampTimeReadsEnvelope(t *testing.T) {
	t.Parallel()

	earlier, later := mock.BaseTime.Add(-week), mock.BaseTime

	p := newAffinityCorpus(nil)
	addPayload(p, mock.Journey(mock.PersonaFestiveSofia, mock.WithTimestamp(earlier)))
	addPayload(p, mock.BrowseJourney(mock.PersonaBusinessKate, mock.WithTimestamp(later)))
	p.Add(envelope{}, mock.AddToCart(mock.PersonaBusinessKate.TopMatch(mock.Catalog())).AnyEvent())

	stamps := p.Derive(func(events []testDecorated) []time.Time {
		out := make([]time.Time, 0, len(events))
		for _, e := range events {
			out = append(out, e.Decoration.At)
		}

		return out
	})

	want := append(append(
		slices.Repeat([]time.Time{earlier}, len(choicesIn(mock.Journey(mock.PersonaFestiveSofia).Events))),
		slices.Repeat([]time.Time{later}, len(choicesIn(mock.BrowseJourney(mock.PersonaBusinessKate).Events)))...),
		time.Time{})
	assert.Equal(t, want, stamps)
}

// TestDecayHalvesAtHalfLife pins the curve itself on a single purchase: full weight now,
// half after one half-life, a quarter after two — and no boost for an event from after
// now.
func TestDecayHalvesAtHalfLife(t *testing.T) {
	t.Parallel()

	pick := mock.PersonaBusinessKate.TopMatch(mock.Catalog())
	cart := mock.AddToCart(pick).AnyEvent()

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

			p := newAffinityCorpus(nil)
			p.Add(envelope{At: mock.BaseTime.Add(-tt.age)}, cart)

			full := p.Derive(flat)
			got := p.Derive(decayed(mock.BaseTime, week))

			require.NotEmpty(t, full)
			for dimension, v := range full {
				assert.InDelta(t, float64(v*tt.want), float64(got[dimension]), 1e-6, dimension)
			}
		})
	}
}

// eventsOf is every event of a history, in order.
func eventsOf(history []mpv2.Payload[any]) []testEvent {
	var out []testEvent
	for _, payload := range history {
		out = append(out, payload.Events...)
	}

	return out
}

// addHistory feeds a shopper's month into the corpus, one routed batch per visit.
func addHistory(p *testCorpus, history []mpv2.Payload[any]) {
	for _, payload := range history {
		addPayload(p, payload)
	}
}

// drifted is a month that starts with one taste and settles on another: the first three
// visits, up to two weeks ago and including the first purchase, are old's; the rest,
// ending with today's purchase, are recent's.
func drifted(old, recent mock.Persona) []mpv2.Payload[any] {
	visits := mock.Month(recent)
	for i := range 3 {
		visits[i].Persona = old
	}

	return mock.History(visits)
}

// TestMonthRecoversPersona is TestAffinityRecoversPersona over a month of visits rather
// than a single session, derived with decay: a steady taste must still come out as
// itself, and read per category as in TestAffinityNearestItemIsPurchase.
func TestMonthRecoversPersona(t *testing.T) {
	t.Parallel()

	for _, persona := range mock.Personas() {
		t.Run(persona.Name, func(t *testing.T) {
			t.Parallel()

			history := mock.History(mock.Month(persona))
			p := newAffinityCorpus(nil)
			addHistory(p, history)

			got := p.Derive(decayed(mock.BaseTime, week))
			assert.Equal(t, persona.Name, nearestPersona(got, mock.Personas()).Name)
			assertPerCategory(t, got, persona, eventsOf(history))
		})
	}
}

// TestDecayFollowsRecentTaste is what decay is for. A shopper spends the first half of
// the month on one persona's taste and the second half on another's; with a one-week
// half-life the affinity must follow whichever came last — for every ordered pair, so
// it is recency deciding and not which of the two tastes happens to be louder.
func TestDecayFollowsRecentTaste(t *testing.T) {
	t.Parallel()

	for _, old := range mock.Personas() {
		for _, recent := range mock.Personas() {
			if old.Name == recent.Name {
				continue
			}

			t.Run(old.Name+" then "+recent.Name, func(t *testing.T) {
				t.Parallel()

				history := drifted(old, recent)
				p := newAffinityCorpus(nil)
				addHistory(p, history)

				got := p.Derive(decayed(mock.BaseTime, week))
				assert.Equal(t, recent.Name, nearestPersona(got, mock.Personas()).Name)
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
		old, recent, flat mock.Persona
	}{
		"louder old taste wins": {old: mock.PersonaFestiveSofia, recent: mock.PersonaBusinessKate, flat: mock.PersonaFestiveSofia},
		"old taste lingers":     {old: mock.PersonaModernMira, recent: mock.PersonaCasualLena, flat: mock.PersonaModernMira},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			p := newAffinityCorpus(nil)
			addHistory(p, drifted(tt.old, tt.recent))

			assert.Equal(t, tt.flat.Name, nearestPersona(p.Derive(flat), mock.Personas()).Name)
		})
	}
}

// TestDecayIsRepeatable keeps decay a pure read: the same now gives a bit-identical
// state, and moving now forward only ever fades the corpus, never rewrites it.
func TestDecayIsRepeatable(t *testing.T) {
	t.Parallel()

	p := newAffinityCorpus(nil)
	addPayload(p, mock.Journey(mock.PersonaBusinessKate, mock.WithTimestamp(mock.BaseTime)))

	now := p.Derive(decayed(mock.BaseTime, week))
	for range 4 {
		assert.Equal(t, now, p.Derive(decayed(mock.BaseTime, week)))
	}

	later := p.Derive(decayed(mock.BaseTime.Add(week), week))
	for dimension, v := range now {
		assert.InDelta(t, float64(v/2), float64(later[dimension]), 1e-6, dimension)
	}

	assert.Equal(t, now, p.Derive(decayed(mock.BaseTime, week)), "deriving later modified the corpus")
}

// TestAddWithoutTimeDecaysAway pins what Add documents: an event with no recording time
// is infinitely old to a time-aware deriver, and contributes nothing once decayed.
func TestAddWithoutTimeDecaysAway(t *testing.T) {
	t.Parallel()

	p := newAffinityCorpus(nil)
	p.Add(envelope{}, kateJourney()...)

	require.NotEmpty(t, p.Derive(flat))
	for dimension, v := range p.Derive(decayed(mock.BaseTime, week)) {
		assert.Zero(t, v, dimension)
	}
}

// --- taste phases -----------------------------------------------------------------
//
// A phase is a window of a history in which the shopper's taste holds steady. The
// session decorator is what makes it identifiable: it lets the deriver see where one
// visit ends and the next begins, and a visit is the unit taste is judged in — a single
// event is too small, a whole month too large.

// phaseThreshold is the cosine a visit must reach against the phase so far to belong to
// it. Across every persona pair a visit within a phase scores at least 0.94 and the first
// visit after a drift at most 0.63 — Mira to Lena, who share a minimal, modern taste, is
// the close call — so 0.9 sits in the gap, nearer its top. A visit is only its choices;
// the listings it scrolled past add nothing to it. The within-phase side is the tight
// one: a shopper does not open the same products every visit (Sofia drifts between her
// sequin and her tulle dress), so even one taste's visits differ.
const phaseThreshold personalization.Value = 0.9

// window is a run of consecutive sessions, with the affinity shown across them. A phase
// is a window of steady taste.
type window struct {
	From, To time.Time
	Sessions []string
	Affinity personalization.Map
}

// sessionWindows groups the corpus into one window per session, in order. Events arrive in
// order and a session's events arrive together, so a change of session opens a new window.
func sessionWindows(events []testDecorated) []window {
	var out []window

	for i, e := range events {
		if i == 0 || e.Decoration.Session != events[i-1].Decoration.Session {
			out = append(out, window{From: e.Decoration.At, Sessions: []string{e.Decoration.Session}, Affinity: personalization.Map{}})
		}

		v := &out[len(out)-1]
		v.To = e.Decoration.At
		for dimension, w := range e.Decoration.Affinity {
			v.Affinity[dimension] += w
		}
	}

	return out
}

// phases merges consecutive visits into windows of steady taste: a visit joins the
// current phase if it is at least threshold similar to the phase so far, and opens a new
// one otherwise. A visit that says nothing about taste joins whatever phase it falls in.
//
// Phases are contiguous. A shopper who drifts away and comes back gets three windows,
// not two — the return is news about when the taste held, which a merge would lose.
func phases(threshold personalization.Value) corpus.StateDeriver[any, decoration, []window] {
	return func(events []testDecorated) []window {
		var out []window

		for _, v := range sessionWindows(events) {
			if len(out) > 0 {
				current := &out[len(out)-1]
				if len(v.Affinity) == 0 || cosine(v.Affinity, current.Affinity) >= threshold {
					current.To = v.To
					current.Sessions = append(current.Sessions, v.Sessions...)
					for dimension, w := range v.Affinity {
						current.Affinity[dimension] += w
					}

					continue
				}
			}

			out = append(out, v)
		}

		return out
	}
}

// phasePersonas names the persona each phase is nearest to, which is how a test reads a
// list of phases back.
func phasePersonas(ps []window) []string {
	out := make([]string, 0, len(ps))
	for _, p := range ps {
		out = append(out, nearestPersona(p.Affinity, mock.Personas()).Name)
	}

	return out
}

// TestStampSessionReadsEnvelope pins where the session comes from: the envelope, not
// the event. Every event of a visit carries that visit's session id, and an
// event added with a zero envelope carries none.
func TestStampSessionReadsEnvelope(t *testing.T) {
	t.Parallel()

	history := mock.History(mock.Month(mock.PersonaBusinessKate))

	p := newAffinityCorpus(nil)
	addHistory(p, history)

	want := make([]string, 0, len(history))
	for _, payload := range history {
		want = append(want, payload.SessionID)
	}

	got := p.Derive(func(events []testDecorated) []string {
		var out []string
		for _, v := range sessionWindows(events) {
			out = append(out, v.Sessions...)
		}

		return out
	})
	assert.Equal(t, want, got)

	bare := newAffinityCorpus(nil)
	bare.Add(envelope{}, kateJourney()...)
	assert.Equal(t, []string{""}, bare.Derive(func(events []testDecorated) []string {
		return sessionWindows(events)[0].Sessions
	}))
}

// TestSteadyMonthIsOnePhase is the baseline: a shopper who never drifts has one window,
// spanning the whole month, and it reads as them.
func TestSteadyMonthIsOnePhase(t *testing.T) {
	t.Parallel()

	for _, persona := range mock.Personas() {
		t.Run(persona.Name, func(t *testing.T) {
			t.Parallel()

			p := newAffinityCorpus(nil)
			addHistory(p, mock.History(mock.Month(persona)))

			got := p.Derive(phases(phaseThreshold))
			require.Len(t, got, 1)
			assert.Equal(t, []string{persona.Name}, phasePersonas(got))
			assert.Equal(t, mock.BaseTime.Add(-27*mock.Day), got[0].From)
			assert.Equal(t, mock.BaseTime, got[0].To)
			assert.Len(t, got[0].Sessions, len(mock.Month(persona)))
		})
	}
}

// TestDriftSplitsIntoPhases is what the windows are for. A month that drifts from one
// taste to another must split exactly where it drifted — after the third visit, two
// weeks ago — and each side must read as the persona who was shopping then. Unlike
// decay, which only answers "who now", this recovers who the shopper was and when.
func TestDriftSplitsIntoPhases(t *testing.T) {
	t.Parallel()

	for _, old := range mock.Personas() {
		for _, recent := range mock.Personas() {
			if old.Name == recent.Name {
				continue
			}

			t.Run(old.Name+" then "+recent.Name, func(t *testing.T) {
				t.Parallel()

				p := newAffinityCorpus(nil)
				addHistory(p, drifted(old, recent))

				got := p.Derive(phases(phaseThreshold))
				require.Len(t, got, 2)
				assert.Equal(t, []string{old.Name, recent.Name}, phasePersonas(got))

				assert.Equal(t, mock.BaseTime.Add(-14*mock.Day), got[0].To, "the old taste ends with the first purchase")
				assert.Equal(t, mock.BaseTime.Add(-9*mock.Day), got[1].From, "the new taste starts with the next visit")
				assert.Len(t, got[0].Sessions, 3)
				assert.Len(t, got[1].Sessions, 4)
			})
		}
	}
}

// TestReturningTasteIsANewPhase pins that windows are contiguous: Kate detours into
// Sofia's taste for two visits mid-month and comes back, which is three phases, not a
// Kate phase with a hole in it.
func TestReturningTasteIsANewPhase(t *testing.T) {
	t.Parallel()

	visitsOf := mock.Month(mock.PersonaBusinessKate)
	visitsOf[2].Persona = mock.PersonaFestiveSofia
	visitsOf[3].Persona = mock.PersonaFestiveSofia

	p := newAffinityCorpus(nil)
	addHistory(p, mock.History(visitsOf))

	got := p.Derive(phases(phaseThreshold))
	assert.Equal(t, []string{"Kate", "Sofia", "Kate"}, phasePersonas(got))
}

// TestPhasesNeedSessions shows the decorator doing the work: the same drifted month
// through decorators without it is one undivided visit, so there is nothing to split.
func TestPhasesNeedSessions(t *testing.T) {
	t.Parallel()

	p := corpus.New(testConfig{Decorators: []testDecorator{stampTime, addAttractors, weighByIntent}})
	addHistory(p, drifted(mock.PersonaFestiveSofia, mock.PersonaBusinessKate))

	assert.Len(t, p.Derive(phases(phaseThreshold)), 1)
}

// consented keeps only events from batches whose shopper consented to personalization.
// It is a filter by where an event came from rather than by what it is, which only the
// envelope can answer.
func consented(env envelope, _ testEvent) bool {
	return env.Personalization
}

// TestFilterSeesEnvelope is why the filter gets the envelope too. Sofia shops twice
// without consent in the middle of Kate's month; a personalization corpus must never
// see those visits, so the month reads as Kate's in one unbroken phase — where, admitted,
// the same visits would split it.
func TestFilterSeesEnvelope(t *testing.T) {
	t.Parallel()

	visitsOf := mock.Month(mock.PersonaBusinessKate)
	visitsOf[2].Persona = mock.PersonaFestiveSofia
	visitsOf[3].Persona = mock.PersonaFestiveSofia

	history := mock.History(visitsOf)
	for _, i := range []int{2, 3} {
		history[i].Consent = mock.ConsentDenied()
	}

	all, filtered := newAffinityCorpus(nil), newAffinityCorpus(consented)
	addHistory(all, history)
	addHistory(filtered, history)

	admitted := 0
	for i, payload := range history {
		if i != 2 && i != 3 {
			admitted += len(choicesIn(payload.Events))
		}
	}

	assert.Equal(t, admitted, filtered.Len())
	assert.Equal(t, []string{"Kate", "Sofia", "Kate"}, phasePersonas(all.Derive(phases(phaseThreshold))))
	assert.Equal(t, []string{"Kate"}, phasePersonas(filtered.Derive(phases(phaseThreshold))))
}

func clone(m personalization.Map) personalization.Map {
	out := make(personalization.Map, len(m))
	for dimension, v := range m {
		out[dimension] = v
	}

	return out
}
