package affinity

import (
	"math"
	"sort"

	"github.com/foomo/schaumamoi/pkg/personalization"
)

// Fade derives the shopper's personalization vector with recency counted in events, not
// in time: what matters is how much the shopper has done since, not how long ago it was.
// A shopper who comes back after three weeks is not a stranger.
//
// Three rules, applied in event order:
//
//   - Every admitted event fades everything before it by PerEvent: the vector is an
//     exponentially weighted sum of the contributions, the newest counting most.
//   - Only the last Window events count at all. With a PerEvent below 1 the tail is
//     tiny long before the window ends; the window bounds the work and makes the cut
//     explicit.
//   - A strong change of intent forgets faster. After each event the last Recent
//     contributions are compared with the rest of the vector. When they point clearly
//     elsewhere — a cosine below Shift, over the dimensions both name — the shopper is
//     taken to want something else now, and the rest is multiplied by Collapse.
//
// The comparison only looks at dimensions both sides name. A shopper moving from clothes
// to lamps brings dimensions the clothes never had (cozy, natural) — that is a new
// category, not a new intent, and must not wipe the taste the two share.
type Fade struct {
	PerEvent personalization.Value
	Window   int
	Recent   int
	Shift    personalization.Value
	Collapse personalization.Value
}

// DefaultFade is the fade the shop ranks with. A visit has about ten choices, so a
// PerEvent of 0.95 keeps roughly 60% of the vector from one visit to the next. The window
// of 1000 events is far beyond what the mock data reach.
var DefaultFade = Fade{PerEvent: 0.95, Window: 1000, Recent: 3, Shift: 0.5, Collapse: 0.2}

// Vector is the deriver: the shopper's personalization vector after every admitted event.
func (f Fade) Vector(events []Decorated) personalization.Map {
	return f.walk(events, nil)
}

// Step is the vector after one event, with what the change detection saw: the cosine of
// the recent contributions against the rest — 1 before there are Recent events to
// compare — and whether that collapsed the rest.
type Step struct {
	Cosine    personalization.Value
	Collapsed bool
	Vector    personalization.Map
}

// Trace is Vector event by event, for looking at how the vector moves: one Step per
// admitted event, each with its own copy of the vector. It runs the same loop as Vector.
func (f Fade) Trace(events []Decorated) []Step {
	out := make([]Step, 0, len(events))
	f.walk(events, func(cosine personalization.Value, collapsed bool, vector personalization.Map) {
		out = append(out, Step{Cosine: cosine, Collapsed: collapsed, Vector: clone(vector)})
	})

	return out
}

// walk is the one loop behind Vector and Trace; step, if given, sees every event's result.
func (f Fade) walk(events []Decorated, step func(cosine personalization.Value, collapsed bool, vector personalization.Map)) personalization.Map {
	if len(events) > f.Window {
		events = events[len(events)-f.Window:]
	}

	all := personalization.Map{}

	for i, e := range events {
		for d, v := range all {
			all[d] = v * f.PerEvent
		}

		for d, v := range e.Decoration.Affinity {
			all[d] += v
		}

		if i+1 <= f.Recent {
			if step != nil {
				step(1, false, all)
			}

			continue
		}

		recent := f.recent(events[i+1-f.Recent : i+1])
		rest := personalization.Map{}

		for d, v := range all {
			if r := v - recent[d]; r != 0 {
				rest[d] = r
			}
		}

		cosine := sharedCosine(recent, rest)
		collapsed := cosine < f.Shift

		if collapsed {
			for d, v := range rest {
				all[d] = recent[d] + v*f.Collapse
			}
		}

		if step != nil {
			step(cosine, collapsed, all)
		}
	}

	return all
}

// recent is the faded sum of the last few contributions, as they stand in the vector.
func (f Fade) recent(events []Decorated) personalization.Map {
	out := personalization.Map{}

	for i, e := range events {
		w := personalization.Value(math.Pow(float64(f.PerEvent), float64(len(events)-1-i)))
		for d, v := range e.Decoration.Affinity {
			out[d] += v * w
		}
	}

	return out
}

// sharedCosine is the cosine of two vectors over the dimensions both name — 1 when they
// share none, so that a step into new dimensions alone never reads as a change.
func sharedCosine(a, b personalization.Map) personalization.Value {
	shared := make([]personalization.Dimension, 0, len(a))

	for d := range a {
		if _, ok := b[d]; ok {
			shared = append(shared, d)
		}
	}

	if len(shared) == 0 {
		return 1
	}

	sort.Slice(shared, func(i, j int) bool { return shared[i] < shared[j] })

	var dot, na, nb float64
	for _, d := range shared {
		dot += float64(a[d] * b[d])
		na += float64(a[d] * a[d])
		nb += float64(b[d] * b[d])
	}

	if na == 0 || nb == 0 {
		return 1
	}

	return personalization.Value(dot / math.Sqrt(na*nb))
}
