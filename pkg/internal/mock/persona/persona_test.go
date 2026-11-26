package persona_test

import (
	"testing"

	"github.com/foomo/schaumamoi/pkg/internal/mock/catalog"
	"github.com/foomo/schaumamoi/pkg/internal/mock/persona"
	"github.com/stretchr/testify/assert"
)

// TestPersonaTopMatch is what keeps the generated traffic worth having: four different tastes
// must reach four different products. Kate and Mira both dress for business, so occasion
// alone must not decide the outcome; and the categories compete, each scored with its own
// weights, so three of the four end up in trousers — Kate because the trouser weights
// count tailoring more than the dress weights do.
func TestPersonaTopMatch(t *testing.T) {
	t.Parallel()

	products := catalog.All()

	for _, tt := range []struct {
		persona  persona.Persona
		occasion string
		want     string
	}{
		{persona.Kate, catalog.OccasionBusiness, "TROUSERS_01"},
		{persona.Sofia, catalog.OccasionFestive, "DRESS_08"},
		{persona.Lena, catalog.OccasionCasual, "TROUSERS_10"},
		{persona.Mira, catalog.OccasionBusiness, "TROUSERS_02"},
	} {
		t.Run(tt.persona.Name, func(t *testing.T) {
			t.Parallel()

			got := tt.persona.TopMatch(products)

			assert.Equal(t, tt.occasion, tt.persona.Occasion())
			assert.Equal(t, tt.want, got.ItemID,
				"%s, %s, matched %q: %s", tt.persona.Name, tt.persona.Description, got.ItemName, got.Description)
		})
	}

	seen := map[string]string{}

	for _, p := range persona.All() {
		match := p.TopMatch(products)
		assert.NotContains(t, seen, match.ItemID,
			"%s and %s share a top match, %q: %s", seen[match.ItemID], p.Name, match.ItemName, match.Description)
		seen[match.ItemID] = p.Name
	}
}
