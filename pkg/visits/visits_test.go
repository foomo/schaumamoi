package visits_test

import (
	"testing"
	"time"

	"github.com/foomo/schaumamoi/pkg/distillery"
	"github.com/foomo/schaumamoi/pkg/distillery/stamp"
	"github.com/foomo/schaumamoi/pkg/personalization"
	"github.com/foomo/schaumamoi/pkg/visits"
	"github.com/stretchr/testify/assert"
)

// infusion is the plainest stamped infusion with a taste.
type infusion struct {
	stamp.Stamp

	Taste personalization.Map
}

type event = distillery.Infused[any, infusion]

var start = time.Date(2026, time.September, 1, 10, 0, 0, 0, time.UTC)

// at is an event in session s, minute m, with the given taste.
func at(s string, m int, taste personalization.Map) event {
	return event{Infusion: infusion{Stamp: stamp.Stamp{At: start.Add(time.Duration(m) * time.Minute), Session: s}, Taste: taste}}
}

var (
	classic = personalization.Map{"classic": 1}
	modern  = personalization.Map{"modern": 1}
	phases  = visits.Phases[any, infusion](0.9, func(i infusion) personalization.Map { return i.Taste })
)

func TestOfSplitsOnSessionChange(t *testing.T) {
	t.Parallel()

	got := visits.Of([]event{at("a", 0, nil), at("a", 1, nil), at("b", 5, nil), at("a", 9, nil)})

	var sessions []string
	for _, v := range got {
		sessions = append(sessions, v.Session)
	}

	assert.Equal(t, []string{"a", "b", "a"}, sessions, "a session that comes back is a new visit")
	assert.Equal(t, start, got[0].From)
	assert.Equal(t, start.Add(time.Minute), got[0].To)
	assert.Len(t, got[0].Events, 2)
	assert.Empty(t, visits.Of([]event{}))
}

func TestPhasesMergeSteadyVisits(t *testing.T) {
	t.Parallel()

	got := phases([]event{at("a", 0, classic), at("b", 10, classic), at("c", 20, modern), at("d", 30, modern)})

	assert.Len(t, got, 2)
	assert.Len(t, got[0].Visits, 2)
	assert.Equal(t, personalization.Map{"classic": 2}, got[0].Taste)
	assert.Equal(t, start.Add(10*time.Minute), got[0].To)
	assert.Equal(t, start.Add(20*time.Minute), got[1].From)
}

func TestVisitWithoutTasteJoinsItsPhase(t *testing.T) {
	t.Parallel()

	got := phases([]event{at("a", 0, classic), at("b", 10, nil), at("c", 20, classic)})

	assert.Len(t, got, 1, "a visit that says nothing about taste does not split a phase")
	assert.Len(t, got[0].Visits, 3)
}

func TestPhasesAreContiguous(t *testing.T) {
	t.Parallel()

	got := phases([]event{at("a", 0, classic), at("b", 10, modern), at("c", 20, classic)})

	assert.Len(t, got, 3, "a return is a new phase, not a phase with a hole in it")
}
