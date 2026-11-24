package corpus_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/foomo/schaumamoi/pkg/internal/report"
	"github.com/foomo/schaumamoi/pkg/mock"
	"github.com/foomo/schaumamoi/pkg/personalization"
	"github.com/stretchr/testify/require"
)

// reportPath is where TestReport writes the corpus report, relative to this package.
var reportPath = filepath.Join("testdata", "report.html")

// imageRoot is where the report finds the mock images, relative to the report itself.
const imageRoot = "../../mock/" + mock.FixtureDir + "/"

type (
	// similarity is how close a derived affinity sits to one persona.
	similarity struct {
		Persona mock.Persona
		Score   personalization.Value
		Best    bool
	}

	// categoryReading is a derived taste read in one category the shopper chose from: the
	// product the taste ranks first there, the persona's own favourite, and whether the
	// persona likes the first — within mock.GoodEnough of its favourite.
	categoryReading struct {
		Name     string
		Nearest  mock.Item
		Expected mock.Item
		OK       bool
	}

	// reading is what one derive made of a corpus: the taste, its similarity to every
	// persona, and the reading in each category the shopper chose from.
	reading struct {
		Affinity   personalization.Map
		Sims       []similarity
		Persona    mock.Persona
		Categories []categoryReading
		Correct    int
	}

	chip struct {
		Persona  mock.Persona
		From, To time.Time
		Sessions int
	}

	sessionRow struct {
		Persona   mock.Persona
		Purchased mock.Item
		Read      reading
	}

	monthRow struct {
		Persona mock.Persona
		Read    reading
		Phases  []chip
	}

	driftRow struct {
		Old, Recent mock.Persona
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

	for _, p := range mock.Personas() {
		events := mock.Journey(p).Events
		c := newAffinityCorpus(nil)
		c.Add(envelope{}, events...)
		sessions = append(sessions, sessionRow{Persona: p, Purchased: purchased(t, events), Read: read(t, c.Derive(affinity), p, events)})

		month := mock.History(mock.Month(p))
		c = newAffinityCorpus(nil)
		addHistory(c, month)
		months = append(months, monthRow{Persona: p, Read: read(t, c.Derive(decayed(mock.BaseTime, week)), p, eventsOf(month)), Phases: chips(c.Derive(phases(phaseThreshold)))})
	}

	for _, old := range mock.Personas() {
		for _, recent := range mock.Personas() {
			if old.Name == recent.Name {
				continue
			}

			history := drifted(old, recent)
			c := newAffinityCorpus(nil)
			addHistory(c, history)
			drifts = append(drifts, driftRow{
				Old:     old,
				Recent:  recent,
				Flat:    read(t, c.Derive(affinity), recent, eventsOf(history[3:])),
				Decayed: read(t, c.Derive(decayed(mock.BaseTime, week)), recent, eventsOf(history[3:])),
				Phases:  chips(c.Derive(phases(phaseThreshold))),
			})
		}
	}

	detour := mock.Month(mock.PersonaBusinessKate)
	detour[2].Persona = mock.PersonaFestiveSofia
	detour[3].Persona = mock.PersonaFestiveSofia
	history := mock.History(detour)

	c := newAffinityCorpus(nil)
	addHistory(c, history)
	detourPhases := chips(c.Derive(phases(phaseThreshold)))

	for _, i := range []int{2, 3} {
		history[i].Consent = mock.ConsentDenied()
	}

	consentedOnly := newAffinityCorpus(consented)
	addHistory(consentedOnly, history)

	third := mock.History(mock.Month(mock.PersonaBusinessKate))[2]
	pipe, err := pipeline("Kate's third visit of her month, two weeks before BaseTime: the shop has learned from two visits by then, and this one ends in the cart", third)
	require.NoError(t, err)

	require.NoError(t, report.Write(reportPath, "schaumamoi · corpus report", pipelineReport+corpusReport, map[string]any{
		"Pipeline":  pipe,
		"Sessions":  sessions,
		"Months":    months,
		"Drifts":    drifts,
		"Detour":    detourPhases,
		"Consented": chips(consentedOnly.Derive(phases(phaseThreshold))),
		"Threshold": phaseThreshold,
		"BaseTime":  mock.BaseTime,
	}, map[string]any{
		"img": func(path string) string { return imageRoot + path },
	}))

	for path := range mock.Images() {
		require.FileExists(t, filepath.Join("..", "mock", mock.FixtureDir, path), "the report shows an image that was never generated, run `go generate ./...`")
	}

	t.Logf("wrote %s", reportPath)
}

// read reads a derived taste the way assertPerCategory does — in every category the
// shopper chose from, against the persona that should have produced it — but records the
// outcome instead of failing on it.
func read(t *testing.T, aff personalization.Map, want mock.Persona, events []testEvent) reading {
	t.Helper()

	r := reading{Affinity: aff, Persona: nearestPersona(aff, mock.Personas())}

	for _, category := range touchedCategories(t, events) {
		cr := categoryReading{
			Name:     category,
			Nearest:  nearestIn(aff, category),
			Expected: want.TopMatch(mock.Filter{Category: category}.Apply(mock.Catalog())),
		}
		cr.OK = want.Likes(cr.Nearest, mock.Filter{Category: category}.Apply(mock.Catalog()))

		if cr.OK {
			r.Correct++
		}

		r.Categories = append(r.Categories, cr)
	}

	for _, p := range mock.Personas() {
		r.Sims = append(r.Sims, similarity{Persona: p, Score: cosine(aff, p.Affinity), Best: p.Name == r.Persona.Name})
	}

	return r
}

func chips(ws []window) []chip {
	out := make([]chip, 0, len(ws))
	for _, w := range ws {
		out = append(out, chip{
			Persona:  nearestPersona(w.Affinity, mock.Personas()),
			From:     w.From,
			To:       w.To,
			Sessions: len(w.Sessions),
		})
	}

	return out
}

const corpusReport = `
{{define "sims"}}<div class="sims">{{range .}}<div class="sim{{if .Best}} best{{end}}"><img class="avatar small" src="{{img .Persona.ImagePath}}" alt="{{.Persona.Name}}"><br>{{.Persona.Name}}<br>{{score .Score}}</div>{{end}}</div>{{end}}
{{define "phases"}}<div class="phases">{{range .}}<div class="phase"><img class="avatar small" src="{{img .Persona.ImagePath}}" alt="{{.Persona.Name}}"><span><b>{{.Persona.Name}}</b><br>{{date .From}} – {{date .To}} · {{.Sessions}} visits</span></div>{{end}}</div>{{end}}
{{define "categories"}}<div class="cats">{{range .}}<div class="cat"><div class="meta"><b>{{.Name}}</b> {{if .OK}}<span class="tick">✓</span>{{else}}<span class="cross">✗</span>{{end}}</div><div class="row"><div class="thumb{{if .OK}} top{{end}}"><img src="{{img .Nearest.ImagePath}}" alt="{{.Nearest.ItemName}}" title="{{.Nearest.ItemName}} — {{.Nearest.Description}}"><b>{{.Nearest.ItemID}}</b>nearest</div>{{if ne .Nearest.ItemID .Expected.ItemID}}<div class="thumb"><img src="{{img .Expected.ImagePath}}" alt="{{.Expected.ItemName}}"><b>{{.Expected.ItemID}}</b>favourite</div>{{end}}</div></div>{{end}}</div>{{end}}

<h1>Corpus report</h1>
<p class="lede">Generated by <code>TestReport</code> in <code>pkg/corpus</code>. Each block feeds generated traffic through the
affinity corpus used by the tests and shows what it reads back, next to the persona that actually produced the
traffic. The taste is one Map across every category; similarities to personas are cosine, and the highlighted avatar is
the reading. It is then read per category, in every category the shopper chose from — dresses, trousers, and the home
category of its pick: the product the taste ranks first there, by the shop's own score — the taste seen through that category's weights —
must be one the persona likes —
within 80% of its favourite there. Shoppers do not always open their favourite, so the favourite is shown where they differ.
Home decoration is reached only through the dimensions it shares with fashion, and through the shopper's home picks.
Decay uses a one-week half-life at BaseTime, {{date .BaseTime}}; phases split below a similarity of {{score .Threshold}}.</p>
<nav><a href="#pipeline">How the corpus works</a><a href="#session">One session</a><a href="#month">Steady month</a><a href="#drift">Drift</a><a href="#detour">Detour</a><a href="#consent">Consent</a></nav>

{{template "pipeline" .Pipeline}}

<h2 id="session">One session</h2>
<p class="note">A single converting journey per persona — fashion, then home decoration — summed flat. The reading should
be the persona itself, right in every category it chose from, and in the category of the purchase the product bought.</p>
<div class="grid">
{{range .Sessions}}
  <div class="card">
    <div class="row">
      <div class="who"><img src="{{img .Persona.ImagePath}}" alt="{{.Persona.Name}}"><b>{{.Persona.Name}}</b><br><span class="muted">shopped</span></div>
      <div>{{template "sims" .Read.Sims}}</div>
    </div>
    {{template "categories" .Read.Categories}}
    <p class="meta">bought <b>{{.Purchased.ItemID}}</b> {{.Purchased.ItemName}}</p>
    {{taste .Read.Affinity}}
  </div>
{{end}}
</div>

<h2 id="month">Steady month</h2>
<p class="note">A month of one persona's visits, read with decay and cut into phases. A steady shopper should read as
themselves, per category as in a single session, and stay one phase.</p>
<div class="grid">
{{range .Months}}
  <div class="card">
    <div class="row">
      <div class="who"><img src="{{img .Persona.ImagePath}}" alt="{{.Persona.Name}}"><b>{{.Persona.Name}}</b><br><span class="muted">shopped</span></div>
      <div>{{template "sims" .Read.Sims}}</div>
    </div>
    {{template "categories" .Read.Categories}}
    <div style="margin-top:10px">{{template "phases" .Phases}}</div>
  </div>
{{end}}
</div>

<h2 id="drift">Drift</h2>
<p class="note">Every ordered pair: the first three visits (up to two weeks ago, including a purchase) are one persona,
the last four another. Summed flat, the old taste too often wins; decayed, the recent one should — as a persona, and in
every category the recent half chose from. Phases should recover both, split where the taste changed.</p>
<div class="scroll">
<table class="data">
  <tr><th>Shopped</th><th>Read flat</th><th>Read decayed</th><th>Categories, decayed</th><th>Phases</th></tr>
  {{range .Drifts}}
  <tr>
    <td style="white-space:nowrap"><img class="avatar small" src="{{img .Old.ImagePath}}" alt="{{.Old.Name}}"> → <img class="avatar small" src="{{img .Recent.ImagePath}}" alt="{{.Recent.Name}}"><br><span class="muted">{{.Old.Name}} then {{.Recent.Name}}</span></td>
    <td class="{{if eq .Flat.Persona.Name .Recent.Name}}hit{{else}}miss{{end}}"><img class="avatar small" src="{{img .Flat.Persona.ImagePath}}" alt="{{.Flat.Persona.Name}}"> {{.Flat.Persona.Name}}<br><span class="muted">{{.Flat.Correct}}/{{len .Flat.Categories}} categories</span></td>
    <td class="{{if eq .Decayed.Persona.Name .Recent.Name}}hit{{else}}miss{{end}}"><img class="avatar small" src="{{img .Decayed.Persona.ImagePath}}" alt="{{.Decayed.Persona.Name}}"> {{.Decayed.Persona.Name}}<br><span class="muted">{{.Decayed.Correct}}/{{len .Decayed.Categories}} categories</span></td>
    <td><div class="row">{{range .Decayed.Categories}}<div class="thumb{{if .OK}} top{{end}}"><img src="{{img .Nearest.ImagePath}}" alt="{{.Nearest.ItemName}}" title="{{.Name}}: {{.Nearest.ItemName}}" style="width:56px;height:75px">{{.Nearest.ItemID}}</div>{{end}}</div></td>
    <td>{{template "phases" .Phases}}</td>
  </tr>
  {{end}}
</table>
</div>

<h2 id="detour">Detour</h2>
<p class="note">Kate's month with visits three and four shopped by Sofia. Phases are contiguous, so the return is a
third phase rather than a Kate phase with a hole in it.</p>
<div class="card">{{template "phases" .Detour}}</div>

<h2 id="consent">Consent</h2>
<p class="note">The same detour, but Sofia's two visits were made without personalization consent. The consent filter
drops them on the way in, so the corpus never sees them and Kate's month is one unbroken phase.</p>
<div class="card">{{template "phases" .Consented}}</div>
`
