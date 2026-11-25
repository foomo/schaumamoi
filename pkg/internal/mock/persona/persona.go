package persona

import (
	"github.com/foomo/schaumamoi/pkg/internal/mock/catalog"
	"github.com/foomo/schaumamoi/pkg/internal/mock/internal/seed"
	"github.com/foomo/schaumamoi/pkg/internal/mock/taste"
	"github.com/foomo/schaumamoi/pkg/personalization"
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
	Kate = Persona{
		Name:        "Kate",
		Description: "50 year old conservative lawyer",
		Affinity: personalization.Map{
			taste.Business:   0.9,
			taste.Classic:    0.99,
			taste.Modern:     -1.0,
			taste.Structured: 0.8,
			taste.Edgy:       -0.5,
			taste.Retro:      0.3,
			taste.Cozy:       0.5,
		},
	}

	Sofia = Persona{
		Name:        "Sofia",
		Description: "28 year old event host with a taste for glamour",
		Affinity: personalization.Map{
			taste.Festive:      0.95,
			taste.Opulent:      0.85,
			taste.Feminine:     0.8,
			taste.Minimalistic: -0.7,
			taste.Romantic:     0.9,
			taste.Bold:         0.7,
			taste.Colourful:    0.5,
		},
	}

	Lena = Persona{
		Name:        "Lena",
		Description: "34 year old designer who dresses down and hates fuss",
		Affinity: personalization.Map{
			taste.Casual:       0.9,
			taste.Minimalistic: 0.8,
			taste.Modern:       0.6,
			taste.Opulent:      -0.9,
			taste.Bohemian:     0.5,
			taste.Edgy:         0.4,
			taste.Natural:      0.8,
		},
	}

	Mira = Persona{
		Name:        "Mira",
		Description: "41 year old architect, business wear but strictly contemporary",
		Affinity: personalization.Map{
			taste.Business:     0.7,
			taste.Modern:       0.9,
			taste.Minimalistic: 0.75,
			taste.Classic:      -0.6,
			taste.Edgy:         0.7,
			taste.Structured:   0.6,
			taste.Industrial:   0.8,
		},
	}
)

// Personas returns every persona in a stable order.
func All() []Persona {
	return []Persona{
		Kate,
		Sofia,
		Lena,
		Mira,
	}
}

// Occasion is the section of the shop this persona browses: the occasion dimension it
// scores highest on. Occasions double as personalization dimensions, so taste picks the
// section rather than it being hardcoded per persona.
func (p Persona) Occasion() string {
	best := catalog.OccasionCasual
	found := false

	// Iterated in a fixed order so ties resolve the same way on every run.
	for _, o := range []struct {
		occasion  string
		dimension personalization.Dimension
	}{
		{catalog.OccasionBusiness, taste.Business},
		{catalog.OccasionFestive, taste.Festive},
		{catalog.OccasionCasual, taste.Casual},
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
	case catalog.OccasionBusiness:
		return taste.Business
	case catalog.OccasionFestive:
		return taste.Festive
	default:
		return taste.Casual
	}
}

// TopMatch is the item this persona scores highest — by Score, in each item's category —
// ties broken by item id so the result
// is stable.
func (p Persona) TopMatch(items []catalog.Item) catalog.Item {
	var (
		best  catalog.Item
		score personalization.Value
	)

	for i, item := range items {
		v := catalog.Score(p.Affinity, item)
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
func (p Persona) Likes(item catalog.Item, among []catalog.Item) bool {
	best := catalog.Score(p.Affinity, p.TopMatch(among))

	return catalog.Score(p.Affinity, item) >= GoodEnough*best
}

// Choose is the product the persona opens from a listing, in the listing's order. It
// considers the products it Likes among the first ScanDepth positions and picks one, the
// earlier positions more likely — position n weighs 1/n. If none of them is good enough,
// it scrolls on and opens its favourite in the whole listing.
//
// The pick is pseudo-random but seeded: the same persona on the same listing with the
// same key opens the same product, so the generated traffic stays reproducible while a
// shopper does not simply take the top of every list — nor, from a listing that is not
// ranked for it yet, necessarily its best match.
func (p Persona) Choose(listing []catalog.Item, key string) catalog.Item {
	var (
		candidates []catalog.Item
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

	u := float64(seed.Hash(p.Name+"\x00"+key)>>11) / float64(1<<53) * total

	for n, w := range weights {
		if u < w {
			return candidates[n]
		}

		u -= w
	}

	return candidates[len(candidates)-1]
}
