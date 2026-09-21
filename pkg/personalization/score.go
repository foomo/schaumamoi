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
