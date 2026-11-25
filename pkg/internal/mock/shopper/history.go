package shopper

import (
	"cmp"
	"slices"
	"strconv"
	"time"

	"github.com/foomo/schaumamoi/pkg/internal/mock/persona"
	"github.com/foomo/schaumamoi/pkg/internal/mock/wire"
)

// Day is a calendar day, the unit visit schedules are written in.
const Day = 24 * time.Hour

// Visit is one session in a shopper's history: whose taste drives it, how long before
// wire.BaseTime it started, and whether it ends in the cart.
//
// The persona is per visit rather than per history so that taste can drift: a shopper
// who spends the first half of the month hunting for a gala dress and the second half
// back in office wear is two personas on one client id. Who the shopper is on the wire
// is the envelope's business, set through the options History takes.
type Visit struct {
	Persona persona.Persona
	Ago     time.Duration
	Convert bool
}

// Month is a month of visits for one persona, ending at wire.BaseTime: mostly browsing, with
// two conversions, visits getting more frequent towards the end as a returning shopper's
// do. The schedule is fixed so that fixtures built from it do not churn.
func Month(p persona.Persona) []Visit {
	return []Visit{
		{Persona: p, Ago: 27 * Day},
		{Persona: p, Ago: 20 * Day},
		{Persona: p, Ago: 14 * Day, Convert: true},
		{Persona: p, Ago: 9 * Day},
		{Persona: p, Ago: 5 * Day},
		{Persona: p, Ago: 2 * Day},
		{Persona: p, Convert: true},
	}
}

// History replays visits as one wire.Session each, oldest first, which is the order a
// pipeline expects them in. A visit starts at its scheduled time and gets that time, in
// unix seconds, as its session id — the way GA4 derives one; its requests follow at the
// pace the shopper takes. Time and session id are set after opts, so an option cannot
// break the schedule.
//
// The visits are one shopper's, so the shopper's corpus carries over: the first visit
// starts neutral and every later one is ranked by the vector of everything before it,
// whichever persona happened to be shopping.
func History(visits []Visit, opts ...wire.Option) []wire.Session {
	ordered := slices.Clone(visits)
	slices.SortStableFunc(ordered, func(a, b Visit) int { return cmp.Compare(b.Ago, a.Ago) })

	out := make([]wire.Session, 0, len(ordered))
	s := newShopper()

	for _, v := range ordered {
		at := wire.BaseTime.Add(-v.Ago)
		session := strconv.FormatInt(at.Unix(), 10)
		visitOpts := append(slices.Clone(opts), wire.WithTimestamp(at), wire.WithSessionID(session))

		s.start(wire.NewContext(visitOpts...))
		visit(v.Persona, s, v.Convert)
		out = append(out, s.session(opts...))
	}

	return out
}
