package docs_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/foomo/schaumamoi/pkg/internal/docs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestIndex writes the site's index page, which links the pages the other packages'
// tests write.
func TestIndex(t *testing.T) {
	t.Parallel()

	require.NoError(t, docs.Page("index", nil, nil))

	body, err := os.ReadFile(filepath.Join(docs.Dir(), "index.html"))
	require.NoError(t, err)

	for _, p := range docs.Pages {
		assert.Contains(t, string(body), `href="`+p.File()+`"`, "the index links %s", p.Name)

		for _, s := range p.Sections {
			assert.Contains(t, string(body), `href="`+p.File()+`#`+s.Anchor+`"`, "the index links %s#%s", p.Name, s.Anchor)
		}
	}

	assert.FileExists(t, filepath.Join(docs.Dir(), "assets", "style.css"))
	assert.FileExists(t, filepath.Join(docs.Dir(), ".nojekyll"))
}

// TestPageRendersInLayout renders a page into a temporary site and checks it comes out
// inside the shared layout, with the header linking every page.
func TestPageRendersInLayout(t *testing.T) {
	t.Setenv(docs.SiteEnv, t.TempDir())

	require.NoError(t, docs.Page("index", nil, nil))

	body, err := os.ReadFile(filepath.Join(docs.Dir(), "index.html"))
	require.NoError(t, err)

	assert.Contains(t, string(body), `<link rel="stylesheet" href="assets/style.css">`)
	assert.Contains(t, string(body), `<title>Overview · schaumamoi</title>`)
	assert.Contains(t, string(body), `<a href="index.html" aria-current="page">Overview</a>`)
}
