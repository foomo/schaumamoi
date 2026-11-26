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
		left, right personalization.Vector
		want        personalization.Value
	}{
		"disjoint maps score nothing": {
			left:  personalization.Vector{a: 1},
			right: personalization.Vector{b: 1},
			want:  0,
		},
		"only shared dimensions count": {
			left:  personalization.Vector{a: 0.5, b: 1},
			right: personalization.Vector{a: 0.5, c: 1},
			want:  0.25,
		},
		"opposing taste scores negative": {
			left:  personalization.Vector{a: 1},
			right: personalization.Vector{a: -1},
			want:  -1,
		},
		"empty scores nothing": {
			left:  personalization.Vector{},
			right: personalization.Vector{a: 1},
			want:  0,
		},
		"nil scores nothing": {
			left:  nil,
			right: personalization.Vector{a: 1},
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

	left := personalization.Vector{}
	right := personalization.Vector{}

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

	left, right := personalization.Vector{a: 1, b: 2}, personalization.Vector{a: 3, b: 4}

	for name, tt := range map[string]struct {
		weights personalization.Vector
		want    personalization.Value
	}{
		"unit weights are Score":           {weights: personalization.Vector{a: 1, b: 1}, want: left.Score(right)},
		"a weight scales its dimension":    {weights: personalization.Vector{a: 2, b: 1}, want: 2*3 + 8},
		"an unnamed dimension counts zero": {weights: personalization.Vector{a: 1}, want: 3},
		"no weights score nothing":         {weights: nil, want: 0},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			assert.InDelta(t, float64(tt.want), float64(left.WeightedScore(right, tt.weights)), 1e-6)
			assert.InDelta(t, float64(tt.want), float64(right.WeightedScore(left, tt.weights)), 1e-6, "weighted score must be symmetric")
		})
	}
}

func TestCosine(t *testing.T) {
	t.Parallel()

	a := personalization.Vector{"classic": 1, "modern": -0.5}

	tests := map[string]struct {
		other personalization.Vector
		want  float64
	}{
		"itself":                  {a, 1},
		"scaled, same direction":  {personalization.Vector{"classic": 10, "modern": -5}, 1},
		"opposite":                {personalization.Vector{"classic": -2, "modern": 1}, -1},
		"nothing shared":          {personalization.Vector{"cozy": 1}, 0},
		"empty":                   {personalization.Vector{}, 0},
		"half way, one dimension": {personalization.Vector{"classic": 1}, 1 / 1.118034},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			assert.InDelta(t, tt.want, float64(a.Cosine(tt.other)), 1e-6)
		})
	}
}
