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

	// reading is what one derive made of a corpus: the affinity, its similarity to every
	// persona, and the dress nearest to it.
	reading struct {
		Affinity personalization.Map
		Sims     []similarity
		Persona  mock.Persona
		Item     mock.Item
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
		sessions = append(sessions, sessionRow{Persona: p, Purchased: purchased(t, events), Read: read(c.Derive(affinity))})

		c = newAffinityCorpus(nil)
		addHistory(c, mock.History(mock.Month(p)))
		months = append(months, monthRow{Persona: p, Read: read(c.Derive(decayed(mock.BaseTime, week))), Phases: chips(c.Derive(phases(phaseThreshold)))})
	}

	for _, old := range mock.Personas() {
		for _, recent := range mock.Personas() {
			if old.Name == recent.Name {
				continue
			}

			c := newAffinityCorpus(nil)
			addHistory(c, drifted(old, recent))
			drifts = append(drifts, driftRow{
				Old:     old,
				Recent:  recent,
				Flat:    read(c.Derive(affinity)),
				Decayed: read(c.Derive(decayed(mock.BaseTime, week))),
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

	require.NoError(t, report.Write(reportPath, "schaumamoi · corpus report", corpusReport, map[string]any{
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

func read(aff personalization.Map) reading {
	r := reading{
		Affinity: aff,
		Persona:  nearestPersona(aff, mock.Personas()),
		Item:     nearestItem(aff, mock.Catalog()),
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
{{define "verdict"}}{{if eq .Got.Name .Want.Name}}<span class="tick">✓</span>{{else}}<span class="cross">✗</span>{{end}}{{end}}

<h1>Corpus report</h1>
<p class="lede">Generated by <code>TestReport</code> in <code>pkg/corpus</code>. Each block feeds generated traffic through the
affinity corpus used by the tests (stamp time and session, add attractors, weigh by intent) and shows what it reads back,
next to the persona that actually produced the traffic. Similarities are cosine; the highlighted avatar is the reading.
Decay uses a one-week half-life at BaseTime, {{date .BaseTime}}; phases split below a similarity of {{score .Threshold}}.</p>
<nav><a href="#session">One session</a><a href="#month">Steady month</a><a href="#drift">Drift</a><a href="#detour">Detour</a><a href="#consent">Consent</a></nav>

<h2 id="session">One session</h2>
<p class="note">A single converting journey per persona, summed flat. The reading should be the persona itself, and the
dress nearest the affinity the one it bought.</p>
<div class="grid">
{{range .Sessions}}
  <div class="card">
    <div class="row">
      <div class="who"><img src="{{img .Persona.ImagePath}}" alt="{{.Persona.Name}}"><b>{{.Persona.Name}}</b><br><span class="muted">shopped</span></div>
      <div>{{template "sims" .Read.Sims}}</div>
    </div>
    <div class="row" style="margin-top:10px">
      <div class="thumb"><img src="{{img .Purchased.ImagePath}}" alt="{{.Purchased.ItemName}}"><b>bought</b>{{.Purchased.ItemID}}</div>
      <div class="thumb top"><img src="{{img .Read.Item.ImagePath}}" alt="{{.Read.Item.ItemName}}"><b>nearest</b>{{.Read.Item.ItemID}}</div>
      <div style="align-self:center;font-size:20px">{{if eq .Purchased.ItemID .Read.Item.ItemID}}<span class="tick">✓</span>{{else}}<span class="cross">✗</span>{{end}}</div>
    </div>
    {{taste .Read.Affinity}}
  </div>
{{end}}
</div>

<h2 id="month">Steady month</h2>
<p class="note">A month of one persona's visits, read with decay and cut into phases. A steady shopper should read as
themselves and stay one phase.</p>
<div class="grid">
{{range .Months}}
  <div class="card">
    <div class="row">
      <div class="who"><img src="{{img .Persona.ImagePath}}" alt="{{.Persona.Name}}"><b>{{.Persona.Name}}</b><br><span class="muted">shopped</span></div>
      <div>{{template "sims" .Read.Sims}}</div>
    </div>
    <div class="row" style="margin-top:10px">
      <div class="thumb top"><img src="{{img .Read.Item.ImagePath}}" alt="{{.Read.Item.ItemName}}"><b>nearest</b>{{.Read.Item.ItemID}}</div>
      <div>{{template "phases" .Phases}}</div>
    </div>
  </div>
{{end}}
</div>

<h2 id="drift">Drift</h2>
<p class="note">Every ordered pair: the first three visits (up to two weeks ago, including a purchase) are one persona,
the last four another. Summed flat, the louder or the average taste wins; decayed, the recent one should; phases should
recover both, split where the taste changed.</p>
<div class="scroll">
<table class="data">
  <tr><th>Shopped</th><th>Read flat</th><th>Read decayed</th><th>Nearest dress, decayed</th><th>Phases</th></tr>
  {{range .Drifts}}
  <tr>
    <td style="white-space:nowrap"><img class="avatar small" src="{{img .Old.ImagePath}}" alt="{{.Old.Name}}"> → <img class="avatar small" src="{{img .Recent.ImagePath}}" alt="{{.Recent.Name}}"><br><span class="muted">{{.Old.Name}} then {{.Recent.Name}}</span></td>
    <td class="{{if eq .Flat.Persona.Name .Recent.Name}}hit{{else}}miss{{end}}"><img class="avatar small" src="{{img .Flat.Persona.ImagePath}}" alt="{{.Flat.Persona.Name}}"> {{.Flat.Persona.Name}}</td>
    <td class="{{if eq .Decayed.Persona.Name .Recent.Name}}hit{{else}}miss{{end}}"><img class="avatar small" src="{{img .Decayed.Persona.ImagePath}}" alt="{{.Decayed.Persona.Name}}"> {{.Decayed.Persona.Name}}</td>
    <td><div class="thumb"><img src="{{img .Decayed.Item.ImagePath}}" alt="{{.Decayed.Item.ItemName}}" style="width:56px;height:75px">{{.Decayed.Item.ItemID}}</div></td>
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
