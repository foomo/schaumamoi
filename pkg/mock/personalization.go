package mock

import "github.com/foomo/schaumamoi/pkg/personalization"

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

// TopMatch is the item this persona scores highest, ties broken by item id so the result
// is stable.
func (p Persona) TopMatch(items []Item) Item {
	var (
		best  Item
		score personalization.Value
	)

	for i, item := range items {
		v := p.Affinity.Score(item.Attractor)
		if i == 0 || v > score || (v == score && item.ItemID < best.ItemID) {
			best, score = item, v
		}
	}

	return best
}
