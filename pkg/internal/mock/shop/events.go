package shop

import (
	"math"

	"github.com/foomo/schaumamoi/pkg/internal/mock/catalog"
	"github.com/foomo/sesamy-go/pkg/event/params"
	"github.com/foomo/sesamy-go/pkg/sesamy"
)

// indexed returns a copy of items with Index stamped to the list position, the way GA4
// reports items that were rendered as part of a list.
func indexed(items []catalog.Item) []catalog.Item {
	out := make([]catalog.Item, len(items))
	for i, item := range items {
		item.Index = i
		out[i] = item
	}

	return out
}

// listed returns a copy of items tagged with the list they were surfaced in.
func listed(listID, listName string, items []catalog.Item) []catalog.Item {

	for i := range items {
		items[i].ItemListID = listID
		items[i].ItemListName = listName
	}

	return items
}

// quantity defaults a missing quantity to a single unit.
func quantity(item catalog.Item) float64 {
	if item.Quantity == 0 {
		return 1
	}

	return item.Quantity
}

// value totals the net line values of items, i.e. (price - discount) * quantity.
func value(items []catalog.Item) float64 {
	var total float64
	for _, item := range items {
		total += (item.Price - item.Discount) * quantity(item)
	}

	return round(total)
}

// round trims a monetary value to two decimals, keeping generated fixtures free of
// floating point noise.
func round(v float64) float64 {
	return math.Round(v*100) / 100
}

// The listings the single event fixtures browse. Every listing the shop renders is
// personalized; these are as a new shopper sees them, ranked by a neutral profile.
// Journeys re-rank them with Personalize as the shop learns.
var (
	// CategoryFilter is the listing a shopper lands on: the whole department.
	CategoryFilter = Filter{Category: catalog.CategoryWomensFashion, Sort: SortAffinity}

	// DressesFilter and TrousersFilter are the two categories below it.
	DressesFilter  = Filter{Category: catalog.CategoryDresses, Sort: SortAffinity}
	TrousersFilter = Filter{Category: catalog.CategoryTrousers, Sort: SortAffinity}

	// NarrowedFilter is the dress listing after the shopper applies facets.
	NarrowedFilter = Filter{Category: catalog.CategoryDresses, Occasion: catalog.OccasionFestive, Length: catalog.LengthMaxi, Sort: SortAffinity}

	// AllProductsFilter is the top of the tree: every product, both departments.
	AllProductsFilter = Filter{Sort: SortAffinity}

	// HomeFilter is the home decoration department, and the three categories below it.
	HomeFilter     = Filter{Category: catalog.CategoryHomeDecoration, Sort: SortAffinity}
	CushionsFilter = Filter{Category: catalog.CategoryCushions, Sort: SortAffinity}
	LampsFilter    = Filter{Category: catalog.CategoryLamps, Sort: SortAffinity}
	VasesFilter    = Filter{Category: catalog.CategoryVases, Sort: SortAffinity}
)

// --- events ---------------------------------------------------------------------
//
// All four go through sesamy.NewEvent rather than the stock event.New* constructors:
// those pin the item type to params.Item, while these events carry catalog.Item - the type
// the enrichment stage decodes back into once attractors are joined on.

// ViewItemList reports the listing the filter selects out of the catalog, along with the
// filter that produced it.
func ViewItemList(filter Filter) sesamy.Event[ItemListParams] {
	items := listed(filter.ListID(), filter.ListName(), filter.Apply(catalog.All()))

	p := ItemListParams{
		ItemListID:     filter.ListID(),
		ItemListName:   filter.ListName(),
		Items:          items,
		FilterCategory: filter.Category,
		FilterOccasion: filter.Occasion,
		FilterLength:   filter.Length,
		FilterColour:   filter.Colour,
		FilterMaxPrice: filter.MaxPrice,
		SortBy:         filter.Sort,
		Personalized:   filter.Personalized(),
		TasteSignals:   filter.Profile.Signals,
		ResultCount:    len(items),
	}

	return sesamy.NewEvent(sesamy.EventNameViewItemList, p)
}

// SelectItem is the shopper opening one entry of a listing.
func SelectItem(filter Filter, items ...catalog.Item) sesamy.Event[params.SelectItem[catalog.Item]] {
	return sesamy.NewEvent(sesamy.EventNameSelectItem, params.SelectItem[catalog.Item]{
		ItemListID:   filter.ListID(),
		ItemListName: filter.ListName(),
		Items:        listed(filter.ListID(), filter.ListName(), items),
	})
}

// ViewItem is a product detail view.
func ViewItem(items ...catalog.Item) sesamy.Event[params.ViewItem[catalog.Item]] {
	return sesamy.NewEvent(sesamy.EventNameViewItem, params.ViewItem[catalog.Item]{
		Currency: catalog.DefaultCurrency,
		Value:    value(items),
		Items:    items,
	})
}

// AddToCart is the shopper putting an item in the cart.
func AddToCart(items ...catalog.Item) sesamy.Event[params.AddToCart[catalog.Item]] {
	return sesamy.NewEvent(sesamy.EventNameAddToCart, params.AddToCart[catalog.Item]{
		Currency: catalog.DefaultCurrency,
		Value:    value(items),
		Items:    items,
	})
}
