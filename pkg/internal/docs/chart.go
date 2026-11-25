package docs

import (
	"encoding/json"
	"fmt"
	"html/template"
	"math"
	"strings"
)

// Chart is a line chart over an ordinal x axis: one or more series sharing one y axis,
// with optional bands (stretches of x labelled at the top — who was shopping), reference
// lines (a threshold) and markers (where something happened). It renders to inline SVG
// with a crosshair and tooltip, and a table view for reading the numbers.
//
// The marks follow the site's charting rules: 2px lines, hairline solid gridlines,
// series colours from the fixed categorical slots (never cycled, never by rank), direct
// labels at the line ends alongside a legend, text in text colours only.
type Chart struct {
	Title  string
	YLabel string
	XLabel string
	XTicks []Tick  // labelled positions on the x axis
	YMin   float64 // y domain
	YMax   float64
	YTicks []float64
	Series []Series
	Bands  []Band
	Refs   []Ref
	Marks  []Marker
	// Rows names every x position for the tooltip and the table view.
	Rows []string
}

// Series is one line. Slot is its categorical colour slot, 1-based; a NaN value leaves a
// gap.
type Series struct {
	Name   string
	Slot   int
	Values []float64
}

// Band is a stretch of x, [From, To), labelled at the top of the plot.
type Band struct {
	From, To int
	Label    string
}

// Ref is a horizontal reference line.
type Ref struct {
	Y     float64
	Label string
}

// Marker is a point event drawn on top of the lines.
type Marker struct {
	X     int
	Y     float64
	Label string
}

// Tick is a labelled x position.
type Tick struct {
	X     int
	Label string
}

const (
	chartW, chartH                       = 960.0, 280.0
	padLeft, padRight, padTop, padBottom = 44.0, 96.0, 26.0, 34.0
)

// SVG renders the chart: a legend row, the plot, and a collapsible table view.
func (c Chart) SVG() template.HTML {
	n := 0
	for _, s := range c.Series {
		n = max(n, len(s.Values))
	}

	x := func(i float64) float64 {
		if n <= 1 {
			return padLeft
		}

		return padLeft + i*(chartW-padLeft-padRight)/float64(n-1)
	}
	y := func(v float64) float64 {
		return padTop + (c.YMax-v)/(c.YMax-c.YMin)*(chartH-padTop-padBottom)
	}

	var b strings.Builder

	b.WriteString(`<figure class="chart">`)
	if c.Title != "" {
		fmt.Fprintf(&b, `<figcaption>%s</figcaption>`, template.HTMLEscapeString(c.Title))
	}

	if len(c.Series) > 1 {
		b.WriteString(`<div class="legend">`)
		for _, s := range c.Series {
			fmt.Fprintf(&b, `<span><i style="background:var(--series-%d)"></i>%s</span>`, s.Slot, template.HTMLEscapeString(s.Name))
		}
		b.WriteString(`</div>`)
	}

	data, _ := json.Marshal(c.hoverData())
	fmt.Fprintf(&b, `<div class="plot" data-chart="%s">`, template.HTMLEscapeString(string(data)))
	fmt.Fprintf(&b, `<svg viewBox="0 0 %g %g" role="img" aria-label="%s">`, chartW, chartH, template.HTMLEscapeString(c.Title))

	// bands: alternate a faint fill so neighbouring stretches stay apart, label on top
	for i, band := range c.Bands {
		x0, x1 := x(float64(band.From)-0.5), x(float64(band.To)-0.5)
		x0, x1 = math.Max(x0, padLeft), math.Min(x1, chartW-padRight)
		if i%2 == 1 {
			fmt.Fprintf(&b, `<rect class="band" x="%.1f" y="%.1f" width="%.1f" height="%.1f"/>`, x0, padTop, x1-x0, chartH-padTop-padBottom)
		}

		fmt.Fprintf(&b, `<text class="band-label" x="%.1f" y="%.1f">%s</text>`, x0+4, padTop-8, template.HTMLEscapeString(band.Label))
	}

	// grid and y axis
	for _, t := range c.YTicks {
		fmt.Fprintf(&b, `<line class="grid" x1="%.1f" x2="%.1f" y1="%.1f" y2="%.1f"/>`, padLeft, chartW-padRight, y(t), y(t))
		fmt.Fprintf(&b, `<text class="tick" x="%.1f" y="%.1f" text-anchor="end">%s</text>`, padLeft-6, y(t)+4, trimFloat(t))
	}

	for _, t := range c.XTicks {
		fmt.Fprintf(&b, `<text class="tick" x="%.1f" y="%.1f" text-anchor="middle">%s</text>`, x(float64(t.X)), chartH-padBottom+16, template.HTMLEscapeString(t.Label))
	}

	if c.XLabel != "" {
		fmt.Fprintf(&b, `<text class="axis-label" x="%.1f" y="%.1f" text-anchor="middle">%s</text>`, (padLeft+chartW-padRight)/2, chartH-4, template.HTMLEscapeString(c.XLabel))
	}

	if c.YLabel != "" {
		fmt.Fprintf(&b, `<text class="axis-label" x="%.1f" y="%.1f">%s</text>`, 4.0, padTop-8, template.HTMLEscapeString(c.YLabel))
	}

	// reference lines
	for _, r := range c.Refs {
		fmt.Fprintf(&b, `<line class="ref" x1="%.1f" x2="%.1f" y1="%.1f" y2="%.1f"/>`, padLeft, chartW-padRight, y(r.Y), y(r.Y))
		fmt.Fprintf(&b, `<text class="ref-label" x="%.1f" y="%.1f">%s</text>`, chartW-padRight+6, y(r.Y)+4, template.HTMLEscapeString(r.Label))
	}

	// lines, and a direct label at each line's end
	type end struct {
		y    float64
		name string
		slot int
	}

	var ends []end

	for _, s := range c.Series {
		var d strings.Builder

		pen, last := false, -1
		for i, v := range s.Values {
			if math.IsNaN(v) {
				pen = false

				continue
			}

			if pen {
				d.WriteString(" L")
			} else {
				d.WriteString(" M")
			}

			fmt.Fprintf(&d, "%.1f,%.1f", x(float64(i)), y(v))
			pen, last = true, i
		}

		fmt.Fprintf(&b, `<path class="line" d="%s" style="stroke:var(--series-%d)"/>`, strings.TrimSpace(d.String()), s.Slot)

		if last >= 0 && len(c.Series) <= 4 {
			ends = append(ends, end{y(s.Values[last]), s.Name, s.Slot})
		}
	}

	// keep end labels from overlapping: nudge apart top to bottom
	for i := 1; i < len(ends); i++ {
		for j := i; j > 0 && ends[j].y < ends[j-1].y; j-- {
			ends[j], ends[j-1] = ends[j-1], ends[j]
		}
	}

	for i := 1; i < len(ends); i++ {
		ends[i].y = math.Max(ends[i].y, ends[i-1].y+13)
	}

	for _, e := range ends {
		fmt.Fprintf(&b, `<line class="end-key" x1="%.1f" x2="%.1f" y1="%.1f" y2="%.1f" style="stroke:var(--series-%d)"/>`, chartW-padRight+4, chartW-padRight+14, e.y, e.y, e.slot)
		fmt.Fprintf(&b, `<text class="end-label" x="%.1f" y="%.1f">%s</text>`, chartW-padRight+18, e.y+4, template.HTMLEscapeString(e.name))
	}

	for _, m := range c.Marks {
		fmt.Fprintf(&b, `<circle class="mark" cx="%.1f" cy="%.1f" r="4.5"><title>%s</title></circle>`, x(float64(m.X)), y(m.Y), template.HTMLEscapeString(m.Label))
	}

	// the crosshair and its hit area, driven by assets/chart.js
	fmt.Fprintf(&b, `<line class="crosshair" x1="0" x2="0" y1="%.1f" y2="%.1f"/>`, padTop, chartH-padBottom)
	fmt.Fprintf(&b, `<rect class="hit" x="%.1f" y="%.1f" width="%.1f" height="%.1f" data-x0="%.1f" data-x1="%.1f" data-n="%d"/>`,
		padLeft, padTop, chartW-padLeft-padRight, chartH-padTop-padBottom, padLeft, chartW-padRight, n)
	b.WriteString(`</svg><div class="tooltip" hidden></div></div>`)

	b.WriteString(c.table())
	b.WriteString(`</figure>`)

	return template.HTML(b.String()) //nolint:gosec // every string is escaped, every number formatted
}

type hoverSeries struct {
	Name   string     `json:"name"`
	Slot   int        `json:"slot"`
	Values []*float64 `json:"values"`
}

type hoverData struct {
	Rows   []string      `json:"rows"`
	Series []hoverSeries `json:"series"`
}

func (c Chart) hoverData() hoverData {
	out := hoverData{Rows: c.Rows}

	for _, s := range c.Series {
		hs := hoverSeries{Name: s.Name, Slot: s.Slot}
		for _, v := range s.Values {
			if math.IsNaN(v) {
				hs.Values = append(hs.Values, nil)

				continue
			}

			r := math.Round(v*1000) / 1000
			hs.Values = append(hs.Values, &r)
		}

		out.Series = append(out.Series, hs)
	}

	return out
}

// table is the chart's numbers as a table, collapsed under the plot.
func (c Chart) table() string {
	var b strings.Builder

	b.WriteString(`<details class="table-view"><summary>Table view</summary><div class="scroll"><table class="data"><thead><tr><th></th>`)
	for _, s := range c.Series {
		fmt.Fprintf(&b, `<th>%s</th>`, template.HTMLEscapeString(s.Name))
	}
	b.WriteString(`</tr></thead><tbody>`)

	n := 0
	for _, s := range c.Series {
		n = max(n, len(s.Values))
	}

	for i := range n {
		label := fmt.Sprint(i + 1)
		if i < len(c.Rows) {
			label = c.Rows[i]
		}

		fmt.Fprintf(&b, `<tr><td>%s</td>`, template.HTMLEscapeString(label))
		for _, s := range c.Series {
			if i < len(s.Values) && !math.IsNaN(s.Values[i]) {
				fmt.Fprintf(&b, `<td class="num">%.3f</td>`, s.Values[i])
			} else {
				b.WriteString(`<td class="num">—</td>`)
			}
		}
		b.WriteString(`</tr>`)
	}

	b.WriteString(`</tbody></table></div></details>`)

	return b.String()
}

func trimFloat(v float64) string {
	return strings.TrimSuffix(strings.TrimRight(fmt.Sprintf("%.2f", v), "0"), ".")
}
