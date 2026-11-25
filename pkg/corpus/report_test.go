package corpus_test

import (
	"path/filepath"
	"testing"
	"time"

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
	// similarity is how close a derived affinity sits to one persona.
	similarity struct {
		Persona persona.Persona
		Score   personalization.Value
		Best    bool
	}

	// categoryReading is a derived taste read in one category the shopper chose from: the
	// product the taste ranks first there, the persona's own favourite, and whether the
	// persona likes the first — within persona.GoodEnough of its favourite.
	categoryReading struct {
		Name     string
		Nearest  catalog.Item
		Expected catalog.Item
		OK       bool
	}

	// reading is what one derive made of a corpus: the taste, its similarity to every
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
)

// TestReport writes testdata/report.html: what the corpus recovers from generated
// traffic, shown with the mock images. A single session, a steady month, every drift
// between two personas read flat and with decay and cut into phases, a detour, and the
// consent filter — each beside the persona that actually produced it, so a misreading
// is visible at a glance. Open it in a browser after `go test ./pkg/corpus`.
func TestReport(t *testing.T) {
	t.Parallel()

	var (
		sessions []sessionRow
		months   []monthRow
		drifts   []driftRow
	)

	for _, p := range persona.All() {
		events := shopper.Journey(p).Events()
		c := newAffinityCorpus(nil)
		c.Add(envelope{}, events...)
		sessions = append(sessions, sessionRow{Persona: p, Purchased: purchased(t, events), Read: read(t, c.Derive(flat), p, events)})

		month := shopper.History(shopper.Month(p))
		c = newAffinityCorpus(nil)
		addHistory(c, month)
		months = append(months, monthRow{Persona: p, Read: read(t, c.Derive(decayed(wire.BaseTime, week)), p, eventsOf(month)), Phases: chips(c.Derive(phases(phaseThreshold)))})
	}

	for _, old := range persona.All() {
		for _, recent := range persona.All() {
			if old.Name == recent.Name {
				continue
			}

			history := drifted(old, recent)
			c := newAffinityCorpus(nil)
			addHistory(c, history)
			drifts = append(drifts, driftRow{
				Old:     old,
				Recent:  recent,
				Flat:    read(t, c.Derive(flat), recent, eventsOf(history[3:])),
				Decayed: read(t, c.Derive(decayed(wire.BaseTime, week)), recent, eventsOf(history[3:])),
				Phases:  chips(c.Derive(phases(phaseThreshold))),
			})
		}
	}

	detour := shopper.Month(persona.Kate)
	detour[2].Persona = persona.Sofia
	detour[3].Persona = persona.Sofia
	history := shopper.History(detour)

	c := newAffinityCorpus(nil)
	addHistory(c, history)
	detourPhases := chips(c.Derive(phases(phaseThreshold)))

	for _, i := range []int{2, 3} {
		history[i] = history[i].WithConsent(wire.ConsentDenied())
	}

	consentedOnly := newAffinityCorpus(consented)
	addHistory(consentedOnly, history)

	third := shopper.History(shopper.Month(persona.Kate))[2]
	pipe, err := pipeline("Kate's third visit of her month, two weeks before BaseTime: the shop knows two visits of hers by then, and this one ends in the cart", third)
	require.NoError(t, err)

	require.NoError(t, docs.Page("corpus", map[string]any{
		"Pipeline":  pipe,
		"Sessions":  sessions,
		"Months":    months,
		"Drifts":    drifts,
		"Detour":    detourPhases,
		"Consented": chips(consentedOnly.Derive(phases(phaseThreshold))),
		"Threshold": phaseThreshold,
		"BaseTime":  wire.BaseTime,
	}, nil))

	t.Logf("wrote %s", filepath.Join(docs.Dir(), "reports", "corpus.md"))
}

// read reads a derived taste the way assertPerCategory does — in every category the
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
