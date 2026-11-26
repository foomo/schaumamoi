// Package visits holds recipes that read a still visit by visit: Of groups its events
// into visits, and Phases merges consecutive visits of steady taste.
//
// A visit is one session's run of events. It is the unit taste is judged in: a single
// event is too small to say much, a whole month too large to say when the taste changed.
// The recipes work on any still whose infusion is stamp.Stamped; Phases is handed the
// taste to judge by, so it does not need to know what else the infusion holds.
package visits

import (
	"time"

	"github.com/foomo/schaumamoi/pkg/distillery"
	"github.com/foomo/schaumamoi/pkg/distillery/stamp"
	"github.com/foomo/schaumamoi/pkg/personalization"
)

type (
	// Visit is one session's run of events, in arrival order: from the first event's time
	// to the last's.
	Visit[Params any, Infusion stamp.Stamped] struct {
		Session  string
		From, To time.Time
		Events   []distillery.Infused[Params, Infusion]
	}

	// Phase is a run of consecutive visits of steady taste, with the taste summed over all
	// of them.
	Phase[Params any, Infusion stamp.Stamped] struct {
		From, To time.Time
		Visits   []Visit[Params, Infusion]
		Taste    personalization.Vector
	}
)

// Of is the recipe that groups a still's events into visits. Events arrive in order and a
// session's events arrive together, so a change of session opens a new visit.
func Of[Params any, Infusion stamp.Stamped](events []distillery.Infused[Params, Infusion]) []Visit[Params, Infusion] {
	var out []Visit[Params, Infusion]

	for i, e := range events {
		s := e.Infusion.Stamped()
		if i == 0 || s.Session != events[i-1].Infusion.Stamped().Session {
			out = append(out, Visit[Params, Infusion]{Session: s.Session, From: s.At})
		}

		v := &out[len(out)-1]
		v.To = s.At
		v.Events = append(v.Events, e)
	}

	return out
}

// Phases is the recipe that merges consecutive visits into phases of steady taste: a
// visit joins the current phase if its taste is at least threshold similar (Cosine) to
// the phase's so far, and opens a new phase otherwise. A visit that says nothing about
// taste joins whatever phase it falls in.
//
// Phases are contiguous. A shopper who drifts away and comes back gets three phases, not
// two — the return is news about when the taste held, which a merge would lose.
func Phases[Params any, Infusion stamp.Stamped](threshold personalization.Value, taste func(Infusion) personalization.Vector) distillery.Recipe[Params, Infusion, []Phase[Params, Infusion]] {
	return func(events []distillery.Infused[Params, Infusion]) []Phase[Params, Infusion] {
		var out []Phase[Params, Infusion]

		for _, v := range Of(events) {
			t := personalization.Vector{}
			for _, e := range v.Events {
				for d, w := range taste(e.Infusion) {
					t[d] += w
				}
			}

			if len(out) > 0 {
				current := &out[len(out)-1]
				if len(t) == 0 || t.Cosine(current.Taste) >= threshold {
					current.To = v.To
					current.Visits = append(current.Visits, v)
					for d, w := range t {
						current.Taste[d] += w
					}

					continue
				}
			}

			out = append(out, Phase[Params, Infusion]{From: v.From, To: v.To, Visits: []Visit[Params, Infusion]{v}, Taste: t})
		}

		return out
	}
}
