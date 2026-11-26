package personalization

import (
	"math"
	"sort"
)

// Score is the sparse dot product of two vectors: a dimension missing from either
// side contributes nothing, so the vectors only have to name the dimensions they care
// about.
//
// Dimensions are summed in sorted order. Float addition is not associative and Go
// randomises map iteration, so summing in map order would make the result wobble in the
// last bits between runs — enough to flip the order of two near-tied products.
func (v Vector) Score(other Vector) Value {
	shared := make([]Dimension, 0, len(v))

	for dimension := range v {
		if _, ok := other[dimension]; ok {
			shared = append(shared, dimension)
		}
	}

	sort.Slice(shared, func(i, j int) bool { return shared[i] < shared[j] })

	var total Value
	for _, dimension := range shared {
		total += v[dimension] * other[dimension]
	}

	return total
}

// WeightedScore is Score seen through a context: each shared dimension counts
// weights[dimension] times. A dimension the weights do not name counts for nothing, so
// the weights are a complete statement of what the context looks at — the two vectors can
// share a dimension and still not score on it.
//
// Like Score, it sums in sorted dimension order, so the result is stable to the last bit.
func (v Vector) WeightedScore(other, weights Vector) Value {
	shared := make([]Dimension, 0, len(v))

	for dimension := range v {
		if _, ok := other[dimension]; ok {
			shared = append(shared, dimension)
		}
	}

	sort.Slice(shared, func(i, j int) bool { return shared[i] < shared[j] })

	var total Value
	for _, dimension := range shared {
		total += weights[dimension] * v[dimension] * other[dimension]
	}

	return total
}

// Cosine is the similarity of two vectors independent of their magnitude: a shopper seen
// for six events and one seen for sixty compare on equal terms. It is 1 for the same
// direction, -1 for the opposite one, and 0 when they share nothing or either is empty.
// Built on Score, it is just as stable.
func (v Vector) Cosine(other Vector) Value {
	norm := float64(v.Score(v)) * float64(other.Score(other))
	if norm == 0 {
		return 0
	}

	return Value(float64(v.Score(other)) / math.Sqrt(norm))
}
