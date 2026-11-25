package affinity_test

import (
	"fmt"
	"testing"

	"github.com/foomo/schaumamoi/pkg/affinity"
	"github.com/foomo/schaumamoi/pkg/internal/mock"
	"github.com/foomo/sesamy-go/pkg/sesamy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// How quickly a new intent has to be picked up, in visits from the first visit of the
// new intent, by the vector's reading after the visit, the visit's closing look and the
// next landing page — which is ranked before the shopper has done anything on its visit,
// so it is always a visit behind.
//
// A strong change — to a taste that does not overlap the old one — is what the vector's
// change detection is for: the vector reads it on the very first visit, the shop serves
// it by the second. A gentle change, between tastes that overlap (Mira and Lena are both
// minimal and modern), is not taken for a new intent; the vector only fades into it,
// which takes a few visits. Without the change detection strong changes take as long as
// gentle ones; without any fade the shop never catches up at all.
var (
	strongPickup = pickup{vector: 1, closing: 2, landing: 2}
	gentlePickup = pickup{vector: 3, closing: 3, landing: 4}
)

type pickup struct{ vector, closing, landing int }

// intentOrders are long histories with a changing intent: the persona changes every
// phaseVisits visits. Together they hold every ordered pair of personas, so every
// direction of change is tested — each deep into a history, behind earlier intents, which
// is where a learner that never forgets stops keeping up.
func intentOrders() [][]mock.Persona {
	k, s, l, m := mock.PersonaBusinessKate, mock.PersonaFestiveSofia, mock.PersonaCasualLena, mock.PersonaModernMira

	return [][]mock.Persona{
		{k, s, l, m},
		{m, l, s, k},
		{s, m, k, l},
		{k, m, s, l, k},
	}
}

// intentReading is how one visit served and read the intent it was made with.
type intentReading struct {
	Persona mock.Persona
	Landing bool // the visit's first listing opens on something the intent likes
	Closing bool // so does its closing look at the department
	Vector  bool // after the visit, the emitted vector reads as the intent's persona
}

// readIntents runs a history with the given order of intents (see history) through the
// shop and the shop's own affinity corpus, and reads every visit against the persona that
// made it.
func readIntents(t *testing.T, order []mock.Persona) []intentReading {
	t.Helper()

	visits, history := history(order...)
	c := affinity.New(nil)
	out := make([]intentReading, 0, len(history))

	for i, session := range history {
		want := visits[i].Persona
		lists := listingsOf(t, session)
		require.GreaterOrEqual(t, len(lists), 5)

		add(c, session)

		out = append(out, intentReading{
			Persona: want,
			Landing: want.Likes(lists[0].Items[0], lists[0].Items),
			Closing: want.Likes(lists[4].Items[0], lists[4].Items),
			Vector:  nearestPersona(c.Derive(affinity.DefaultFade.Vector)).Name == want.Name,
		})
	}

	return out
}

func listingsOf(t *testing.T, session mock.Session) []mock.ItemListParams {
	t.Helper()

	var out []mock.ItemListParams

	for _, e := range session.Events() {
		if e.Name != sesamy.EventNameViewItemList {
			continue
		}

		var p mock.ItemListParams
		require.NoError(t, e.DecodeParams(&p))
		out = append(out, p)
	}

	return out
}

// TestIntentPicksUpQuickly runs long histories with a changing intent through the shop as
// it is — the same route every visit, the persona changing every few visits — and checks
// that the vector and the shop follow each change quickly, and stay with it: within the
// pickup for the kind of change the vector reads the new intent, the closing look and the
// landing page serve it, and none of them falls back for the rest of the phase.
func TestIntentPicksUpQuickly(t *testing.T) {
	t.Parallel()

	for _, order := range intentOrders() {
		readings := readIntents(t, order)

		for phase, persona := range order {
			from := phase * phaseVisits
			name, bound := persona.Name, strongPickup
			if phase > 0 {
				name = order[phase-1].Name + " then " + persona.Name
				if cosine(order[phase-1].Affinity, persona.Affinity) > 0 {
					name, bound = name+" (gentle)", gentlePickup
				}
			}

			t.Run(fmt.Sprintf("%s/%d %s", names(order), phase+1, name), func(t *testing.T) {
				t.Parallel()

				visits := readings[from : from+phaseVisits]

				assertPickedUp(t, visits, func(r intentReading) bool { return r.Vector }, bound.vector, "the vector")
				assertPickedUp(t, visits, func(r intentReading) bool { return r.Closing }, bound.closing, "the closing look")
				assertPickedUp(t, visits, func(r intentReading) bool { return r.Landing }, bound.landing, "the landing page")
			})
		}
	}
}

// assertPickedUp checks that ok holds from some visit within the first within of the
// phase, and on every visit after it.
func assertPickedUp(t *testing.T, visits []intentReading, ok func(intentReading) bool, within int, what string) {
	t.Helper()

	first := -1

	for i, r := range visits {
		if ok(r) && first < 0 {
			first = i
		}

		if first >= 0 {
			assert.True(t, ok(r), "%s fell back on visit %d of the phase", what, i+1)
		}
	}

	require.GreaterOrEqual(t, first, 0, "%s never picked up the intent", what)
	assert.Less(t, first, within, "%s picked up the intent only on visit %d", what, first+1)
}

func names(order []mock.Persona) string {
	out := ""
	for _, p := range order {
		out += p.Name[:1]
	}

	return out
}

// intentRow is one long history as the vector report shows it: every visit's reading,
// with the first visit of each new intent marked.
type intentRow struct {
	Visits []intentVisit
}

type intentVisit struct {
	intentReading
	PhaseStart bool
}

func intentRows(t *testing.T) []intentRow {
	t.Helper()

	out := make([]intentRow, 0, len(intentOrders()))

	for _, order := range intentOrders() {
		var row intentRow
		for i, r := range readIntents(t, order) {
			row.Visits = append(row.Visits, intentVisit{intentReading: r, PhaseStart: i > 0 && i%phaseVisits == 0})
		}

		out = append(out, row)
	}

	return out
}
