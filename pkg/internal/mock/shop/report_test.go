package shop_test

import (
	"path/filepath"
	"testing"

	"github.com/foomo/schaumamoi/pkg/distillery/stills/affinity"
	"github.com/foomo/schaumamoi/pkg/internal/docs"
	"github.com/foomo/schaumamoi/pkg/internal/mock/catalog"
	"github.com/foomo/schaumamoi/pkg/internal/mock/persona"
	"github.com/foomo/schaumamoi/pkg/internal/mock/shop"
	"github.com/foomo/schaumamoi/pkg/internal/mock/shopper"
	"github.com/stretchr/testify/require"
)

type (
	listing struct {
		ID, Name, Query string
		Items           []catalog.Item
	}

	// learned is the department listing as the shop ranks it for a shopper after one
	// visit. Who is for the reader only: the shop never saw the persona, only the events.
	learned struct {
		Who     persona.Persona
		Signals int
		Items   []catalog.Item
	}
)

// TestReport writes the shop page: every listing of the category tree as a new shopper
// sees it, and the department listing re-ranked for four shoppers after one visit each,
// by nothing but the vector their events add up to.
func TestReport(t *testing.T) {
	t.Parallel()

	var listings []listing
	for _, f := range []shop.Filter{
		shop.AllProductsFilter,
		shop.CategoryFilter, shop.DressesFilter, shop.TrousersFilter,
		shop.HomeFilter, shop.CushionsFilter, shop.LampsFilter, shop.VasesFilter,
	} {
		listings = append(listings, listing{ID: f.ListID(), Name: f.ListName(), Query: f.Query(), Items: f.Apply(catalog.All())})
	}

	var after []learned

	for _, p := range persona.All() {
		c := affinity.New(nil)
		c.Feed(affinity.Envelope{}, shopper.Journey(p).Events()...)

		f := shop.CategoryFilter.Personalize(shop.Profile{Taste: c.Distill(affinity.DefaultFade.Vector), Signals: c.Len()})
		after = append(after, learned{Who: p, Signals: c.Len(), Items: f.Apply(catalog.All())})
	}

	require.NoError(t, docs.Page("shop", map[string]any{
		"Listings": listings,
		"Learned":  after,
	}, nil))

	t.Logf("wrote %s", filepath.Join(docs.Dir(), "reports", "shop.md"))
}
