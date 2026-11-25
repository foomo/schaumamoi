package persona

import (
	"fmt"
	"hash/fnv"
	"math"
	"strings"

	"github.com/foomo/schaumamoi/pkg/internal/mock/catalog"
	"github.com/foomo/schaumamoi/pkg/internal/mock/internal/svg"
	"github.com/foomo/schaumamoi/pkg/internal/mock/taste"
	"github.com/foomo/schaumamoi/pkg/personalization"
)

// ImagePath is where the persona's avatar is written, relative to mock.FixtureDir.
func (p Persona) ImagePath() string {
	return catalog.ImageDir + "/persona_" + strings.ToLower(p.Name) + ".svg"
}

// SVG draws the persona as an avatar, wearing the colour of the dress they score highest.
//
// As with the dresses, nothing is drawn per name. The hairstyle is the taste the persona
// leans to most — classic and structured wear a bob, romantic and feminine long hair,
// casual and bohemian a bun, modern and edgy a pixie cut — and the accessories follow
// the strongest single traits. Skin and hair colour come from a palette by the persona's
// place in All(), so the four never share a look; a stated age of fifty or more
// turns the hair silver.
func (p Persona) SVG() string {
	a := p.Affinity
	top := p.TopMatch(catalog.All())
	outfit := catalog.Swatch(top.ItemVariant)
	skins := []string{"#f2d3b8", "#e0ac82", "#c68642", "#8d5524"}
	hairs := []string{"#2b1d14", "#6b3f22", "#a0522d", "#d6b06a"}
	n := p.slot()
	skin, hair := skins[n%len(skins)], hairs[(n+1)%len(hairs)]

	var age int
	if _, err := fmt.Sscanf(p.Description, "%d", &age); err == nil && age >= 50 {
		hair = "#c4c4c8"
	}

	shade := svg.Mix(skin, "#000000", 0.12)
	id := strings.ToLower(p.Name)

	var b strings.Builder

	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 300 300" width="300" height="300" role="img" aria-label="%s, %s">`+"\n", p.Name, p.Description)
	fmt.Fprintf(&b, "<title>%s, %s</title>\n", p.Name, p.Description)
	fmt.Fprintf(&b, `<defs><clipPath id="avatar-%s"><circle cx="150" cy="150" r="150"/></clipPath></defs>`+"\n", id)
	fmt.Fprintf(&b, `<g clip-path="url(#avatar-%s)">`+"\n", id)
	fmt.Fprintf(&b, `<rect width="300" height="300" fill="%s"/>`+"\n", svg.Mix(outfit, "#ffffff", 0.78))

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

	if a[taste.Edgy] >= 0.7 {
		// square, heavy frames
		b.WriteString(`<g fill="none" stroke="#1d1d1d" stroke-width="3"><rect x="120" y="136" width="26" height="20" rx="3"/><rect x="154" y="136" width="26" height="20" rx="3"/><path d="M146,144 L154,144"/></g>` + "\n")
	}

	if a[taste.Opulent] >= 0.8 {
		b.WriteString(`<circle cx="108" cy="162" r="6" fill="#d4af37"/><circle cx="192" cy="162" r="6" fill="#d4af37"/>` + "\n")
	}

	if a[taste.Classic] >= 0.9 {
		// a string of pearls along the neckline
		for n := range 9 {
			t := float64(n) / 8
			x := 126 + 48*t
			y := 222 + 18*math.Sin(t*math.Pi)
			fmt.Fprintf(&b, `<circle cx="%s" cy="%s" r="3.6" fill="#f8f5ee" stroke="#d8d2c4"/>`, svg.Num(x), svg.Num(y))
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
		{"bob", [2]personalization.Dimension{taste.Classic, taste.Structured}},
		{"long", [2]personalization.Dimension{taste.Romantic, taste.Feminine}},
		{"bun", [2]personalization.Dimension{taste.Casual, taste.Bohemian}},
		{"pixie", [2]personalization.Dimension{taste.Modern, taste.Edgy}},
	} {
		v := a[g.dims[0]] + a[g.dims[1]]
		if n == 0 || v > score {
			best, score = g.style, v
		}
	}

	return best
}

// slot is the persona's position in All(), which picks its palette entries. A
// persona defined elsewhere falls back to a hash of its name: stable, if not distinct.
func (p Persona) slot() int {
	for n, known := range All() {
		if known.Name == p.Name {
			return n
		}
	}

	h := fnv.New32a()
	_, _ = h.Write([]byte(p.Name))

	return int(h.Sum32() % 1024)
}
