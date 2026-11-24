package personalization

import "sort"

// Score is the sparse dot product of two affinity maps: a dimension missing from either
// side contributes nothing, so the maps only have to name the dimensions they care
// about.
//
// Dimensions are summed in sorted order. Float addition is not associative and Go
// randomises map iteration, so summing in map order would make the result wobble in the
// last bits between runs — enough to flip the order of two near-tied products.
func (m Map) Score(other Map) Value {
	shared := make([]Dimension, 0, len(m))

	for dimension := range m {
		if _, ok := other[dimension]; ok {
			shared = append(shared, dimension)
		}
	}

	sort.Slice(shared, func(i, j int) bool { return shared[i] < shared[j] })

	var total Value
	for _, dimension := range shared {
		total += m[dimension] * other[dimension]
	}

	return total
}

// WeightedScore is Score seen through a context: each shared dimension counts
// weights[dimension] times. A dimension the weights do not name counts for nothing, so
// the weights are a complete statement of what the context looks at — the two maps can
// share a dimension and still not score on it.
//
// Like Score, it sums in sorted dimension order, so the result is stable to the last bit.
func (m Map) WeightedScore(other, weights Map) Value {
	shared := make([]Dimension, 0, len(m))

	for dimension := range m {
		if _, ok := other[dimension]; ok {
			shared = append(shared, dimension)
		}
	}

	sort.Slice(shared, func(i, j int) bool { return shared[i] < shared[j] })

	var total Value
	for _, dimension := range shared {
		total += weights[dimension] * m[dimension] * other[dimension]
	}

	return total
}
