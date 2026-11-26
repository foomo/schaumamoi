package affinity_test

import (
	"fmt"

	"github.com/foomo/schaumamoi/pkg/distillery/stills/affinity"
	"github.com/foomo/schaumamoi/pkg/internal/mock/journeys"
	"github.com/foomo/schaumamoi/pkg/internal/mock/persona"
	"github.com/foomo/schaumamoi/pkg/personalization"
)

// A shopper's month, fed into an affinity still request by request, distilled two ways,
// and one visit drawn off on request.
func Example() {
	still := affinity.New(nil)

	month := journeys.History(journeys.Month(persona.Kate))
	for _, visit := range month {
		for _, request := range visit {
			env := affinity.Envelope(request) // a sesamy-go payload, as it comes off the stream
			still.Feed(env, request.Events...)
		}
	}

	vector := still.Distill(affinity.DefaultFade.Vector)
	phases := still.Distill(affinity.Phases(affinity.DefaultPhaseThreshold))

	fmt.Printf("fed %d visits, kept %d choices\n", len(month), still.Len())
	fmt.Printf("the vector leads with %s, the month is %d phase\n", strongest(vector), len(phases))

	drawn := still.DrawOff(affinity.Rules.InSession(month[0].ID()))
	fmt.Printf("forgetting the first visit draws off %d choices\n", drawn)

	// Output:
	// fed 7 visits, kept 65 choices
	// the vector leads with classic, the month is 1 phase
	// forgetting the first visit draws off 9 choices
}

// strongest is the dimension a vector scores highest on, ties by name.
func strongest(m personalization.Vector) personalization.Dimension {
	var (
		best personalization.Dimension
		top  personalization.Value
	)

	for d, v := range m {
		if best == "" || v > top || (v == top && d < best) {
			best, top = d, v
		}
	}

	return best
}
