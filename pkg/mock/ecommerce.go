package mock

import (
	"github.com/foomo/sesamy-go/pkg/encoding/mpv2"
	"github.com/foomo/sesamy-go/pkg/event/params"
	"github.com/foomo/sesamy-go/pkg/sesamy"
)

// The listings the single event fixtures browse. Every listing the shop renders is
// personalized; these are as a new shopper sees them, ranked by a neutral profile.
// Journeys re-rank them with Personalize as the shop learns.
var (
	// CategoryFilter is the listing a shopper lands on: the whole department.
	CategoryFilter = Filter{Category: CategoryWomensFashion, Sort: SortAffinity}

	// DressesFilter and TrousersFilter are the two categories below it.
	DressesFilter  = Filter{Category: CategoryDresses, Sort: SortAffinity}
	TrousersFilter = Filter{Category: CategoryTrousers, Sort: SortAffinity}

	// NarrowedFilter is the dress listing after the shopper applies facets.
	NarrowedFilter = Filter{Category: CategoryDresses, Occasion: OccasionFestive, Length: LengthMaxi, Sort: SortAffinity}

	// AllProductsFilter is the top of the tree: every product, both departments.
	AllProductsFilter = Filter{Sort: SortAffinity}

	// HomeFilter is the home decoration department, and the three categories below it.
	HomeFilter     = Filter{Category: CategoryHomeDecoration, Sort: SortAffinity}
	CushionsFilter = Filter{Category: CategoryCushions, Sort: SortAffinity}
	LampsFilter    = Filter{Category: CategoryLamps, Sort: SortAffinity}
	VasesFilter    = Filter{Category: CategoryVases, Sort: SortAffinity}
)

// SectionFilter is the persona's occasion across the whole department, dresses and
// trousers together: the facet the shopper narrows to. How it is ranked is the shop's
// business, set with Personalize.
func SectionFilter(p Persona) Filter {
	return Filter{Category: CategoryWomensFashion, Occasion: p.Occasion(), Sort: SortAffinity}
}

// --- events ---------------------------------------------------------------------
//
// All four go through sesamy.NewEvent rather than the stock event.New* constructors:
// those pin the item type to params.Item, while these events carry mock.Item - the type
// the enrichment stage decodes back into once attractors are joined on.

// ViewItemList reports the listing the filter selects out of the catalog, along with the
// filter that produced it.
func ViewItemList(filter Filter) sesamy.Event[ItemListParams] {
	items := listed(filter.ListID(), filter.ListName(), filter.Apply(Catalog()))

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
func SelectItem(filter Filter, items ...Item) sesamy.Event[params.SelectItem[Item]] {
	return sesamy.NewEvent(sesamy.EventNameSelectItem, params.SelectItem[Item]{
		ItemListID:   filter.ListID(),
		ItemListName: filter.ListName(),
		Items:        listed(filter.ListID(), filter.ListName(), items),
	})
}

// ViewItem is a product detail view.
func ViewItem(items ...Item) sesamy.Event[params.ViewItem[Item]] {
	return sesamy.NewEvent(sesamy.EventNameViewItem, params.ViewItem[Item]{
		Currency: DefaultCurrency,
		Value:    value(items),
		Items:    items,
	})
}

// AddToCart is the shopper putting an item in the cart.
func AddToCart(items ...Item) sesamy.Event[params.AddToCart[Item]] {
	return sesamy.NewEvent(sesamy.EventNameAddToCart, params.AddToCart[Item]{
		Currency: DefaultCurrency,
		Value:    value(items),
		Items:    items,
	})
}

// --- single event payloads ------------------------------------------------------
//
// Each returns a fully typed envelope, so callers can assert on param fields directly
// rather than decoding through sesamy.Event[any].

func ViewItemListPayload(opts ...Option) mpv2.Payload[ItemListParams] {
	return Payload([]sesamy.Event[ItemListParams]{ViewItemList(CategoryFilter)}, opts...)
}

func AllProductsItemListPayload(opts ...Option) mpv2.Payload[ItemListParams] {
	return Payload([]sesamy.Event[ItemListParams]{ViewItemList(AllProductsFilter)}, opts...)
}

func DressesItemListPayload(opts ...Option) mpv2.Payload[ItemListParams] {
	return Payload([]sesamy.Event[ItemListParams]{ViewItemList(DressesFilter)}, opts...)
}

func TrousersItemListPayload(opts ...Option) mpv2.Payload[ItemListParams] {
	return Payload([]sesamy.Event[ItemListParams]{ViewItemList(TrousersFilter)}, opts...)
}

func FilteredItemListPayload(opts ...Option) mpv2.Payload[ItemListParams] {
	return Payload([]sesamy.Event[ItemListParams]{ViewItemList(NarrowedFilter)}, opts...)
}

// PersonalizedItemListPayload is the persona's section as its first visit reaches it:
// ranked by what the shop learned from the three listings before.
func PersonalizedItemListPayload(p Persona, opts ...Option) mpv2.Payload[ItemListParams] {
	stops, _ := route(p, Profile{})

	return Payload([]sesamy.Event[ItemListParams]{ViewItemList(stops[len(stops)-1].Filter)}, opts...)
}

func SelectItemPayload(opts ...Option) mpv2.Payload[params.SelectItem[Item]] {
	return Payload([]sesamy.Event[params.SelectItem[Item]]{
		SelectItem(NarrowedFilter, PersonaFestiveSofia.TopMatch(Catalog())),
	}, opts...)
}

func ViewItemPayload(opts ...Option) mpv2.Payload[params.ViewItem[Item]] {
	return Payload([]sesamy.Event[params.ViewItem[Item]]{
		ViewItem(PersonaFestiveSofia.TopMatch(Catalog())),
	}, opts...)
}

func AddToCartPayload(opts ...Option) mpv2.Payload[params.AddToCart[Item]] {
	return Payload([]sesamy.Event[params.AddToCart[Item]]{
		AddToCart(PersonaFestiveSofia.TopMatch(Catalog())),
	}, opts...)
}
