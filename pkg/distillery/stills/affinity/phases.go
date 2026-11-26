package affinity

import (
	"github.com/foomo/schaumamoi/pkg/distillery/recipes/visits"
	"github.com/foomo/schaumamoi/pkg/personalization"
)

// DefaultPhaseThreshold is the similarity a visit's taste must reach against the phase
// so far to belong to it. Across every persona pair of the test shop, a visit within a
// phase scores at least 0.94 and the first visit after a drift at most 0.63 — Mira to
// Lena, who share a minimal, modern taste, is the close call — so 0.9 sits in the gap,
// nearer its top. A visit is only its choices; the listings it scrolled past never get
// past the cut. The within-phase side is the tight one: a shopper does not open the same
// products every visit (Sofia drifts between her sequin and her tulle dress), so even one
// taste's visits differ.
const DefaultPhaseThreshold personalization.Value = 0.9

// Phase is a run of a shopper's visits of steady taste.
type Phase = visits.Phase[any, Infusion]

// Phases is the recipe that cuts a shopper's history into phases of steady taste:
// visits.Phases, judging each visit by the taste its choices add up to.
func Phases(threshold personalization.Value) Recipe[[]Phase] {
	return visits.Phases[any, Infusion](threshold, func(i Infusion) personalization.Map { return i.Affinity })
}
