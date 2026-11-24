package personalization_test

import (
	"testing"

	"github.com/foomo/schaumamoi/pkg/personalization"
	"github.com/stretchr/testify/assert"
)

func TestScore(t *testing.T) {
	t.Parallel()

	const (
		a personalization.Dimension = "a"
		b personalization.Dimension = "b"
		c personalization.Dimension = "c"
	)

	for name, tt := range map[string]struct {
		left, right personalization.Map
		want        personalization.Value
	}{
		"disjoint maps score nothing": {
			left:  personalization.Map{a: 1},
			right: personalization.Map{b: 1},
			want:  0,
		},
		"only shared dimensions count": {
			left:  personalization.Map{a: 0.5, b: 1},
			right: personalization.Map{a: 0.5, c: 1},
			want:  0.25,
		},
		"opposing taste scores negative": {
			left:  personalization.Map{a: 1},
			right: personalization.Map{a: -1},
			want:  -1,
		},
		"empty scores nothing": {
			left:  personalization.Map{},
			right: personalization.Map{a: 1},
			want:  0,
		},
		"nil scores nothing": {
			left:  nil,
			right: personalization.Map{a: 1},
			want:  0,
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			assert.InDelta(t, float64(tt.want), float64(tt.left.Score(tt.right)), 1e-6)
			assert.InDelta(t, float64(tt.want), float64(tt.right.Score(tt.left)), 1e-6,
				"score should be symmetric")
		})
	}
}

// TestScoreStable pins the reason Score sorts its dimensions: float addition is not
// associative and Go randomises map iteration, so summing in map order would make the
// result wobble between runs and flip the order of near-tied products.
func TestScoreStable(t *testing.T) {
	t.Parallel()

	left := personalization.Map{}
	right := personalization.Map{}

	for _, d := range []personalization.Dimension{"a", "b", "c", "d", "e", "f", "g", "h"} {
		left[d] = 0.1
		right[d] = 0.7
	}

	want := left.Score(right)
	for range 100 {
		assert.Equal(t, want, left.Score(right))
	}
}

func TestWeightedScore(t *testing.T) {
	t.Parallel()

	const (
		a personalization.Dimension = "a"
		b personalization.Dimension = "b"
	)

	left, right := personalization.Map{a: 1, b: 2}, personalization.Map{a: 3, b: 4}

	for name, tt := range map[string]struct {
		weights personalization.Map
		want    personalization.Value
	}{
		"unit weights are Score":           {weights: personalization.Map{a: 1, b: 1}, want: left.Score(right)},
		"a weight scales its dimension":    {weights: personalization.Map{a: 2, b: 1}, want: 2*3 + 8},
		"an unnamed dimension counts zero": {weights: personalization.Map{a: 1}, want: 3},
		"no weights score nothing":         {weights: nil, want: 0},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			assert.InDelta(t, float64(tt.want), float64(left.WeightedScore(right, tt.weights)), 1e-6)
			assert.InDelta(t, float64(tt.want), float64(right.WeightedScore(left, tt.weights)), 1e-6, "weighted score must be symmetric")
		})
	}
}
