// Package report renders the HTML pages tests write for visual verification: a shared
// page shell, and the few helpers every report needs to show a taste vector or a score.
//
// The pages are test output, not fixtures. They are deterministic, so rerunning the
// tests leaves them byte-identical, but nothing asserts on them — they exist to be
// looked at.
package report

import (
	"cmp"
	"fmt"
	"html/template"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/foomo/schaumamoi/pkg/personalization"
)

// Write renders body, a template executed against data, inside the shared page shell
// and writes it to path. funcs extends the helpers below for this one page.
func Write(path, title, body string, data any, funcs template.FuncMap) error {
	all := template.FuncMap{
		"taste":   Taste,
		"score":   Score,
		"date":    Date,
		"title":   func() string { return title },
		"lower":   strings.ToLower,
		"plusOne": func(i int) int { return i + 1 },
	}
	for name, fn := range funcs {
		all[name] = fn
	}

	tmpl, err := template.New("page").Funcs(all).Parse(shell)
	if err != nil {
		return fmt.Errorf("parse shell: %w", err)
	}

	if _, err := tmpl.New("body").Parse(body); err != nil {
		return fmt.Errorf("parse body: %w", err)
	}

	var b strings.Builder
	if err := tmpl.Execute(&b, data); err != nil {
		return fmt.Errorf("render %s: %w", path, err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	return os.WriteFile(path, []byte(b.String()), 0o600)
}

// Taste draws a vector as one diverging bar per dimension, strongest first: positive
// to the right of the axis, negative to the left. Dimensions it does not mention are
// left out rather than drawn as zero.
func Taste(m personalization.Map) template.HTML {
	dims := make([]personalization.Dimension, 0, len(m))
	for d := range m {
		dims = append(dims, d)
	}

	slices.SortFunc(dims, func(a, b personalization.Dimension) int {
		if c := cmp.Compare(math.Abs(float64(m[b])), math.Abs(float64(m[a]))); c != 0 {
			return c
		}

		return cmp.Compare(a, b)
	})

	var b strings.Builder

	b.WriteString(`<table class="taste">`)

	for _, d := range dims {
		v := float64(m[d])
		width := math.Min(math.Abs(v), 1) * 50
		side, left := "pos", 50.0
		if v < 0 {
			side, left = "neg", 50-width
		}

		fmt.Fprintf(&b, `<tr><th>%s</th><td><div class="track"><div class="bar %s" style="left:%.1f%%;width:%.1f%%"></div></div></td><td class="num">%+.2f</td></tr>`,
			template.HTMLEscapeString(string(d)), side, left, width, v)
	}

	b.WriteString(`</table>`)

	return template.HTML(b.String()) //nolint:gosec // built from escaped names and formatted numbers only
}

// Score formats a score or similarity to two decimals.
func Score(v personalization.Value) string {
	return fmt.Sprintf("%.2f", float64(v))
}

// Date formats a timestamp the way the reports show visits: day and time, in UTC.
func Date(t time.Time) string {
	if t.IsZero() {
		return "—"
	}

	return t.UTC().Format("Mon 2 Jan 15:04")
}

const shell = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{title}}</title>
<style>
:root {
  --bg: #fafaf8; --fg: #1d1f23; --muted: #6b7078; --card: #ffffff; --line: #e3e4e6;
  --accent: #2f6feb; --pos: #2f8a57; --neg: #c2463b; --hit: #e9f6ee; --miss: #fbeae8;
}
@media (prefers-color-scheme: dark) {
  :root {
    --bg: #16181c; --fg: #e6e7e9; --muted: #9aa0a8; --card: #1f2227; --line: #33373e;
    --accent: #6ea0ff; --pos: #5cc28a; --neg: #ef7a6f; --hit: #1d3326; --miss: #3a2320;
  }
}
* { box-sizing: border-box; }
body { margin: 0; padding: 24px 16px 64px; background: var(--bg); color: var(--fg);
  font: 14px/1.45 system-ui, -apple-system, "Segoe UI", sans-serif; }
main { max-width: 1280px; margin: 0 auto; }
h1 { font-size: 26px; margin: 0 0 4px; }
h2 { font-size: 19px; margin: 40px 0 4px; padding-top: 16px; border-top: 1px solid var(--line); }
h3 { font-size: 15px; margin: 0 0 4px; }
p.lede, p.note { color: var(--muted); margin: 0 0 16px; max-width: 80ch; }
nav { display: flex; flex-wrap: wrap; gap: 6px 16px; margin: 12px 0 0; }
nav a { color: var(--accent); text-decoration: none; }
.grid { display: grid; gap: 16px; grid-template-columns: repeat(auto-fill, minmax(260px, 1fr)); }
.card { background: var(--card); border: 1px solid var(--line); border-radius: 10px; padding: 14px; }
.card img.product { width: 100%; height: auto; border-radius: 6px; display: block; }
.row { display: flex; gap: 10px; align-items: flex-start; overflow-x: auto; padding-bottom: 6px; }
.row + .row { margin-top: 14px; }
.who { flex: 0 0 120px; text-align: center; }
.who img, img.avatar { width: 88px; height: 88px; border-radius: 50%; display: block; margin: 0 auto 4px; }
img.avatar.small { width: 44px; height: 44px; display: inline-block; margin: 0; vertical-align: middle; }
.thumb { flex: 0 0 84px; text-align: center; font-size: 12px; color: var(--muted); }
.thumb img { width: 84px; height: 112px; border-radius: 6px; display: block; border: 2px solid transparent; }
.thumb.top img { border-color: var(--accent); }
.thumb b { color: var(--fg); display: block; }
.step { flex: 0 0 auto; background: var(--card); border: 1px solid var(--line); border-radius: 8px; padding: 8px; }
.step .thumbs { display: flex; gap: 4px; flex-wrap: wrap; max-width: 260px; }
.step .thumbs img { width: 40px; height: 53px; border-radius: 4px; }
.step .thumbs img.big { width: 84px; height: 112px; }
.step h4 { margin: 0 0 6px; font-size: 12px; font-weight: 600; font-family: ui-monospace, monospace; }
.arrow { align-self: center; color: var(--muted); font-size: 18px; }
.muted { color: var(--muted); }
.meta { color: var(--muted); font-size: 12px; margin: 6px 0; }
table.data { border-collapse: collapse; width: 100%; background: var(--card); border: 1px solid var(--line); border-radius: 10px; overflow: hidden; }
table.data th, table.data td { padding: 8px 10px; border-bottom: 1px solid var(--line); text-align: left; vertical-align: middle; }
table.data th { font-size: 12px; color: var(--muted); font-weight: 600; }
table.data td.hit { background: var(--hit); }
table.data td.miss { background: var(--miss); }
.scroll { overflow-x: auto; }
table.taste { border-collapse: collapse; width: 100%; font-size: 12px; margin-top: 8px; }
table.taste th { text-align: left; font-weight: 400; color: var(--muted); width: 92px; padding: 1px 6px 1px 0; }
table.taste td { padding: 1px 0; }
table.taste td.num { width: 44px; text-align: right; font-variant-numeric: tabular-nums; }
.track { position: relative; height: 8px; background: var(--line); border-radius: 4px; }
.track::after { content: ""; position: absolute; left: 50%; top: -2px; bottom: -2px; width: 1px; background: var(--muted); }
.bar { position: absolute; top: 0; bottom: 0; border-radius: 4px; }
.bar.pos { background: var(--pos); }
.bar.neg { background: var(--neg); }
.phases { display: flex; gap: 6px; flex-wrap: wrap; }
.phase { display: flex; gap: 6px; align-items: center; background: var(--bg); border: 1px solid var(--line); border-radius: 22px; padding: 2px 10px 2px 2px; font-size: 12px; }
.sims { display: flex; gap: 8px; }
.sim { text-align: center; font-size: 11px; color: var(--muted); }
.sim.best { color: var(--fg); font-weight: 600; }
.sim.best img { outline: 2px solid var(--accent); outline-offset: 1px; }
.tick { color: var(--pos); font-weight: 600; }
.cross { color: var(--neg); font-weight: 600; }
</style>
</head>
<body>
<main>
{{template "body" .}}
</main>
</body>
</html>
`
