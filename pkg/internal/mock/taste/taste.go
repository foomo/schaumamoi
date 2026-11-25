package taste

import (
	"github.com/foomo/schaumamoi/pkg/personalization"
)

const (
	Business     personalization.Dimension = "business"
	Festive      personalization.Dimension = "festive"
	Casual       personalization.Dimension = "casual"
	Modern       personalization.Dimension = "modern"
	Classic      personalization.Dimension = "classic"
	Minimalistic personalization.Dimension = "minimalistic"
	Opulent      personalization.Dimension = "opulent"
	Feminine     personalization.Dimension = "feminine"
	Bohemian     personalization.Dimension = "bohemian"
	Romantic     personalization.Dimension = "romantic"
	Structured   personalization.Dimension = "structured"
	Retro        personalization.Dimension = "retro"
	Edgy         personalization.Dimension = "edgy"
	Bold         personalization.Dimension = "bold"
	Colourful    personalization.Dimension = "colourful"

	// Home decoration speaks some of the dimensions above — modern, classic, minimalistic,
	// opulent, bohemian, romantic, retro, edgy, bold, colourful — and these of its own.
	// The fashion-only ones (business, festive, casual, feminine, structured) never appear
	// on a home item, so taste learned from clothes reaches the home department only
	// through what the two share.
	Cozy       personalization.Dimension = "cozy"
	Natural    personalization.Dimension = "natural"
	Industrial personalization.Dimension = "industrial"
)
