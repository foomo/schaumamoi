package catalog_test

import (
	"testing"

	"github.com/foomo/schaumamoi/pkg/internal/mock/catalog"
	"github.com/stretchr/testify/assert"
)

// TestSwatchesCoverCatalog keeps every colour variant drawable: a new colour in the
// catalog without a swatch would quietly render grey.
func TestSwatchesCoverCatalog(t *testing.T) {
	t.Parallel()

	for _, item := range catalog.All() {
		assert.Contains(t, catalog.Swatches, item.ItemVariant, item.ItemID)
	}
}
