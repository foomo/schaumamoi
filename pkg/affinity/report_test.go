package affinity_test

import (
	"fmt"
	"math"
	"path/filepath"
	"testing"
	"time"

	"github.com/foomo/schaumamoi/pkg/affinity"
	"github.com/foomo/schaumamoi/pkg/internal/report"
	"github.com/foomo/schaumamoi/pkg/mock"
	"github.com/foomo/schaumamoi/pkg/personalization"
	"github.com/foomo/sesamy-go/pkg/encoding/mpv2"
	"github.com/stretchr/testify/require"
)

// reportPath is where TestReport writes the affinity report, relative to this package.
var reportPath = filepath.Join("testdata", "report.html")

// imageRoot is where the report finds the mock images, relative to the report itself.
const imageRoot = "../../mock/" + mock.FixtureDir + "/"

const (
	phaseVisits = 6
	gapDays     = 2
)

// history is a shopper whose intent changes every phaseVisits visits, one visit every
// gapDays days, every third visit converting — the shape TestIntentPicksUpQuickly checks.
func history(order ...mock.Persona) ([]mock.Visit, []mpv2.Payload[any]) {
	n := len(order) * phaseVisits
	visits := make([]mock.Visit, 0, n)

	for i := range n {
		visits = append(visits, mock.Visit{
			Persona: order[i/phaseVisits],
			Ago:     time.Duration(n-1-i) * gapDays * mock.Day,
			Convert: i%3 == 2,
		})
	}

	return visits, mock.History(visits)
}

// traced is a history run through an affinity corpus and traced choice by choice: every
// admitted event, which visit it belongs to, and the fade's step after it.
type traced struct {
	visits  []mock.Visit
	visitOf []int
	steps   []affinity.Step
	events  []affinity.Decorated
}

func trace(t *testing.T, order ...mock.Persona) traced {
	t.Helper()

	visits, payloads := history(order...)
	c := affinity.New(nil)
	out := traced{visits: visits}

	for v, payload := range payloads {
		before := c.Len()
		c.Add(affinity.Envelope{At: time.UnixMicro(payload.TimestampMicros).UTC(), Session: payload.SessionID}, payload.Events...)

		for range c.Len() - before {
			out.visitOf = append(out.visitOf, v)
		}
	}

	out.events = c.Derive(func(events []affinity.Decorated) []affinity.Decorated { return events })
	out.steps = affinity.DefaultFade.Trace(out.events)
	require.Len(t, out.steps, len(out.events))

	return out
}

// bands are the phases of a traced history, as chart bands: who was shopping.
func (tr traced) bands() []report.Band {
	var out []report.Band

	for i, v := range tr.visitOf {
		name := tr.visits[v].Persona.Name
		if len(out) == 0 || out[len(out)-1].Label != name+" shopping" {
			out = append(out, report.Band{From: i, Label: name + " shopping"})
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
func (tr traced) visitTicks() []report.Tick {
	var out []report.Tick

	for i, v := range tr.visitOf {
		if (i == 0 || tr.visitOf[i-1] != v) && v%phaseVisits == 0 {
			out = append(out, report.Tick{X: i, Label: fmt.Sprintf("visit %d", v+1)})
		}
	}

	return out
}

// readsAs charts the vector's similarity to every persona after every choice.
func (tr traced) readsAs(title string) report.Chart {
	c := report.Chart{
		Title: title, YLabel: "similarity", XLabel: "choices, in order", YMin: -1, YMax: 1,
		YTicks: []float64{-1, -0.5, 0, 0.5, 1}, Bands: tr.bands(), Rows: tr.rows(), XTicks: tr.visitTicks(),
	}

	for slot, p := range mock.Personas() {
		s := report.Series{Name: p.Name, Slot: slot + 1}
		for _, step := range tr.steps {
			s.Values = append(s.Values, float64(cosine(step.Vector, p.Affinity)))
		}

		c.Series = append(c.Series, s)
	}

	return c
}

// detection charts what the change detection saw after every choice, the threshold, and
// where it collapsed the past.
func (tr traced) detection(title string) report.Chart {
	c := report.Chart{
		Title: title, YLabel: "cosine, recent vs rest", XLabel: "choices, in order", YMin: -1, YMax: 1,
		YTicks: []float64{-1, -0.5, 0, 0.5, 1}, Bands: tr.bands(), Rows: tr.rows(), XTicks: tr.visitTicks(),
		Refs: []report.Ref{{Y: float64(affinity.DefaultFade.Shift), Label: fmt.Sprintf("threshold %.1f", affinity.DefaultFade.Shift)}},
	}

	s := report.Series{Name: "recent vs rest", Slot: 1}

	for i, step := range tr.steps {
		if i < affinity.DefaultFade.Recent {
			s.Values = append(s.Values, math.NaN())

			continue
		}

		s.Values = append(s.Values, float64(step.Cosine))
		if step.Collapsed {
			c.Marks = append(c.Marks, report.Marker{X: i, Y: float64(step.Cosine), Label: "collapse · " + tr.rows()[i]})
		}
	}

	c.Series = []report.Series{s}

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

// comparison is how three readings of the same corpus read each visit: summed flat,
// decayed by time, and faded per event.
type comparison struct {
	Visit   int
	Shopper mock.Persona
	Flat    mock.Persona
	Decayed mock.Persona
	Faded   mock.Persona
}

func compare(t *testing.T, order ...mock.Persona) []comparison {
	t.Helper()

	visits, payloads := history(order...)
	c := affinity.New(nil)
	out := make([]comparison, 0, len(payloads))

	for v, payload := range payloads {
		at := time.UnixMicro(payload.TimestampMicros).UTC()
		c.Add(affinity.Envelope{At: at, Session: payload.SessionID}, payload.Events...)

		out = append(out, comparison{
			Visit:   v + 1,
			Shopper: visits[v].Persona,
			Flat:    nearestPersona(c.Derive(affinity.Flat)),
			Decayed: nearestPersona(c.Derive(affinity.Decayed(at, 7*24*time.Hour))),
			Faded:   nearestPersona(c.Derive(affinity.DefaultFade.Vector)),
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

	k, s, l, m := mock.PersonaBusinessKate, mock.PersonaFestiveSofia, mock.PersonaCasualLena, mock.PersonaModernMira
	f := affinity.DefaultFade

	curve := report.Chart{
		Title: "What a choice still counts for, by how many choices ago it was", YLabel: "weight", XLabel: "choices ago",
		YMin: 0, YMax: 1, YTicks: []float64{0, 0.25, 0.5, 0.75, 1},
		Refs: []report.Ref{{Y: 0.5, Label: "half"}},
	}
	weight := report.Series{Name: "weight", Slot: 1}

	for n := 0; n <= 80; n++ {
		weight.Values = append(weight.Values, math.Pow(float64(f.PerEvent), float64(n)))
		curve.Rows = append(curve.Rows, fmt.Sprintf("%d choices ago", n))

		if n%10 == 0 {
			label := "now"
			if n > 0 {
				label = fmt.Sprintf("%d visit%s", n/10, map[bool]string{true: "", false: "s"}[n == 10])
			}

			curve.XTicks = append(curve.XTicks, report.Tick{X: n, Label: label})
		}
	}

	curve.Series = []report.Series{weight}

	strong := trace(t, k, s, l, m)
	gentle := trace(t, m, l)

	require.NoError(t, report.Write(reportPath, "schaumamoi · affinity report", affinityReport, map[string]any{
		"Fade":           f,
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
	}, map[string]any{
		"img": func(path string) string { return imageRoot + path },
		"pct": func(v float64) string { return fmt.Sprintf("%.0f%%", v) },
		"num": func(v float64) string { return fmt.Sprintf("%.1f", v) },
	}))

	t.Logf("wrote %s", reportPath)
}

func cosine(a, b personalization.Map) personalization.Value {
	norm := float64(a.Score(a)) * float64(b.Score(b))
	if norm == 0 {
		return 0
	}

	return personalization.Value(float64(a.Score(b)) / math.Sqrt(norm))
}

func nearestPersona(aff personalization.Map) mock.Persona {
	var (
		best  mock.Persona
		score personalization.Value
	)

	for i, p := range mock.Personas() {
		if v := cosine(aff, p.Affinity); i == 0 || v > score {
			best, score = p, v
		}
	}

	return best
}

const affinityReport = `
{{define "who"}}<img class="avatar small" src="{{img .ImagePath}}" alt="{{.Name}}" title="{{.Name}}">{{end}}

<h1>Affinity report</h1>
<p class="lede">Generated by <code>TestReport</code> in <code>pkg/affinity</code>. The personalization vector is what the
affinity corpus emits from a shopper's choices — selections, views and carts — and what the shop ranks every listing
with. This report shows how <code>Fade.Vector</code> lets it follow a shopper whose intent changes: how fast the past
fades, when the change detection steps in, and what the vector reads as along the way. The histories are the shape
<code>TestIntentPicksUpQuickly</code> checks: a visit every two days, the persona changing every six visits.</p>
<nav><a href="#fade">The fade</a><a href="#strong">Strong changes</a><a href="#gentle">A gentle change</a><a href="#compare">Against flat and time decay</a></nav>

<h2 id="fade">The fade</h2>
<p class="note">Every choice fades everything before it by the same factor, so recency is counted in choices, not in
time: a shopper who comes back after three weeks is not a stranger. Only the last {{.Fade.Window}} choices count at all,
far beyond what the mock data reach. A visit has about ten choices.</p>
<div class="grid">
  <div class="card"><div class="meta">factor per choice</div><div class="hero">{{.Fade.PerEvent}}</div></div>
  <div class="card"><div class="meta">choices until a choice counts half</div><div class="hero">{{num .HalfLife}}</div></div>
  <div class="card"><div class="meta">left of a visit, one visit later</div><div class="hero">{{pct .AfterVisit}}</div></div>
  <div class="card"><div class="meta">left of a visit, a week later (≈35 choices)</div><div class="hero">{{pct .AfterWeek}}</div></div>
</div>
{{.Curve.SVG}}

<p class="note" style="margin-top:16px">On top of the fade, a change of intent forgets faster. After every choice the last
{{.Fade.Recent}} contributions are compared with the rest of the vector, over the dimensions both name. When their cosine
falls below {{.Fade.Shift}}, the shopper is taken to want something else now and the rest collapses to
{{.Fade.Collapse}} of itself. A step into a new category's own dimensions — clothes to cushions — shares too little to
compare and never triggers it.</p>

<h2 id="strong">Strong changes</h2>
<p class="note">Kate, then Sofia, then Lena, then Mira: {{.StrongChoices}} choices, {{.StrongCollapse}} collapses. The first
two changes are strong — the tastes do not overlap — so the cosine should drop below the threshold within the first
choices of the new intent, collapse the past, and let the new persona take over the vector straight after. The third,
Lena to Mira, is gentle: both are minimal and modern, the detection stays quiet and the fade carries the vector across —
as in the next section. Hover a chart for every value.</p>
{{.StrongDetect.SVG}}
{{.StrongReads.SVG}}

<h2 id="gentle">A gentle change</h2>
<p class="note">Mira, then Lena: tastes that overlap — both minimal and modern. Lena's picks do not point away from what
the vector holds, so the change detection should not fire ({{.GentleCollapse}} collapses); the fade alone carries the
vector across, more slowly.</p>
{{.GentleDetect.SVG}}
{{.GentleReads.SVG}}

<h2 id="compare">Against a flat sum and a decay by time</h2>
<p class="note">The same history read three ways after every visit: summed flat, decayed by time (a one-week
half-life), and faded per choice. The flat sum never lets go of the first intent; time decay follows, but only as fast
as the calendar allows; the fade follows the choices.</p>
<div class="scroll">
<table class="data">
  <tr><th>visit</th><th>shopping</th><th>flat</th><th>decayed by time</th><th>faded per choice</th></tr>
  {{range .Compare}}
  <tr>
    <td>{{.Visit}}</td>
    <td>{{template "who" .Shopper}} {{.Shopper.Name}}</td>
    <td class="{{if eq .Flat.Name .Shopper.Name}}hit{{else}}miss{{end}}">{{template "who" .Flat}} {{.Flat.Name}}</td>
    <td class="{{if eq .Decayed.Name .Shopper.Name}}hit{{else}}miss{{end}}">{{template "who" .Decayed}} {{.Decayed.Name}}</td>
    <td class="{{if eq .Faded.Name .Shopper.Name}}hit{{else}}miss{{end}}">{{template "who" .Faded}} {{.Faded.Name}}</td>
  </tr>
  {{end}}
</table>
</div>
`
