// Package docs renders the exhibits the tests contribute to the documentation site:
// what the test shop generates, what the distillery makes of it, and how the
// personalization vector behaves — every product and persona by its image, every claim
// next to the test that checks it.
//
// The site is a VitePress site in docs/, like the other foomo projects': pages written by
// hand, some of which show an exhibit. The tests compute; this package presents. Each
// package's TestExhibit hands its exhibit the data with Exhibit, which renders the
// exhibit's template, templates/<name>.html, into a fragment, docs/.exhibits/<name>.md.
// The page that shows it, committed like any other, includes the fragment with
// VitePress's <!--@include: …-->: a page of its own under docs/exhibits/, or a guide page
// that opens with it. Styles and the chart hover live in the VitePress theme.
//
// The fragments are test output, not sources: they are deterministic, but nothing asserts
// on them, and they are not kept in version control. `mise run docs` generates them and
// builds the site.
package docs

import (
	"cmp"
	"embed"
	"errors"
	"fmt"
	"html/template"
	"math"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/foomo/schaumamoi/pkg/personalization"
)

// DocsEnv names the environment variable that overrides where the VitePress site lives.
const DocsEnv = "SCHAUMAMOI_DOCS"

// FragmentDir is where the exhibits are written, relative to the site: a dot directory,
// so VitePress makes no pages of it — the pages that include the fragments do that.
const FragmentDir = ".exhibits"

//go:embed templates
var files embed.FS

// ExhibitInfo is one exhibit: its name, which names its template and its fragment, and the
// page that shows it, relative to the site and without .md. Relative image paths in an
// included fragment resolve against that page, so the exhibit has to know it.
type ExhibitInfo struct {
	Name, Page string
}

// Exhibits are the exhibits the tests write, and the pages that show them.
var Exhibits = []ExhibitInfo{
	{Name: "catalog", Page: "guide/test-shop/catalog"},
	{Name: "personas", Page: "guide/test-shop/personas"},
	{Name: "shop", Page: "exhibits/shop"},
	{Name: "journeys", Page: "exhibits/journeys"},
	{Name: "distillery", Page: "exhibits/distillery"},
	{Name: "affinity", Page: "exhibits/affinity"},
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

// Exhibit renders templates/<name>.html against data and writes it to
// docs/.exhibits/<name>.md: one block of raw HTML, for its page to include. funcs adds
// helpers for this one exhibit to the shared ones.
func Exhibit(name string, data any, funcs template.FuncMap) error {
	i := slices.IndexFunc(Exhibits, func(e ExhibitInfo) bool { return e.Name == name })
	if i < 0 {
		return fmt.Errorf("no exhibit %q in docs.Exhibits", name)
	}

	info := Exhibits[i]
	all := template.FuncMap{
		"taste":   Taste,
		"score":   Score,
		"date":    Date,
		"lower":   strings.ToLower,
		"plusOne": func(i int) int { return i + 1 },
		"pct":     func(v float64) string { return fmt.Sprintf("%.0f%%", v) },
		"num":     func(v float64) string { return fmt.Sprintf("%.1f", v) },
		// img resolves an image path from catalog.Item.ImagePath or persona.Persona.ImagePath
		// to the image Image wrote, relative to the page that shows the exhibit.
		"img": func(p string) string { return ImagePath(info.Page, p) },
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

	fragment := `<div class="exhibit" v-pre>` + "\n" + flatten(html.String()) + "\n</div>\n"

	return write(filepath.Join(FragmentDir, name+".md"), []byte(fragment))
}

// ImagePath is where an image Image wrote is, seen from page (relative to the site,
// without .md). VitePress resolves it like any image in that page and bundles it; the ./
// or ../ matters, since without it images/x.svg would read as a package.
func ImagePath(page, image string) string {
	rel, err := filepath.Rel(filepath.Dir(filepath.FromSlash(page)), filepath.Join(FragmentDir, filepath.FromSlash(image)))
	if err != nil {
		return path.Join("/", FragmentDir, image)
	}

	rel = filepath.ToSlash(rel)
	if !strings.HasPrefix(rel, ".") || strings.HasPrefix(rel, FragmentDir) {
		rel = "./" + rel
	}

	return rel
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

// Image writes an image the exhibits show, at path (e.g. images/dress_01.svg) next to the
// fragments. The img template function points each exhibit at it.
func Image(path string, content []byte) error {
	return write(filepath.Join(FragmentDir, filepath.FromSlash(path)), content)
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

// Date formats a timestamp the way the exhibits show visits: day and time, in UTC.
func Date(t time.Time) string {
	if t.IsZero() {
		return "—"
	}

	return t.UTC().Format("Mon 2 Jan 15:04")
}
