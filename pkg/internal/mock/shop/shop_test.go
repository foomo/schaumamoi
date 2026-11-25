package shop_test

import (
	"testing"

	"github.com/foomo/schaumamoi/pkg/internal/mock/catalog"
	"github.com/foomo/schaumamoi/pkg/internal/mock/persona"
	"github.com/foomo/schaumamoi/pkg/internal/mock/shop"
	"github.com/foomo/schaumamoi/pkg/internal/mock/shopper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestValueMatchesItems is the property that makes these mocks usable as aggregation
// input: an event must never report a total that contradicts its own line items.
func TestValueMatchesItems(t *testing.T) {
	t.Parallel()

	products := catalog.All()
	e := shop.AddToCart(products[0], products[4])

	var want float64
	for _, item := range e.Params.Items {
		want += (item.Price - item.Discount) * item.Quantity
	}

	assert.InDelta(t, want, e.Params.Value, 0.01)
}

// TestCategoryTree pins the listings the tree gives: the department lists everything,
// each category its own rail, and each list id is the category's path through the tree.
func TestCategoryTree(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		filter shop.Filter
		listID string
		want   []catalog.Item
	}{
		{shop.CategoryFilter, "womens-fashion", catalog.Fashion()},
		{shop.DressesFilter, "womens-fashion/dresses", catalog.Dresses()},
		{shop.TrousersFilter, "womens-fashion/trousers", catalog.Trousers()},
		{shop.HomeFilter, "home-decoration", catalog.HomeDecoration()},
		{shop.CushionsFilter, "home-decoration/cushions", inCategory(catalog.HomeDecoration(), catalog.CategoryCushions)},
		{shop.LampsFilter, "home-decoration/lamps", inCategory(catalog.HomeDecoration(), catalog.CategoryLamps)},
		{shop.VasesFilter, "home-decoration/vases", inCategory(catalog.HomeDecoration(), catalog.CategoryVases)},
	} {
		t.Run(tt.listID, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.listID, tt.filter.ListID())
			assert.Equal(t, tt.filter.Category, tt.filter.ListName())
			assert.Equal(t, itemIDs(tt.want), itemIDs(tt.filter.Apply(catalog.All())))
		})
	}

	assert.Len(t, catalog.Dresses(), 18)
	assert.Len(t, catalog.Trousers(), 18)
}

// TestAffinitySort covers the ranking itself: best match first, and stable across runs
// even when scores tie.
func TestAffinitySort(t *testing.T) {
	t.Parallel()

	// a profile that has learned Mira's taste exactly
	f := shopper.SectionFilter(persona.Mira).Personalize(shop.Profile{Taste: persona.Mira.Affinity, Signals: 1})
	got := f.Apply(catalog.All())

	require.NotEmpty(t, got)

	// The ranking in readable form, so -v says what the order means rather than which
	// ids it happened to produce.
	t.Logf("ranked for %s, %s:", persona.Mira.Name, persona.Mira.Description)

	for i, item := range got {
		t.Logf("  %d. %-26s %s", i+1, item.ItemName, item.Description)
	}

	assert.Equal(t, "TROUSERS_02", got[0].ItemID, "best match should rank first")

	for _, item := range got {
		assert.Equal(t, catalog.OccasionBusiness, item.Occasion())
	}

	want := itemIDs(got)
	for range 20 {
		assert.Equal(t, want, itemIDs(f.Apply(catalog.All())))
	}
}

// TestNeutralProfileKeepsCatalogOrder is where every shopper starts: a profile that has
// learned nothing scores every product the same, and the listing falls back to the
// catalog order rather than to anything arbitrary.
func TestNeutralProfileKeepsCatalogOrder(t *testing.T) {
	t.Parallel()

	got := shop.Filter{Category: catalog.CategoryWomensFashion, Sort: shop.SortAffinity}.Apply(catalog.All())
	assert.Equal(t, itemIDs(catalog.Fashion()), itemIDs(got))
}

// TestFilterApply pins that a listing is genuinely the output of its filter, rather than
// a hand-picked set that could contradict the facets it reports.
func TestFilterApply(t *testing.T) {
	t.Parallel()

	broad := shop.ViewItemList(shop.CategoryFilter)
	narrow := shop.ViewItemList(shop.NarrowedFilter)

	assert.Equal(t,
		itemIDs(shop.NarrowedFilter.Apply(catalog.All())),
		itemIDs(narrow.Params.Items),
	)

	assert.Equal(t, len(narrow.Params.Items), narrow.Params.ResultCount)
	assert.Subset(t, itemIDs(broad.Params.Items), itemIDs(narrow.Params.Items))
	assert.Less(t, len(narrow.Params.Items), len(broad.Params.Items),
		"the facets should actually narrow the listing")

	for _, item := range narrow.Params.Items {
		assert.Equal(t, shop.NarrowedFilter.Occasion, item.Occasion())
		assert.Equal(t, shop.NarrowedFilter.Length, item.Length())
	}
}

func TestFilterQuery(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "length=maxi&occasion=festive&sort=affinity", shop.NarrowedFilter.Query())
	assert.Equal(t, "sort=affinity", shop.CategoryFilter.Query())
	assert.Equal(t, "occasion=business&sort=affinity", shopper.SectionFilter(persona.Kate).Query())
}

// inCategory is the items of one category, in their order.
func inCategory(items []catalog.Item, category string) []catalog.Item {
	var out []catalog.Item

	for _, item := range items {
		if item.Category() == category {
			out = append(out, item)
		}
	}

	return out
}

func itemIDs(items []catalog.Item) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, item.ItemID)
	}

	return out
}
