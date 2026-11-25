package catalog_test

import (
	"testing"

	"github.com/foomo/schaumamoi/pkg/internal/mock/catalog"
	"github.com/foomo/schaumamoi/pkg/internal/mock/taste"
	"github.com/foomo/schaumamoi/pkg/personalization"
	"github.com/stretchr/testify/assert"
)

var categories = []string{catalog.CategoryDresses, catalog.CategoryTrousers, catalog.CategoryCushions, catalog.CategoryLamps, catalog.CategoryVases}

// TestCategoryWeightsAreComplete keeps the weights a complete statement: every dimension
// a product carries is named in its category's weights. A dimension the weights left out
// would silently count for nothing — the opposite of a definition.
func TestCategoryWeightsAreComplete(t *testing.T) {
	t.Parallel()

	for _, item := range catalog.All() {
		w := catalog.CategoryWeights(item.Category())
		for d := range item.Attractor {
			assert.Contains(t, w, d, "%s carries %s, which %s does not weigh", item.ItemID, d, item.Category())
		}
	}
}

// TestCategoryWeightsAverageOne pins the scaling: every category's weights average 1 over
// the dimensions they name, so they set emphasis and not scale, and a listing that mixes
// categories ranks them on one scale.
func TestCategoryWeightsAverageOne(t *testing.T) {
	t.Parallel()

	for _, category := range categories {
		w := catalog.CategoryWeights(category)

		var sum personalization.Value
		for _, v := range w {
			assert.Positive(t, v, category)
			sum += v
		}

		assert.InDelta(t, 1, float64(sum)/float64(len(w)), 1e-5, category)
	}
}

// TestFashionOnlyDimensionsDoNotCountAtHome is what the home weights say by leaving them
// out: a shopper's business or festive taste, learned from clothes, does not rank a lamp.
func TestFashionOnlyDimensionsDoNotCountAtHome(t *testing.T) {
	t.Parallel()

	liked := personalization.Map{taste.Business: 1, taste.Festive: 1}
	for _, item := range catalog.HomeDecoration() {
		withBusiness := personalization.Map{}
		for d, v := range item.Attractor {
			withBusiness[d] = v
		}

		withBusiness[taste.Business] = 1 // even if a home item carried it

		item.Attractor = withBusiness
		assert.Zero(t, catalog.Score(liked, item), item.ItemID)
	}
}
