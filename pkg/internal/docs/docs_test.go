package docs_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/foomo/schaumamoi/pkg/affinity"
	"github.com/foomo/schaumamoi/pkg/internal/docs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPageIsVitePressMarkdown renders a page into a temporary site and checks what
// VitePress needs from it: front matter with the page's title, and the report as one
// block of raw HTML that Vue leaves alone — no blank line to end it early, no indented
// line to turn into a code block.
func TestPageIsVitePressMarkdown(t *testing.T) {
	t.Setenv(docs.DocsEnv, t.TempDir())

	// the smallest data the affinity page renders with
	chart := docs.Chart{Title: "c", YMin: 0, YMax: 1, Series: []docs.Series{{Name: "s", Slot: 1, Values: []float64{1, 0.5}}}}
	data := map[string]any{
		"Fade": affinity.DefaultFade, "HalfLife": 13.5, "AfterVisit": 60.0, "AfterWeek": 17.0,
		"Curve": chart, "StrongReads": chart, "StrongDetect": chart, "GentleReads": chart, "GentleDetect": chart,
		"StrongChoices": 1, "StrongCollapse": 0, "GentleCollapse": 0, "Compare": []struct{}{},
	}

	require.NoError(t, docs.Page("affinity", data, nil))

	raw, err := os.ReadFile(filepath.Join(docs.Dir(), "reports", "affinity.md"))
	require.NoError(t, err)

	md := string(raw)
	assert.True(t, strings.HasPrefix(md, "---\ntitle: \"The vector\"\n"), "front matter first, values quoted for YAML")
	assert.Contains(t, md, "pageClass: report-page")

	_, body, found := strings.Cut(md, `<div class="report" v-pre>`+"\n")
	require.True(t, found)

	body = strings.TrimSuffix(strings.TrimSpace(body), "</div>")
	for i, line := range strings.Split(strings.TrimSpace(body), "\n") {
		assert.NotEmpty(t, strings.TrimSpace(line), "line %d is blank", i+1)
		assert.False(t, strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t"), "line %d is indented: %q", i+1, line)
	}
}

func TestPageMustBeKnown(t *testing.T) {
	t.Setenv(docs.DocsEnv, t.TempDir())

	assert.Error(t, docs.Page("nope", nil, nil))
}

// TestImageGoesNextToThePages: images land next to the generated pages, so a page's
// relative image paths resolve.
func TestImageGoesNextToThePages(t *testing.T) {
	t.Setenv(docs.DocsEnv, t.TempDir())

	require.NoError(t, docs.Image("images/x.svg", []byte("<svg/>")))
	assert.FileExists(t, filepath.Join(docs.Dir(), "reports", "images", "x.svg"))
}
