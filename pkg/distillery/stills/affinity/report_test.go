package affinity_test

import (
	"fmt"
	"math"
	"path/filepath"
	"testing"
	"time"

	"github.com/foomo/schaumamoi/pkg/distillery/stills/affinity"
	"github.com/foomo/schaumamoi/pkg/internal/docs"
	"github.com/foomo/schaumamoi/pkg/internal/mock/persona"
	"github.com/foomo/schaumamoi/pkg/internal/mock/shopper"
	"github.com/foomo/schaumamoi/pkg/internal/mock/wire"
	"github.com/foomo/schaumamoi/pkg/personalization"
	"github.com/stretchr/testify/require"
)

const (
	phaseVisits = 6
	gapDays     = 2
)

// history is a shopper whose intent changes every phaseVisits visits, one visit every
// gapDays days, every third visit converting — the shape TestIntentPicksUpQuickly checks.
func history(order ...persona.Persona) ([]shopper.Visit, []wire.Session) {
	n := len(order) * phaseVisits
	visits := make([]shopper.Visit, 0, n)

	for i := range n {
		visits = append(visits, shopper.Visit{
			Persona: order[i/phaseVisits],
			Ago:     time.Duration(n-1-i) * gapDays * shopper.Day,
			Convert: i%3 == 2,
		})
	}

	return visits, shopper.History(visits)
}

// add feeds a visit into the still request by request, each under its own envelope.
func add(c *affinity.Still, session wire.Session) {
	for _, hit := range session {
		c.Feed(affinity.Envelope{At: time.UnixMicro(hit.TimestampMicros).UTC(), Session: hit.SessionID}, hit.Events...)
	}
}

// traced is a history run through an affinity still and traced choice by choice: every
// kept event, which visit it belongs to, and the fade's step after it.
type traced struct {
	visits  []shopper.Visit
	visitOf []int
	steps   []affinity.Step
	events  []affinity.Infused
}

func trace(t *testing.T, order ...persona.Persona) traced {
	t.Helper()

	visits, sessions := history(order...)
	c := affinity.New(nil)
	out := traced{visits: visits}

	for v, session := range sessions {
		before := c.Len()
		add(c, session)

		for range c.Len() - before {
			out.visitOf = append(out.visitOf, v)
		}
	}

	out.events = c.Distill(func(events []affinity.Infused) []affinity.Infused { return events })
	out.steps = affinity.DefaultFade.Trace(out.events)
	require.Len(t, out.steps, len(out.events))

	return out
}

// bands are the phases of a traced history, as chart bands: who was shopping.
func (tr traced) bands() []docs.Band {
	var out []docs.Band

	for i, v := range tr.visitOf {
		name := tr.visits[v].Persona.Name
		if len(out) == 0 || out[len(out)-1].Label != name+" shopping" {
			out = append(out, docs.Band{From: i, Label: name + " shopping"})
		}

		out[len(out)-1].To = i + 1
	}

	return out
}

// rows name every choice for the tooltip and the table view.
func (tr traced) rows() []string {
	out := make([]string, 0, len(tr.events))

	for i, e := range tr.events {
		v := tr.visitOf[i]
		out = append(out, fmt.Sprintf("choice %d · visit %d (%s) · %s", i+1, v+1, tr.visits[v].Persona.Name, e.Event.Name))
	}

	return out
}

// visitTicks label the first choice of every phase with its visit number.
func (tr traced) visitTicks() []docs.Tick {
	var out []docs.Tick

	for i, v := range tr.visitOf {
		if (i == 0 || tr.visitOf[i-1] != v) && v%phaseVisits == 0 {
			out = append(out, docs.Tick{X: i, Label: fmt.Sprintf("visit %d", v+1)})
		}
	}

	return out
}

// readsAs charts the vector's similarity to every persona after every choice.
func (tr traced) readsAs(title string) docs.Chart {
	c := docs.Chart{
		Title: title, YLabel: "similarity", XLabel: "choices, in order", YMin: -1, YMax: 1,
		YTicks: []float64{-1, -0.5, 0, 0.5, 1}, Bands: tr.bands(), Rows: tr.rows(), XTicks: tr.visitTicks(),
	}

	for slot, p := range persona.All() {
		s := docs.Series{Name: p.Name, Slot: slot + 1}
		for _, step := range tr.steps {
			s.Values = append(s.Values, float64(step.Vector.Cosine(p.Affinity)))
		}

		c.Series = append(c.Series, s)
	}

	return c
}

// detection charts what the change detection saw after every choice, the threshold, and
// where it collapsed the past.
func (tr traced) detection(title string) docs.Chart {
	c := docs.Chart{
		Title: title, YLabel: "cosine, recent vs rest", XLabel: "choices, in order", YMin: -1, YMax: 1,
		YTicks: []float64{-1, -0.5, 0, 0.5, 1}, Bands: tr.bands(), Rows: tr.rows(), XTicks: tr.visitTicks(),
		Refs: []docs.Ref{{Y: float64(affinity.DefaultFade.Shift), Label: fmt.Sprintf("threshold %.1f", affinity.DefaultFade.Shift)}},
	}

	s := docs.Series{Name: "recent vs rest", Slot: 1}

	for i, step := range tr.steps {
		if i < affinity.DefaultFade.Recent {
			s.Values = append(s.Values, math.NaN())

			continue
		}

		s.Values = append(s.Values, float64(step.Cosine))
		if step.Collapsed {
			c.Marks = append(c.Marks, docs.Marker{X: i, Y: float64(step.Cosine), Label: "collapse · " + tr.rows()[i]})
		}
	}

	c.Series = []docs.Series{s}

	return c
}

func (tr traced) collapses() int {
	n := 0
	for _, s := range tr.steps {
		if s.Collapsed {
			n++
		}
	}

	return n
}

// comparison is how three readings of the same still read each visit: summed flat,
// decayed by time, and faded per event.
type comparison struct {
	Visit   int
	Shopper persona.Persona
	Flat    persona.Persona
	Decayed persona.Persona
	Faded   persona.Persona
}

func compare(t *testing.T, order ...persona.Persona) []comparison {
	t.Helper()

	visits, sessions := history(order...)
	c := affinity.New(nil)
	out := make([]comparison, 0, len(sessions))

	for v, session := range sessions {
		add(c, session)
		at := time.UnixMicro(session[len(session)-1].TimestampMicros).UTC()

		out = append(out, comparison{
			Visit:   v + 1,
			Shopper: visits[v].Persona,
			Flat:    nearestPersona(c.Distill(affinity.Flat)),
			Decayed: nearestPersona(c.Distill(affinity.Decayed(at, 7*24*time.Hour))),
			Faded:   nearestPersona(c.Distill(affinity.DefaultFade.Vector)),
		})
	}

	return out
}

// TestReport writes testdata/report.html: how the personalization vector fades, and how
// it follows a shopper whose intent changes — the curve, the change detection choice by
// choice, who the vector reads as, a gentle change, and the fade against a flat sum and
// a decay by time. Open it in a browser after `go test ./pkg/affinity`.
func TestReport(t *testing.T) {
	t.Parallel()

	k, s, l, m := persona.Kate, persona.Sofia, persona.Lena, persona.Mira
	f := affinity.DefaultFade

	curve := docs.Chart{
		Title: "What a choice still counts for, by how many choices ago it was", YLabel: "weight", XLabel: "choices ago",
		YMin: 0, YMax: 1, YTicks: []float64{0, 0.25, 0.5, 0.75, 1},
		Refs: []docs.Ref{{Y: 0.5, Label: "half"}},
	}
	weight := docs.Series{Name: "weight", Slot: 1}

	for n := 0; n <= 80; n++ {
		weight.Values = append(weight.Values, math.Pow(float64(f.PerEvent), float64(n)))
		curve.Rows = append(curve.Rows, fmt.Sprintf("%d choices ago", n))

		if n%10 == 0 {
			label := "now"
			if n > 0 {
				label = fmt.Sprintf("%d visit%s", n/10, map[bool]string{true: "", false: "s"}[n == 10])
			}

			curve.XTicks = append(curve.XTicks, docs.Tick{X: n, Label: label})
		}
	}

	curve.Series = []docs.Series{weight}

	strong := trace(t, k, s, l, m)
	gentle := trace(t, m, l)

	require.NoError(t, docs.Page("affinity", map[string]any{
		"Fade":           f,
		"Keep":           affinity.DefaultKeep,
		"HalfLife":       math.Log(0.5) / math.Log(float64(f.PerEvent)),
		"AfterVisit":     100 * math.Pow(float64(f.PerEvent), 10),
		"AfterWeek":      100 * math.Pow(float64(f.PerEvent), 35),
		"Curve":          curve,
		"StrongReads":    strong.readsAs("Who the vector reads as: Kate, then Sofia, then Lena, then Mira"),
		"StrongDetect":   strong.detection("Change detection: Kate, then Sofia, then Lena, then Mira"),
		"StrongChoices":  len(strong.steps),
		"StrongCollapse": strong.collapses(),
		"GentleReads":    gentle.readsAs("Who the vector reads as: Mira, then Lena"),
		"GentleDetect":   gentle.detection("Change detection: Mira, then Lena"),
		"GentleCollapse": gentle.collapses(),
		"Compare":        compare(t, k, s, l, m),
		"Intents":        intentRows(t),
	}, nil))

	t.Logf("wrote %s", filepath.Join(docs.Dir(), "reports", "affinity.md"))
}

func nearestPersona(aff personalization.Map) persona.Persona {
	var (
		best  persona.Persona
		score personalization.Value
	)

	for i, p := range persona.All() {
		if v := aff.Cosine(p.Affinity); i == 0 || v > score {
			best, score = p, v
		}
	}

	return best
}
