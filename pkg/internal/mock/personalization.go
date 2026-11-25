package mock

import (
	"hash/fnv"

	"github.com/foomo/schaumamoi/pkg/personalization"
)

const (
	PersoDimensionBusiness     personalization.Dimension = "business"
	PersoDimensionFestive      personalization.Dimension = "festive"
	PersoDimensionCasual       personalization.Dimension = "casual"
	PersoDimensionModern       personalization.Dimension = "modern"
	PersoDimensionClassic      personalization.Dimension = "classic"
	PersoDimensionMinimalistic personalization.Dimension = "minimalistic"
	PersoDimensionOpulent      personalization.Dimension = "opulent"
	PersoDimensionFeminine     personalization.Dimension = "feminine"
	PersoDimensionBohemian     personalization.Dimension = "bohemian"
	PersoDimensionRomantic     personalization.Dimension = "romantic"
	PersoDimensionStructured   personalization.Dimension = "structured"
	PersoDimensionRetro        personalization.Dimension = "retro"
	PersoDimensionEdgy         personalization.Dimension = "edgy"
	PersoDimensionBold         personalization.Dimension = "bold"
	PersoDimensionColourful    personalization.Dimension = "colourful"

	// Home decoration speaks some of the dimensions above — modern, classic, minimalistic,
	// opulent, bohemian, romantic, retro, edgy, bold, colourful — and these of its own.
	// The fashion-only ones (business, festive, casual, feminine, structured) never appear
	// on a home item, so taste learned from clothes reaches the home department only
	// through what the two share.
	PersoDimensionCozy       personalization.Dimension = "cozy"
	PersoDimensionNatural    personalization.Dimension = "natural"
	PersoDimensionIndustrial personalization.Dimension = "industrial"
)

type Persona struct {
	Name        string
	Description string
	Affinity    personalization.Map
}

// The personas are deliberately chosen to discriminate. Kate and Mira are both
// business-minded but sit at opposite ends of classic and modern, so occasion alone must
// never decide what a shopper ends up with — each persona's top-scoring dress is a
// different one. See TestPersonaTopMatch.
var (
	PersonaBusinessKate = Persona{
		Name:        "Kate",
		Description: "50 year old conservative lawyer",
		Affinity: personalization.Map{
			PersoDimensionBusiness:   0.9,
			PersoDimensionClassic:    0.99,
			PersoDimensionModern:     -1.0,
			PersoDimensionStructured: 0.8,
			PersoDimensionEdgy:       -0.5,
			PersoDimensionRetro:      0.3,
			PersoDimensionCozy:       0.5,
		},
	}

	PersonaFestiveSofia = Persona{
		Name:        "Sofia",
		Description: "28 year old event host with a taste for glamour",
		Affinity: personalization.Map{
			PersoDimensionFestive:      0.95,
			PersoDimensionOpulent:      0.85,
			PersoDimensionFeminine:     0.8,
			PersoDimensionMinimalistic: -0.7,
			PersoDimensionRomantic:     0.9,
			PersoDimensionBold:         0.7,
			PersoDimensionColourful:    0.5,
		},
	}

	PersonaCasualLena = Persona{
		Name:        "Lena",
		Description: "34 year old designer who dresses down and hates fuss",
		Affinity: personalization.Map{
			PersoDimensionCasual:       0.9,
			PersoDimensionMinimalistic: 0.8,
			PersoDimensionModern:       0.6,
			PersoDimensionOpulent:      -0.9,
			PersoDimensionBohemian:     0.5,
			PersoDimensionEdgy:         0.4,
			PersoDimensionNatural:      0.8,
		},
	}

	PersonaModernMira = Persona{
		Name:        "Mira",
		Description: "41 year old architect, business wear but strictly contemporary",
		Affinity: personalization.Map{
			PersoDimensionBusiness:     0.7,
			PersoDimensionModern:       0.9,
			PersoDimensionMinimalistic: 0.75,
			PersoDimensionClassic:      -0.6,
			PersoDimensionEdgy:         0.7,
			PersoDimensionStructured:   0.6,
			PersoDimensionIndustrial:   0.8,
		},
	}
)

// Personas returns every persona in a stable order.
func Personas() []Persona {
	return []Persona{
		PersonaBusinessKate,
		PersonaFestiveSofia,
		PersonaCasualLena,
		PersonaModernMira,
	}
}

// Occasion is the section of the shop this persona browses: the occasion dimension it
// scores highest on. Occasions double as personalization dimensions, so taste picks the
// section rather than it being hardcoded per persona.
func (p Persona) Occasion() string {
	best := OccasionCasual
	found := false

	// Iterated in a fixed order so ties resolve the same way on every run.
	for _, o := range []struct {
		occasion  string
		dimension personalization.Dimension
	}{
		{OccasionBusiness, PersoDimensionBusiness},
		{OccasionFestive, PersoDimensionFestive},
		{OccasionCasual, PersoDimensionCasual},
	} {
		v, ok := p.Affinity[o.dimension]
		if !ok {
			continue
		}

		if !found || v > p.Affinity[occasionDimension(best)] {
			best, found = o.occasion, true
		}
	}

	return best
}

func occasionDimension(occasion string) personalization.Dimension {
	switch occasion {
	case OccasionBusiness:
		return PersoDimensionBusiness
	case OccasionFestive:
		return PersoDimensionFestive
	default:
		return PersoDimensionCasual
	}
}

// TopMatch is the item this persona scores highest — by Score, in each item's category —
// ties broken by item id so the result
// is stable.
func (p Persona) TopMatch(items []Item) Item {
	var (
		best  Item
		score personalization.Value
	)

	for i, item := range items {
		v := Score(p.Affinity, item)
		if i == 0 || v > score || (v == score && item.ItemID < best.ItemID) {
			best, score = item, v
		}
	}

	return best
}

// How a shopper picks from a listing. Nobody reads a listing to the end and takes the
// optimum: they scan the first few products, consider those they like well enough, and
// lean towards what is shown first.
const (
	// ScanDepth is how many positions of a listing a shopper looks at.
	ScanDepth = 6

	// GoodEnough is how close to its favourite in a listing a product has to score for
	// the persona to consider it, as a share of the favourite's score.
	GoodEnough personalization.Value = 0.8
)

// Likes reports whether the persona would consider item, out of the products in among:
// its score is at least GoodEnough of the best score there.
func (p Persona) Likes(item Item, among []Item) bool {
	best := Score(p.Affinity, p.TopMatch(among))

	return Score(p.Affinity, item) >= GoodEnough*best
}

// Choose is the product the persona opens from a listing, in the listing's order. It
// considers the products it Likes among the first ScanDepth positions and picks one, the
// earlier positions more likely — position n weighs 1/n. If none of them is good enough,
// it scrolls on and opens its favourite in the whole listing.
//
// The pick is pseudo-random but seeded: the same persona on the same listing with the
// same seed opens the same product, so the generated traffic stays reproducible while a
// shopper does not simply take the top of every list — nor, from a listing that is not
// ranked for it yet, necessarily its best match.
func (p Persona) Choose(listing []Item, seed string) Item {
	var (
		candidates []Item
		weights    []float64
		total      float64
	)

	for n, item := range listing[:min(ScanDepth, len(listing))] {
		if p.Likes(item, listing) {
			w := 1 / float64(n+1)
			candidates, weights, total = append(candidates, item), append(weights, w), total+w
		}
	}

	if len(candidates) == 0 {
		return p.TopMatch(listing)
	}

	u := float64(seedHash(p.Name+"\x00"+seed)>>11) / float64(1<<53) * total

	for n, w := range weights {
		if u < w {
			return candidates[n]
		}

		u -= w
	}

	return candidates[len(candidates)-1]
}

// seedHash hashes a seed to 64 well-spread bits: FNV-1a, then the 64-bit finalizer from
// MurmurHash3. FNV alone mixes the last bytes of its input poorly into the high bits, and
// seeds that differ only in a trailing counter — the same listing on successive visits —
// would all land in the same corner of the range.
func seedHash(seed string) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(seed))
	x := h.Sum64()

	x ^= x >> 33
	x *= 0xff51afd7ed558ccd
	x ^= x >> 33
	x *= 0xc4ceb9fe1a85ec53
	x ^= x >> 33

	return x
}
