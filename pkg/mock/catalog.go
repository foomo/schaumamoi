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

// The category tree: two departments with their categories below. In fashion, occasion
// and length sit below the category in the GA4 path, so the full path of a dress reads
// Women's Fashion > Dresses > Festive > Maxi; home decoration has no such facets and stops
// at the category, e.g. Home Decoration > Lamps.
//
// Categories are where products are offered: entry points to browse and choose from.
// They are not a signal about the shopper — what the shopper selects, views and buys is.
// Nor do they partition taste: a dress and a pair of trousers are described in largely
// the same dimensions, so taste carries between them, while a lamp shares only some of
// them (modern, classic, minimalistic …) and adds its own (cozy, natural, industrial).
// The sparse personalization.Map is what expresses that.
const (
	CategoryWomensFashion = "Women's Fashion"
	CategoryDresses       = "Dresses"
	CategoryTrousers      = "Trousers"

	CategoryHomeDecoration = "Home Decoration"
	CategoryCushions       = "Cushions"
	CategoryLamps          = "Lamps"
	CategoryVases          = "Vases"
)

// categoryParent is the tree above, child to parent, which is what turns a category
// into a list path such as womens-fashion/dresses.
var categoryParent = map[string]string{
	CategoryDresses:  CategoryWomensFashion,
	CategoryTrousers: CategoryWomensFashion,

	CategoryCushions: CategoryHomeDecoration,
	CategoryLamps:    CategoryHomeDecoration,
	CategoryVases:    CategoryHomeDecoration,
}

// Occasions a dress is cut for. They double as personalization dimensions, which is what
// lets a persona's taste pick the section it browses.
const (
	OccasionBusiness = "business"
	OccasionFestive  = "festive"
	OccasionCasual   = "casual"
)

// Dress hemlines.
const (
	LengthMini = "mini"
	LengthMidi = "midi"
	LengthMaxi = "maxi"
)

// Trouser leg lengths.
const (
	LengthCropped = "cropped"
	LengthAnkle   = "ankle"
	LengthFull    = "full"
)

// The two labels in the catalog.
const (
	BrandAtelier = "Foomo Atelier"
	BrandStudio  = "Foomo Studio"

	// AtelierFrom is the price at which a dress belongs to the premium label.
	AtelierFrom = 199.0
)

// Affiliation is the store every item is sold by.
const Affiliation = "Foomo Fashion"

// product builds a catalog item. Its place in the tree goes into the GA4 category path —
// Women's Fashion > Category > Occasion > Length — which is the only place it is
// stored; Item.Category, Item.Occasion and Item.Length read it back out.
//
// The description sits next to the attractor in the signature because it is the same
// statement twice: once for a reader, once for the scorer.
func product(category, id, name, occasion, length, colour string, price float64, description string, attractor personalization.Map) Item {
	brand := BrandStudio
	if price >= AtelierFrom {
		brand = BrandAtelier
	}

	return Item{
		Item: params.Item{
			Affiliation:   Affiliation,
			ItemBrand:     brand,
			ItemCategory:  categoryParent[category],
			ItemCategory2: category,
			ItemCategory3: title(occasion),
			ItemCategory4: title(length),
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

func dress(id, name, occasion, length, colour string, price float64, description string, attractor personalization.Map) Item {
	return product(CategoryDresses, id, name, occasion, length, colour, price, description, attractor)
}

func trousers(id, name, occasion, length, colour string, price float64, description string, attractor personalization.Map) Item {
	return product(CategoryTrousers, id, name, occasion, length, colour, price, description, attractor)
}

// Catalog is everything the shop sells — dresses, trousers, then home decoration — in a
// stable order.
//
// The attractor vectors are chosen so the four personas each land on a different product,
// see TestPersonaTopMatch. A catalog where every taste led to the same product would
// make the generated traffic useless for testing personalization.
//
// Each product also carries a description saying in prose what its vector says in numbers.
// That is what makes a result analyzable without arithmetic: a ranking, or an assertion
// that failed on DRESS_07, can be read back against the taste it was supposed to match.
// The two must be kept in step by hand — the description is documentation of the vector,
// not an input to it.
func Catalog() []Item {
	return append(Fashion(), HomeDecoration()...)
}

// Fashion is the women's fashion department: dresses then trousers.
func Fashion() []Item {
	return append(Dresses(), Trousers()...)
}

// Dresses is the dress rail, four per occasion.
func Dresses() []Item {
	return []Item{
		dress("DRESS_01", "Whitehall Sheath Dress", OccasionBusiness, LengthMidi, "navy", 189,
			"Sharply tailored navy sheath, classic structured line — conservative boardroom wear, nothing modern about it.",
			personalization.Map{
				PersoDimensionBusiness: 0.9, PersoDimensionClassic: 0.8, PersoDimensionMinimalistic: 0.5, PersoDimensionFeminine: 0.2,
				PersoDimensionStructured: 0.9, PersoDimensionRetro: 0.3,
			}),
		dress("DRESS_02", "Margot Shift Dress", OccasionBusiness, LengthMidi, "charcoal", 159,
			"Charcoal wool shift with a soft sixties line: understated, classic office wear.",
			personalization.Map{
				PersoDimensionBusiness: 0.85, PersoDimensionClassic: 0.7, PersoDimensionMinimalistic: 0.6,
				PersoDimensionStructured: 0.6, PersoDimensionRetro: 0.5,
			}),
		dress("DRESS_03", "Monolith Column Dress", OccasionBusiness, LengthMaxi, "black", 229,
			"Severe black column, architectural seams and hard edges — contemporary business wear for a minimalist.",
			personalization.Map{
				PersoDimensionBusiness: 0.7, PersoDimensionModern: 0.9, PersoDimensionMinimalistic: 0.8,
				PersoDimensionStructured: 0.9, PersoDimensionEdgy: 0.8, PersoDimensionBold: 0.5,
			}),
		dress("DRESS_04", "Laurel Wrap Dress", OccasionBusiness, LengthMidi, "burgundy", 139,
			"Fluid burgundy crepe wrap, seventies cut — softly feminine, the least formal of the business rail.",
			personalization.Map{
				PersoDimensionBusiness: 0.6, PersoDimensionClassic: 0.5, PersoDimensionFeminine: 0.6,
				PersoDimensionRetro: 0.8, PersoDimensionRomantic: 0.4, PersoDimensionBold: 0.4, PersoDimensionStructured: 0.2,
			}),
		dress("DRESS_05", "Starlet Sequin Dress", OccasionFestive, LengthMini, "gold", 249,
			"All-over gold sequins, short and loud — maximum party, the opposite of understated.",
			personalization.Map{
				PersoDimensionFestive: 0.95, PersoDimensionOpulent: 0.9, PersoDimensionFeminine: 0.7, PersoDimensionMinimalistic: -0.8,
				PersoDimensionBold: 0.9, PersoDimensionColourful: 0.8,
			}),
		dress("DRESS_06", "Belvedere Velvet Gown", OccasionFestive, LengthMaxi, "emerald", 329,
			"Floor-length emerald velvet: rich, saturated, classic gala glamour.",
			personalization.Map{
				PersoDimensionFestive: 0.9, PersoDimensionOpulent: 0.85, PersoDimensionClassic: 0.5, PersoDimensionFeminine: 0.6,
				PersoDimensionBold: 0.8, PersoDimensionColourful: 0.8, PersoDimensionRomantic: 0.5,
			}),
		dress("DRESS_07", "Lune Slip Dress", OccasionFestive, LengthMidi, "champagne", 199,
			"Bias-cut champagne silk, no ornament — the restrained, modern way to dress up.",
			personalization.Map{
				PersoDimensionFestive: 0.7, PersoDimensionModern: 0.7, PersoDimensionMinimalistic: 0.6, PersoDimensionFeminine: 0.5,
				PersoDimensionRomantic: 0.5, PersoDimensionEdgy: 0.3, PersoDimensionColourful: 0.3,
			}),
		dress("DRESS_08", "Odette Tulle Ballgown", OccasionFestive, LengthMaxi, "blush", 389,
			"Layered blush tulle over a full skirt — unapologetically romantic, the most opulent piece in the catalog.",
			personalization.Map{
				PersoDimensionFestive: 0.95, PersoDimensionOpulent: 0.95, PersoDimensionFeminine: 0.95,
				PersoDimensionClassic: 0.6, PersoDimensionMinimalistic: -0.9,
				PersoDimensionRomantic: 0.9, PersoDimensionBold: 0.5, PersoDimensionColourful: 0.3,
			}),
		dress("DRESS_09", "Harbour Shirt Dress", OccasionCasual, LengthMidi, "sky", 79,
			"Crisp sky-blue cotton, collar and buttons — plain, practical everyday wear.",
			personalization.Map{
				PersoDimensionCasual: 0.9, PersoDimensionMinimalistic: 0.6, PersoDimensionClassic: 0.3,
				PersoDimensionStructured: 0.4, PersoDimensionColourful: 0.3,
			}),
		dress("DRESS_10", "Sunday Tee Dress", OccasionCasual, LengthMini, "grey", 49,
			"Grey jersey cut like a long t-shirt — the most stripped-back, fuss-free thing on the rail.",
			personalization.Map{
				PersoDimensionCasual: 0.95, PersoDimensionMinimalistic: 0.85, PersoDimensionModern: 0.5,
				PersoDimensionEdgy: 0.4,
			}),
		dress("DRESS_11", "Meadow Smock Dress", OccasionCasual, LengthMaxi, "sand", 99,
			"Loose sand linen smock, ankle length — easy bohemian summer wear.",
			personalization.Map{
				PersoDimensionCasual: 0.85, PersoDimensionMinimalistic: 0.5, PersoDimensionFeminine: 0.4,
				PersoDimensionBohemian: 0.8, PersoDimensionRomantic: 0.2,
			}),
		dress("DRESS_12", "Primrose Tea Dress", OccasionCasual, LengthMidi, "rose", 89,
			"Rose floral print, forties silhouette — nostalgic and feminine, the dressiest piece on the casual rail.",
			personalization.Map{
				PersoDimensionCasual: 0.6, PersoDimensionFeminine: 0.8, PersoDimensionClassic: 0.6,
				PersoDimensionBohemian: 0.4, PersoDimensionRomantic: 0.7, PersoDimensionRetro: 0.9, PersoDimensionBold: 0.4, PersoDimensionColourful: 0.6,
			}),
		dress("DRESS_13", "Carter Belted Shirt Dress", OccasionBusiness, LengthMidi, "camel", 149,
			"Camel cotton shirt dress with a collar, buttons and a self belt — practical, classic office wear.",
			personalization.Map{
				PersoDimensionBusiness: 0.7, PersoDimensionClassic: 0.6, PersoDimensionStructured: 0.5,
				PersoDimensionMinimalistic: 0.4, PersoDimensionCasual: 0.3,
			}),
		dress("DRESS_14", "Chancery Pencil Dress", OccasionBusiness, LengthMidi, "navy", 179,
			"Navy pinstripe pencil dress with sharp shoulders — traditional, structured power dressing.",
			personalization.Map{
				PersoDimensionBusiness: 0.9, PersoDimensionClassic: 0.8, PersoDimensionStructured: 0.8,
				PersoDimensionBold: 0.3, PersoDimensionRetro: 0.3,
			}),
		dress("DRESS_15", "Juniper Satin Dress", OccasionFestive, LengthMidi, "emerald", 229,
			"Emerald satin, softly draped — elegant evening wear, romantic without the frills.",
			personalization.Map{
				PersoDimensionFestive: 0.8, PersoDimensionRomantic: 0.6, PersoDimensionFeminine: 0.7, PersoDimensionOpulent: 0.5,
				PersoDimensionClassic: 0.4, PersoDimensionColourful: 0.5,
			}),
		dress("DRESS_16", "Flamingo Feather Mini", OccasionFestive, LengthMini, "blush", 259,
			"Blush mini with a feather hem — playful, glamorous party wear.",
			personalization.Map{
				PersoDimensionFestive: 0.9, PersoDimensionOpulent: 0.7, PersoDimensionFeminine: 0.8, PersoDimensionBold: 0.7,
				PersoDimensionRomantic: 0.5, PersoDimensionColourful: 0.4, PersoDimensionMinimalistic: -0.6,
			}),
		dress("DRESS_17", "Fenna Knit Dress", OccasionCasual, LengthMini, "grey", 69,
			"Grey rib-knit jumper dress — warm, simple, made for the weekend.",
			personalization.Map{
				PersoDimensionCasual: 0.9, PersoDimensionMinimalistic: 0.7, PersoDimensionModern: 0.4, PersoDimensionClassic: 0.2,
			}),
		dress("DRESS_18", "Rowan Pinafore Dress", OccasionCasual, LengthMidi, "indigo", 79,
			"Indigo denim pinafore with patch pockets — utilitarian and a little playful.",
			personalization.Map{
				PersoDimensionCasual: 0.85, PersoDimensionEdgy: 0.4, PersoDimensionRetro: 0.4, PersoDimensionStructured: 0.3,
				PersoDimensionBold: 0.2,
			}),
	}
}

// Trousers is the trouser rail, four per occasion, cut against the same dimensions as
// the dresses so one taste can rank both. Two of the personas land here rather than on
// a dress: Lena on the joggers, Mira on the cigarette trousers — see TestPersonaTopMatch.
func Trousers() []Item {
	return []Item{
		trousers("TROUSERS_01", "Temple Pleated Trousers", OccasionBusiness, LengthFull, "navy", 169,
			"Front-pleated navy wool, high waist and a pressed straight leg — classic boardroom tailoring.",
			personalization.Map{
				PersoDimensionBusiness: 0.9, PersoDimensionClassic: 0.8, PersoDimensionStructured: 0.8,
				PersoDimensionRetro: 0.4, PersoDimensionMinimalistic: 0.4,
			}),
		trousers("TROUSERS_02", "Grid Cigarette Trousers", OccasionBusiness, LengthAnkle, "black", 149,
			"Slim black cigarette trousers with a sharp crease and nothing else — modern, minimal, a little severe.",
			personalization.Map{
				PersoDimensionBusiness: 0.8, PersoDimensionModern: 0.9, PersoDimensionMinimalistic: 0.85,
				PersoDimensionStructured: 0.8, PersoDimensionEdgy: 0.7, PersoDimensionClassic: -0.3,
			}),
		trousers("TROUSERS_03", "Riviera Wide-Leg Trousers", OccasionBusiness, LengthFull, "camel", 159,
			"Fluid camel crepe, high waist and a wide seventies leg — softly feminine office wear.",
			personalization.Map{
				PersoDimensionBusiness: 0.6, PersoDimensionRetro: 0.75, PersoDimensionFeminine: 0.5,
				PersoDimensionClassic: 0.4, PersoDimensionRomantic: 0.3, PersoDimensionBold: 0.3,
			}),
		trousers("TROUSERS_04", "Windsor Check Trousers", OccasionBusiness, LengthFull, "grey", 179,
			"Prince of Wales check with a straight leg — traditional suiting with a little pattern.",
			personalization.Map{
				PersoDimensionBusiness: 0.8, PersoDimensionClassic: 0.75, PersoDimensionStructured: 0.7,
				PersoDimensionRetro: 0.5, PersoDimensionBold: 0.3, PersoDimensionColourful: 0.2,
			}),
		trousers("TROUSERS_05", "Glitterball Sequin Flares", OccasionFestive, LengthFull, "silver", 239,
			"Silver sequins head to toe, flared from the knee — seventies disco, loud and shiny.",
			personalization.Map{
				PersoDimensionFestive: 0.9, PersoDimensionOpulent: 0.85, PersoDimensionBold: 0.9, PersoDimensionRetro: 0.6,
				PersoDimensionColourful: 0.6, PersoDimensionFeminine: 0.4, PersoDimensionMinimalistic: -0.7,
			}),
		trousers("TROUSERS_06", "Palermo Velvet Palazzo", OccasionFestive, LengthFull, "plum", 219,
			"Plum velvet palazzo, wide and floor-sweeping — rich, romantic evening wear.",
			personalization.Map{
				PersoDimensionFestive: 0.85, PersoDimensionOpulent: 0.8, PersoDimensionFeminine: 0.7, PersoDimensionRomantic: 0.6,
				PersoDimensionBold: 0.5, PersoDimensionColourful: 0.6, PersoDimensionClassic: 0.3,
			}),
		trousers("TROUSERS_07", "Soirée Tuxedo Trousers", OccasionFestive, LengthFull, "black", 199,
			"Black tuxedo trousers with a satin side stripe — sharp, modern eveningwear.",
			personalization.Map{
				PersoDimensionFestive: 0.7, PersoDimensionModern: 0.7, PersoDimensionStructured: 0.7,
				PersoDimensionMinimalistic: 0.5, PersoDimensionEdgy: 0.5, PersoDimensionClassic: 0.3,
			}),
		trousers("TROUSERS_08", "Medici Brocade Trousers", OccasionFestive, LengthCropped, "gold", 209,
			"Gold brocade, slim and cropped above the ankle — ornate fabric on a sharp line.",
			personalization.Map{
				PersoDimensionFestive: 0.8, PersoDimensionOpulent: 0.7, PersoDimensionBold: 0.6, PersoDimensionColourful: 0.5,
				PersoDimensionStructured: 0.5, PersoDimensionRetro: 0.3, PersoDimensionClassic: 0.3,
			}),
		trousers("TROUSERS_09", "Mile Straight Jeans", OccasionCasual, LengthFull, "indigo", 89,
			"Mid-blue denim, straight leg, five pockets — the everyday default.",
			personalization.Map{
				PersoDimensionCasual: 0.9, PersoDimensionClassic: 0.5, PersoDimensionMinimalistic: 0.5,
				PersoDimensionStructured: 0.4,
			}),
		trousers("TROUSERS_10", "Haven Joggers", OccasionCasual, LengthAnkle, "grey", 59,
			"Grey jersey joggers with a drawstring and cuffs — comfort first, the opposite of dressed up.",
			personalization.Map{
				PersoDimensionCasual: 0.95, PersoDimensionMinimalistic: 0.85, PersoDimensionModern: 0.6,
				PersoDimensionEdgy: 0.4, PersoDimensionOpulent: -0.5,
			}),
		trousers("TROUSERS_11", "Dune Linen Trousers", OccasionCasual, LengthFull, "olive", 79,
			"Loose olive linen, drawstring waist and a relaxed wide leg — easy bohemian summer wear.",
			personalization.Map{
				PersoDimensionCasual: 0.85, PersoDimensionBohemian: 0.8, PersoDimensionMinimalistic: 0.4,
				PersoDimensionFeminine: 0.3, PersoDimensionRomantic: 0.2,
			}),
		trousers("TROUSERS_12", "Field Cargo Trousers", OccasionCasual, LengthFull, "khaki", 95,
			"Khaki cotton cargos with patch pockets and a utility cut — practical and a bit tough.",
			personalization.Map{
				PersoDimensionCasual: 0.8, PersoDimensionEdgy: 0.6, PersoDimensionStructured: 0.5,
				PersoDimensionModern: 0.4, PersoDimensionBold: 0.3,
			}),
		trousers("TROUSERS_13", "Atrium Wide-Leg Trousers", OccasionBusiness, LengthFull, "charcoal", 189,
			"Charcoal wool, high waist and a crisp wide leg with a sharp crease — modern tailoring.",
			personalization.Map{
				PersoDimensionBusiness: 0.8, PersoDimensionModern: 0.7, PersoDimensionStructured: 0.7,
				PersoDimensionMinimalistic: 0.6, PersoDimensionEdgy: 0.3,
			}),
		trousers("TROUSERS_14", "Ledger Suit Trousers", OccasionBusiness, LengthFull, "navy", 159,
			"Navy suiting with a pressed crease and a straight leg — classic and dependable.",
			personalization.Map{
				PersoDimensionBusiness: 0.85, PersoDimensionClassic: 0.8, PersoDimensionStructured: 0.75,
				PersoDimensionMinimalistic: 0.3,
			}),
		trousers("TROUSERS_15", "Lola Velvet Flares", OccasionFestive, LengthFull, "burgundy", 229,
			"Burgundy velvet, flared from the knee — seventies glamour for the evening.",
			personalization.Map{
				PersoDimensionFestive: 0.85, PersoDimensionOpulent: 0.75, PersoDimensionRetro: 0.7, PersoDimensionBold: 0.6,
				PersoDimensionFeminine: 0.5, PersoDimensionRomantic: 0.4, PersoDimensionColourful: 0.5,
			}),
		trousers("TROUSERS_16", "Gala Cigarette Trousers", OccasionFestive, LengthAnkle, "champagne", 179,
			"Champagne satin cigarette trousers with a crease — sleek, minimal evening wear.",
			personalization.Map{
				PersoDimensionFestive: 0.7, PersoDimensionModern: 0.7, PersoDimensionMinimalistic: 0.6,
				PersoDimensionFeminine: 0.4, PersoDimensionEdgy: 0.3,
			}),
		trousers("TROUSERS_17", "Canyon Corduroy Trousers", OccasionCasual, LengthFull, "camel", 89,
			"Camel corduroy with a wide, soft leg — retro and easy-going.",
			personalization.Map{
				PersoDimensionCasual: 0.85, PersoDimensionRetro: 0.6, PersoDimensionBohemian: 0.5, PersoDimensionClassic: 0.3,
			}),
		trousers("TROUSERS_18", "Sprint Track Pants", OccasionCasual, LengthAnkle, "black", 65,
			"Black jersey track pants with a side stripe and cuffs — sporty, minimal, fuss-free.",
			personalization.Map{
				PersoDimensionCasual: 0.9, PersoDimensionMinimalistic: 0.8, PersoDimensionModern: 0.6, PersoDimensionEdgy: 0.5,
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

// homeItem builds a home decoration product. Home items have no occasion or length: their
// GA4 path stops at the category.
func homeItem(category, id, name, colour string, price float64, description string, attractor personalization.Map) Item {
	return product(category, id, name, "", "", colour, price, description, attractor)
}

// HomeDecoration is the home department: four cushions, four lamps, four vases. Their
// attractors use the shared style dimensions and the home-only ones, never the
// fashion-only ones, so each persona's fashion taste ranks them only by what carries over.
func HomeDecoration() []Item {
	return []Item{
		homeItem(CategoryCushions, "HOME_01", "Boudoir Velvet Cushion", "plum", 69,
			"Plum velvet with gold tassels — plush, romantic and a little theatrical.",
			personalization.Map{
				PersoDimensionOpulent: 0.85, PersoDimensionRomantic: 0.7, PersoDimensionColourful: 0.6,
				PersoDimensionClassic: 0.3, PersoDimensionCozy: 0.5, PersoDimensionMinimalistic: -0.6,
			}),
		homeItem(CategoryCushions, "HOME_02", "Fireside Knit Cushion", "cream", 49,
			"Cream cable knit, soft and heavy — traditional comfort for the armchair.",
			personalization.Map{
				PersoDimensionCozy: 0.9, PersoDimensionClassic: 0.7, PersoDimensionNatural: 0.4, PersoDimensionRetro: 0.3,
			}),
		homeItem(CategoryCushions, "HOME_03", "Driftwood Linen Cushion", "sand", 39,
			"Sand washed linen, no trim at all — plain, natural and easy.",
			personalization.Map{
				PersoDimensionNatural: 0.85, PersoDimensionMinimalistic: 0.75, PersoDimensionBohemian: 0.4, PersoDimensionCozy: 0.3,
			}),
		homeItem(CategoryCushions, "HOME_04", "Zigzag Print Cushion", "mustard", 45,
			"Mustard and black geometric print — graphic, bold, a nod to the seventies.",
			personalization.Map{
				PersoDimensionModern: 0.6, PersoDimensionBold: 0.8, PersoDimensionColourful: 0.7, PersoDimensionRetro: 0.6,
				PersoDimensionEdgy: 0.3,
			}),
		homeItem(CategoryLamps, "HOME_05", "Archive Brass Table Lamp", "gold", 189,
			"Polished brass with a pleated shade — classic, warm library light.",
			personalization.Map{
				PersoDimensionClassic: 0.9, PersoDimensionOpulent: 0.4, PersoDimensionRetro: 0.4, PersoDimensionCozy: 0.5,
				PersoDimensionModern: -0.5,
			}),
		homeItem(CategoryLamps, "HOME_06", "Halden Concrete Pendant", "grey", 159,
			"Raw concrete pendant on a black cord — industrial, severe and modern.",
			personalization.Map{
				PersoDimensionIndustrial: 0.9, PersoDimensionModern: 0.85, PersoDimensionMinimalistic: 0.75, PersoDimensionEdgy: 0.6,
				PersoDimensionClassic: -0.4,
			}),
		homeItem(CategoryLamps, "HOME_07", "Bali Rattan Floor Lamp", "sand", 129,
			"Woven rattan shade on a tall stand — natural, bohemian, soft evening light.",
			personalization.Map{
				PersoDimensionNatural: 0.9, PersoDimensionBohemian: 0.85, PersoDimensionCozy: 0.4,
			}),
		homeItem(CategoryLamps, "HOME_08", "Versailles Crystal Lamp", "champagne", 249,
			"Cut crystal base under a silk shade — glamorous, romantic, made to sparkle.",
			personalization.Map{
				PersoDimensionOpulent: 0.95, PersoDimensionRomantic: 0.75, PersoDimensionBold: 0.5, PersoDimensionClassic: 0.4,
				PersoDimensionMinimalistic: -0.7,
			}),
		homeItem(CategoryVases, "HOME_09", "Kiln Stoneware Vase", "sand", 59,
			"Hand-thrown stoneware with a raw foot — natural, quiet and minimal.",
			personalization.Map{
				PersoDimensionNatural: 0.85, PersoDimensionMinimalistic: 0.7, PersoDimensionCozy: 0.2, PersoDimensionModern: 0.2,
			}),
		homeItem(CategoryVases, "HOME_10", "Obsidian Glass Vase", "black", 79,
			"Smoked black glass cylinder — sharp, modern, with an industrial edge.",
			personalization.Map{
				PersoDimensionModern: 0.85, PersoDimensionMinimalistic: 0.8, PersoDimensionEdgy: 0.6, PersoDimensionIndustrial: 0.4,
			}),
		homeItem(CategoryVases, "HOME_11", "Canton Ginger Jar", "navy", 99,
			"Blue and white porcelain with a lid — classic, traditional, a little old-fashioned.",
			personalization.Map{
				PersoDimensionClassic: 0.95, PersoDimensionRetro: 0.5, PersoDimensionColourful: 0.3, PersoDimensionModern: -0.6,
			}),
		homeItem(CategoryVases, "HOME_12", "Carnevale Murano Vase", "rose", 149,
			"Swirled Murano glass in rose and amber — loud, colourful and ornate.",
			personalization.Map{
				PersoDimensionColourful: 0.9, PersoDimensionBold: 0.8, PersoDimensionOpulent: 0.6, PersoDimensionRetro: 0.4,
				PersoDimensionMinimalistic: -0.5,
			}),
		homeItem(CategoryCushions, "HOME_13", "Cloud Bouclé Cushion", "cream", 59,
			"Cream bouclé, nubby and soft — cozy, and quietly modern.",
			personalization.Map{
				PersoDimensionCozy: 0.85, PersoDimensionMinimalistic: 0.5, PersoDimensionModern: 0.4, PersoDimensionNatural: 0.3,
			}),
		homeItem(CategoryCushions, "HOME_14", "Posy Embroidered Cushion", "rose", 55,
			"Rose cotton with embroidered floral sprays — romantic and a little retro.",
			personalization.Map{
				PersoDimensionRomantic: 0.7, PersoDimensionRetro: 0.6, PersoDimensionColourful: 0.6, PersoDimensionClassic: 0.4,
				PersoDimensionCozy: 0.4,
			}),
		homeItem(CategoryLamps, "HOME_15", "Meridian Arc Floor Lamp", "black", 229,
			"Black steel arc floor lamp with a dome shade — modern, graphic, a little industrial.",
			personalization.Map{
				PersoDimensionModern: 0.85, PersoDimensionIndustrial: 0.6, PersoDimensionMinimalistic: 0.6, PersoDimensionEdgy: 0.4,
				PersoDimensionBold: 0.3,
			}),
		homeItem(CategoryLamps, "HOME_16", "Grove Ceramic Table Lamp", "olive", 139,
			"Olive glazed ceramic base under a linen shade — natural, calm and homely.",
			personalization.Map{
				PersoDimensionNatural: 0.7, PersoDimensionCozy: 0.6, PersoDimensionClassic: 0.3, PersoDimensionMinimalistic: 0.3,
			}),
		homeItem(CategoryVases, "HOME_17", "Coupe Fluted Vase", "champagne", 89,
			"Champagne fluted glass — elegant and romantic, it catches the light.",
			personalization.Map{
				PersoDimensionRomantic: 0.6, PersoDimensionOpulent: 0.6, PersoDimensionClassic: 0.5,
			}),
		homeItem(CategoryVases, "HOME_18", "Sienna Terracotta Vase", "terracotta", 49,
			"Earthy terracotta with a raw foot — rustic, natural and bohemian.",
			personalization.Map{
				PersoDimensionNatural: 0.8, PersoDimensionBohemian: 0.7, PersoDimensionCozy: 0.4, PersoDimensionRetro: 0.2,
			}),
	}
}
