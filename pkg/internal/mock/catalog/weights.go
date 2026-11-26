package catalog

import (
	"maps"
	"slices"

	"github.com/foomo/schaumamoi/pkg/internal/mock/taste"
	"github.com/foomo/schaumamoi/pkg/personalization"
)

// Every product is scored in the context of its category: a shopper's taste and the
// product's attractor are compared through that category's weights, which say which
// dimensions the category looks at and how much each counts there. Taste stays one Vector
// across the shop — what the shopper opens, views and buys — and the weights decide how
// it applies to dresses, to trousers, to lamps.
//
// The weights are a complete statement: a dimension a category does not name counts for
// nothing there (personalization.Vector.WeightedScore). TestCategoryWeightsAreComplete
// keeps every dimension a category's products carry named in its weights.
//
// They are defined per department and refined per category: a category starts from its
// department's weights and overrides only what differs. Each category's weights are then
// scaled to average 1 over the dimensions they name, so they set emphasis, not scale —
// a perfect match scores on the same scale in every category, and a listing that mixes
// categories (a department, all products) can rank them against each other.

// departmentWeights is what each department looks at.
var departmentWeights = map[string]personalization.Vector{
	// Fashion: the occasion a piece is for comes first, then its style. Colour counts a
	// little less — it is the easiest thing to like in another piece.
	CategoryWomensFashion: {
		taste.Business: 1.3, taste.Festive: 1.3, taste.Casual: 1.3,
		taste.Modern: 1, taste.Classic: 1, taste.Minimalistic: 1,
		taste.Opulent: 1, taste.Feminine: 1, taste.Bohemian: 1,
		taste.Romantic: 1, taste.Structured: 1, taste.Retro: 1,
		taste.Edgy: 1, taste.Bold: 1, taste.Colourful: 0.8,
	},

	// Home: the home dimensions lead — how a room should feel — and the style shared with
	// fashion follows at a little less, the flamboyant end of it the least. The occasion
	// and cut of clothes mean nothing here, so the fashion-only dimensions are not named.
	CategoryHomeDecoration: {
		taste.Cozy: 1.3, taste.Natural: 1.3, taste.Industrial: 1.3,
		taste.Modern: 1, taste.Classic: 1, taste.Minimalistic: 1,
		taste.Bohemian: 1, taste.Opulent: 0.8, taste.Romantic: 0.8,
		taste.Retro: 0.8, taste.Bold: 0.8, taste.Colourful: 0.8,
		taste.Edgy: 0.6,
	},
}

// categoryWeightOverrides is where a category departs from its department.
var categoryWeightOverrides = map[string]personalization.Vector{
	// a dress is chosen for its line: how feminine, how romantic, less for its tailoring
	CategoryDresses: {taste.Feminine: 1.3, taste.Romantic: 1.2, taste.Structured: 0.8},

	// trousers are chosen for their cut: tailoring and edge count more, femininity less
	CategoryTrousers: {taste.Structured: 1.3, taste.Edgy: 1.2, taste.Feminine: 0.7},

	// a cushion is about comfort and texture: cozy and natural lead, the cold end of
	// modern and industrial hardly matters, a little colour and romance do
	CategoryCushions: {
		taste.Cozy: 1.6, taste.Natural: 1.4, taste.Industrial: 0.4,
		taste.Modern: 0.6, taste.Colourful: 1, taste.Romantic: 1,
	},

	// a lamp is a design object: industrial, modern and classic decide it
	CategoryLamps: {taste.Industrial: 1.5, taste.Modern: 1.2, taste.Classic: 1.2, taste.Cozy: 1},

	// a vase is about colour, material and statement
	CategoryVases: {taste.Colourful: 1.3, taste.Natural: 1.3, taste.Bold: 1.1, taste.Cozy: 0.6},
}

// categoryWeights are the resolved weights of every category: department, overrides,
// scaled to average 1.
var categoryWeights = resolveWeights()

func resolveWeights() map[string]personalization.Vector {
	out := map[string]personalization.Vector{}

	for category, department := range categoryParent {
		w := personalization.Vector{}
		for d, v := range departmentWeights[department] {
			w[d] = v
		}

		for d, v := range categoryWeightOverrides[category] {
			w[d] = v
		}

		// summed in sorted dimension order: float addition is not associative and map order
		// is random, so a sum in map order would differ in its last bits from one run to the
		// next, and so would every score.
		dims := slices.Sorted(maps.Keys(w))

		var sum personalization.Value
		for _, d := range dims {
			sum += w[d]
		}

		mean := sum / personalization.Value(len(w))
		for d := range w {
			w[d] /= mean
		}

		out[category] = w
	}

	return out
}

// CategoryWeights are the weights products of a category are scored with. A category
// without weights scores nothing. The vector is a copy; changing it changes nothing.
func CategoryWeights(category string) personalization.Vector {
	out := personalization.Vector{}
	for d, v := range categoryWeights[category] {
		out[d] = v
	}

	return out
}

// Score is how well a taste matches an item in the item's own category: the weighted
// dot product of the taste and the item's attractor under CategoryWeights. It is the one
// scorer the shop ranks with and the personas choose by.
func Score(taste personalization.Vector, item Item) personalization.Value {
	return taste.WeightedScore(item.Attractor, categoryWeights[item.Category()])
}
