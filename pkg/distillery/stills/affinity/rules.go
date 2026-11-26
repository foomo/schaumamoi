package affinity

import "github.com/foomo/schaumamoi/pkg/distillery/stamp"

// Rules builds the draw-off rules for an affinity still: KeepLast, Named, Where and
// their combinators Any, All and Not, and — since a choice is stamped — OlderThan and
// InSession.
var Rules stamp.Rules[any, Infusion]

// DefaultKeep is how many choices an affinity still holds: the newest thousand. A visit
// has about ten, and with DefaultFade every choice fades the ones before it by 0.95, so
// the thousandth counts for nothing long before it is drawn off — the bound is on the
// work and the memory, not on the vector.
const DefaultKeep = 1000
