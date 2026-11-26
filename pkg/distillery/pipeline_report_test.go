package distillery_test

import (
	"cmp"
	"fmt"
	"reflect"
	"runtime"
	"slices"
	"strings"

	"github.com/foomo/schaumamoi/pkg/distillery/stills/affinity"
	"github.com/foomo/schaumamoi/pkg/internal/mock/shop"
	"github.com/foomo/schaumamoi/pkg/internal/mock/wire"
	"github.com/foomo/schaumamoi/pkg/personalization"
)

// The pipeline section of the still report: how the still is set up and what it does
// to one batch. The infuser chain is read off affinityInfusers and the trace is run
// through it, so both stay true to the code; only the one-line docs are written here,
// and an infuser without one fails the report.

// stage is one named step of the pipeline with a line saying what it does.
type stage struct {
	Name, Doc string
}

// infuserDocs says what each infuser in the affinity chain contributes.
var infuserDocs = map[string]string{
	"Stamp":         "copies the batch's time and session off the envelope, so a recipe can decay by age and group by visit",
	"AddAttractors": "sums the attractors of the items the event carries into one vector, whatever their category",
	"WeighByIntent": "scales that by how deliberate the choice is — select and view 1, the cart 3",
}

// recipes are the questions the tests ask of one still, in the order the report uses them.
var recipes = []stage{
	{"Fade.Vector", "the personalization vector the shop ranks every listing with: faded per event, collapsing its past on a strong change of intent"},
	{"Flat", "taste, summed in full"},
	{"Decayed", "taste, every event halved per half-life of age"},
	{"nearestIn", "a taste read in one category: the product the shop's score ranks first there"},
	{"phases", "the history cut into windows of steady taste, a session at a time"},
	{"funnel", "events counted by name — a question that ignores taste altogether"},
}

// traceStep is what one infuser did to one event's infusion.
type traceStep struct {
	Summary string
}

// traceRow is one event of the traced batch on its way through the chain.
type traceRow struct {
	N       int
	Event   string
	Subject string
	Items   int
	Kept    bool
	Consent bool
	Steps   []traceStep
}

// pipelineSection is the data behind the section: the chain, the recipes, and a batch
// traced through it.
type pipelineSection struct {
	Infusers []stage
	Keep     int
	Recipes  []stage
	Batch    string
	Envelope envelope
	Trace    []traceRow
}

// funcName is a function's own name, without its package — how the report labels a
// infuser it read off the chain.
func funcName(fn any) string {
	name := runtime.FuncForPC(reflect.ValueOf(fn).Pointer()).Name()

	return name[strings.LastIndex(name, ".")+1:]
}

// pipeline builds the section from affinityInfusers, tracing a visit through it
// request by request, each under its own envelope. It returns an error naming any
// infuser the docs do not cover.
func pipeline(label string, session wire.Session) (pipelineSection, error) {
	s := pipelineSection{Recipes: recipes, Keep: affinity.DefaultKeep, Batch: label, Envelope: envelopeOf(session[0])}

	for _, d := range affinityInfusers {
		name := funcName(d)

		doc, ok := infuserDocs[name]
		if !ok {
			return s, fmt.Errorf("infuser %s has no doc in infuserDocs", name)
		}

		s.Infusers = append(s.Infusers, stage{Name: name, Doc: doc})
	}

	for n, hit := range session {
		env, e := envelopeOf(hit), hit.Events[0]

		var p shop.ItemListParams // every event here carries its items under "items"
		_ = e.DecodeParams(&p)

		row := traceRow{N: n + 1, Event: string(e.Name), Items: len(p.Items), Kept: choices(env, e), Consent: consented(env, e)}

		switch {
		case p.ItemListID != "" && len(p.Items) != 1:
			row.Subject = p.ItemListID
		case len(p.Items) == 1:
			row.Subject = p.Items[0].ItemName
		}

		// a dropped event never reaches the infusers
		var d infusion
		for _, infuse := range affinityInfusers {
			if !row.Kept {
				break
			}

			before := d
			d = infuse(env, e, d)
			row.Steps = append(row.Steps, traceStep{Summary: summarize(funcName(infuse), e, before, d)})
		}

		s.Trace = append(s.Trace, row)
	}

	return s, nil
}

// summarize says what one infuser changed, in the terms of that infuser.
func summarize(name string, e testEvent, before, after infusion) string {
	switch name {
	case "Stamp":
		return after.At.UTC().Format("Mon 2 Jan 15:04") + " · " + after.Session
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
func strongest(m personalization.Vector, n int) string {
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
