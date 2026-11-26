package shop

import (
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/foomo/schaumamoi/pkg/internal/mock/catalog"
	"github.com/foomo/schaumamoi/pkg/personalization"
)

// Sort orders a product listing can be rendered in.
const (
	SortDefault   = ""
	SortPriceAsc  = "price_asc"
	SortPriceDesc = "price_desc"

	// SortAffinity ranks by how well each item matches the shopper's vector, as the
	// affinity still distills it into Filter.Profile. A neutral one scores everything the
	// same, which leaves the catalog order.
	SortAffinity = "affinity"
)

// The top-level listing above both departments: every product the shop sells, which is
// what a filter with no category selects.
const (
	AllProductsListID   = "all-products"
	AllProductsListName = "All Products"
)

// Filter is the facet selection behind a product listing: the category being browsed,
// whatever the shopper narrowed it down by, and how the results are ranked — for
// SortAffinity, by the profile the shop has learned for this shopper.
type Filter struct {
	Category string
	Occasion string
	Length   string
	Colour   string
	MaxPrice float64
	Sort     string
	Profile  Profile
}

// Apply returns the subset of items the filter selects, in the filter's sort order.
// Listings are always produced through this, never hand-picked, so a fixture cannot
// report items that contradict its own facets.
func (f Filter) Apply(items []catalog.Item) []catalog.Item {
	out := make([]catalog.Item, 0, len(items))

	for _, item := range items {
		if f.matches(item) {
			out = append(out, item)
		}
	}

	switch {
	case f.Sort == SortPriceAsc:
		sort.SliceStable(out, func(i, j int) bool { return out[i].Price < out[j].Price })
	case f.Sort == SortPriceDesc:
		sort.SliceStable(out, func(i, j int) bool { return out[i].Price > out[j].Price })
	case f.Sort == SortAffinity:
		f.sortByAffinity(out)
	}

	return out
}

// sortByAffinity ranks best match first. Equal scores fall back to the item id: without
// that tie-break the order of two equally good products would be arbitrary and the
// generated fixtures would not be reproducible — and under a neutral profile, where every
// score is equal, it is the whole order.
func (f Filter) sortByAffinity(items []catalog.Item) {
	scores := make(map[string]personalization.Value, len(items))
	for _, item := range items {
		scores[item.ItemID] = catalog.Score(f.Profile.Taste, item)
	}

	sort.SliceStable(items, func(i, j int) bool {
		if scores[items[i].ItemID] != scores[items[j].ItemID] {
			return scores[items[i].ItemID] > scores[items[j].ItemID]
		}

		return items[i].ItemID < items[j].ItemID
	})
}

func (f Filter) matches(item catalog.Item) bool {
	switch {
	case f.Category != "" && !hasCategory(item, f.Category):
		return false
	case f.Occasion != "" && item.Occasion() != f.Occasion:
		return false
	case f.Length != "" && item.Length() != f.Length:
		return false
	case f.Colour != "" && item.ItemVariant != f.Colour:
		return false
	case f.MaxPrice > 0 && item.Price > f.MaxPrice:
		return false
	default:
		return true
	}
}

// hasCategory matches against the whole category path, so browsing "Women's Fashion",
// "Dresses" and "Festive" all work.
func hasCategory(item catalog.Item, category string) bool {
	for _, v := range []string{
		item.ItemCategory,
		item.ItemCategory2,
		item.ItemCategory3,
		item.ItemCategory4,
		item.ItemCategory5,
	} {
		if v == category {
			return true
		}
	}

	return false
}

// Narrowed reports whether any facet is applied on top of the plain category listing.
func (f Filter) Narrowed() bool {
	return f.Occasion != "" || f.Length != "" || f.Colour != "" || f.MaxPrice > 0
}

// Personalized reports whether the listing is ranked by the shopper's learned taste — even
// a neutral one: the first listing a new shopper sees is personalized, the shop just has
// nothing to go on yet.
func (f Filter) Personalized() bool {
	return f.Sort == SortAffinity
}

// Personalize is the filter ranked by what the shop has learned so far.
func (f Filter) Personalize(p Profile) Filter {
	f.Sort, f.Profile = SortAffinity, p

	return f
}

// ListID is the identifier of the listing being browsed: the category's path through the
// tree, e.g. womens-fashion/dresses. Facets deliberately stay out of it: they travel as
// their own fields, so a consumer can aggregate over them without having to parse a
// string back apart.
func (f Filter) ListID() string {
	if f.Category == "" {
		return AllProductsListID
	}

	path := slug(f.Category)
	for parent := catalog.Parent(f.Category); parent != ""; parent = catalog.Parent(parent) {
		path = slug(parent) + "/" + path
	}

	return path
}

// ListName is the human readable name of the listing.
func (f Filter) ListName() string {
	if f.Category == "" {
		return AllProductsListName
	}

	return f.Category
}

// Query renders the facets the way a storefront would put them in a URL, e.g.
// "occasion=festive&sort=affinity".
func (f Filter) Query() string {
	values := url.Values{}

	if f.Occasion != "" {
		values.Set("occasion", f.Occasion)
	}

	if f.Length != "" {
		values.Set("length", f.Length)
	}

	if f.Colour != "" {
		values.Set("colour", f.Colour)
	}

	if f.MaxPrice > 0 {
		values.Set("max_price", strconv.FormatFloat(f.MaxPrice, 'f', -1, 64))
	}

	if f.Sort != SortDefault {
		values.Set("sort", f.Sort)
	}

	return values.Encode()
}

// slug is a category name as a path segment: "Women's Fashion" becomes womens-fashion.
func slug(v string) string {
	return strings.ToLower(strings.NewReplacer(" ", "-", "'", "").Replace(v))
}

// ItemListParams are the params of a view_item_list event, extended with the filter the
// listing was produced by.
//
// sesamy.Event[P] is generic over any param type, and sesamy-go documents pairing a
// custom struct with a built-in event name ("Custom typed params" in its events guide),
// so this needs no library support.
//
// Two constraints shape the struct:
//
//   - Facet fields are flat scalars. mpv2encode.GTag routes unknown params into ep./epn.
//     by runtime type and stringifies anything else with %v, so a nested value would
//     degrade to "[{...}]" on the gtag wire.
//   - Nothing is embedded here. Fields are spelled out so that both encoding/json and
//     sesamy.Decode see the same shape - mapstructure does not inline an untagged
//     embedded field, which is the trap catalog.Item documents.
type ItemListParams struct {
	ItemListID   string         `json:"item_list_id,omitempty"`
	ItemListName string         `json:"item_list_name,omitempty"`
	Items        []catalog.Item `json:"items,omitempty"`

	FilterCategory string  `json:"filter_category,omitempty"`
	FilterOccasion string  `json:"filter_occasion,omitempty"`
	FilterLength   string  `json:"filter_length,omitempty"`
	FilterColour   string  `json:"filter_colour,omitempty"`
	FilterMaxPrice float64 `json:"filter_max_price,omitempty"`
	SortBy         string  `json:"sort_by,omitempty"`
	Personalized   bool    `json:"personalized,omitempty"`
	// TasteSignals is how many interactions the ranking was learned from: zero is the
	// neutral ranking a new shopper starts with. No omitempty, so the zero shows.
	TasteSignals int `json:"taste_signals"`
	ResultCount  int `json:"result_count,omitempty"`
}
