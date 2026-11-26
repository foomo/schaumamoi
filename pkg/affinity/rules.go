package affinity

import (
	"time"

	"github.com/foomo/schaumamoi/pkg/distillery"
	"github.com/foomo/schaumamoi/pkg/distillery/stamp"
	"github.com/foomo/sesamy-go/pkg/sesamy"
)

// DefaultKeep is how many choices an affinity still holds: the newest thousand. A visit
// has about ten, and with DefaultFade every choice fades the ones before it by 0.95, so
// the thousandth counts for nothing long before it is drawn off — the bound is on the
// work and the memory, not on the vector.
const DefaultKeep = 1000

// KeepLast draws off all but the newest n choices.
func KeepLast(n int) Rule { return distillery.KeepLast[any, Infusion](n) }

// Named draws off the choices with one of the given names: views, say, which a policy
// may forget sooner than carts.
func Named(names ...sesamy.EventName) Rule { return distillery.Named[any, Infusion](names...) }

// OlderThan draws off the choices made more than age before now: stamp.OlderThan for an
// affinity still. A choice without a time counts as infinitely old.
func OlderThan(now time.Time, age time.Duration) Rule {
	return stamp.OlderThan[any, Infusion](now, age)
}

// InSession draws off the choices of one session: a visit the shopper asked to have
// forgotten. It is stamp.InSession for an affinity still.
func InSession(id string) Rule { return stamp.InSession[any, Infusion](id) }
