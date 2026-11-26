package docs_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/foomo/schaumamoi/pkg/distillery/stills/affinity"
	"github.com/foomo/schaumamoi/pkg/internal/docs"
	"github.com/foomo/schaumamoi/pkg/personalization"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestExhibitIsOneHTMLBlock renders an exhibit into a temporary site and checks what the
// page including it needs: no front matter of its own, and one block of raw HTML that Vue
// leaves alone — no blank line to end it early, no indented line to turn into a code
// block.
func TestExhibitIsOneHTMLBlock(t *testing.T) {
	t.Setenv(docs.DocsEnv, t.TempDir())

	// the smallest data the affinity exhibit renders with
	chart := docs.Chart{Title: "c", YMin: 0, YMax: 1, Series: []docs.Series{{Name: "s", Slot: 1, Values: []float64{1, 0.5}}}}
	data := map[string]any{
		"Fade": affinity.DefaultFade, "HalfLife": 13.5, "AfterVisit": 60.0, "AfterWeek": 17.0,
		"Curve": chart, "StrongReads": chart, "StrongDetect": chart, "GentleReads": chart, "GentleDetect": chart,
		"StrongChoices": 1, "StrongCollapse": 0, "GentleCollapse": 0, "Compare": []struct{}{}, "Intents": []struct{}{}, "Keep": 1000,
	}

	require.NoError(t, docs.Exhibit("affinity", data, nil))

	raw, err := os.ReadFile(filepath.Join(docs.Dir(), docs.FragmentDir, "affinity.md"))
	require.NoError(t, err)

	md := string(raw)
	assert.False(t, strings.HasPrefix(md, "---"), "no front matter: the including page has it")

	body, found := strings.CutPrefix(md, `<div class="exhibit" v-pre>`+"\n")
	require.True(t, found, "the fragment is the exhibit block, from its first line")

	body = strings.TrimSuffix(strings.TrimSpace(body), "</div>")
	for i, line := range strings.Split(strings.TrimSpace(body), "\n") {
		assert.NotEmpty(t, strings.TrimSpace(line), "line %d is blank", i+1)
		assert.False(t, strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t"), "line %d is indented: %q", i+1, line)
	}
}

func TestExhibitMustBeKnown(t *testing.T) {
	t.Setenv(docs.DocsEnv, t.TempDir())

	assert.Error(t, docs.Exhibit("nope", nil, nil))
}

// TestImagePathIsRelativeToItsPage: images land next to the fragments, and an exhibit
// points at them from the page that includes it — an included fragment's relative paths
// resolve against that page, wherever it sits.
func TestImagePathIsRelativeToItsPage(t *testing.T) {
	t.Setenv(docs.DocsEnv, t.TempDir())

	require.NoError(t, docs.Image("images/x.svg", []byte("<svg/>")))
	assert.FileExists(t, filepath.Join(docs.Dir(), docs.FragmentDir, "images", "x.svg"))

	assert.Equal(t, "../.exhibits/images/x.svg", docs.ImagePath("exhibits/shop", "images/x.svg"))
	assert.Equal(t, "../../.exhibits/images/x.svg", docs.ImagePath("guide/test-shop/catalog", "images/x.svg"))
	assert.Equal(t, "./.exhibits/images/x.svg", docs.ImagePath("index", "images/x.svg"))
}

// rowOutsideSection is a table row directly inside <table>: valid only because browsers
// repair it with an implied <tbody>, which Vue's renderer does not — the page's server
// render and its hydration would disagree, and Vue warns about every one.
var rowOutsideSection = regexp.MustCompile(`<table[^>]*>\s*<tr|</tr>\s*</table>`)

// TestTablesHaveSections keeps every table the docs generate in <thead> and <tbody>: the
// taste bars, a chart's table view, and the tables of a page template.
func TestTablesHaveSections(t *testing.T) {
	t.Setenv(docs.DocsEnv, t.TempDir())

	taste := string(docs.Taste(personalization.Vector{"classic": 0.8, "modern": -0.4}))
	assert.NotRegexp(t, rowOutsideSection, taste, "taste bars")

	chart := docs.Chart{Title: "c", YMin: 0, YMax: 1, Series: []docs.Series{{Name: "s", Slot: 1, Values: []float64{1, 0.5}}}}
	assert.NotRegexp(t, rowOutsideSection, string(chart.SVG()), "chart table view")

	data := map[string]any{
		"Fade": affinity.DefaultFade, "HalfLife": 13.5, "AfterVisit": 60.0, "AfterWeek": 17.0,
		"Curve": chart, "StrongReads": chart, "StrongDetect": chart, "GentleReads": chart, "GentleDetect": chart,
		"StrongChoices": 1, "StrongCollapse": 0, "GentleCollapse": 0, "Compare": []struct{}{}, "Intents": []struct{}{}, "Keep": 1000,
	}
	require.NoError(t, docs.Exhibit("affinity", data, nil))

	page, err := os.ReadFile(filepath.Join(docs.Dir(), docs.FragmentDir, "affinity.md"))
	require.NoError(t, err)
	assert.NotRegexp(t, rowOutsideSection, string(page), "affinity exhibit")
}
