package catalog_test

import (
	"testing"

	"github.com/foomo/schaumamoi/pkg/internal/mock/catalog"
	"github.com/foomo/schaumamoi/pkg/internal/mock/taste"
	"github.com/foomo/schaumamoi/pkg/personalization"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCatalog pins the shape of the catalog the rest of the fixtures rest on.
func TestCatalog(t *testing.T) {
	t.Parallel()

	products := catalog.All()
	require.Len(t, products, 54)

	byRail := map[string]int{}
	lengths := map[string][]string{
		catalog.CategoryDresses:  {catalog.LengthMini, catalog.LengthMidi, catalog.LengthMaxi},
		catalog.CategoryTrousers: {catalog.LengthCropped, catalog.LengthAnkle, catalog.LengthFull},
		catalog.CategoryCushions: {""},
		catalog.CategoryLamps:    {""},
		catalog.CategoryVases:    {""},
	}
	department := map[string]string{
		catalog.CategoryDresses: catalog.CategoryWomensFashion, catalog.CategoryTrousers: catalog.CategoryWomensFashion,
		catalog.CategoryCushions: catalog.CategoryHomeDecoration, catalog.CategoryLamps: catalog.CategoryHomeDecoration, catalog.CategoryVases: catalog.CategoryHomeDecoration,
	}

	for _, item := range products {
		assert.NotEmpty(t, item.ItemID)
		assert.NotEmpty(t, item.ItemName)
		assert.NotEmpty(t, item.Attractor, "every product needs a taste vector")
		assert.NotEmpty(t, item.Description, "every product needs a description")
		assert.Equal(t, department[item.Category()], item.ItemCategory, item.ItemID)
		assert.Contains(t, lengths[item.Category()], item.Length(), "%s has a length its category does not cut", item.ItemID)
		assert.NotZero(t, item.Price)
		byRail[item.Category()+" "+item.Occasion()]++
	}

	// six of each occasion on each fashion rail, six of each home category
	assert.Equal(t, map[string]int{
		"Dresses business": 6, "Dresses festive": 6, "Dresses casual": 6,
		"Trousers business": 6, "Trousers festive": 6, "Trousers casual": 6,
		"Cushions ": 6, "Lamps ": 6, "Vases ": 6,
	}, byRail)
}

// TestHomeSpeaksItsOwnDimensions pins how the Vector separates the departments: a home item
// never carries a fashion-only dimension, so what the shop learns from clothes reaches
// the home department only through the dimensions the two share — and every home item
// shares some, or fashion taste could not rank it at all.
func TestHomeSpeaksItsOwnDimensions(t *testing.T) {
	t.Parallel()

	fashionOnly := []personalization.Dimension{
		taste.Business, taste.Festive, taste.Casual,
		taste.Feminine, taste.Structured,
	}
	homeOnly := []personalization.Dimension{taste.Cozy, taste.Natural, taste.Industrial}

	fashion := personalization.Vector{}
	for _, item := range catalog.Fashion() {
		for d := range item.Attractor {
			fashion[d] = 1
		}

		for _, d := range homeOnly {
			assert.NotContains(t, item.Attractor, d, "%s carries the home-only %s", item.ItemID, d)
		}
	}

	for _, item := range catalog.HomeDecoration() {
		for _, d := range fashionOnly {
			assert.NotContains(t, item.Attractor, d, "%s carries the fashion-only %s", item.ItemID, d)
		}

		shared := 0
		for d := range item.Attractor {
			if _, ok := fashion[d]; ok {
				shared++
			}
		}

		assert.NotZero(t, shared, "%s shares no dimension with fashion", item.ItemID)
	}
}
