// Package docs renders the pages the tests contribute to the documentation site: pages
// that show what the mock shop generates, what the distillery makes of it, and how
// the personalization vector behaves — every product and persona by its image, every
// claim next to the test that checks it.
//
// The site itself is a VitePress site in docs/, like the other foomo projects': guides
// written by hand, and a Reports section these pages fill. The tests compute; this
// package presents. Each package's TestReport hands its page the data with Page, which
// renders the page's template — templates/<name>.html, inside templates/page.md — into
// docs/reports/<name>.md. Styles and the chart hover live in the VitePress theme.
//
// The generated pages are test output, not sources: they are deterministic, but nothing
// asserts on them, and they are not kept in version control. `mise run docs` generates
// them and builds the site.
package docs

import (
	"cmp"
	"embed"
	"errors"
	"fmt"
	"html/template"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	texttemplate "text/template"
	"time"

	"github.com/foomo/schaumamoi/pkg/personalization"
)

// DocsEnv names the environment variable that overrides where the VitePress site lives.
const DocsEnv = "SCHAUMAMOI_DOCS"

//go:embed templates
var files embed.FS

// PageInfo is one generated page: its name, which is its file name in docs/reports,
// its title, and a line on what it shows.
type PageInfo struct {
	Name, Title, Summary string
}

// Pages are the generated pages, in the order the site's Reports section lists them.
var Pages = []PageInfo{
	{
		Name: "catalog", Title: "The catalog",
		Summary: "What the test shop sells: the category tree, the dimensions each department speaks, the weights every category scores with, and every product with its copy and attractor.",
	},
	{
		Name: "personas", Title: "The personas",
		Summary: "The four shoppers the traffic is generated for: their taste, their favourite in every category, how they choose, and how each ranks the whole catalog.",
	},
	{
		Name: "shop", Title: "The shop",
		Summary: "The listings the shop renders, filtered out of the catalog and ranked by nothing but what a shopper has done.",
	},
	{
		Name: "shoppers", Title: "The shoppers",
		Summary: "Each persona taken through the shop: its journey step by step, its month of visits, and the shop's final ranking for it.",
	},
	{
		Name: "distillery", Title: "The distillery",
		Summary: "What the distillery makes of the shoppers' traffic: how a still is set up, and what it distills from sessions, months, drifts, detours and consent.",
	},
	{
		Name: "affinity", Title: "The vector",
		Summary: "How the personalization vector fades and follows a shopper whose intent changes — checked visit by visit on long histories — and how that compares to a flat sum and a decay by time.",
	},
}

// Dir is the VitePress site's directory: SCHAUMAMOI_DOCS if set, else docs/ at the root
// of the module the tests run in.
func Dir() string {
	if dir := os.Getenv(DocsEnv); dir != "" {
		return dir
	}

	root, err := moduleRoot()
	if err != nil {
		return "docs"
	}

	return filepath.Join(root, "docs")
}

// moduleRoot walks up from the working directory — a package directory, under go test —
// to the directory holding go.mod.
func moduleRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("no go.mod above the working directory")
		}

		dir = parent
	}
}

// Page renders templates/<name>.html inside templates/page.md against data and writes it
// to docs/reports/<name>.md. funcs adds helpers for this one page to the shared ones.
func Page(name string, data any, funcs template.FuncMap) error {
	i := slices.IndexFunc(Pages, func(p PageInfo) bool { return p.Name == name })
	if i < 0 {
		return fmt.Errorf("no page %q in docs.Pages", name)
	}

	info := Pages[i]
	all := template.FuncMap{
		"taste":   Taste,
		"score":   Score,
		"date":    Date,
		"lower":   strings.ToLower,
		"plusOne": func(i int) int { return i + 1 },
		"pct":     func(v float64) string { return fmt.Sprintf("%.0f%%", v) },
		"num":     func(v float64) string { return fmt.Sprintf("%.1f", v) },
		// img resolves an image path from catalog.Item.ImagePath or persona.Persona.ImagePath to the
		// image next to the page. The ./ matters: VitePress resolves image paths as imports,
		// and without it images/x.svg reads as a package.
		"img":  func(path string) string { return "./" + path },
		"page": func() PageInfo { return info },
	}
	for k, fn := range funcs {
		all[k] = fn
	}

	body, err := template.New(name+".html").Funcs(all).ParseFS(files, "templates/partials/*.html", "templates/"+name+".html")
	if err != nil {
		return fmt.Errorf("parse %s: %w", name, err)
	}

	var html strings.Builder
	if err := body.ExecuteTemplate(&html, "body", data); err != nil {
		return fmt.Errorf("render %s: %w", name, err)
	}

	// the page around it is Markdown with YAML front matter, not HTML: text/template, so
	// titles and descriptions are not HTML-escaped. The body above is html/template's.
	page, err := texttemplate.New("page.md").ParseFS(files, "templates/page.md")
	if err != nil {
		return fmt.Errorf("parse page.md: %w", err)
	}

	var md strings.Builder
	if err := page.Execute(&md, map[string]any{"Page": info, "Body": flatten(html.String())}); err != nil {
		return fmt.Errorf("render %s: %w", name, err)
	}

	return write(filepath.Join("reports", name+".md"), []byte(md.String()))
}

// leadingSpace is the indentation of a line.
var leadingSpace = regexp.MustCompile(`(?m)^[ \t]+`)

// flatten makes rendered HTML safe to embed in Markdown: in Markdown a blank line ends an
// HTML block, and an indented line inside one can turn into a code block. Without either,
// the whole page stays one block of raw HTML. Nothing in the pages is whitespace-sensitive.
func flatten(html string) string {
	html = leadingSpace.ReplaceAllString(html, "")

	lines := strings.Split(html, "\n")
	out := lines[:0]

	for _, l := range lines {
		if strings.TrimSpace(l) != "" {
			out = append(out, l)
		}
	}

	return strings.Join(out, "\n")
}

// Image writes an image next to the generated pages, at path relative to them — where the
// pages' relative image paths point. VitePress bundles them like any other page asset.
func Image(path string, content []byte) error {
	return write(filepath.Join("reports", filepath.FromSlash(path)), content)
}

// write writes a file under the site's directory atomically: the package tests run in
// parallel and may write the same file at once, and a reader must never see half of one.
func write(path string, content []byte) error {
	target := filepath.Join(Dir(), path)
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}

	tmp, err := os.CreateTemp(filepath.Dir(target), ".tmp-*")
	if err != nil {
		return err
	}

	if _, err := tmp.Write(content); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())

		return err
	}

	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmp.Name())

		return err
	}

	if err := os.Chmod(tmp.Name(), 0o644); err != nil {
		return err
	}

	return os.Rename(tmp.Name(), target)
}

// Taste draws a vector as one diverging bar per dimension, strongest first: positive
// to the right of the axis, negative to the left. Dimensions it does not mention are
// left out rather than drawn as zero.
func Taste(m personalization.Vector) template.HTML {
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

	b.WriteString(`<table class="taste"><tbody>`)

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

	b.WriteString(`</tbody></table>`)

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
