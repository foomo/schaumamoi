package mock

import (
	"fmt"
	"hash/fnv"
	"math"
	"strings"

	"github.com/foomo/schaumamoi/pkg/personalization"
)

// ImageDir is the directory the generated images live in, relative to FixtureDir.
const ImageDir = "images"

// Images returns every generated image keyed by its path relative to FixtureDir: one
// product shot per dress and one avatar per persona. Like Fixtures, it is shared by the
// generator and the golden test.
func Images() map[string]string {
	out := map[string]string{}
	for _, item := range Catalog() {
		out[item.ImagePath()] = item.SVG()
	}

	for _, p := range Personas() {
		out[p.ImagePath()] = p.SVG()
	}

	return out
}

// ImagePath is where the item's product shot is written, relative to FixtureDir.
func (i Item) ImagePath() string { return ImageDir + "/" + strings.ToLower(i.ItemID) + ".svg" }

// ImagePath is where the persona's avatar is written, relative to FixtureDir.
func (p Persona) ImagePath() string { return ImageDir + "/persona_" + strings.ToLower(p.Name) + ".svg" }

// Swatches maps every colour variant in the catalog to the fill its image is drawn in.
// TestSwatchesCoverCatalog keeps the two in step.
var Swatches = map[string]string{
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
}

// backdrops tint each section of the shop, so a product shot says which rail it hangs on.
var backdrops = map[string]string{
	OccasionBusiness: "#eceff3",
	OccasionFestive:  "#f6efe4",
	OccasionCasual:   "#edf3ec",
}

// SVG draws the dress as a flat product shot on a hanger.
//
// Nothing is drawn per item id. The cut follows from the item's facets and attractor —
// the hem from its length, the waist from how feminine it is, the flare of the skirt
// from how opulent, romantic or bohemian it is against how structured, sleeves for
// structured and casual cuts and straps for the rest. The fabric finish follows from the
// material its description names. Change a dress's vector or copy and its picture
// follows, the same way a persona's journey follows its vector.
func (i Item) SVG() string {
	cut := cutOf(i)
	fill := swatch(i.ItemVariant)
	line := mix(fill, "#000000", 0.35)
	id := strings.ToLower(i.ItemID)
	body := cut.path()

	var b strings.Builder

	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 300 400" width="300" height="400" role="img" aria-label="%s, %s">`+"\n", i.ItemName, i.ItemVariant)
	fmt.Fprintf(&b, "<title>%s (%s)</title>\n", i.ItemName, i.ItemID)
	fmt.Fprintf(&b, `<rect width="300" height="400" fill="%s"/>`+"\n", backdrop(i.Occasion()))

	// the hanger, behind the dress
	fmt.Fprintf(&b, `<path d="M150,30 C150,18 162,18 162,26 M150,30 L88,62 L212,62 Z" fill="none" stroke="#8a8f96" stroke-width="3" stroke-linejoin="round" stroke-linecap="round"/>`+"\n")

	fmt.Fprintf(&b, `<defs><clipPath id="body-%s"><path d="%s"/></clipPath>`, id, body)

	paint := fill

	switch {
	case i.mentions("velvet"):
		// pile catches the light down the middle and falls dark at the edges
		fmt.Fprintf(&b, `<linearGradient id="fabric-%s" x1="0" x2="1" y1="0" y2="0">`, id)
		stops(&b, [][2]string{{"0", mix(fill, "#000000", 0.45)}, {"0.38", fill}, {"0.5", mix(fill, "#ffffff", 0.2)}, {"0.62", fill}, {"1", mix(fill, "#000000", 0.45)}})
		b.WriteString(`</linearGradient>`)

		paint = "url(#fabric-" + id + ")"
	case i.mentions("silk"):
		// a bias-cut sheen running diagonally across the body
		fmt.Fprintf(&b, `<linearGradient id="fabric-%s" x1="0" x2="1" y1="0" y2="1">`, id)
		stops(&b, [][2]string{{"0", fill}, {"0.42", mix(fill, "#ffffff", 0.45)}, {"0.58", fill}, {"1", mix(fill, "#000000", 0.15)}})
		b.WriteString(`</linearGradient>`)

		paint = "url(#fabric-" + id + ")"
	}

	b.WriteString("</defs>\n")

	if !cut.sleeves {
		fmt.Fprintf(&b, `<path d="M%s L%s M%s L%s" stroke="%s" stroke-width="3" stroke-linecap="round"/>`+"\n",
			pt(cut.c-34, cut.top+1), pt(cut.c-28, 54), pt(cut.c+34, cut.top+1), pt(cut.c+28, 54), line)
	}

	fmt.Fprintf(&b, `<path d="%s" fill="%s"/>`+"\n", body, paint)
	fmt.Fprintf(&b, `<g clip-path="url(#body-%s)">`+"\n", id)
	finish(&b, i, cut, fill, line)
	b.WriteString("</g>\n")
	fmt.Fprintf(&b, `<path d="%s" fill="none" stroke="%s" stroke-width="2" stroke-linejoin="round"/>`+"\n", body, line)

	if i.mentions("collar") {
		// a pointed collar lying over the neckline
		for _, side := range []float64{-1, 1} {
			fmt.Fprintf(&b, `<path d="M%s L%s L%s Z" fill="%s" stroke="%s" stroke-width="2" stroke-linejoin="round"/>`+"\n",
				pt(cut.c+side*cut.neck, 60), pt(cut.c+side*3, 60+cut.depth+4), pt(cut.c+side*(cut.neck+10), 82), mix(fill, "#ffffff", 0.25), line)
		}
	}

	if cut.v && i.mentions("wrap") {
		// the wrap's crossover, tied at the waist
		fmt.Fprintf(&b, `<path d="M%s L%s M%s l10,34 M%s l18,30" fill="none" stroke="%s" stroke-width="2" stroke-linecap="round"/>`+"\n",
			pt(cut.c-28, cut.top+2), pt(cut.c+cut.waist, cut.waistY), pt(cut.c+cut.waist, cut.waistY), pt(cut.c+cut.waist, cut.waistY), line)
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

	c.sleeves = a[PersoDimensionStructured] >= 0.4 || a[PersoDimensionCasual] >= 0.85
	c.waist = 42 - 14*float64(a[PersoDimensionFeminine])

	flare := a[PersoDimensionOpulent] + 0.5*a[PersoDimensionRomantic] + 0.5*a[PersoDimensionBohemian] - 0.5*a[PersoDimensionStructured]
	c.hem = c.waist + 8 + 60*clamp(float64(flare), 0, 1.3)/1.3*(c.hemY-c.waistY)/200

	switch {
	case c.sleeves:
		c.top = 60
	case a[PersoDimensionRomantic] >= 0.7:
		c.top, c.heart = 100, true
	case a[PersoDimensionMinimalistic] >= 0.5:
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

		b.WriteString(pt(c.c-p[0], p[1]))
	}

	fmt.Fprintf(&b, " Q%s %s", pt(c.c, c.hemY+10), pt(c.c+c.hem, c.hemY))

	for n := len(left) - 2; n >= 0; n-- {
		b.WriteString(" L" + pt(c.c+left[n][0], left[n][1]))
	}

	start := left[0]

	switch {
	case c.heart:
		fmt.Fprintf(&b, " Q%s %s Q%s %s", pt(c.c+22, c.top-14), pt(c.c, c.top+8), pt(c.c-22, c.top-14), pt(c.c-start[0], start[1]))
	case c.v:
		fmt.Fprintf(&b, " L%s L%s", pt(c.c, c.top+c.depth), pt(c.c-start[0], start[1]))
	case c.sleeves:
		fmt.Fprintf(&b, " Q%s %s", pt(c.c, 60+2*c.depth), pt(c.c-start[0], start[1]))
	default:
		fmt.Fprintf(&b, " L%s", pt(c.c-start[0], start[1]))
	}

	b.WriteString(" Z")

	return b.String()
}

// halfWidth is the half-width of the skirt at height y, for details that follow it.
func (c cut) halfWidth(y float64) float64 {
	return c.waist + (c.hem-c.waist)*(y-c.waistY)/(c.hemY-c.waistY)
}

// finish draws the fabric and construction details inside the body, which clips them.
func finish(b *strings.Builder, i Item, c cut, fill, line string) {
	switch {
	case i.mentions("sequin"):
		light, dark := mix(fill, "#ffffff", 0.5), mix(fill, "#000000", 0.2)
		for row, y := 0, 50.0; y < c.hemY+12; row, y = row+1, y+10 {
			for x := 70.0 + float64(row%2)*5; x < 232; x += 10 {
				colour := light
				if (row+int(x/10))%3 == 0 {
					colour = dark
				}

				fmt.Fprintf(b, `<circle cx="%s" cy="%s" r="2.6" fill="%s"/>`, num(x), num(y), colour)
			}
		}

		b.WriteString("\n")
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
				pt(c.c-w, y), pt(c.c, y+12), pt(c.c+w, y), mix(fill, "#ffffff", 0.3+0.15*float64(n)))
		}
	}

	if i.mentions("seams") {
		fmt.Fprintf(b, `<path d="M%s L%s M%s L%s" stroke="%s" stroke-width="2"/>`+"\n",
			pt(c.c-16, 60), pt(c.c-22, c.hemY+10), pt(c.c+16, 60), pt(c.c+22, c.hemY+10), mix(fill, "#ffffff", 0.18))
	}

	if i.mentions("buttons") {
		fmt.Fprintf(b, `<path d="M%s L%s" stroke="%s" stroke-width="1.5"/>`, pt(c.c, 60+c.depth), pt(c.c, c.hemY+10), line)
		for y := 60 + c.depth + 14; y < c.hemY-6; y += 28 {
			fmt.Fprintf(b, `<circle cx="%s" cy="%s" r="3" fill="#f7f7f2" stroke="%s"/>`, num(c.c+6), num(y), line)
		}

		b.WriteString("\n")
	}

	if i.mentions("smock") {
		// gathered rows across the yoke
		for y := 120.0; y <= 144; y += 8 {
			fmt.Fprintf(b, `<path d="M%s q6,-4 12,0 t12,0 t12,0 t12,0 t12,0 t12,0 t12,0" fill="none" stroke="%s" stroke-width="1.5" stroke-opacity="0.6"/>`, pt(c.c-42, y), line)
		}

		b.WriteString("\n")
	}

	if i.Occasion() == OccasionBusiness && !c.v && !i.mentions("seams") {
		// a tailored waist seam
		fmt.Fprintf(b, `<path d="M%s L%s" stroke="%s" stroke-width="1.5" stroke-opacity="0.7"/>`+"\n",
			pt(c.c-c.waist-2, c.waistY), pt(c.c+c.waist+2, c.waistY), line)
	}
}

func flower(b *strings.Builder, x, y float64) {
	for n := range 5 {
		angle := float64(n) * 2 * math.Pi / 5
		fmt.Fprintf(b, `<circle cx="%s" cy="%s" r="3.6" fill="#fbf3ea"/>`, num(x+5*math.Sin(angle)), num(y-5*math.Cos(angle)))
	}

	fmt.Fprintf(b, `<circle cx="%s" cy="%s" r="2.4" fill="#e3b341"/><path d="M%s q4,6 10,6" fill="none" stroke="#5f8a52" stroke-width="2"/>`,
		num(x), num(y), pt(x+4, y+6))
}

// SVG draws the persona as an avatar, wearing the colour of the dress they score highest.
//
// As with the dresses, nothing is drawn per name. The hairstyle is the taste the persona
// leans to most — classic and structured wear a bob, romantic and feminine long hair,
// casual and bohemian a bun, modern and edgy a pixie cut — and the accessories follow
// the strongest single traits. Skin and hair colour come from a palette by the persona's
// place in Personas(), so the four never share a look; a stated age of fifty or more
// turns the hair silver.
func (p Persona) SVG() string {
	a := p.Affinity
	top := p.TopMatch(Catalog())
	outfit := swatch(top.ItemVariant)
	skins := []string{"#f2d3b8", "#e0ac82", "#c68642", "#8d5524"}
	hairs := []string{"#2b1d14", "#6b3f22", "#a0522d", "#d6b06a"}
	n := p.slot()
	skin, hair := skins[n%len(skins)], hairs[(n+1)%len(hairs)]

	var age int
	if _, err := fmt.Sscanf(p.Description, "%d", &age); err == nil && age >= 50 {
		hair = "#c4c4c8"
	}

	shade := mix(skin, "#000000", 0.12)
	id := strings.ToLower(p.Name)

	var b strings.Builder

	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 300 300" width="300" height="300" role="img" aria-label="%s, %s">`+"\n", p.Name, p.Description)
	fmt.Fprintf(&b, "<title>%s, %s</title>\n", p.Name, p.Description)
	fmt.Fprintf(&b, `<defs><clipPath id="avatar-%s"><circle cx="150" cy="150" r="150"/></clipPath></defs>`+"\n", id)
	fmt.Fprintf(&b, `<g clip-path="url(#avatar-%s)">`+"\n", id)
	fmt.Fprintf(&b, `<rect width="300" height="300" fill="%s"/>`+"\n", mix(outfit, "#ffffff", 0.78))

	style := hairstyle(a)

	switch style {
	case "long":
		fmt.Fprintf(&b, `<path d="M100,140 C96,80 204,80 200,140 L210,250 C180,262 120,262 90,250 Z" fill="%s"/>`+"\n", hair)
	case "bob":
		fmt.Fprintf(&b, `<path d="M102,145 C98,82 202,82 198,145 L200,188 L100,188 Z" fill="%s"/>`+"\n", hair)
	case "bun":
		fmt.Fprintf(&b, `<circle cx="150" cy="82" r="24" fill="%s"/>`+"\n", hair)
	}

	// neck, shoulders and the neckline of the dress
	fmt.Fprintf(&b, `<rect x="134" y="170" width="32" height="56" rx="10" fill="%s"/>`+"\n", shade)
	fmt.Fprintf(&b, `<path d="M52,300 C54,238 96,218 150,218 C204,218 246,238 248,300 Z" fill="%s"/>`+"\n", outfit)
	fmt.Fprintf(&b, `<path d="M128,218 L150,248 L172,218 Z" fill="%s"/>`+"\n", shade)

	// head and face
	fmt.Fprintf(&b, `<ellipse cx="150" cy="140" rx="42" ry="50" fill="%s"/>`+"\n", skin)
	fmt.Fprintf(&b, `<circle cx="134" cy="146" r="3.5" fill="#2a2a2a"/><circle cx="166" cy="146" r="3.5" fill="#2a2a2a"/>`)
	fmt.Fprintf(&b, `<path d="M139,168 Q150,176 161,168" fill="none" stroke="#7a3b32" stroke-width="2.5" stroke-linecap="round"/>`+"\n")

	switch style {
	case "long":
		fmt.Fprintf(&b, `<path d="M106,145 C102,86 198,86 194,145 C186,110 160,104 150,104 C132,106 114,116 106,145 Z" fill="%s"/>`+"\n", hair)
	case "bob":
		fmt.Fprintf(&b, `<path d="M104,150 C100,86 200,86 196,150 C190,124 176,116 150,116 C126,116 112,124 104,150 Z" fill="%s"/>`+"\n", hair)
	case "bun":
		fmt.Fprintf(&b, `<path d="M108,136 C108,90 192,90 192,136 C176,110 124,110 108,136 Z" fill="%s"/>`+"\n", hair)
	default: // pixie: short, swept to one side
		fmt.Fprintf(&b, `<path d="M106,138 C98,84 200,78 196,130 C176,106 138,112 106,138 Z" fill="%s"/>`+"\n", hair)
	}

	if a[PersoDimensionEdgy] >= 0.7 {
		// square, heavy frames
		b.WriteString(`<g fill="none" stroke="#1d1d1d" stroke-width="3"><rect x="120" y="136" width="26" height="20" rx="3"/><rect x="154" y="136" width="26" height="20" rx="3"/><path d="M146,144 L154,144"/></g>` + "\n")
	}

	if a[PersoDimensionOpulent] >= 0.8 {
		b.WriteString(`<circle cx="108" cy="162" r="6" fill="#d4af37"/><circle cx="192" cy="162" r="6" fill="#d4af37"/>` + "\n")
	}

	if a[PersoDimensionClassic] >= 0.9 {
		// a string of pearls along the neckline
		for n := range 9 {
			t := float64(n) / 8
			x := 126 + 48*t
			y := 222 + 18*math.Sin(t*math.Pi)
			fmt.Fprintf(&b, `<circle cx="%s" cy="%s" r="3.6" fill="#f8f5ee" stroke="#d8d2c4"/>`, num(x), num(y))
		}

		b.WriteString("\n")
	}

	b.WriteString("</g>\n</svg>\n")

	return b.String()
}

// hairstyle is the style of the taste group the persona leans to most. Groups are
// compared in a fixed order, so ties resolve the same way on every run.
func hairstyle(a personalization.Map) string {
	best, score := "", personalization.Value(0)

	for n, g := range []struct {
		style string
		dims  [2]personalization.Dimension
	}{
		{"bob", [2]personalization.Dimension{PersoDimensionClassic, PersoDimensionStructured}},
		{"long", [2]personalization.Dimension{PersoDimensionRomantic, PersoDimensionFeminine}},
		{"bun", [2]personalization.Dimension{PersoDimensionCasual, PersoDimensionBohemian}},
		{"pixie", [2]personalization.Dimension{PersoDimensionModern, PersoDimensionEdgy}},
	} {
		v := a[g.dims[0]] + a[g.dims[1]]
		if n == 0 || v > score {
			best, score = g.style, v
		}
	}

	return best
}

func swatch(colour string) string {
	if hex, ok := Swatches[colour]; ok {
		return hex
	}

	return "#b0b0b0"
}

func backdrop(occasion string) string {
	if hex, ok := backdrops[occasion]; ok {
		return hex
	}

	return "#f2f2f2"
}

func stops(b *strings.Builder, s [][2]string) {
	for _, stop := range s {
		fmt.Fprintf(b, `<stop offset="%s" stop-color="%s"/>`, stop[0], stop[1])
	}
}

// mix blends two #rrggbb colours, t of the way from a to b.
func mix(a, b string, t float64) string {
	var ar, ag, ab, br, bg, bb int
	_, _ = fmt.Sscanf(a, "#%02x%02x%02x", &ar, &ag, &ab)
	_, _ = fmt.Sscanf(b, "#%02x%02x%02x", &br, &bg, &bb)

	lerp := func(x, y int) int { return int(math.Round(float64(x) + (float64(y)-float64(x))*t)) }

	return fmt.Sprintf("#%02x%02x%02x", lerp(ar, br), lerp(ag, bg), lerp(ab, bb))
}

func clamp(v, lo, hi float64) float64 { return math.Max(lo, math.Min(hi, v)) }

// num formats a coordinate with at most one decimal, so the images stay byte-stable.
func num(v float64) string {
	return strings.TrimSuffix(strings.TrimRight(fmt.Sprintf("%.1f", v), "0"), ".")
}

func pt(x, y float64) string { return num(x) + "," + num(y) }

// slot is the persona's position in Personas(), which picks its palette entries. A
// persona defined elsewhere falls back to a hash of its name: stable, if not distinct.
func (p Persona) slot() int {
	for n, known := range Personas() {
		if known.Name == p.Name {
			return n
		}
	}

	h := fnv.New32a()
	_, _ = h.Write([]byte(p.Name))

	return int(h.Sum32() % 1024)
}
