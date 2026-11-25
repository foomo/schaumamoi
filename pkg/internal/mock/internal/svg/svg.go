package svg

import (
	"fmt"
	"math"
	"strings"
)

func Stops(b *strings.Builder, s [][2]string) {
	for _, stop := range s {
		fmt.Fprintf(b, `<stop offset="%s" stop-color="%s"/>`, stop[0], stop[1])
	}
}

// mix blends two #rrggbb colours, t of the way from a to b.
func Mix(a, b string, t float64) string {
	var ar, ag, ab, br, bg, bb int
	_, _ = fmt.Sscanf(a, "#%02x%02x%02x", &ar, &ag, &ab)
	_, _ = fmt.Sscanf(b, "#%02x%02x%02x", &br, &bg, &bb)

	lerp := func(x, y int) int { return int(math.Round(float64(x) + (float64(y)-float64(x))*t)) }

	return fmt.Sprintf("#%02x%02x%02x", lerp(ar, br), lerp(ag, bg), lerp(ab, bb))
}

func Clamp(v, lo, hi float64) float64 { return math.Max(lo, math.Min(hi, v)) }

// num formats a coordinate with at most one decimal, so the images stay byte-stable.
func Num(v float64) string {
	return strings.TrimSuffix(strings.TrimRight(fmt.Sprintf("%.1f", v), "0"), ".")
}

func Pt(x, y float64) string { return Num(x) + "," + Num(y) }

// sequins covers the band between x0 and x1, from the top of the image down to y1, in
// staggered rows of light and dark discs; the body's clip path trims them to the garment.
func Sequins(b *strings.Builder, fill string, x0, x1, y1 float64) {
	light, dark := Mix(fill, "#ffffff", 0.5), Mix(fill, "#000000", 0.2)
	for row, y := 0, 50.0; y < y1; row, y = row+1, y+10 {
		for x := x0 + float64(row%2)*5; x < x1; x += 10 {
			colour := light
			if (row+int(x/10))%3 == 0 {
				colour = dark
			}

			fmt.Fprintf(b, `<circle cx="%s" cy="%s" r="2.6" fill="%s"/>`, Num(x), Num(y), colour)
		}
	}

	b.WriteString("\n")
}
