package affinity

import (
	"time"

	"github.com/foomo/schaumamoi/pkg/distillery"
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

// OlderThan draws off the choices made more than age before now. Now is handed in, not
// read off the clock: as the standing policy it is the time of the batch being fed. A
// choice without a time counts as infinitely old — it would weigh nothing when decayed
// either.
func OlderThan(now time.Time, age time.Duration) Rule {
	cutoff := now.Add(-age)

	return distillery.Where(func(e Infused) bool {
		return e.Infusion.At.IsZero() || e.Infusion.At.Before(cutoff)
	})
}

// InSession draws off the choices of one session: a visit the shopper asked to have
// forgotten.
func InSession(id string) Rule {
	return distillery.Where(func(e Infused) bool {
		return e.Infusion.Session == id
	})
}
