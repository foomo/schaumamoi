package affinity_test

import (
	"testing"

	"github.com/foomo/schaumamoi/pkg/distillery/stills/affinity"
	"github.com/foomo/schaumamoi/pkg/personalization"
	"github.com/stretchr/testify/assert"
)

const (
	classic personalization.Dimension = "classic"
	modern  personalization.Dimension = "modern"
	cozy    personalization.Dimension = "cozy"
)

// events builds infused events from their contributions, in order.
func events(contributions ...personalization.Vector) []affinity.Infused {
	out := make([]affinity.Infused, 0, len(contributions))
	for _, c := range contributions {
		out = append(out, affinity.Infused{Infusion: affinity.Infusion{Affinity: c}})
	}

	return out
}

func repeat(m personalization.Vector, n int) []personalization.Vector {
	out := make([]personalization.Vector, n)
	for i := range out {
		out[i] = m
	}

	return out
}

// noShift is a fade without change detection: a cosine never falls below -2.
var noShift = affinity.Fade{PerEvent: 0.5, Recent: 2, Shift: -2, Collapse: 0}

func TestFadePerEvent(t *testing.T) {
	t.Parallel()

	got := noShift.Vector(events(personalization.Vector{classic: 1}, personalization.Vector{classic: 1}, personalization.Vector{classic: 1}))

	// 1·0.25 + 1·0.5 + 1: every event halves everything before it
	assert.InDelta(t, 1.75, float64(got[classic]), 1e-6)
}

// TestFadeCollapsesOnAStrongChange: after a long run of one taste, recent picks pointing
// the opposite way collapse the old vector, so the new taste leads at once.
func TestFadeCollapsesOnAStrongChange(t *testing.T) {
	t.Parallel()

	old, recent := personalization.Vector{classic: 1, modern: -1}, personalization.Vector{classic: -1, modern: 1}
	history := events(append(repeat(old, 10), repeat(recent, 3)...)...)

	f := affinity.Fade{PerEvent: 0.95, Recent: 3, Shift: 0.5, Collapse: 0.2}
	assert.Greater(t, f.Vector(history)[modern], personalization.Value(0), "with change detection the new taste leads")

	f.Shift = -2
	assert.Less(t, f.Vector(history)[modern], personalization.Value(0), "by fading alone the old taste still leads")
}

// TestFadeKeepsTasteOnAGentleChange: recent picks that agree with the vector, only less
// strongly, are no change of intent — nothing collapses.
func TestFadeKeepsTasteOnAGentleChange(t *testing.T) {
	t.Parallel()

	old, recent := personalization.Vector{classic: 1, modern: 0.2}, personalization.Vector{classic: 0.6, modern: 0.4}
	history := events(append(repeat(old, 10), repeat(recent, 3)...)...)

	with := affinity.Fade{PerEvent: 0.95, Recent: 3, Shift: 0.5, Collapse: 0.2}
	without := with
	without.Shift = -2

	assert.Equal(t, without.Vector(history), with.Vector(history))
}

// TestFadeKeepsTasteInANewCategory: moving into dimensions the vector never had — from
// clothes to cushions — is a new category, not a new intent; the taste the two share
// must survive.
func TestFadeKeepsTasteInANewCategory(t *testing.T) {
	t.Parallel()

	clothes, cushions := personalization.Vector{classic: 1}, personalization.Vector{cozy: 1}
	history := events(append(repeat(clothes, 10), repeat(cushions, 3)...)...)

	f := affinity.Fade{PerEvent: 1, Recent: 3, Shift: 0.5, Collapse: 0}
	assert.InDelta(t, 10, float64(f.Vector(history)[classic]), 1e-6)
}

func TestFadeIsRepeatable(t *testing.T) {
	t.Parallel()

	history := events(append(repeat(personalization.Vector{classic: 1, modern: -1}, 7), repeat(personalization.Vector{classic: -1, modern: 1, cozy: 0.3}, 4)...)...)

	want := affinity.DefaultFade.Vector(history)
	for range 10 {
		assert.Equal(t, want, affinity.DefaultFade.Vector(history))
	}
}

// TestTraceEndsOnVector keeps the two views of the loop in step: the last step of a trace
// is the vector, and a step collapses exactly when its cosine falls below Shift.
func TestTraceEndsOnVector(t *testing.T) {
	t.Parallel()

	history := events(append(repeat(personalization.Vector{classic: 1, modern: -1}, 7), repeat(personalization.Vector{classic: -1, modern: 1, cozy: 0.3}, 4)...)...)
	steps := affinity.DefaultFade.Trace(history)

	assert.Len(t, steps, len(history))
	assert.Equal(t, affinity.DefaultFade.Vector(history), steps[len(steps)-1].Vector)

	collapsed := 0
	for _, s := range steps {
		assert.Equal(t, s.Cosine < affinity.DefaultFade.Shift, s.Collapsed)
		if s.Collapsed {
			collapsed++
		}
	}

	assert.Positive(t, collapsed, "the turn from classic to modern is a change of intent")
}
