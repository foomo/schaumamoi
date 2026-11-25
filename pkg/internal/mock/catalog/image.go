package catalog

import (
	"fmt"
	"math"
	"strings"

	"github.com/foomo/schaumamoi/pkg/internal/mock/internal/svg"
	"github.com/foomo/schaumamoi/pkg/internal/mock/taste"
)

// ImageDir is the directory the generated images live in, relative to mock.FixtureDir.
const ImageDir = "images"

// ImagePath is where the item's product shot is written, relative to mock.FixtureDir.
func (i Item) ImagePath() string { return ImageDir + "/" + strings.ToLower(i.ItemID) + ".svg" }

// Swatches maps every colour variant in the catalog to the fill its image is drawn in.
// TestSwatchesCoverCatalog keeps the two in step.
var Swatches = map[string]string{
	// dresses
	"navy":      "#1f2a44",
	"charcoal":  "#3a3d42",
	"black":     "#1a1a1a",
	"burgundy":  "#6d1a2c",
	"gold":      "#c9a227",
	"emerald":   "#0f6b4a",
	"champagne": "#e3cfa2",
	"blush":     "#f1c4c6",
	"sky":       "#8ec5e8",
	"grey":      "#9a9ea3",
	"sand":      "#d9c4a0",
	"rose":      "#d97a8a",

	// trousers, where the colour is not already on the dress rail
	"camel":  "#c19a6b",
	"silver": "#c0c4c9",
	"plum":   "#5e2750",
	"indigo": "#2f4a7a",
	"olive":  "#6b6b3a",
	"khaki":  "#a39a6b",

	// home decoration, where the colour is not already in the catalog
	"cream":      "#efe6d2",
	"mustard":    "#d4a017",
	"terracotta": "#c2643c",
}

// backdrops tint each section of the shop, so a product shot says which rail it hangs on.
var backdrops = map[string]string{
	OccasionBusiness: "#eceff3",
	OccasionFestive:  "#f6efe4",
	OccasionCasual:   "#edf3ec",
}

// SVG draws the product as a flat shot on a hanger, the way the shop would photograph
// it: a dress on a shoulder hanger, trousers on a clip hanger, a home piece standing on
// its own.
//
// Nothing is drawn per item id. The cut follows from the item's facets and attractor, and
// the fabric finish from the material its description names. Change a product's vector
// or copy and its picture follows, the same way a persona's journey follows its vector.
func (i Item) SVG() string {
	switch i.Category() {
	case CategoryTrousers:
		return i.trousersSVG()
	case CategoryCushions, CategoryLamps, CategoryVases:
		return i.homeSVG()
	}

	return i.dressSVG()
}

// open starts an item's image: the frame, its title and the backdrop of its occasion.
func (i Item) open(b *strings.Builder) {
	fmt.Fprintf(b, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 300 400" width="300" height="400" role="img" aria-label="%s, %s">`+"\n", i.ItemName, i.ItemVariant)
	fmt.Fprintf(b, "<title>%s (%s)</title>\n", i.ItemName, i.ItemID)
	fmt.Fprintf(b, `<rect width="300" height="400" fill="%s"/>`+"\n", Backdrop(i.Occasion()))
}

// fabric writes the gradient a sheen fabric is painted with, if the copy names one, and
// returns the paint for the body: the gradient, or the flat fill.
func (i Item) fabric(b *strings.Builder, id, fill string) string {
	paint := fill

	switch {
	case i.mentions("velvet"):
		// pile catches the light down the middle and falls dark at the edges
		fmt.Fprintf(b, `<linearGradient id="fabric-%s" x1="0" x2="1" y1="0" y2="0">`, id)
		svg.Stops(b, [][2]string{{"0", svg.Mix(fill, "#000000", 0.45)}, {"0.38", fill}, {"0.5", svg.Mix(fill, "#ffffff", 0.2)}, {"0.62", fill}, {"1", svg.Mix(fill, "#000000", 0.45)}})
		b.WriteString(`</linearGradient>`)

		paint = "url(#fabric-" + id + ")"
	case i.mentions("silk"):
		// a bias-cut sheen running diagonally across the body
		fmt.Fprintf(b, `<linearGradient id="fabric-%s" x1="0" x2="1" y1="0" y2="1">`, id)
		svg.Stops(b, [][2]string{{"0", fill}, {"0.42", svg.Mix(fill, "#ffffff", 0.45)}, {"0.58", fill}, {"1", svg.Mix(fill, "#000000", 0.15)}})
		b.WriteString(`</linearGradient>`)

		paint = "url(#fabric-" + id + ")"
	}

	return paint
}

// dressSVG draws a dress. The hem follows its length, the waist how feminine it is, the
// flare of the skirt how opulent, romantic or bohemian it is against how structured;
// structured and casual cuts get sleeves, the rest straps.
func (i Item) dressSVG() string {
	cut := cutOf(i)
	fill := Swatch(i.ItemVariant)
	line := svg.Mix(fill, "#000000", 0.35)
	id := strings.ToLower(i.ItemID)
	body := cut.path()

	var b strings.Builder

	i.open(&b)

	// the hanger, behind the dress
	fmt.Fprintf(&b, `<path d="M150,30 C150,18 162,18 162,26 M150,30 L88,62 L212,62 Z" fill="none" stroke="#8a8f96" stroke-width="3" stroke-linejoin="round" stroke-linecap="round"/>`+"\n")

	fmt.Fprintf(&b, `<defs><clipPath id="body-%s"><path d="%s"/></clipPath>`, id, body)
	paint := i.fabric(&b, id, fill)
	b.WriteString("</defs>\n")

	if !cut.sleeves {
		fmt.Fprintf(&b, `<path d="M%s L%s M%s L%s" stroke="%s" stroke-width="3" stroke-linecap="round"/>`+"\n",
			svg.Pt(cut.c-34, cut.top+1), svg.Pt(cut.c-28, 54), svg.Pt(cut.c+34, cut.top+1), svg.Pt(cut.c+28, 54), line)
	}

	fmt.Fprintf(&b, `<path d="%s" fill="%s"/>`+"\n", body, paint)
	fmt.Fprintf(&b, `<g clip-path="url(#body-%s)">`+"\n", id)
	dressFinish(&b, i, cut, fill, line)
	b.WriteString("</g>\n")
	fmt.Fprintf(&b, `<path d="%s" fill="none" stroke="%s" stroke-width="2" stroke-linejoin="round"/>`+"\n", body, line)

	if i.mentions("collar") {
		// a pointed collar lying over the neckline
		for _, side := range []float64{-1, 1} {
			fmt.Fprintf(&b, `<path d="M%s L%s L%s Z" fill="%s" stroke="%s" stroke-width="2" stroke-linejoin="round"/>`+"\n",
				svg.Pt(cut.c+side*cut.neck, 60), svg.Pt(cut.c+side*3, 60+cut.depth+4), svg.Pt(cut.c+side*(cut.neck+10), 82), svg.Mix(fill, "#ffffff", 0.25), line)
		}
	}

	if cut.v && i.mentions("wrap") {
		// the wrap's crossover, tied at the waist
		fmt.Fprintf(&b, `<path d="M%s L%s M%s l10,34 M%s l18,30" fill="none" stroke="%s" stroke-width="2" stroke-linecap="round"/>`+"\n",
			svg.Pt(cut.c-28, cut.top+2), svg.Pt(cut.c+cut.waist, cut.waistY), svg.Pt(cut.c+cut.waist, cut.waistY), svg.Pt(cut.c+cut.waist, cut.waistY), line)
	}

	b.WriteString("</svg>\n")

	return b.String()
}

// mentions reports whether the item's copy names a material or detail.
func (i Item) mentions(word string) bool {
	return strings.Contains(strings.ToLower(i.Description), word)
}

// cut is the geometry of one dress, derived once and shared by the outline and the
// finish drawn inside it.
type cut struct {
	c       float64 // centre line
	top     float64 // where the body starts: the shoulders, or the bodice under the straps
	neck    float64 // half-width of the neckline
	depth   float64 // how far the neckline drops
	waist   float64 // half-width at the waist
	waistY  float64
	hem     float64 // half-width at the hem
	hemY    float64
	sleeves bool
	v       bool // V neckline
	heart   bool // sweetheart neckline
}

func cutOf(i Item) cut {
	a := i.Attractor
	c := cut{c: 150, waistY: 170, neck: 22, depth: 14}

	switch i.Length() {
	case LengthMini:
		c.hemY = 250
	case LengthMaxi:
		c.hemY = 372
	default:
		c.hemY = 310
	}

	c.sleeves = a[taste.Structured] >= 0.4 || a[taste.Casual] >= 0.85
	c.waist = 42 - 14*float64(a[taste.Feminine])

	flare := a[taste.Opulent] + 0.5*a[taste.Romantic] + 0.5*a[taste.Bohemian] - 0.5*a[taste.Structured]
	c.hem = c.waist + 8 + 60*svg.Clamp(float64(flare), 0, 1.3)/1.3*(c.hemY-c.waistY)/200

	switch {
	case c.sleeves:
		c.top = 60
	case a[taste.Romantic] >= 0.7:
		c.top, c.heart = 100, true
	case a[taste.Minimalistic] >= 0.5:
		c.top = 100
	default:
		c.top, c.v, c.depth = 100, true, 34
	}

	return c
}

// path is the outline: down the left side, round the hem, up the right side, and back
// along the neckline.
func (c cut) path() string {
	var left [][2]float64
	if c.sleeves {
		left = [][2]float64{{c.neck, 60}, {50, 66}, {76, 112}, {54, 120}, {44, 110}}
	} else {
		left = [][2]float64{{42, c.top}}
	}

	left = append(left, [2]float64{c.waist, c.waistY}, [2]float64{c.hem, c.hemY})

	var b strings.Builder

	for n, p := range left {
		if n == 0 {
			b.WriteString("M")
		} else {
			b.WriteString(" L")
		}

		b.WriteString(svg.Pt(c.c-p[0], p[1]))
	}

	fmt.Fprintf(&b, " Q%s %s", svg.Pt(c.c, c.hemY+10), svg.Pt(c.c+c.hem, c.hemY))

	for n := len(left) - 2; n >= 0; n-- {
		b.WriteString(" L" + svg.Pt(c.c+left[n][0], left[n][1]))
	}

	start := left[0]

	switch {
	case c.heart:
		fmt.Fprintf(&b, " Q%s %s Q%s %s", svg.Pt(c.c+22, c.top-14), svg.Pt(c.c, c.top+8), svg.Pt(c.c-22, c.top-14), svg.Pt(c.c-start[0], start[1]))
	case c.v:
		fmt.Fprintf(&b, " L%s L%s", svg.Pt(c.c, c.top+c.depth), svg.Pt(c.c-start[0], start[1]))
	case c.sleeves:
		fmt.Fprintf(&b, " Q%s %s", svg.Pt(c.c, 60+2*c.depth), svg.Pt(c.c-start[0], start[1]))
	default:
		fmt.Fprintf(&b, " L%s", svg.Pt(c.c-start[0], start[1]))
	}

	b.WriteString(" Z")

	return b.String()
}

// halfWidth is the half-width of the skirt at height y, for details that follow it.
func (c cut) halfWidth(y float64) float64 {
	return c.waist + (c.hem-c.waist)*(y-c.waistY)/(c.hemY-c.waistY)
}

// dressFinish draws the fabric and construction details inside the body, which clips them.
func dressFinish(b *strings.Builder, i Item, c cut, fill, line string) {
	switch {
	case i.mentions("sequin"):
		svg.Sequins(b, fill, 70, 232, c.hemY+12)
	case i.mentions("floral"):
		for row, y := 0, 70.0; y < c.hemY+10; row, y = row+1, y+30 {
			for x := 80.0 + float64(row%2)*16; x < 230; x += 32 {
				flower(b, x, y)
			}
		}

		b.WriteString("\n")
	case i.mentions("tulle"):
		// the layers of the skirt, each a scalloped edge a little lighter than the last
		for n, k := range []float64{0.35, 0.6, 0.85} {
			y := c.waistY + (c.hemY-c.waistY)*k
			w := c.halfWidth(y)
			fmt.Fprintf(b, `<path d="M%s Q%s %s" fill="none" stroke="%s" stroke-width="2" stroke-opacity="0.8"/>`+"\n",
				svg.Pt(c.c-w, y), svg.Pt(c.c, y+12), svg.Pt(c.c+w, y), svg.Mix(fill, "#ffffff", 0.3+0.15*float64(n)))
		}
	}

	if i.mentions("seams") {
		fmt.Fprintf(b, `<path d="M%s L%s M%s L%s" stroke="%s" stroke-width="2"/>`+"\n",
			svg.Pt(c.c-16, 60), svg.Pt(c.c-22, c.hemY+10), svg.Pt(c.c+16, 60), svg.Pt(c.c+22, c.hemY+10), svg.Mix(fill, "#ffffff", 0.18))
	}

	if i.mentions("buttons") {
		fmt.Fprintf(b, `<path d="M%s L%s" stroke="%s" stroke-width="1.5"/>`, svg.Pt(c.c, 60+c.depth), svg.Pt(c.c, c.hemY+10), line)
		for y := 60 + c.depth + 14; y < c.hemY-6; y += 28 {
			fmt.Fprintf(b, `<circle cx="%s" cy="%s" r="3" fill="#f7f7f2" stroke="%s"/>`, svg.Num(c.c+6), svg.Num(y), line)
		}

		b.WriteString("\n")
	}

	if i.mentions("smock") {
		// gathered rows across the yoke
		for y := 120.0; y <= 144; y += 8 {
			fmt.Fprintf(b, `<path d="M%s q6,-4 12,0 t12,0 t12,0 t12,0 t12,0 t12,0 t12,0" fill="none" stroke="%s" stroke-width="1.5" stroke-opacity="0.6"/>`, svg.Pt(c.c-42, y), line)
		}

		b.WriteString("\n")
	}

	if i.Occasion() == OccasionBusiness && !c.v && !i.mentions("seams") {
		// a tailored waist seam
		fmt.Fprintf(b, `<path d="M%s L%s" stroke="%s" stroke-width="1.5" stroke-opacity="0.7"/>`+"\n",
			svg.Pt(c.c-c.waist-2, c.waistY), svg.Pt(c.c+c.waist+2, c.waistY), line)
	}
}

func flower(b *strings.Builder, x, y float64) {
	for n := range 5 {
		angle := float64(n) * 2 * math.Pi / 5
		fmt.Fprintf(b, `<circle cx="%s" cy="%s" r="3.6" fill="#fbf3ea"/>`, svg.Num(x+5*math.Sin(angle)), svg.Num(y-5*math.Cos(angle)))
	}

	fmt.Fprintf(b, `<circle cx="%s" cy="%s" r="2.4" fill="#e3b341"/><path d="M%s q4,6 10,6" fill="none" stroke="#5f8a52" stroke-width="2"/>`,
		svg.Num(x), svg.Num(y), svg.Pt(x+4, y+6))
}

func Swatch(colour string) string {
	if hex, ok := Swatches[colour]; ok {
		return hex
	}

	return "#b0b0b0"
}

func Backdrop(occasion string) string {
	if hex, ok := backdrops[occasion]; ok {
		return hex
	}

	return "#f2f2f2"
}

// legs is the geometry of a pair of trousers, derived once and shared by the outline and
// the details drawn on it. Only the left leg is stored; the right mirrors it.
type legs struct {
	c       float64 // centre line
	waistY  float64 // top of the waistband
	bandY   float64 // bottom of the waistband
	waist   float64 // half-width at the waist
	hipY    float64
	hip     float64 // half-width at the hip
	crotchY float64
	hemY    float64
	width   float64 // half-width of one leg at the hem
	centre  float64 // distance of a leg's centre from the centre line at the hem
	kneeY   float64 // where a flared leg is narrowest; zero for a straight one
}

// legsOf derives the cut. The hem follows the length. The leg's width at the hem is how
// opulent, romantic, bohemian, feminine and retro the trousers are, held in by how edgy,
// minimal and structured: the palazzo comes out wide, the cigarette trousers tapered.
// Copy that names a flare narrows the leg at the knee and lets it out below.
func legsOf(i Item) legs {
	a := i.Attractor
	l := legs{c: 150, waistY: 70, bandY: 84, waist: 50, hipY: 140, hip: 56, crotchY: 160}

	switch i.Length() {
	case LengthCropped:
		l.hemY = 290
	case LengthAnkle:
		l.hemY = 335
	default:
		l.hemY = 372
	}

	wide := 0.6*a[taste.Opulent] + a[taste.Romantic] + 0.8*a[taste.Bohemian] +
		0.5*a[taste.Feminine] + 0.5*a[taste.Retro] -
		0.6*a[taste.Edgy] - 0.4*a[taste.Minimalistic] - 0.4*a[taste.Structured]

	l.width = 22 + 12*svg.Clamp(float64(wide), -0.75, 1.9)
	l.centre = 30 + 0.4*math.Max(0, l.width-22)

	if i.mentions("flare") {
		l.kneeY = l.crotchY + (l.hemY-l.crotchY)*0.45
	}

	return l
}

// outer and inner are the x of the left leg's edges at the hem. A wide leg's inner edge
// stops just short of the centre line, so the two legs never merge into a skirt.
func (l legs) outer() float64 { return l.c - l.centre - l.width }

func (l legs) inner() float64 { return math.Min(l.c-l.centre+l.width, l.c-3) }

// path is the outline: down the outside of the left leg, across its hem, up to the
// crotch, down and across the right leg, up its outside, and back along the waist.
func (l legs) path() string {
	left := [][2]float64{{l.c - l.waist, l.waistY}, {l.c - l.hip, l.hipY}}
	if l.kneeY > 0 {
		left = append(left, [2]float64{l.c - l.centre - 16, l.kneeY})
	}

	left = append(left, [2]float64{l.outer(), l.hemY}, [2]float64{l.inner(), l.hemY})
	if l.kneeY > 0 {
		left = append(left, [2]float64{l.c - l.centre + 16, l.kneeY})
	}

	var b strings.Builder

	for n, p := range left {
		if n == 0 {
			b.WriteString("M")
		} else {
			b.WriteString(" L")
		}

		b.WriteString(svg.Pt(p[0], p[1]))
	}

	b.WriteString(" L" + svg.Pt(l.c, l.crotchY))

	for n := len(left) - 1; n >= 0; n-- {
		b.WriteString(" L" + svg.Pt(2*l.c-left[n][0], left[n][1]))
	}

	b.WriteString(" Z")

	return b.String()
}

// trousersSVG draws a pair of trousers hanging from a clip hanger by the waistband.
func (i Item) trousersSVG() string {
	l := legsOf(i)
	fill := Swatch(i.ItemVariant)
	line := svg.Mix(fill, "#000000", 0.35)
	id := strings.ToLower(i.ItemID)
	body := l.path()

	var b strings.Builder

	i.open(&b)

	// the hook and bar of the hanger, behind the trousers
	b.WriteString(`<path d="M150,30 C150,18 162,18 162,26 M150,30 L150,48 M84,48 L216,48" fill="none" stroke="#8a8f96" stroke-width="3" stroke-linecap="round"/>` + "\n")

	fmt.Fprintf(&b, `<defs><clipPath id="body-%s"><path d="%s"/></clipPath>`, id, body)
	paint := i.fabric(&b, id, fill)
	b.WriteString("</defs>\n")

	fmt.Fprintf(&b, `<path d="%s" fill="%s"/>`+"\n", body, paint)
	fmt.Fprintf(&b, `<g clip-path="url(#body-%s)">`+"\n", id)
	trousersFinish(&b, i, l, fill, line)
	b.WriteString("</g>\n")
	fmt.Fprintf(&b, `<path d="%s" fill="none" stroke="%s" stroke-width="2" stroke-linejoin="round"/>`+"\n", body, line)

	if i.mentions("drawstring") {
		fmt.Fprintf(&b, `<path d="M%s q-2,14 -6,28 M%s q4,12 10,24" fill="none" stroke="%s" stroke-width="2.5" stroke-linecap="round"/>`+"\n",
			svg.Pt(l.c-4, l.bandY-4), svg.Pt(l.c+4, l.bandY-4), svg.Mix(fill, "#ffffff", 0.6))
	}

	// the clips, over the waistband
	for _, x := range []float64{l.c - 44, l.c + 34} {
		fmt.Fprintf(&b, `<rect x="%s" y="44" width="10" height="34" rx="2" fill="#8a8f96"/>`, svg.Num(x))
	}

	b.WriteString("\n</svg>\n")

	return b.String()
}

// trousersFinish draws the fabric and construction details inside the body, which clips
// them.
func trousersFinish(b *strings.Builder, i Item, l legs, fill, line string) {
	switch {
	case i.mentions("sequin"):
		svg.Sequins(b, fill, 20, 280, l.hemY+12)
	case i.mentions("brocade"):
		// a small woven lozenge, alternating light and dark
		for row, y := 0, 92.0; y < l.hemY+10; row, y = row+1, y+14 {
			for x := 24.0 + float64(row%2)*9; x < 280; x += 18 {
				colour := svg.Mix(fill, "#ffffff", 0.35)
				if row%2 == 1 {
					colour = svg.Mix(fill, "#000000", 0.25)
				}

				fmt.Fprintf(b, `<path d="M%s l5,6 l-5,6 l-5,-6 Z" fill="%s"/>`, svg.Pt(x, y-6), colour)
			}
		}

		b.WriteString("\n")
	case i.mentions("check"):
		// a Prince of Wales check: a wide pale grid over a fine dark one
		for x := 24.0; x < 280; x += 24 {
			fmt.Fprintf(b, `<path d="M%s V400 M%s V400" stroke="%s" stroke-width="1"/>`, svg.Pt(x, 0), svg.Pt(x+6, 0), svg.Mix(fill, "#000000", 0.25))
			fmt.Fprintf(b, `<path d="M%s V400" stroke="%s" stroke-width="2" stroke-opacity="0.6"/>`, svg.Pt(x+12, 0), svg.Mix(fill, "#ffffff", 0.35))
		}

		for y := 90.0; y < 400; y += 24 {
			fmt.Fprintf(b, `<path d="M%s H300 M%s H300" stroke="%s" stroke-width="1"/>`, svg.Pt(0, y), svg.Pt(0, y+6), svg.Mix(fill, "#000000", 0.25))
			fmt.Fprintf(b, `<path d="M%s H300" stroke="%s" stroke-width="2" stroke-opacity="0.6"/>`, svg.Pt(0, y+12), svg.Mix(fill, "#ffffff", 0.35))
		}

		b.WriteString("\n")
	}

	// the waistband
	fmt.Fprintf(b, `<rect x="0" y="%s" width="300" height="%s" fill="%s" fill-opacity="0.55"/>`+"\n",
		svg.Num(l.waistY), svg.Num(l.bandY-l.waistY), svg.Mix(fill, "#000000", 0.3))

	if i.mentions("stripe") {
		// a satin stripe down each outside seam
		for _, side := range []float64{-1, 1} {
			fmt.Fprintf(b, `<path d="M%s L%s L%s" fill="none" stroke="%s" stroke-width="6"/>`,
				svg.Pt(l.c+side*(l.waist-2), l.bandY), svg.Pt(l.c+side*(l.hip-2), l.hipY), svg.Pt(l.c+side*(l.centre+l.width-2), l.hemY+4), svg.Mix(fill, "#ffffff", 0.3))
		}

		b.WriteString("\n")
	}

	if !i.mentions("drawstring") {
		// the fly
		fmt.Fprintf(b, `<path d="M%s L%s Q%s %s" fill="none" stroke="%s" stroke-width="1.5"/>`+"\n",
			svg.Pt(l.c, l.bandY), svg.Pt(l.c, l.crotchY-26), svg.Pt(l.c, l.crotchY-14), svg.Pt(l.c-12, l.crotchY-12), line)
	}

	for _, side := range []float64{-1, 1} {
		legX := l.c + side*l.centre // the leg's centre at the hem

		if i.mentions("pleat") {
			fmt.Fprintf(b, `<path d="M%s L%s" stroke="%s" stroke-width="1.5"/>`, svg.Pt(l.c+side*22, l.bandY), svg.Pt(l.c+side*26, l.hipY+10), line)
		}

		if i.mentions("pleat") || i.mentions("crease") {
			// a pressed crease down the front of the leg
			fmt.Fprintf(b, `<path d="M%s L%s" stroke="%s" stroke-width="1.2" stroke-opacity="0.8"/>`, svg.Pt(l.c+side*28, l.hipY), svg.Pt(legX, l.hemY), svg.Mix(fill, "#ffffff", 0.25))
		}

		if i.mentions("denim") {
			// scooped front pockets and the orange topstitching that says jeans
			fmt.Fprintf(b, `<path d="M%s Q%s %s" fill="none" stroke="#d08a3a" stroke-width="1.5" stroke-dasharray="3 2"/>`,
				svg.Pt(l.c+side*(l.waist-4), l.bandY), svg.Pt(l.c+side*(l.waist-6), l.bandY+26), svg.Pt(l.c+side*(l.waist-26), l.bandY+28))
		}

		if i.mentions("patch pocket") {
			// on the outside of the thigh, with a flap
			x := l.c - l.hip + 4
			if side > 0 {
				x = l.c + l.hip - 4 - 30
			}

			fmt.Fprintf(b, `<rect x="%s" y="200" width="30" height="36" rx="2" fill="none" stroke="%s" stroke-width="1.5"/><path d="M%s h30" stroke="%s" stroke-width="1.5"/>`,
				svg.Num(x), line, svg.Pt(x, 210), line)
		}

		if i.mentions("cuffs") {
			x0, x1 := l.outer(), l.inner()
			if side > 0 {
				x0, x1 = 2*l.c-l.inner(), 2*l.c-l.outer()
			}

			fmt.Fprintf(b, `<rect x="%s" y="%s" width="%s" height="14" fill="%s"/>`, svg.Num(x0), svg.Num(l.hemY-14), svg.Num(x1-x0), svg.Mix(fill, "#000000", 0.2))
		}
	}

	b.WriteString("\n")
}

// homeSVG draws a home decoration piece standing on its own. The shape follows the
// category — a cushion, a lamp, a vase — and the form and finish follow the copy, as for
// the clothes: a pendant hangs, a floor lamp stands tall, a ginger jar has its lid, a
// pleated shade is pleated.
func (i Item) homeSVG() string {
	fill := Swatch(i.ItemVariant)
	line := svg.Mix(fill, "#000000", 0.35)
	id := strings.ToLower(i.ItemID)

	var b strings.Builder

	i.open(&b)

	switch i.Category() {
	case CategoryCushions:
		i.cushion(&b, id, fill, line)
	case CategoryLamps:
		i.lamp(&b, id, fill, line)
	default:
		i.vase(&b, id, fill, line)
	}

	b.WriteString("</svg>\n")

	return b.String()
}

// shadow grounds a piece: a soft ellipse where it stands.
func shadow(b *strings.Builder, y, rx float64) {
	fmt.Fprintf(b, `<ellipse cx="150" cy="%s" rx="%s" ry="8" fill="#000000" fill-opacity="0.08"/>`+"\n", svg.Num(y), svg.Num(rx))
}

// clipped draws body, clips whatever finish draws to it, and outlines it.
func clipped(b *strings.Builder, id, body, paint, line string, width float64, finish func()) {
	fmt.Fprintf(b, `<defs><clipPath id="body-%s"><path d="%s"/></clipPath></defs>`+"\n", id, body)
	fmt.Fprintf(b, `<path d="%s" fill="%s"/>`+"\n", body, paint)
	fmt.Fprintf(b, `<g clip-path="url(#body-%s)">`+"\n", id)
	finish()
	b.WriteString("</g>\n")
	fmt.Fprintf(b, `<path d="%s" fill="none" stroke="%s" stroke-width="%s" stroke-linejoin="round"/>`+"\n", body, line, svg.Num(width))
}

func (i Item) cushion(b *strings.Builder, id, fill, line string) {
	shadow(b, 322, 100)

	body := "M60,120 Q150,104 240,120 Q256,210 240,300 Q150,316 60,300 Q44,210 60,120 Z"

	var defs strings.Builder
	paint := i.fabric(&defs, id+"-fabric", fill)
	if defs.Len() > 0 {
		fmt.Fprintf(b, "<defs>%s</defs>\n", defs.String())
	}

	clipped(b, id, body, paint, line, 3, func() {
		switch {
		case i.mentions("knit"):
			// cable braids running top to bottom
			for x := 84.0; x <= 216; x += 33 {
				fmt.Fprintf(b, `<path d="M%s c12,14 -12,28 0,42 c12,14 -12,28 0,42 c12,14 -12,28 0,42 c12,14 -12,28 0,42 c12,14 -12,28 0,42" fill="none" stroke="%s" stroke-width="7" stroke-opacity="0.3"/>`,
					svg.Pt(x, 100), line)
			}

			b.WriteString("\n")
		case i.mentions("linen"):
			// a fine, uneven weave
			for y := 112.0; y < 312; y += 6 {
				fmt.Fprintf(b, `<path d="M40,%s H260" stroke="%s" stroke-width="1" stroke-opacity="0.12"/>`, svg.Num(y), line)
			}

			b.WriteString("\n")
		case i.mentions("floral"):
			for row, y := 0, 124.0; y < 310; row, y = row+1, y+34 {
				for x := 70.0 + float64(row%2)*20; x < 240; x += 40 {
					flower(b, x, y)
				}
			}

			b.WriteString("\n")
		case i.mentions("bouclé"):
			// the nubby loops of bouclé
			for row, y := 0, 112.0; y < 312; row, y = row+1, y+9 {
				for x := 52.0 + float64(row%2)*5; x < 250; x += 10 {
					fmt.Fprintf(b, `<circle cx="%s" cy="%s" r="3" fill="none" stroke="%s" stroke-opacity="0.35"/>`, svg.Num(x), svg.Num(y), line)
				}
			}

			b.WriteString("\n")
		case i.mentions("geometric"):
			for row, y := 0, 104.0; y < 320; row, y = row+1, y+36 {
				for x := 40.0 + float64(row%2)*20; x < 260; x += 40 {
					fmt.Fprintf(b, `<path d="M%s l40,0 l-20,32 Z" fill="#1d1d1d"/>`, svg.Pt(x, y))
				}
			}

			b.WriteString("\n")
		}
	})

	if i.mentions("tassel") {
		for _, c := range [][2]float64{{60, 120}, {240, 120}, {240, 300}, {60, 300}} {
			dx, dy := c[0]-150, c[1]-210
			dx, dy = dx/math.Hypot(dx, dy)*18, dy/math.Hypot(dx, dy)*18
			fmt.Fprintf(b, `<path d="M%s l%s,%s m-4,0 l%s,%s m8,0 l%s,%s" stroke="#d4af37" stroke-width="2.5" stroke-linecap="round"/><circle cx="%s" cy="%s" r="4" fill="#d4af37"/>`,
				svg.Pt(c[0], c[1]), svg.Num(dx), svg.Num(dy), svg.Num(dx), svg.Num(dy), svg.Num(dx), svg.Num(dy), svg.Num(c[0]), svg.Num(c[1]))
		}

		b.WriteString("\n")
	}
}

func (i Item) lamp(b *strings.Builder, id, fill, line string) {
	shade := svg.Mix(fill, "#ffffff", 0.55)
	if i.mentions("concrete") {
		shade = fill
	}

	shadeLine := svg.Mix(shade, "#000000", 0.35)

	switch {
	case i.mentions("pendant"):
		fmt.Fprintf(b, `<path d="M150,0 V122" stroke="#1d1d1d" stroke-width="3"/>`+"\n")
		fmt.Fprintf(b, `<ellipse cx="150" cy="208" rx="36" ry="9" fill="#fff3c4"/>`+"\n")
		body := "M78,206 Q78,122 150,120 Q222,122 222,206 Z"
		clipped(b, id, body, shade, shadeLine, 2, func() {
			if i.mentions("concrete") {
				// the pitted surface of cast concrete
				for n := range 40 {
					x, y := 84+float64((n*37)%136), 128+float64((n*53)%76)
					fmt.Fprintf(b, `<circle cx="%s" cy="%s" r="1.6" fill="%s"/>`, svg.Num(x), svg.Num(y), shadeLine)
				}

				b.WriteString("\n")
			}
		})
	case i.mentions("floor"):
		shadow(b, 372, 48)
		fmt.Fprintf(b, `<ellipse cx="150" cy="368" rx="40" ry="7" fill="%s"/><path d="M150,368 V150" stroke="%s" stroke-width="5"/>`+"\n", line, line)
		fmt.Fprintf(b, `<ellipse cx="150" cy="152" rx="60" ry="10" fill="#fff3c4" fill-opacity="0.7"/>`+"\n")
		body := "M102,56 L198,56 L226,150 L74,150 Z"
		clipped(b, id, body, shade, shadeLine, 2, func() {
			if i.mentions("woven") || i.mentions("rattan") {
				for x := 60.0; x < 250; x += 10 {
					fmt.Fprintf(b, `<path d="M%s l40,100 M%s l-40,100" stroke="%s" stroke-width="2" stroke-opacity="0.45"/>`, svg.Pt(x, 52), svg.Pt(x, 52), shadeLine)
				}

				b.WriteString("\n")
			}
		})
	default: // a table lamp: a base, a rod, a shade
		shadow(b, 346, 56)
		fmt.Fprintf(b, `<path d="M150,214 V246" stroke="%s" stroke-width="4"/>`+"\n", line)

		var base string

		switch {
		case i.mentions("crystal"):
			base = "M150,232 L192,282 L150,334 L108,282 Z"
		default:
			base = "M132,340 L168,340 L164,304 Q192,272 164,240 L136,240 Q108,272 136,304 Z"
		}

		clipped(b, id+"-base", base, fill, line, 2, func() {
			if i.mentions("crystal") {
				fmt.Fprintf(b, `<path d="M150,232 L150,334 M108,282 L192,282 M129,257 L171,307 M171,257 L129,307" stroke="#ffffff" stroke-width="1.5" stroke-opacity="0.7"/>`+"\n")
			} else {
				// a polished highlight
				fmt.Fprintf(b, `<path d="M140,244 Q124,272 142,300" fill="none" stroke="#ffffff" stroke-width="4" stroke-opacity="0.45"/>`+"\n")
			}
		})

		if i.mentions("crystal") {
			fmt.Fprintf(b, `<rect x="122" y="334" width="56" height="10" rx="3" fill="%s"/>`+"\n", line)
		}

		clipped(b, id, "M98,118 L202,118 L234,214 L66,214 Z", shade, shadeLine, 2, func() {
			if i.mentions("pleat") {
				for n := 0; n <= 14; n++ {
					t := float64(n) / 14
					fmt.Fprintf(b, `<path d="M%s L%s" stroke="%s" stroke-width="1.2" stroke-opacity="0.5"/>`, svg.Pt(98+104*t, 118), svg.Pt(66+168*t, 214), shadeLine)
				}

				b.WriteString("\n")
			}
		})
	}
}

func (i Item) vase(b *strings.Builder, id, fill, line string) {
	shadow(b, 346, 64)

	var body string

	switch {
	case i.mentions("cylinder"):
		body = "M112,150 L188,150 L188,342 Q150,348 112,342 Z"
	case i.mentions("jar"):
		body = "M114,196 Q78,262 116,342 L184,342 Q222,262 186,196 Z"
	case i.mentions("stoneware"):
		body = "M126,168 L174,168 L171,190 Q236,252 186,340 L114,340 Q64,252 129,190 Z"
	default: // a blown amphora
		body = "M130,138 L170,138 Q160,170 186,200 Q232,282 176,342 L124,342 Q68,282 114,200 Q140,170 130,138 Z"
	}

	var defs strings.Builder
	paint := fill

	if i.mentions("smoked") {
		// light passing through the middle of tinted glass
		fmt.Fprintf(&defs, `<linearGradient id="glass-%s" x1="0" x2="1" y1="0" y2="0">`, id)
		svg.Stops(&defs, [][2]string{{"0", fill}, {"0.45", svg.Mix(fill, "#ffffff", 0.25)}, {"0.6", fill}, {"1", svg.Mix(fill, "#000000", 0.4)}})
		defs.WriteString(`</linearGradient>`)

		paint = "url(#glass-" + id + ")"
	}

	if defs.Len() > 0 {
		fmt.Fprintf(b, "<defs>%s</defs>\n", defs.String())
	}

	clipped(b, id, body, paint, line, 2, func() {
		switch {
		case i.mentions("blue and white"):
			// painted sprays and a band at the shoulder
			fmt.Fprintf(b, `<path d="M90,226 H210 M90,232 H210" stroke="#f4f1ea" stroke-width="2"/>`)
			for _, c := range [][2]float64{{130, 270}, {172, 296}, {142, 320}, {176, 252}} {
				flower(b, c[0], c[1])
			}

			b.WriteString("\n")
		case i.mentions("swirl"):
			for n, colour := range []string{"#e0a030", svg.Mix(fill, "#ffffff", 0.45), "#e0a030"} {
				x := 104 + float64(n)*30
				fmt.Fprintf(b, `<path d="M%s C%s %s %s" fill="none" stroke="%s" stroke-width="9" stroke-opacity="0.75"/>`,
					svg.Pt(x, 150), svg.Pt(x+60, 210), svg.Pt(x-50, 280), svg.Pt(x+20, 350), colour)
			}

			b.WriteString("\n")
		case i.mentions("raw foot"):
			// the unglazed foot, and speckles in the glaze
			fmt.Fprintf(b, `<rect x="60" y="318" width="180" height="30" fill="%s"/>`, svg.Mix(fill, "#ffffff", 0.35))
			for n := range 30 {
				x, y := 90+float64((n*41)%120), 180+float64((n*29)%130)
				fmt.Fprintf(b, `<circle cx="%s" cy="%s" r="1.4" fill="%s"/>`, svg.Num(x), svg.Num(y), line)
			}

			b.WriteString("\n")
		}
	})

	if i.mentions("lid") {
		fmt.Fprintf(b, `<path d="M118,198 Q150,164 182,198 Z" fill="%s" stroke="%s" stroke-width="2"/><circle cx="150" cy="170" r="7" fill="%s" stroke="%s" stroke-width="2"/>`+"\n",
			fill, line, fill, line)
	}
}
