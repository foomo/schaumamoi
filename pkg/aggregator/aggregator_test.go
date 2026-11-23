package aggregator_test

import (
	"math"
	"testing"
	"time"

	"github.com/foomo/schaumamoi/pkg/aggregator"
	"github.com/foomo/schaumamoi/pkg/mock"
	"github.com/foomo/schaumamoi/pkg/personalization"
	"github.com/foomo/sesamy-go/pkg/encoding/mpv2"
	"github.com/foomo/sesamy-go/pkg/sesamy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// batch is the envelope the test pipelines ingest with: the fields of an mpv2 envelope
// that differ between the batches of one shopper.
type batch struct {
	At      time.Time
	Session string
	// Personalization is whether the shopper consented to personalization storage.
	Personalization bool
}

// decoration is what the test pipelines fold out of each event: the event's contribution
// to the shopper's affinity, i.e. the attractors of the items it touched, weighted by how
// strong a signal of taste the event is — and, copied off the envelope, when and in which
// session it happened, which is what lets a deriver decay by time and cut a history into
// windows. The envelope is gone once an event is admitted; this is what remains of it.
type decoration struct {
	At       time.Time
	Session  string
	Affinity personalization.Map
}

type (
	testEvent      = sesamy.Event[any]
	testDecorated  = aggregator.DecoratedEvent[any, decoration]
	testDecorator  = aggregator.EventDecorator[batch, any, decoration]
	testFilter     = aggregator.EventFilter[batch, any]
	testPipelineFn = aggregator.PipelineConfig[batch, any, decoration]
	testPipeline   = aggregator.Pipeline[batch, any, decoration]
)

// itemsParams is the one thing every ecommerce event has in common. Decoding into
// mock.Item keeps the attractor, which is all the affinity needs — see TestItemDecode
// for why the GA4 fields come back zeroed.
type itemsParams struct {
	Items []mock.Item `json:"items"`
}

// intentWeight is how much an event says about taste. A listing shows everything in a
// section, so each listed dress counts for little; opening, viewing and above all buying
// a dress is a deliberate choice.
var intentWeight = map[sesamy.EventName]personalization.Value{
	sesamy.EventNameViewItemList: 0.1,
	sesamy.EventNameSelectItem:   1,
	sesamy.EventNameViewItem:     1,
	sesamy.EventNameAddToCart:    3,
}

// recorded and session copy the envelope's time and session onto the event. They read
// the envelope rather than the event, because the event itself carries neither; and
// they have to, because the corpus keeps the decoration and not the envelope.
func recorded(b batch, _ testEvent, d decoration) decoration {
	d.At = b.At

	return d
}

func session(b batch, _ testEvent, d decoration) decoration {
	d.Session = b.Session

	return d
}

// touched adds the attractor of every item the event carries to the decoration. An event
// without decodable items leaves the decoration as it was.
func touched(_ batch, e testEvent, d decoration) decoration {
	var p itemsParams
	if err := e.DecodeParams(&p); err != nil {
		return d
	}

	out := clone(d.Affinity)
	for _, item := range p.Items {
		for dimension, v := range item.Attractor {
			out[dimension] += v
		}
	}

	d.Affinity = out

	return d
}

// intent scales the decoration by the weight of the event. Unknown events weigh nothing.
func intent(_ batch, e testEvent, d decoration) decoration {
	w := intentWeight[e.Name]

	out := make(personalization.Map, len(d.Affinity))
	for dimension, v := range d.Affinity {
		out[dimension] = v * w
	}

	d.Affinity = out

	return d
}

func clone(m personalization.Map) personalization.Map {
	out := make(personalization.Map, len(m))
	for dimension, v := range m {
		out[dimension] = v
	}

	return out
}

func newAffinityPipeline(filter testFilter) *testPipeline {
	return aggregator.NewPipeline(testPipelineFn{
		PreFilter:      filter,
		DecoratorChain: []testDecorator{recorded, session, touched, intent},
	})
}

func only(names ...sesamy.EventName) testFilter {
	return func(_ batch, e testEvent) bool {
		for _, name := range names {
			if e.Name == name {
				return true
			}
		}

		return false
	}
}

// kateJourney is the corpus the pipeline will actually see: six heterogeneous GA4 events
// type-erased to sesamy.Event[any], as sesamy-go hands them downstream.
func kateJourney() []testEvent {
	return mock.Journey(mock.PersonaBusinessKate).Events
}

// affinity is the domain state: the shopper's affinity to the attractors of the items
// they interacted with. It sums into a fresh map in slice order — events arrive in order
// and Add appends, so the float sums are stable, for the reason Map.Score documents — and
// never writes into a decoration, which the pipeline hands out by reference.
func affinity(events []testDecorated) personalization.Map {
	out := personalization.Map{}
	for _, e := range events {
		for dimension, v := range e.Decoration.Affinity {
			out[dimension] += v
		}
	}

	return out
}

// decayed is affinity with recency: an event's contribution halves every halfLife it is
// older than now. Now is handed in rather than read off the clock, so deriving stays a
// pure read and a test can move time by hand. An event from after now counts in full
// rather than growing.
func decayed(now time.Time, halfLife time.Duration) aggregator.StateDeriver[any, decoration, personalization.Map] {
	return func(events []testDecorated) personalization.Map {
		out := personalization.Map{}
		for _, e := range events {
			w := personalization.Value(math.Exp2(-float64(max(now.Sub(e.Decoration.At), 0)) / float64(halfLife)))
			for dimension, v := range e.Decoration.Affinity {
				out[dimension] += v * w
			}
		}

		return out
	}
}

// addPayload is how a routed batch reaches the pipeline: its events, with the envelope
// fields that differ between batches resolved into a batch.
func addPayload(p *testPipeline, payload mpv2.Payload[any]) {
	p.Add(batch{
		At:              time.UnixMicro(payload.TimestampMicros).UTC(),
		Session:         payload.SessionID,
		Personalization: granted(payload.Consent),
	}, payload.Events...)
}

// granted reads personalization consent off an envelope. No consent block means no
// decision was made, which is not consent.
func granted(c *mpv2.ConsentData) bool {
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

			p := newAffinityPipeline(nil)
			p.Add(batch{}, mock.Journey(persona).Events...)

			aff := p.DeriveState(affinity)
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

// TestAffinityNearestItemIsPurchase measures the other distance: of the whole catalog,
// the dress closest to the derived affinity is the one the shopper bought.
func TestAffinityNearestItemIsPurchase(t *testing.T) {
	t.Parallel()

	for _, persona := range mock.Personas() {
		t.Run(persona.Name, func(t *testing.T) {
			t.Parallel()

			events := mock.Journey(persona).Events

			p := newAffinityPipeline(nil)
			p.Add(batch{}, events...)

			want := purchased(t, events)
			got := nearestItem(p.DeriveState(affinity), mock.Catalog())
			assert.Equal(t, want.ItemID, got.ItemID, "want %q, got %q", want.Description, got.Description)
		})
	}
}

// TestAddAppliesPreFilter narrows the corpus to deliberate interactions. The listings
// are gone, and the choices alone still say who the shopper is.
func TestAddAppliesPreFilter(t *testing.T) {
	t.Parallel()

	p := newAffinityPipeline(only(sesamy.EventNameSelectItem, sesamy.EventNameViewItem, sesamy.EventNameAddToCart))
	p.Add(batch{}, kateJourney()...)

	assert.Equal(t, 3, p.Len())
	assert.Equal(t, map[sesamy.EventName]int{
		sesamy.EventNameSelectItem: 1,
		sesamy.EventNameViewItem:   1,
		sesamy.EventNameAddToCart:  1,
	}, p.DeriveState(funnel))
	assert.Equal(t, mock.PersonaBusinessKate.Name, nearestPersona(p.DeriveState(affinity), mock.Personas()).Name)
}

// TestAddWithoutPreFilterAdmitsEverything pins the zero-value config: a pipeline that
// only collects must not have to write a keep-everything filter, and must not panic.
func TestAddWithoutPreFilterAdmitsEverything(t *testing.T) {
	t.Parallel()

	events := kateJourney()

	p := aggregator.NewPipeline(testPipelineFn{})
	require.NotPanics(t, func() { p.Add(batch{}, events...) })

	assert.Equal(t, len(events), p.Len())
	assert.Empty(t, p.DeriveState(affinity), "an empty chain decorates nothing")
}

// TestDecoratorChainFoldsInOrder uses the fact that the affinity chain does not commute:
// weighting before anything has been collected scales an empty map.
func TestDecoratorChainFoldsInOrder(t *testing.T) {
	t.Parallel()

	pick := mock.PersonaBusinessKate.TopMatch(mock.Catalog())
	cart := []testEvent{mock.AddToCart(pick).AnyEvent()}

	derive := func(chain ...testDecorator) personalization.Map {
		p := aggregator.NewPipeline(testPipelineFn{DecoratorChain: chain})
		p.Add(batch{}, cart...)

		return p.DeriveState(affinity)
	}

	inOrder, reversed := derive(touched, intent), derive(intent, touched)

	require.NotEmpty(t, pick.Attractor)
	for dimension, v := range pick.Attractor {
		assert.InDelta(t, float64(v*intentWeight[sesamy.EventNameAddToCart]), float64(inOrder[dimension]), 1e-6, "touched, intent: %s", dimension)
		assert.InDelta(t, float64(v), float64(reversed[dimension]), 1e-6, "intent, touched: %s", dimension)
	}
}

// TestDeriveStateIsRepeatable is the regression. Decoration happens once, on arrival, so
// deriving state is a pure read: the affinity must not drift with the number of derives,
// and deriving must not write through into the decorations it was handed.
func TestDeriveStateIsRepeatable(t *testing.T) {
	t.Parallel()

	p := newAffinityPipeline(nil)
	p.Add(batch{}, kateJourney()...)

	decorations := func(events []testDecorated) []personalization.Map {
		out := make([]personalization.Map, 0, len(events))
		for _, e := range events {
			out = append(out, clone(e.Decoration.Affinity))
		}

		return out
	}

	before := p.DeriveState(decorations)
	first := p.DeriveState(affinity)

	require.NotEmpty(t, first)
	for i := range 4 {
		assert.Equal(t, first, p.DeriveState(affinity), "derive %d", i+2)
	}
	assert.Equal(t, before, p.DeriveState(decorations), "deriving modified the corpus")
}

// TestAddAfterDeriveStateDoesNotRedecorate is the other half of the regression: a later
// batch must not drag the chain back over events that were already folded. Intent scales,
// so an event decorated twice would carry its weight squared.
func TestAddAfterDeriveStateDoesNotRedecorate(t *testing.T) {
	t.Parallel()

	first, second := kateJourney(), mock.BrowseJourney(mock.PersonaCasualLena).Events

	p := newAffinityPipeline(nil)
	p.Add(batch{}, first...)
	require.NotEmpty(t, p.DeriveState(affinity))
	p.Add(batch{}, second...)

	got := p.DeriveState(func(events []testDecorated) []personalization.Map {
		out := make([]personalization.Map, 0, len(events))
		for _, e := range events {
			out = append(out, e.Decoration.Affinity)
		}

		return out
	})

	// Stated per event against a pipeline that saw only that event, so a doubled first
	// batch cannot hide behind anything in the second.
	all := append(append([]testEvent{}, first...), second...)
	require.Len(t, got, len(all))
	for i, e := range all {
		fresh := newAffinityPipeline(nil)
		fresh.Add(batch{}, e)

		assert.Equal(t, fresh.DeriveState(affinity), got[i], "event %d (%s)", i, e.Name)
	}
}

// TestOneCorpusManyQuestions is what moving DomainState off the pipeline buys: derivers
// of unrelated types over a single ingest.
func TestOneCorpusManyQuestions(t *testing.T) {
	t.Parallel()

	p := newAffinityPipeline(nil)
	p.Add(batch{}, kateJourney()...)

	assert.Equal(t, map[sesamy.EventName]int{
		sesamy.EventNameViewItemList: 3,
		sesamy.EventNameSelectItem:   1,
		sesamy.EventNameViewItem:     1,
		sesamy.EventNameAddToCart:    1,
	}, p.DeriveState(funnel))

	assert.Equal(t, mock.PersonaBusinessKate.Name, p.DeriveState(func(events []testDecorated) mock.Persona {
		return nearestPersona(affinity(events), mock.Personas())
	}).Name)
}

// TestDeriveStateIsDeterministic pins the float summation: the same corpus must give a
// bit-identical affinity every time, and so must a second pipeline fed the same stream,
// for the reason Map.Score sorts its dimensions.
func TestDeriveStateIsDeterministic(t *testing.T) {
	t.Parallel()

	p := newAffinityPipeline(nil)
	p.Add(batch{}, kateJourney()...)

	want := p.DeriveState(affinity)
	for range 8 {
		assert.Equal(t, want, p.DeriveState(affinity))

		again := newAffinityPipeline(nil)
		again.Add(batch{}, kateJourney()...)
		assert.Equal(t, want, again.DeriveState(affinity))
	}
}

const week = 7 * 24 * time.Hour

// TestRecordedStampsBatchTime pins where recency comes from: every event of a batch
// carries the time of the envelope it was added with, and a zero envelope leaves the
// zero time.
func TestRecordedStampsBatchTime(t *testing.T) {
	t.Parallel()

	earlier, later := mock.BaseTime.Add(-week), mock.BaseTime

	p := newAffinityPipeline(nil)
	addPayload(p, mock.Journey(mock.PersonaFestiveSofia, mock.WithTimestamp(earlier)))
	addPayload(p, mock.BrowseJourney(mock.PersonaBusinessKate, mock.WithTimestamp(later)))
	p.Add(batch{}, mock.AddToCart(mock.PersonaBusinessKate.TopMatch(mock.Catalog())).AnyEvent())

	stamps := p.DeriveState(func(events []testDecorated) []time.Time {
		out := make([]time.Time, 0, len(events))
		for _, e := range events {
			out = append(out, e.Decoration.At)
		}

		return out
	})

	want := []time.Time{earlier, earlier, earlier, earlier, earlier, earlier, later, later, later, {}}
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

			p := newAffinityPipeline(nil)
			p.Add(batch{At: mock.BaseTime.Add(-tt.age)}, cart)

			full := p.DeriveState(affinity)
			got := p.DeriveState(decayed(mock.BaseTime, week))

			require.NotEmpty(t, full)
			for dimension, v := range full {
				assert.InDelta(t, float64(v*tt.want), float64(got[dimension]), 1e-6, dimension)
			}
		})
	}
}

// addHistory feeds a shopper's month into the pipeline, one routed batch per visit.
func addHistory(p *testPipeline, history []mpv2.Payload[any]) {
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
// itself, and its nearest dress as the one it keeps buying.
func TestMonthRecoversPersona(t *testing.T) {
	t.Parallel()

	for _, persona := range mock.Personas() {
		t.Run(persona.Name, func(t *testing.T) {
			t.Parallel()

			p := newAffinityPipeline(nil)
			addHistory(p, mock.History(mock.Month(persona)))

			aff := p.DeriveState(decayed(mock.BaseTime, week))
			assert.Equal(t, persona.Name, nearestPersona(aff, mock.Personas()).Name)
			assert.Equal(t, persona.TopMatch(mock.Catalog()).ItemID, nearestItem(aff, mock.Catalog()).ItemID)
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

				p := newAffinityPipeline(nil)
				addHistory(p, drifted(old, recent))

				aff := p.DeriveState(decayed(mock.BaseTime, week))
				assert.Equal(t, recent.Name, nearestPersona(aff, mock.Personas()).Name)
				assert.Equal(t, recent.TopMatch(mock.Catalog()).ItemID, nearestItem(aff, mock.Catalog()).ItemID)
			})
		}
	}
}

// TestUndecayedMisreadsDrift is the other side of TestDecayFollowsRecentTaste: summed
// flat, the same months are misread. Sofia's dresses carry the largest attractors, so
// her half of a month outweighs Kate's; and a month of Lena then Kate reads as Mira — an
// average of two tastes is a third one the shopper never had.
func TestUndecayedMisreadsDrift(t *testing.T) {
	t.Parallel()

	for name, tt := range map[string]struct {
		old, recent, flat mock.Persona
	}{
		"louder old taste wins": {old: mock.PersonaFestiveSofia, recent: mock.PersonaBusinessKate, flat: mock.PersonaFestiveSofia},
		"average is a stranger": {old: mock.PersonaCasualLena, recent: mock.PersonaBusinessKate, flat: mock.PersonaModernMira},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			p := newAffinityPipeline(nil)
			addHistory(p, drifted(tt.old, tt.recent))

			assert.Equal(t, tt.flat.Name, nearestPersona(p.DeriveState(affinity), mock.Personas()).Name)
		})
	}
}

// TestDecayIsRepeatable keeps decay a pure read: the same now gives a bit-identical
// state, and moving now forward only ever fades the corpus, never rewrites it.
func TestDecayIsRepeatable(t *testing.T) {
	t.Parallel()

	p := newAffinityPipeline(nil)
	addPayload(p, mock.Journey(mock.PersonaBusinessKate, mock.WithTimestamp(mock.BaseTime)))

	now := p.DeriveState(decayed(mock.BaseTime, week))
	for range 4 {
		assert.Equal(t, now, p.DeriveState(decayed(mock.BaseTime, week)))
	}

	later := p.DeriveState(decayed(mock.BaseTime.Add(week), week))
	for dimension, v := range now {
		assert.InDelta(t, float64(v/2), float64(later[dimension]), 1e-6, dimension)
	}

	assert.Equal(t, now, p.DeriveState(decayed(mock.BaseTime, week)), "deriving later modified the corpus")
}

// TestAddWithoutTimeDecaysAway pins what Add documents: an event with no recording time
// is infinitely old to a time-aware deriver, and contributes nothing once decayed.
func TestAddWithoutTimeDecaysAway(t *testing.T) {
	t.Parallel()

	p := newAffinityPipeline(nil)
	p.Add(batch{}, kateJourney()...)

	require.NotEmpty(t, p.DeriveState(affinity))
	for dimension, v := range p.DeriveState(decayed(mock.BaseTime, week)) {
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
// visit after a drift at most 0.83 — Kate and Mira, the two business shoppers, are the
// close call — so 0.9 sits in the gap.
const phaseThreshold personalization.Value = 0.9

// phase is a window of consecutive sessions, with the affinity shown across them.
type phase struct {
	From, To time.Time
	Sessions []string
	Affinity personalization.Map
}

// visits groups the corpus into one window per session, in order. Events arrive in
// order and a session's events arrive together, so a change of session is a new visit.
func visits(events []testDecorated) []phase {
	var out []phase

	for i, e := range events {
		if i == 0 || e.Decoration.Session != events[i-1].Decoration.Session {
			out = append(out, phase{From: e.Decoration.At, Sessions: []string{e.Decoration.Session}, Affinity: personalization.Map{}})
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
func phases(threshold personalization.Value) aggregator.StateDeriver[any, decoration, []phase] {
	return func(events []testDecorated) []phase {
		var out []phase

		for _, v := range visits(events) {
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

// phaseOwners names the persona each phase is nearest to, which is how a test reads a
// list of phases back.
func phaseOwners(ps []phase) []string {
	out := make([]string, 0, len(ps))
	for _, p := range ps {
		out = append(out, nearestPersona(p.Affinity, mock.Personas()).Name)
	}

	return out
}

// TestSessionDecoratorReadsBatch pins where the session comes from: the envelope, not
// the event. Every event of a visit carries that visit's session id, and an
// event added without a batch carries none.
func TestSessionDecoratorReadsBatch(t *testing.T) {
	t.Parallel()

	history := mock.History(mock.Month(mock.PersonaBusinessKate))

	p := newAffinityPipeline(nil)
	addHistory(p, history)

	want := make([]string, 0, len(history))
	for _, payload := range history {
		want = append(want, payload.SessionID)
	}

	got := p.DeriveState(func(events []testDecorated) []string {
		var out []string
		for _, v := range visits(events) {
			out = append(out, v.Sessions...)
		}

		return out
	})
	assert.Equal(t, want, got)

	bare := newAffinityPipeline(nil)
	bare.Add(batch{}, kateJourney()...)
	assert.Equal(t, []string{""}, bare.DeriveState(func(events []testDecorated) []string {
		return visits(events)[0].Sessions
	}))
}

// TestSteadyMonthIsOnePhase is the baseline: a shopper who never drifts has one window,
// spanning the whole month, and it reads as them.
func TestSteadyMonthIsOnePhase(t *testing.T) {
	t.Parallel()

	for _, persona := range mock.Personas() {
		t.Run(persona.Name, func(t *testing.T) {
			t.Parallel()

			p := newAffinityPipeline(nil)
			addHistory(p, mock.History(mock.Month(persona)))

			got := p.DeriveState(phases(phaseThreshold))
			require.Len(t, got, 1)
			assert.Equal(t, []string{persona.Name}, phaseOwners(got))
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

				p := newAffinityPipeline(nil)
				addHistory(p, drifted(old, recent))

				got := p.DeriveState(phases(phaseThreshold))
				require.Len(t, got, 2)
				assert.Equal(t, []string{old.Name, recent.Name}, phaseOwners(got))

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

	p := newAffinityPipeline(nil)
	addHistory(p, mock.History(visitsOf))

	got := p.DeriveState(phases(phaseThreshold))
	assert.Equal(t, []string{"Kate", "Sofia", "Kate"}, phaseOwners(got))
}

// TestPhasesNeedSessions shows the decorator doing the work: the same drifted month
// through a chain without it is one undivided visit, so there is nothing to split.
func TestPhasesNeedSessions(t *testing.T) {
	t.Parallel()

	p := aggregator.NewPipeline(testPipelineFn{DecoratorChain: []testDecorator{recorded, touched, intent}})
	addHistory(p, drifted(mock.PersonaFestiveSofia, mock.PersonaBusinessKate))

	assert.Len(t, p.DeriveState(phases(phaseThreshold)), 1)
}

// consented keeps only events from batches whose shopper consented to personalization.
// It is a filter by where an event came from rather than by what it is, which only the
// envelope can answer.
func consented(b batch, _ testEvent) bool {
	return b.Personalization
}

// TestPreFilterSeesEnvelope is why the filter gets the envelope too. Sofia shops twice
// without consent in the middle of Kate's month; a personalization pipeline must never
// see those visits, so the month reads as Kate's in one unbroken phase — where, admitted,
// the same visits would split it.
func TestPreFilterSeesEnvelope(t *testing.T) {
	t.Parallel()

	visitsOf := mock.Month(mock.PersonaBusinessKate)
	visitsOf[2].Persona = mock.PersonaFestiveSofia
	visitsOf[3].Persona = mock.PersonaFestiveSofia

	history := mock.History(visitsOf)
	for _, i := range []int{2, 3} {
		history[i].Consent = mock.ConsentDenied()
	}

	all, filtered := newAffinityPipeline(nil), newAffinityPipeline(consented)
	addHistory(all, history)
	addHistory(filtered, history)

	admitted := 0
	for i, payload := range history {
		if i != 2 && i != 3 {
			admitted += len(payload.Events)
		}
	}

	assert.Equal(t, admitted, filtered.Len())
	assert.Equal(t, []string{"Kate", "Sofia", "Kate"}, phaseOwners(all.DeriveState(phases(phaseThreshold))))
	assert.Equal(t, []string{"Kate"}, phaseOwners(filtered.DeriveState(phases(phaseThreshold))))
}
