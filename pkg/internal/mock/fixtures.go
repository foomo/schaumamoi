package mock

import (
	"sort"
	"strings"

	"github.com/foomo/schaumamoi/pkg/internal/mock/catalog"
	"github.com/foomo/schaumamoi/pkg/internal/mock/journeys"
	"github.com/foomo/schaumamoi/pkg/internal/mock/persona"
	"github.com/foomo/schaumamoi/pkg/internal/mock/shop"
	"github.com/foomo/schaumamoi/pkg/internal/mock/wire"
	"github.com/foomo/sesamy-go/pkg/encoding/mpv2"
	"github.com/foomo/sesamy-go/pkg/event/params"
	"github.com/foomo/sesamy-go/pkg/sesamy"
)

// --- single event payloads ------------------------------------------------------
//
// Each returns a fully typed envelope, so callers can assert on param fields directly
// rather than decoding through sesamy.Event[any].

func ViewItemListPayload(opts ...wire.Option) mpv2.Payload[shop.ItemListParams] {
	return wire.Payload([]sesamy.Event[shop.ItemListParams]{shop.ViewItemList(shop.CategoryFilter)}, opts...)
}

func AllProductsItemListPayload(opts ...wire.Option) mpv2.Payload[shop.ItemListParams] {
	return wire.Payload([]sesamy.Event[shop.ItemListParams]{shop.ViewItemList(shop.AllProductsFilter)}, opts...)
}

func DressesItemListPayload(opts ...wire.Option) mpv2.Payload[shop.ItemListParams] {
	return wire.Payload([]sesamy.Event[shop.ItemListParams]{shop.ViewItemList(shop.DressesFilter)}, opts...)
}

func TrousersItemListPayload(opts ...wire.Option) mpv2.Payload[shop.ItemListParams] {
	return wire.Payload([]sesamy.Event[shop.ItemListParams]{shop.ViewItemList(shop.TrousersFilter)}, opts...)
}

func FilteredItemListPayload(opts ...wire.Option) mpv2.Payload[shop.ItemListParams] {
	return wire.Payload([]sesamy.Event[shop.ItemListParams]{shop.ViewItemList(shop.NarrowedFilter)}, opts...)
}

// PersonalizedItemListPayload is the persona's section as its first visit reaches it:
// ranked by what the shop learned from the three listings before.
func PersonalizedItemListPayload(p persona.Persona, opts ...wire.Option) mpv2.Payload[shop.ItemListParams] {
	return wire.Payload([]sesamy.Event[shop.ItemListParams]{shop.ViewItemList(journeys.FirstSection(p))}, opts...)
}

func SelectItemPayload(opts ...wire.Option) mpv2.Payload[params.SelectItem[catalog.Item]] {
	return wire.Payload([]sesamy.Event[params.SelectItem[catalog.Item]]{
		shop.SelectItem(shop.NarrowedFilter, persona.Sofia.TopMatch(catalog.All())),
	}, opts...)
}

func ViewItemPayload(opts ...wire.Option) mpv2.Payload[params.ViewItem[catalog.Item]] {
	return wire.Payload([]sesamy.Event[params.ViewItem[catalog.Item]]{
		shop.ViewItem(persona.Sofia.TopMatch(catalog.All())),
	}, opts...)
}

func AddToCartPayload(opts ...wire.Option) mpv2.Payload[params.AddToCart[catalog.Item]] {
	return wire.Payload([]sesamy.Event[params.AddToCart[catalog.Item]]{
		shop.AddToCart(persona.Sofia.TopMatch(catalog.All())),
	}, opts...)
}

// FixtureDir is the directory the generated wire-format fixtures live in, relative to
// this package.
const FixtureDir = "testdata"

// Fixtures returns every mock payload keyed by the file it is written to. It is the
// single source of truth shared by the generator and the golden test, so a new mock
// cannot be added without its fixture following along.
func Fixtures() map[string]any {
	out := map[string]any{
		// single event, fully typed envelopes
		"view_item_list.mpv2.json":              ViewItemListPayload(),
		"view_item_list_all.mpv2.json":          AllProductsItemListPayload(),
		"view_item_list_dresses.mpv2.json":      DressesItemListPayload(),
		"view_item_list_trousers.mpv2.json":     TrousersItemListPayload(),
		"view_item_list_filtered.mpv2.json":     FilteredItemListPayload(),
		"view_item_list_personalized.mpv2.json": PersonalizedItemListPayload(persona.Kate),
		"select_item.mpv2.json":                 SelectItemPayload(),
		"view_item.mpv2.json":                   ViewItemPayload(),
		"add_to_cart.mpv2.json":                 AddToCartPayload(),

		// a shopper who browses and leaves
		"journey_browse_kate.mpv2.json": journeys.Browsing(persona.Kate),
	}

	// one converting session per persona, each with its own client and session id so
	// they read as separate shoppers rather than one person changing their mind
	for _, p := range persona.All() {
		name := strings.ToLower(p.Name)
		out["journey_"+name+".mpv2.json"] = journeys.Buying(p,
			wire.WithClientID(wire.DefaultClientID),
			wire.WithUserID("U-"+strings.ToUpper(p.Name)),
			wire.WithSessionID(wire.DefaultSessionID),
		)

		// and the month that shopper came back over, one session of requests per visit
		out["history_"+name+".mpv2.json"] = journeys.History(journeys.Month(p),
			wire.WithClientID(wire.DefaultClientID),
			wire.WithUserID("U-"+strings.ToUpper(p.Name)),
		)
	}

	return out
}

// FixtureNames returns the fixture file names in a stable order.
func FixtureNames() []string {
	fixtures := Fixtures()
	names := make([]string, 0, len(fixtures))

	for name := range fixtures {
		names = append(names, name)
	}

	sort.Strings(names)

	return names
}
