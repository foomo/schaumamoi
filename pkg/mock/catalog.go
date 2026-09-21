package mock

import (
	"math"
	"strings"

	"github.com/foomo/gostandards/iso4217"
	"github.com/foomo/schaumamoi/pkg/personalization"
	"github.com/foomo/sesamy-go/pkg/event/params"
)

// DefaultCurrency is the currency every mock event is priced in.
const DefaultCurrency = iso4217.CodeEUR

// The category the shop sells.
const CategoryDresses = "Dresses"

// Occasions a dress is cut for. They double as personalization dimensions, which is what
// lets a persona's taste pick the section it browses.
const (
	OccasionBusiness = "business"
	OccasionFestive  = "festive"
	OccasionCasual   = "casual"
)

// Hemlines.
const (
	LengthMini = "mini"
	LengthMidi = "midi"
	LengthMaxi = "maxi"
)

// The two labels in the catalog.
const (
	BrandAtelier = "Foomo Atelier"
	BrandStudio  = "Foomo Studio"

	// AtelierFrom is the price at which a dress belongs to the premium label.
	AtelierFrom = 199.0
)

// Affiliation is the store every item is sold by.
const Affiliation = "Foomo Dress Shop"

// dress builds a catalog item. Occasion and length go into the GA4 category path —
// Dresses > Occasion > Length — which is the only place they are stored; Item.Occasion
// and Item.Length read them back out.
//
// The description sits next to the attractor in the signature because it is the same
// statement twice: once for a reader, once for the scorer.
func dress(id, name, occasion, length, colour string, price float64, description string, attractor personalization.Map) Item {
	brand := BrandStudio
	if price >= AtelierFrom {
		brand = BrandAtelier
	}

	return Item{
		Item: params.Item{
			Affiliation:   Affiliation,
			ItemBrand:     brand,
			ItemCategory:  CategoryDresses,
			ItemCategory2: title(occasion),
			ItemCategory3: title(length),
			ItemID:        id,
			ItemName:      name,
			ItemVariant:   colour,
			Price:         price,
			Quantity:      1,
		},
		Attractor:   attractor,
		Description: description,
	}
}

// Catalog is the full set of dresses, in a stable order.
//
// The attractor vectors are chosen so the four personas each land on a different dress,
// see TestPersonaTopMatch. A catalog where every taste led to the same product would
// make the generated traffic useless for testing personalization.
//
// Each dress also carries a description saying in prose what its vector says in numbers.
// That is what makes a result analyzable without arithmetic: a ranking, or an assertion
// that failed on DRESS_07, can be read back against the taste it was supposed to match.
// The two must be kept in step by hand — the description is documentation of the vector,
// not an input to it.
func Catalog() []Item {
	return []Item{
		dress("DRESS_01", "Tailored Sheath Dress", OccasionBusiness, LengthMidi, "navy", 189,
			"Sharply tailored navy sheath, classic structured line — conservative boardroom wear, nothing modern about it.",
			personalization.Map{
				PersoDimensionBusiness: 0.9, PersoDimensionClassic: 0.8, PersoDimensionMinimalistic: 0.5, PersoDimensionFeminine: 0.2,
				PersoDimensionStructured: 0.9, PersoDimensionRetro: 0.3,
			}),
		dress("DRESS_02", "Wool Blend Shift Dress", OccasionBusiness, LengthMidi, "charcoal", 159,
			"Charcoal wool shift with a soft sixties line: understated, classic office wear.",
			personalization.Map{
				PersoDimensionBusiness: 0.85, PersoDimensionClassic: 0.7, PersoDimensionMinimalistic: 0.6,
				PersoDimensionStructured: 0.6, PersoDimensionRetro: 0.5,
			}),
		dress("DRESS_03", "Architectural Column Dress", OccasionBusiness, LengthMaxi, "black", 229,
			"Severe black column, architectural seams and hard edges — contemporary business wear for a minimalist.",
			personalization.Map{
				PersoDimensionBusiness: 0.7, PersoDimensionModern: 0.9, PersoDimensionMinimalistic: 0.8,
				PersoDimensionStructured: 0.9, PersoDimensionEdgy: 0.8, PersoDimensionBold: 0.5,
			}),
		dress("DRESS_04", "Crepe Wrap Dress", OccasionBusiness, LengthMidi, "burgundy", 139,
			"Fluid burgundy crepe wrap, seventies cut — softly feminine, the least formal of the business rail.",
			personalization.Map{
				PersoDimensionBusiness: 0.6, PersoDimensionClassic: 0.5, PersoDimensionFeminine: 0.6,
				PersoDimensionRetro: 0.8, PersoDimensionRomantic: 0.4, PersoDimensionBold: 0.4, PersoDimensionStructured: 0.2,
			}),
		dress("DRESS_05", "Sequin Cocktail Dress", OccasionFestive, LengthMini, "gold", 249,
			"All-over gold sequins, short and loud — maximum party, the opposite of understated.",
			personalization.Map{
				PersoDimensionFestive: 0.95, PersoDimensionOpulent: 0.9, PersoDimensionFeminine: 0.7, PersoDimensionMinimalistic: -0.8,
				PersoDimensionBold: 0.9, PersoDimensionColourful: 0.8,
			}),
		dress("DRESS_06", "Velvet Gown", OccasionFestive, LengthMaxi, "emerald", 329,
			"Floor-length emerald velvet: rich, saturated, classic gala glamour.",
			personalization.Map{
				PersoDimensionFestive: 0.9, PersoDimensionOpulent: 0.85, PersoDimensionClassic: 0.5, PersoDimensionFeminine: 0.6,
				PersoDimensionBold: 0.8, PersoDimensionColourful: 0.8, PersoDimensionRomantic: 0.5,
			}),
		dress("DRESS_07", "Silk Slip Dress", OccasionFestive, LengthMidi, "champagne", 199,
			"Bias-cut champagne silk, no ornament — the restrained, modern way to dress up.",
			personalization.Map{
				PersoDimensionFestive: 0.7, PersoDimensionModern: 0.7, PersoDimensionMinimalistic: 0.6, PersoDimensionFeminine: 0.5,
				PersoDimensionRomantic: 0.5, PersoDimensionEdgy: 0.3, PersoDimensionColourful: 0.3,
			}),
		dress("DRESS_08", "Tulle Ballgown", OccasionFestive, LengthMaxi, "blush", 389,
			"Layered blush tulle over a full skirt — unapologetically romantic, the most opulent piece in the catalog.",
			personalization.Map{
				PersoDimensionFestive: 0.95, PersoDimensionOpulent: 0.95, PersoDimensionFeminine: 0.95,
				PersoDimensionClassic: 0.6, PersoDimensionMinimalistic: -0.9,
				PersoDimensionRomantic: 0.9, PersoDimensionBold: 0.5, PersoDimensionColourful: 0.3,
			}),
		dress("DRESS_09", "Cotton Shirt Dress", OccasionCasual, LengthMidi, "sky", 79,
			"Crisp sky-blue cotton, collar and buttons — plain, practical everyday wear.",
			personalization.Map{
				PersoDimensionCasual: 0.9, PersoDimensionMinimalistic: 0.6, PersoDimensionClassic: 0.3,
				PersoDimensionStructured: 0.4, PersoDimensionColourful: 0.3,
			}),
		dress("DRESS_10", "Jersey T-Shirt Dress", OccasionCasual, LengthMini, "grey", 49,
			"Grey jersey cut like a long t-shirt — the most stripped-back, fuss-free thing on the rail.",
			personalization.Map{
				PersoDimensionCasual: 0.95, PersoDimensionMinimalistic: 0.85, PersoDimensionModern: 0.5,
				PersoDimensionEdgy: 0.4,
			}),
		dress("DRESS_11", "Linen Smock Dress", OccasionCasual, LengthMaxi, "sand", 99,
			"Loose sand linen smock, ankle length — easy bohemian summer wear.",
			personalization.Map{
				PersoDimensionCasual: 0.85, PersoDimensionMinimalistic: 0.5, PersoDimensionFeminine: 0.4,
				PersoDimensionBohemian: 0.8, PersoDimensionRomantic: 0.2,
			}),
		dress("DRESS_12", "Floral Tea Dress", OccasionCasual, LengthMidi, "rose", 89,
			"Rose floral print, forties silhouette — nostalgic and feminine, the dressiest piece on the casual rail.",
			personalization.Map{
				PersoDimensionCasual: 0.6, PersoDimensionFeminine: 0.8, PersoDimensionClassic: 0.6,
				PersoDimensionBohemian: 0.4, PersoDimensionRomantic: 0.7, PersoDimensionRetro: 0.9, PersoDimensionBold: 0.4, PersoDimensionColourful: 0.6,
			}),
	}
}

// indexed returns a copy of items with Index stamped to the list position, the way GA4
// reports items that were rendered as part of a list.
func indexed(items []Item) []Item {
	out := make([]Item, len(items))
	for i, item := range items {
		item.Index = i
		out[i] = item
	}

	return out
}

// listed returns a copy of items tagged with the list they were surfaced in.
func listed(listID, listName string, items []Item) []Item {

	for i := range items {
		items[i].ItemListID = listID
		items[i].ItemListName = listName
	}

	return items
}

// quantity defaults a missing quantity to a single unit.
func quantity(item Item) float64 {
	if item.Quantity == 0 {
		return 1
	}

	return item.Quantity
}

// value totals the net line values of items, i.e. (price - discount) * quantity.
func value(items []Item) float64 {
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

// title upper-cases the first letter, for the human readable category path.
func title(v string) string {
	if v == "" {
		return v
	}

	return strings.ToUpper(v[:1]) + v[1:]
}
