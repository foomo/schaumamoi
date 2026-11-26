package persona_test

import (
	"cmp"
	"path/filepath"
	"slices"
	"testing"

	"github.com/foomo/schaumamoi/pkg/internal/docs"
	"github.com/foomo/schaumamoi/pkg/internal/mock/catalog"
	"github.com/foomo/schaumamoi/pkg/internal/mock/persona"
	"github.com/foomo/schaumamoi/pkg/personalization"
	"github.com/stretchr/testify/require"
)

type (
	ranked struct {
		Item  catalog.Item
		Score personalization.Value
	}

	// favourite is a persona's first choice in one category.
	favourite struct {
		Category string
		ranked
	}

	personaReport struct {
		Persona    persona.Persona
		Occasion   string
		Top        ranked
		Favourites []favourite
		Ranking    []ranked
	}
)

// TestExhibit writes the personas exhibit: every persona's taste, the occasion it browses,
// its favourite in every category, and how it ranks the whole catalog. It also writes
// the avatars every page shows.
func TestExhibit(t *testing.T) {
	t.Parallel()

	var personas []personaReport

	for _, p := range persona.All() {
		ranking := rank(p)
		personas = append(personas, personaReport{
			Persona:    p,
			Occasion:   p.Occasion(),
			Top:        ranking[0],
			Favourites: favourites(p),
			Ranking:    ranking,
		})

		require.NoError(t, docs.Image(p.ImagePath(), []byte(p.SVG())))
	}

	require.NoError(t, docs.Exhibit("personas", map[string]any{
		"Personas":   personas,
		"ScanDepth":  persona.ScanDepth,
		"GoodEnough": float64(persona.GoodEnough) * 100,
	}, nil))

	t.Logf("wrote %s", filepath.Join(docs.Dir(), docs.FragmentDir, "personas.md"))
}

// favourites is the persona's first choice in every category.
func favourites(p persona.Persona) []favourite {
	var out []favourite

	for _, category := range []string{catalog.CategoryDresses, catalog.CategoryTrousers, catalog.CategoryCushions, catalog.CategoryLamps, catalog.CategoryVases} {
		var items []catalog.Item

		for _, item := range catalog.All() {
			if item.Category() == category {
				items = append(items, item)
			}
		}

		item := p.TopMatch(items)
		out = append(out, favourite{Category: category, ranked: ranked{Item: item, Score: catalog.Score(p.Affinity, item)}})
	}

	return out
}

// rank is the persona's view of the whole catalog, best first, ties broken by item id
// as in Persona.TopMatch.
func rank(p persona.Persona) []ranked {
	out := make([]ranked, 0, len(catalog.All()))
	for _, item := range catalog.All() {
		out = append(out, ranked{Item: item, Score: catalog.Score(p.Affinity, item)})
	}

	slices.SortFunc(out, func(a, b ranked) int {
		if c := cmp.Compare(b.Score, a.Score); c != 0 {
			return c
		}

		return cmp.Compare(a.Item.ItemID, b.Item.ItemID)
	})

	return out
}
