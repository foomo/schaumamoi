package corpus_test

import (
	"cmp"
	"fmt"
	"reflect"
	"runtime"
	"slices"
	"strings"

	"github.com/foomo/schaumamoi/pkg/mock"
	"github.com/foomo/schaumamoi/pkg/personalization"
)

// The pipeline section of the corpus report: how the corpus is set up and what it does
// to one batch. The decorator chain is read off affinityDecorators and the trace is run
// through it, so both stay true to the code; only the one-line docs are written here,
// and a decorator without one fails the report.

// stage is one named step of the pipeline with a line saying what it does.
type stage struct {
	Name, Doc string
}

// decoratorDocs says what each decorator in the affinity chain contributes.
var decoratorDocs = map[string]string{
	"StampTime":     "copies the batch's time off the envelope, so a deriver can decay by age",
	"StampSession":  "copies the session id off the envelope, so a deriver can window by visit",
	"AddAttractors": "sums the attractors of the items the event carries into one Map, whatever their category",
	"WeighByIntent": "scales that by how deliberate the choice is — select and view 1, the cart 3",
}

// derivers are the questions the tests ask of one corpus, in the order the report uses them.
var derivers = []stage{
	{"Fade.Vector", "the personalization vector the shop ranks every listing with: faded per event, collapsing its past on a strong change of intent"},
	{"Flat", "taste, summed in full"},
	{"Decayed", "taste, every event halved per half-life of age"},
	{"nearestIn", "a taste read in one category: the product the shop's score ranks first there"},
	{"phases", "the history cut into windows of steady taste, a session at a time"},
	{"funnel", "events counted by name — a question that ignores taste altogether"},
}

// traceStep is what one decorator did to one event's decoration.
type traceStep struct {
	Summary string
}

// traceRow is one event of the traced batch on its way through the chain.
type traceRow struct {
	N        int
	Event    string
	Subject  string
	Items    int
	Admitted bool
	Consent  bool
	Steps    []traceStep
}

// pipelineSection is the data behind the section: the chain, the derivers, and a batch
// traced through it.
type pipelineSection struct {
	Decorators []stage
	Derivers   []stage
	Batch      string
	Envelope   envelope
	Trace      []traceRow
}

// funcName is a function's own name, without its package — how the report labels a
// decorator it read off the chain.
func funcName(fn any) string {
	name := runtime.FuncForPC(reflect.ValueOf(fn).Pointer()).Name()

	return name[strings.LastIndex(name, ".")+1:]
}

// pipeline builds the section from affinityDecorators, tracing a visit through it
// request by request, each under its own envelope. It returns an error naming any
// decorator the docs do not cover.
func pipeline(label string, session mock.Session) (pipelineSection, error) {
	s := pipelineSection{Derivers: derivers, Batch: label, Envelope: envelopeOf(session[0])}

	for _, d := range affinityDecorators {
		name := funcName(d)

		doc, ok := decoratorDocs[name]
		if !ok {
			return s, fmt.Errorf("decorator %s has no doc in decoratorDocs", name)
		}

		s.Decorators = append(s.Decorators, stage{Name: name, Doc: doc})
	}

	for n, hit := range session {
		env, e := envelopeOf(hit), hit.Events[0]

		var p mock.ItemListParams // every event here carries its items under "items"
		_ = e.DecodeParams(&p)

		row := traceRow{N: n + 1, Event: string(e.Name), Items: len(p.Items), Admitted: choices(env, e), Consent: consented(env, e)}

		switch {
		case p.ItemListID != "" && len(p.Items) != 1:
			row.Subject = p.ItemListID
		case len(p.Items) == 1:
			row.Subject = p.Items[0].ItemName
		}

		// a dropped event never reaches the decorators
		var d decoration
		for _, decorate := range affinityDecorators {
			if !row.Admitted {
				break
			}

			before := d
			d = decorate(env, e, d)
			row.Steps = append(row.Steps, traceStep{Summary: summarize(funcName(decorate), e, before, d)})
		}

		s.Trace = append(s.Trace, row)
	}

	return s, nil
}

// summarize says what one decorator changed, in the terms of that decorator.
func summarize(name string, e testEvent, before, after decoration) string {
	switch name {
	case "StampTime":
		return after.At.UTC().Format("Mon 2 Jan 15:04")
	case "StampSession":
		return after.Session
	case "AddAttractors":
		return fmt.Sprintf("%d dimensions · %s", len(after.Affinity), strongest(after.Affinity, 2))
	case "WeighByIntent":
		w := intentWeight[e.Name]
		if w == 0 {
			return "× 0 — offered, not chosen"
		}

		return fmt.Sprintf("× %.3g → %s", float64(w), strongest(after.Affinity, 3))
	default:
		return fmt.Sprintf("%+v", after)
	}
}

// strongest lists the n largest dimensions of a vector.
func strongest(m personalization.Map, n int) string {
	dims := make([]personalization.Dimension, 0, len(m))
	for d := range m {
		dims = append(dims, d)
	}

	slices.SortFunc(dims, func(a, b personalization.Dimension) int {
		if c := cmp.Compare(m[b], m[a]); c != 0 {
			return c
		}

		return cmp.Compare(a, b)
	})

	parts := make([]string, 0, n)
	for _, d := range dims[:min(n, len(dims))] {
		parts = append(parts, fmt.Sprintf("%s %.3g", d, float64(m[d])))
	}

	return strings.Join(parts, ", ")
}

const pipelineReport = `
{{define "pipeline"}}
<h2 id="pipeline">How the corpus works</h2>
<p class="note">The corpus is set up once, with a filter and a chain of decorators, and then only ever does two things:
<code>Add</code> a batch, which filters and decorates each event exactly once as it arrives, and <code>Derive</code>, which hands
the decorated events to a deriver and returns whatever state it builds — a pure read, as often as you like. The decorators
below are read off the chain the tests use, in order.</p>
<div class="flow">
  <div class="stage">
    <h4>1 · A request</h4>
    <p>One mpv2 request per interaction, as the client sends it: its event, and an <b>envelope</b> — what upstream knows beyond the event. A visit is many requests under one session id.</p>
    <p class="meta">envelope: <code>At</code> · <code>Session</code> · <code>Personalization</code> consent</p>
  </div>
  <div class="arrow">→</div>
  <div class="stage">
    <h4>2 · Filter</h4>
    <p><code>Config.Filter</code> admits or drops each event. It sees the envelope, so it can drop by where an event came from as
    well as by what it is.</p>
    <p class="meta">affinity corpus: admits selections, views and carts — nothing else, no listings · consent corpus: those, and only from batches with personalization consent</p>
  </div>
  <div class="arrow">→</div>
  <div class="stage wide">
    <h4>3 · Decorators, in order, once per event</h4>
    <ol>{{range .Decorators}}<li><code>{{.Name}}</code> — {{.Doc}}</li>{{end}}</ol>
    <p class="meta">each starts from what the one before returned; the envelope is dropped after this step</p>
  </div>
  <div class="arrow">→</div>
  <div class="stage">
    <h4>4 · The corpus</h4>
    <p>Admitted events with their decoration, in arrival order: <code>DecoratedEvent{Event, Decoration}</code>, where the
    decoration is <code>At · Session · Affinity</code>. Never rewritten.</p>
  </div>
  <div class="arrow">→</div>
  <div class="stage">
    <h4>5 · Derivers</h4>
    <ul>{{range .Derivers}}<li><code>{{.Name}}</code> — {{.Doc}}</li>{{end}}</ul>
    <p class="meta">anything relative to now, like decay, happens here and nowhere earlier</p>
  </div>
</div>

<h3 style="margin-top:20px">One visit, request by request</h3>
<p class="note">{{.Batch}}, run through the chain above one decorator at a time. Each column is what that decorator made
of the event: the envelope stamped on, the taste its items carry — one Map, dresses, trousers and lamps alike — and
finally the weight it is scaled by: a selection in full, the cart three times over. Listings never get that far: the
affinity corpus filters for the choices it learns from, and drops everything else at the door.</p>
<p class="meta">first request: At {{date .Envelope.At}} · Session <code>{{.Envelope.Session}}</code> · personalization consent {{if .Envelope.Personalization}}granted{{else}}not granted{{end}}</p>
<div class="scroll">
<table class="data trace">
  <tr><th>#</th><th>event</th><th>list / item</th><th>items</th><th>affinity filter</th><th>consent filter</th>{{range .Decorators}}<th><code>{{.Name}}</code></th>{{end}}</tr>
  {{range .Trace}}
  <tr>
    <td>{{.N}}</td>
    <td><code>{{.Event}}</code></td>
    <td>{{.Subject}}</td>
    <td>{{.Items}}</td>
    <td>{{if .Admitted}}admit{{else}}<span class="muted">drop</span>{{end}}</td>
    <td>{{if .Consent}}admit{{else}}drop{{end}}</td>
    {{if .Admitted}}{{range .Steps}}<td>{{.Summary}}</td>{{end}}{{else}}<td colspan="{{len $.Decorators}}" class="muted">not admitted — never decorated, never stored</td>{{end}}
  </tr>
  {{end}}
</table>
</div>
{{end}}
`
