package mock

import (
	"github.com/foomo/sesamy-go/pkg/encoding/mpv2"
	"github.com/foomo/sesamy-go/pkg/event/params"
	"github.com/foomo/sesamy-go/pkg/sesamy"
)

// The listings the single event fixtures browse.
var (
	// CategoryFilter is the plain listing a shopper lands on.
	CategoryFilter = Filter{Category: CategoryDresses}

	// NarrowedFilter is the same listing after the shopper applies facets.
	NarrowedFilter = Filter{Category: CategoryDresses, Occasion: OccasionFestive, Length: LengthMaxi}
)

// PersonalizedFilter is the persona's section of the shop, ranked by its taste.
func PersonalizedFilter(p Persona) Filter {
	return Filter{
		Category: CategoryDresses,
		Occasion: p.Occasion(),
		Sort:     SortAffinity,
		Persona:  &p,
	}
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
		ResultCount:    len(items),
	}

	if filter.Personalized() {
		p.Persona = filter.Persona.Name
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

// AddToCart is the shopper putting a dress in the cart.
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

func FilteredItemListPayload(opts ...Option) mpv2.Payload[ItemListParams] {
	return Payload([]sesamy.Event[ItemListParams]{ViewItemList(NarrowedFilter)}, opts...)
}

func PersonalizedItemListPayload(p Persona, opts ...Option) mpv2.Payload[ItemListParams] {
	return Payload([]sesamy.Event[ItemListParams]{ViewItemList(PersonalizedFilter(p))}, opts...)
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
