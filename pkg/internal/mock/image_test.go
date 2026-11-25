package mock_test

import (
	"encoding/xml"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/foomo/schaumamoi/pkg/internal/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestImages keeps the committed images in step with the code that draws them. Run with
// -update to rewrite them.
func TestImages(t *testing.T) {
	t.Parallel()

	for name, svg := range mock.Images() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			path := filepath.Join(mock.FixtureDir, name)

			if *update {
				require.NoError(t, os.WriteFile(path, []byte(svg), 0o600))

				return
			}

			want, err := os.ReadFile(path)
			require.NoError(t, err, "image missing, run `go generate ./...`")
			assert.Equal(t, string(want), svg)
		})
	}
}

// TestImagesAreWellFormed parses every image as XML, so a broken attribute fails here
// rather than as a blank square in a browser.
func TestImagesAreWellFormed(t *testing.T) {
	t.Parallel()

	for name, svg := range mock.Images() {
		d := xml.NewDecoder(strings.NewReader(svg))
		for {
			_, err := d.Token()
			if errors.Is(err, io.EOF) {
				break
			}

			require.NoError(t, err, name)
		}
	}
}

// TestImagesAreDistinct is what drawing from the data has to deliver: no two products and
// no two personas come out as the same picture.
func TestImagesAreDistinct(t *testing.T) {
	t.Parallel()

	seen := map[string]string{}

	for name, svg := range mock.Images() {
		// the title and label name the subject; compare the drawing alone
		body := svg[strings.Index(svg, "</title>"):]
		if other, ok := seen[body]; ok {
			t.Errorf("%s draws the same as %s", name, other)
		}

		seen[body] = name
	}
}

// TestSwatchesCoverCatalog keeps every colour variant drawable: a new colour in the
// catalog without a swatch would quietly render grey.
func TestSwatchesCoverCatalog(t *testing.T) {
	t.Parallel()

	for _, item := range mock.Catalog() {
		assert.Contains(t, mock.Swatches, item.ItemVariant, item.ItemID)
	}
}
