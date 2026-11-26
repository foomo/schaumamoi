package distillery_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/foomo/schaumamoi/pkg/affinity"
	"github.com/foomo/schaumamoi/pkg/distillery"
	"github.com/foomo/schaumamoi/pkg/internal/docs"
	"github.com/foomo/schaumamoi/pkg/internal/mock/catalog"
	"github.com/foomo/schaumamoi/pkg/internal/mock/persona"
	"github.com/foomo/schaumamoi/pkg/internal/mock/shop"
	"github.com/foomo/schaumamoi/pkg/internal/mock/shopper"
	"github.com/foomo/schaumamoi/pkg/internal/mock/wire"
	"github.com/foomo/schaumamoi/pkg/personalization"
	"github.com/stretchr/testify/require"
)

type (
	// similarity is how close a distilled affinity sits to one persona.
	similarity struct {
		Persona persona.Persona
		Score   personalization.Value
		Best    bool
	}

	// categoryReading is a distilled taste read in one category the shopper chose from: the
	// product the taste ranks first there, the persona's own favourite, and whether the
	// persona likes the first — within persona.GoodEnough of its favourite.
	categoryReading struct {
		Name     string
		Nearest  catalog.Item
		Expected catalog.Item
		OK       bool
	}

	// reading is what one distill made of a still: the taste, its similarity to every
	// persona, and the reading in each category the shopper chose from.
	reading struct {
		Affinity   personalization.Map
		Sims       []similarity
		Persona    persona.Persona
		Categories []categoryReading
		Correct    int
	}

	chip struct {
		Persona  persona.Persona
		From, To time.Time
		Sessions int
	}

	sessionRow struct {
		Persona   persona.Persona
		Purchased catalog.Item
		Read      reading
	}

	monthRow struct {
		Persona persona.Persona
		Read    reading
		Phases  []chip
	}

	driftRow struct {
		Old, Recent persona.Persona
		Flat        reading
		Decayed     reading
		Phases      []chip
	}

	// drawOffRow is a drifted month fed into two stills: one holding everything, one
	// drawing off what is older than a policy age after every feed.
	drawOffRow struct {
		Old, Recent   persona.Persona
		Held, HeldAll int
		Bounded       reading
		Unbounded     reading
		Phases        []chip
	}
)

// TestReport writes testdata/report.html: what the still recovers from generated
// traffic, shown with the mock images. A single session, a steady month, every drift
// between two personas read flat and with decay and cut into phases, a detour, and the
// consent cut — each beside the persona that actually produced it, so a misreading
// is visible at a glance. Open it in a browser after `go test ./pkg/distillery`.
func TestReport(t *testing.T) {
	t.Parallel()

	var (
		sessions []sessionRow
		months   []monthRow
		drifts   []driftRow
		drawOffs []drawOffRow
	)

	for _, p := range persona.All() {
		events := shopper.Journey(p).Events()
		c := newAffinityStill(nil)
		c.Feed(envelope{}, events...)
		sessions = append(sessions, sessionRow{Persona: p, Purchased: purchased(t, events), Read: read(t, c.Distill(flat), p, events)})

		month := shopper.History(shopper.Month(p))
		c = newAffinityStill(nil)
		addHistory(c, month)
		months = append(months, monthRow{Persona: p, Read: read(t, c.Distill(decayed(wire.BaseTime, week)), p, eventsOf(month)), Phases: chips(c.Distill(phases(phaseThreshold)))})
	}

	for _, old := range persona.All() {
		for _, recent := range persona.All() {
			if old.Name == recent.Name {
				continue
			}

			history := drifted(old, recent)
			c := newAffinityStill(nil)
			addHistory(c, history)
			drifts = append(drifts, driftRow{
				Old:     old,
				Recent:  recent,
				Flat:    read(t, c.Distill(flat), recent, eventsOf(history[3:])),
				Decayed: read(t, c.Distill(decayed(wire.BaseTime, week)), recent, eventsOf(history[3:])),
				Phases:  chips(c.Distill(phases(phaseThreshold))),
			})
		}
	}

	// the same drifts, each persona to the next, with a standing draw-off of everything older
	// than drawOffAge: by the end of the month the old taste is gone, even summed flat
	for i, old := range persona.All() {
		recent := persona.All()[(i+1)%len(persona.All())]
		history := drifted(old, recent)

		all := newAffinityStill(nil)
		addHistory(all, history)

		bounded := distillery.New(testConfig{
			Cut:      affinity.Choices,
			Infusers: affinity.Infusers,
			DrawOff:  func(env envelope) affinity.Rule { return affinity.OlderThan(env.At, drawOffAge) },
		})
		addHistory(bounded, history)

		drawOffs = append(drawOffs, drawOffRow{
			Old:       old,
			Recent:    recent,
			Held:      bounded.Len(),
			HeldAll:   all.Len(),
			Bounded:   read(t, bounded.Distill(flat), recent, eventsOf(history[3:])),
			Unbounded: read(t, all.Distill(flat), recent, eventsOf(history[3:])),
			Phases:    chips(bounded.Distill(phases(phaseThreshold))),
		})

		row := drawOffs[len(drawOffs)-1]
		require.Equal(t, recent.Name, row.Bounded.Persona.Name, "%s then %s: drawn off, the old taste must be gone", old.Name, recent.Name)
		require.Len(t, row.Phases, 1, "%s then %s: only the recent phase is left", old.Name, recent.Name)
	}

	detour := shopper.Month(persona.Kate)
	detour[2].Persona = persona.Sofia
	detour[3].Persona = persona.Sofia
	history := shopper.History(detour)

	c := newAffinityStill(nil)
	addHistory(c, history)
	detourPhases := chips(c.Distill(phases(phaseThreshold)))

	for _, i := range []int{2, 3} {
		history[i] = history[i].WithConsent(wire.ConsentDenied())
	}

	consentedOnly := newAffinityStill(consented)
	addHistory(consentedOnly, history)

	third := shopper.History(shopper.Month(persona.Kate))[2]
	pipe, err := pipeline("Kate's third visit of her month, two weeks before BaseTime: the shop knows two visits of hers by then, and this one ends in the cart", third)
	require.NoError(t, err)

	require.NoError(t, docs.Page("distillery", map[string]any{
		"Pipeline":   pipe,
		"Sessions":   sessions,
		"Months":     months,
		"Drifts":     drifts,
		"DrawOffs":   drawOffs,
		"DrawOffAge": int(drawOffAge / (24 * time.Hour)),
		"Detour":     detourPhases,
		"Consented":  chips(consentedOnly.Distill(phases(phaseThreshold))),
		"Threshold":  phaseThreshold,
		"BaseTime":   wire.BaseTime,
	}, nil))

	t.Logf("wrote %s", filepath.Join(docs.Dir(), "reports", "distillery.md"))
}

// read reads a distilled taste the way assertPerCategory does — in every category the
// shopper chose from, against the persona that should have produced it — but records the
// outcome instead of failing on it.
func read(t *testing.T, aff personalization.Map, want persona.Persona, events []testEvent) reading {
	t.Helper()

	r := reading{Affinity: aff, Persona: nearestPersona(aff, persona.All())}

	for _, category := range touchedCategories(t, events) {
		cr := categoryReading{
			Name:     category,
			Nearest:  nearestIn(aff, category),
			Expected: want.TopMatch(shop.Filter{Category: category}.Apply(catalog.All())),
		}
		cr.OK = want.Likes(cr.Nearest, shop.Filter{Category: category}.Apply(catalog.All()))

		if cr.OK {
			r.Correct++
		}

		r.Categories = append(r.Categories, cr)
	}

	for _, p := range persona.All() {
		r.Sims = append(r.Sims, similarity{Persona: p, Score: cosine(aff, p.Affinity), Best: p.Name == r.Persona.Name})
	}

	return r
}

func chips(ws []window) []chip {
	out := make([]chip, 0, len(ws))
	for _, w := range ws {
		out = append(out, chip{
			Persona:  nearestPersona(w.Affinity, persona.All()),
			From:     w.From,
			To:       w.To,
			Sessions: len(w.Sessions),
		})
	}

	return out
}

// drawOffAge is the age the report's standing draw-off forgets at: the drifted month's
// three old visits are 14 to 27 days old by its end, the four recent ones 9 days and less.
const drawOffAge = 10 * 24 * time.Hour
