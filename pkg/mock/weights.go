package mock

import "github.com/foomo/schaumamoi/pkg/personalization"

// Every product is scored in the context of its category: a shopper's taste and the
// product's attractor are compared through that category's weights, which say which
// dimensions the category looks at and how much each counts there. Taste stays one Map
// across the shop — what the shopper opens, views and buys — and the weights decide how
// it applies to dresses, to trousers, to lamps.
//
// The weights are a complete statement: a dimension a category does not name counts for
// nothing there (personalization.Map.WeightedScore). TestCategoryWeightsAreComplete
// keeps every dimension a category's products carry named in its weights.
//
// They are defined per department and refined per category: a category starts from its
// department's weights and overrides only what differs. Each category's weights are then
// scaled to average 1 over the dimensions they name, so they set emphasis, not scale —
// a perfect match scores on the same scale in every category, and a listing that mixes
// categories (a department, all products) can rank them against each other.

// departmentWeights is what each department looks at.
var departmentWeights = map[string]personalization.Map{
	// Fashion: the occasion a piece is for comes first, then its style. Colour counts a
	// little less — it is the easiest thing to like in another piece.
	CategoryWomensFashion: {
		PersoDimensionBusiness: 1.3, PersoDimensionFestive: 1.3, PersoDimensionCasual: 1.3,
		PersoDimensionModern: 1, PersoDimensionClassic: 1, PersoDimensionMinimalistic: 1,
		PersoDimensionOpulent: 1, PersoDimensionFeminine: 1, PersoDimensionBohemian: 1,
		PersoDimensionRomantic: 1, PersoDimensionStructured: 1, PersoDimensionRetro: 1,
		PersoDimensionEdgy: 1, PersoDimensionBold: 1, PersoDimensionColourful: 0.8,
	},

	// Home: the home dimensions lead — how a room should feel — and the style shared with
	// fashion follows at a little less, the flamboyant end of it the least. The occasion
	// and cut of clothes mean nothing here, so the fashion-only dimensions are not named.
	CategoryHomeDecoration: {
		PersoDimensionCozy: 1.3, PersoDimensionNatural: 1.3, PersoDimensionIndustrial: 1.3,
		PersoDimensionModern: 1, PersoDimensionClassic: 1, PersoDimensionMinimalistic: 1,
		PersoDimensionBohemian: 1, PersoDimensionOpulent: 0.8, PersoDimensionRomantic: 0.8,
		PersoDimensionRetro: 0.8, PersoDimensionBold: 0.8, PersoDimensionColourful: 0.8,
		PersoDimensionEdgy: 0.6,
	},
}

// categoryWeightOverrides is where a category departs from its department.
var categoryWeightOverrides = map[string]personalization.Map{
	// a dress is chosen for its line: how feminine, how romantic, less for its tailoring
	CategoryDresses: {PersoDimensionFeminine: 1.3, PersoDimensionRomantic: 1.2, PersoDimensionStructured: 0.8},

	// trousers are chosen for their cut: tailoring and edge count more, femininity less
	CategoryTrousers: {PersoDimensionStructured: 1.3, PersoDimensionEdgy: 1.2, PersoDimensionFeminine: 0.7},

	// a cushion is about comfort and texture: cozy and natural lead, the cold end of
	// modern and industrial hardly matters, a little colour and romance do
	CategoryCushions: {
		PersoDimensionCozy: 1.6, PersoDimensionNatural: 1.4, PersoDimensionIndustrial: 0.4,
		PersoDimensionModern: 0.6, PersoDimensionColourful: 1, PersoDimensionRomantic: 1,
	},

	// a lamp is a design object: industrial, modern and classic decide it
	CategoryLamps: {PersoDimensionIndustrial: 1.5, PersoDimensionModern: 1.2, PersoDimensionClassic: 1.2, PersoDimensionCozy: 1},

	// a vase is about colour, material and statement
	CategoryVases: {PersoDimensionColourful: 1.3, PersoDimensionNatural: 1.3, PersoDimensionBold: 1.1, PersoDimensionCozy: 0.6},
}

// categoryWeights are the resolved weights of every category: department, overrides,
// scaled to average 1.
var categoryWeights = resolveWeights()

func resolveWeights() map[string]personalization.Map {
	out := map[string]personalization.Map{}

	for category, department := range categoryParent {
		w := personalization.Map{}
		for d, v := range departmentWeights[department] {
			w[d] = v
		}

		for d, v := range categoryWeightOverrides[category] {
			w[d] = v
		}

		var sum personalization.Value
		for _, v := range w {
			sum += v
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
// without weights scores nothing. The map is a copy; changing it changes nothing.
func CategoryWeights(category string) personalization.Map {
	out := personalization.Map{}
	for d, v := range categoryWeights[category] {
		out[d] = v
	}

	return out
}

// Score is how well a taste matches an item in the item's own category: the weighted
// dot product of the taste and the item's attractor under CategoryWeights. It is the one
// scorer the shop ranks with and the personas choose by.
func Score(taste personalization.Map, item Item) personalization.Value {
	return taste.WeightedScore(item.Attractor, categoryWeights[item.Category()])
}
