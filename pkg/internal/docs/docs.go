// Package docs renders the documentation site the tests write: a set of static pages
// that show what the mock shop generates, what the corpus reads back from it, and how
// the personalization vector behaves — every product and persona by its image, every
// claim next to the test that checks it.
//
// The tests compute; this package presents. Each package's TestReport hands a page its
// data with Page, and the page's template, the shared layout, styles and scripts all
// live here, under templates/. The site is test output, not a fixture: it is
// deterministic, but nothing asserts on it, and it is not kept in version control.
// `mise run docs` builds it into build/site, ready to publish as a GitHub Pages site.
package docs

import (
	"cmp"
	"embed"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/foomo/schaumamoi/pkg/personalization"
)

// SiteEnv names the environment variable that overrides where the site is written.
const SiteEnv = "SCHAUMAMOI_SITE"

//go:embed templates
var files embed.FS

// PageInfo is one page of the site, as the header and the index list it.
type PageInfo struct {
	Name, Title, Summary string
	Sections             []Section
}

// Section is a section of a page: its heading, and the anchor it is linked by.
type Section struct {
	Title, Anchor string
}

// File is the page's file name in the site.
func (p PageInfo) File() string { return p.Name + ".html" }

// Pages are the site's pages, in the order the header and the index show them.
var Pages = []PageInfo{
	{
		Name: "index", Title: "Overview",
		Summary: "What schaumamoi does, and where each part of it is documented.",
	},
	{
		Name: "mock", Title: "The shop",
		Summary:  "The generated traffic: catalog and category tree, personas and their taste, how they browse and choose, and how the shop ranks for them.",
		Sections: []Section{{"Listings", "listings"}, {"Personas", "personas"}, {"Rankings", "rankings"}, {"Journeys", "journeys"}, {"Months", "months"}, {"Final ranking", "final"}, {"Catalog", "catalog"}},
	},
	{
		Name: "corpus", Title: "The corpus",
		Summary:  "What the corpus reads back from that traffic: how it is set up, and how it reads sessions, months, drifts, changing intents, detours and consent.",
		Sections: []Section{{"How the corpus works", "pipeline"}, {"One session", "session"}, {"Steady month", "month"}, {"Drift", "drift"}, {"Changing intent", "intent"}, {"Detour", "detour"}, {"Consent", "consent"}},
	},
	{
		Name: "affinity", Title: "The vector",
		Summary:  "How the personalization vector fades and follows a shopper whose intent changes — and how that compares to a flat sum and a decay by time.",
		Sections: []Section{{"The fade", "fade"}, {"Strong changes", "strong"}, {"A gentle change", "gentle"}, {"Against flat and time decay", "compare"}},
	},
}

// Dir is where the site is written: SCHAUMAMOI_SITE if set, else build/site at the root
// of the module the tests run in.
func Dir() string {
	if dir := os.Getenv(SiteEnv); dir != "" {
		return dir
	}

	root, err := moduleRoot()
	if err != nil {
		return filepath.Join("build", "site")
	}

	return filepath.Join(root, "build", "site")
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

// Page renders templates/<name>.html inside the layout against data and writes it to
// <name>.html in the site, along with the shared assets. funcs adds helpers for this
// one page to the shared ones below.
func Page(name string, data any, funcs template.FuncMap) error {
	all := template.FuncMap{
		"taste":   Taste,
		"score":   Score,
		"date":    Date,
		"lower":   strings.ToLower,
		"plusOne": func(i int) int { return i + 1 },
		"pct":     func(v float64) string { return fmt.Sprintf("%.0f%%", v) },
		"num":     func(v float64) string { return fmt.Sprintf("%.1f", v) },
		// img resolves an image path from mock.Item.ImagePath or Persona.ImagePath: the
		// site keeps the images at the same relative path as the mock package does.
		"img":   func(path string) string { return path },
		"pages": func() []PageInfo { return Pages },
		"page":  func() string { return name },
	}
	for k, fn := range funcs {
		all[k] = fn
	}

	tmpl, err := template.New("layout.html").Funcs(all).ParseFS(files, "templates/layout.html", "templates/partials/*.html", "templates/"+name+".html")
	if err != nil {
		return fmt.Errorf("parse %s: %w", name, err)
	}

	var b strings.Builder
	if err := tmpl.Execute(&b, data); err != nil {
		return fmt.Errorf("render %s: %w", name, err)
	}

	if err := writeAssets(); err != nil {
		return err
	}

	return write(name+".html", []byte(b.String()))
}

// Image writes an image into the site at path, relative to the site root.
func Image(path string, content []byte) error {
	return write(path, content)
}

// writeAssets copies the shared styles and scripts into the site, and marks it as a plain
// static site for GitHub Pages.
func writeAssets() error {
	err := fs.WalkDir(files, "templates/assets", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}

		content, err := files.ReadFile(path)
		if err != nil {
			return err
		}

		return write(strings.TrimPrefix(path, "templates/"), content)
	})
	if err != nil {
		return err
	}

	return write(".nojekyll", nil)
}

// write writes a file into the site atomically: the package tests run in parallel and
// may write the same asset at once, and a reader must never see half of one.
func write(path string, content []byte) error {
	target := filepath.Join(Dir(), filepath.FromSlash(path))
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
