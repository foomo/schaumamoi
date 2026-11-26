package journeys_test

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/foomo/schaumamoi/pkg/distillery/stills/affinity"
	"github.com/foomo/schaumamoi/pkg/internal/docs"
	"github.com/foomo/schaumamoi/pkg/internal/mock/catalog"
	"github.com/foomo/schaumamoi/pkg/internal/mock/journeys"
	"github.com/foomo/schaumamoi/pkg/internal/mock/persona"
	"github.com/foomo/schaumamoi/pkg/internal/mock/shop"
	"github.com/foomo/schaumamoi/pkg/internal/mock/wire"
	"github.com/foomo/schaumamoi/pkg/personalization"
	"github.com/foomo/sesamy-go/pkg/sesamy"
	"github.com/stretchr/testify/require"
)

type (
	// step is one event of a journey: its name, the list it belongs to if any, the facets
	// that list was narrowed by, and the products it showed.
	step struct {
		Name    sesamy.EventName
		ListID  string
		Facets  string
		Signals int
		Items   []catalog.Item
	}

	// visitRow is one visit of a month: when, whether it converted, the product it ended
	// on, and how well the shop knew the shopper by then — how many interactions its ranking had
	// learned from, and where the product ranked on the landing page, overall and on its
	// own rail.
	visitRow struct {
		At         time.Time
		Session    string
		Bought     bool
		Item       catalog.Item
		Signals    int
		Rank       int
		RailRank   int
		LandingTop catalog.Item
	}

	journeyReport struct {
		Persona persona.Persona
		Journey []step
		Month   []visitRow
		Final   []finalRanking
	}

	// finalRanking is the listing of all products a visit ends on, as the shop ranked it:
	// each product with its place, its score under the profile the shop had learned, and
	// whether it is the persona's favourite in its category.
	finalRanking struct {
		Label   string
		Signals int
		Items   []finalItem
	}

	finalItem struct {
		Rank      int
		Item      catalog.Item
		Score     personalization.Value
		Favourite bool
	}
)

// TestExhibit writes the journeys exhibit: each persona's converting journey, step by step,
// its month of visits with how well the shop knew it by each, and the shop's final
// ranking of all products after one visit and after the month.
func TestExhibit(t *testing.T) {
	t.Parallel()

	var rows []journeyReport

	for _, p := range persona.All() {
		rows = append(rows, journeyReport{
			Persona: p,
			Journey: steps(t, journeys.Buying(p).Events()),
			Month:   month(t, p),
			Final: []finalRanking{
				final(t, p, "after one journey", journeys.Buying(p).Events(), affinity.New(nil)),
				finalOfMonth(t, p),
			},
		})
	}

	require.NoError(t, docs.Exhibit("journeys", map[string]any{
		"Journeys": rows,
		"BaseTime": wire.BaseTime,
	}, nil))

	t.Logf("wrote %s", filepath.Join(docs.Dir(), docs.FragmentDir, "journeys.md"))
}

// final reads the listing of all products that events end on. The scores come from
// replaying the events into the shopper's still, as TestListingsAreSortedByWhatWasLearned
// does, on top of whatever the still already holds.
func final(t *testing.T, p persona.Persona, label string, events []sesamy.Event[any], c *affinity.Still) finalRanking {
	t.Helper()

	var (
		last shop.ItemListParams
		prof shop.Profile
	)

	replayListings(t, c, events, func(list shop.ItemListParams, at shop.Profile) {
		last, prof = list, at
	})
	require.Equal(t, shop.AllProductsListID, last.ItemListID)

	favs := map[string]bool{}
	for _, category := range []string{catalog.CategoryDresses, catalog.CategoryTrousers, catalog.CategoryCushions, catalog.CategoryLamps, catalog.CategoryVases} {
		favs[p.TopMatch(shop.Filter{Category: category}.Apply(catalog.All())).ItemID] = true
	}

	out := finalRanking{Label: label, Signals: last.TasteSignals}
	for i, item := range last.Items {
		out.Items = append(out.Items, finalItem{Rank: i + 1, Item: item, Score: catalog.Score(prof.Taste, item), Favourite: favs[item.ItemID]})
	}

	return out
}

// finalOfMonth is the final ranking of the last visit of the persona's month, with the
// shopper's still carried through every visit before it.
func finalOfMonth(t *testing.T, p persona.Persona) finalRanking {
	t.Helper()

	history := journeys.History(journeys.Month(p))
	c := affinity.New(nil)

	for _, session := range history[:len(history)-1] {
		replayListings(t, c, session.Events(), func(shop.ItemListParams, shop.Profile) {})
	}

	return final(t, p, "after a month", history[len(history)-1].Events(), c)
}

// steps reads a journey back into the products each event showed, looked up in the
// catalog by id for their pictures. In memory the params still hold catalog.Item, so the
// ids survive DecodeParams; a decoded wire payload would need params.Item, see
// TestItemDecode.
func steps(t *testing.T, events []sesamy.Event[any]) []step {
	t.Helper()

	byID := map[string]catalog.Item{}
	for _, item := range catalog.All() {
		byID[item.ItemID] = item
	}

	out := make([]step, 0, len(events))

	for _, e := range events {
		var p shop.ItemListParams // select_item shares its list and item fields
		require.NoError(t, e.DecodeParams(&p))

		s := step{Name: e.Name, ListID: p.ItemListID, Signals: p.TasteSignals}
		if p.FilterOccasion != "" {
			s.Facets = p.FilterOccasion
		}

		if e.Name == sesamy.EventNameViewItemList {
			ranking := "neutral ranking"
			switch p.TasteSignals {
			case 0:
			case 1:
				ranking = "ranked by 1 interaction"
			default:
				ranking = fmt.Sprintf("ranked by %d interactions", p.TasteSignals)
			}

			s.Facets = strings.TrimPrefix(s.Facets+", "+ranking, ", ")
		}

		for _, item := range p.Items {
			found, ok := byID[item.ItemID]
			require.True(t, ok, "event %s carries unknown item %q", e.Name, item.ItemID)
			s.Items = append(s.Items, found)
		}

		out = append(out, s)
	}

	return out
}

// month is the persona's history as a table: one row per visit, with the product the
// visit ended on.
func month(t *testing.T, p persona.Persona) []visitRow {
	t.Helper()

	var out []visitRow

	for _, session := range journeys.History(journeys.Month(p)) {
		journey := steps(t, session.Events())

		// the product the visit ended on: the one it viewed from its section
		viewed := slices.IndexFunc(journey, func(s step) bool { return s.Name == sesamy.EventNameViewItem })
		require.GreaterOrEqual(t, viewed, 0)
		last := journey[viewed]
		require.NotEmpty(t, last.Items)
		bought := slices.ContainsFunc(journey, func(s step) bool { return s.Name == sesamy.EventNameAddToCart })

		landing := journey[0]
		rail := shop.Filter{Category: last.Items[0].Category()}.Apply(landing.Items)

		out = append(out, visitRow{
			At:         session.Start(),
			Session:    session.ID(),
			Bought:     bought,
			Item:       last.Items[0],
			Signals:    landing.Signals,
			Rank:       1 + slices.IndexFunc(landing.Items, func(i catalog.Item) bool { return i.ItemID == last.Items[0].ItemID }),
			RailRank:   1 + slices.IndexFunc(rail, func(i catalog.Item) bool { return i.ItemID == last.Items[0].ItemID }),
			LandingTop: landing.Items[0],
		})
	}

	return out
}
