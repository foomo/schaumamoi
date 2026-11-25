package mock_test

import (
	"cmp"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/foomo/schaumamoi/pkg/affinity"
	"github.com/foomo/schaumamoi/pkg/internal/docs"
	"github.com/foomo/schaumamoi/pkg/mock"
	"github.com/foomo/schaumamoi/pkg/personalization"
	"github.com/foomo/sesamy-go/pkg/sesamy"
	"github.com/stretchr/testify/require"
)

type (
	ranked struct {
		Item  mock.Item
		Score personalization.Value
	}

	// step is one event of a journey: its name, the list it belongs to if any, the facets
	// that list was narrowed by, and the products it showed.
	step struct {
		Name    sesamy.EventName
		ListID  string
		Facets  string
		Signals int
		Items   []mock.Item
	}

	// visitRow is one visit of a month: when, whether it converted, the product it ended
	// on, and how well the shop knew the shopper by then — how many interactions its ranking had
	// learned from, and where the product ranked on the landing page, overall and on its
	// own rail.
	visitRow struct {
		At         time.Time
		Session    string
		Bought     bool
		Item       mock.Item
		Signals    int
		Rank       int
		RailRank   int
		LandingTop mock.Item
	}

	listing struct {
		ID, Name string
		Items    []mock.Item
	}

	// favourite is a persona's first choice in one category.
	favourite struct {
		Category string
		ranked
	}

	personaReport struct {
		Persona    mock.Persona
		Occasion   string
		Top        ranked
		Favourites []favourite
		Ranking    []ranked
		Journey    []step
		Month      []visitRow
		Final      []finalRanking
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
		Item      mock.Item
		Score     personalization.Value
		Favourite bool
	}
)

// TestReport writes testdata/report.html: the category listings, the personas, how each
// one ranks the whole catalog, the journeys and months generated from those rankings, and the catalog
// itself — every product and persona shown by its generated image, so the concept can be
// checked by eye. Open the file in a browser after `go test ./pkg/mock`.
func TestReport(t *testing.T) {
	t.Parallel()

	var personas []personaReport

	for _, p := range mock.Personas() {
		ranking := rank(p)
		personas = append(personas, personaReport{
			Persona:    p,
			Occasion:   p.Occasion(),
			Top:        ranking[0],
			Favourites: favourites(p),
			Ranking:    ranking,
			Journey:    steps(t, mock.Journey(p).Events()),
			Month:      month(t, p),
			Final: []finalRanking{
				final(t, p, "after one journey", mock.Journey(p).Events(), affinity.New(nil)),
				finalOfMonth(t, p),
			},
		})
	}

	var listings []listing
	for _, f := range []mock.Filter{
		mock.CategoryFilter, mock.DressesFilter, mock.TrousersFilter,
		mock.HomeFilter, mock.CushionsFilter, mock.LampsFilter, mock.VasesFilter,
		mock.AllProductsFilter,
	} {
		listings = append(listings, listing{ID: f.ListID(), Name: f.ListName(), Items: f.Apply(mock.Catalog())})
	}

	// the images every page shows, at the same relative path as in the mock package
	for path, svg := range mock.Images() {
		require.NoError(t, docs.Image(path, []byte(svg)))
	}

	require.NoError(t, docs.Page("mock", map[string]any{
		"Listings": listings,
		"Personas": personas,
		"Catalog":  mock.Catalog(),
		"BaseTime": mock.BaseTime,
	}, nil))

	t.Logf("wrote %s", filepath.Join(docs.Dir(), "mock.html"))
}

// final reads the listing of all products that events end on. The scores come from
// replaying the events into the shopper's corpus, as TestListingsAreSortedByWhatWasLearned
// does, on top of whatever the corpus already holds.
func final(t *testing.T, p mock.Persona, label string, events []sesamy.Event[any], c *affinity.Corpus) finalRanking {
	t.Helper()

	var (
		last mock.ItemListParams
		prof mock.Profile
	)

	replayListings(t, c, events, func(list mock.ItemListParams, at mock.Profile) {
		last, prof = list, at
	})
	require.Equal(t, mock.AllProductsListID, last.ItemListID)

	favs := map[string]bool{}
	for _, f := range favourites(p) {
		favs[f.Item.ItemID] = true
	}

	out := finalRanking{Label: label, Signals: last.TasteSignals}
	for i, item := range last.Items {
		out.Items = append(out.Items, finalItem{Rank: i + 1, Item: item, Score: mock.Score(prof.Taste, item), Favourite: favs[item.ItemID]})
	}

	return out
}

// finalOfMonth is the final ranking of the last visit of the persona's month, with the
// shopper's corpus carried through every visit before it.
func finalOfMonth(t *testing.T, p mock.Persona) finalRanking {
	t.Helper()

	history := mock.History(mock.Month(p))
	c := affinity.New(nil)

	for _, session := range history[:len(history)-1] {
		replayListings(t, c, session.Events(), func(mock.ItemListParams, mock.Profile) {})
	}

	return final(t, p, "after a month", history[len(history)-1].Events(), c)
}

// favourites is the persona's first choice in every category: the per-category view the
// corpus is checked against.
func favourites(p mock.Persona) []favourite {
	var out []favourite

	for _, category := range []string{mock.CategoryDresses, mock.CategoryTrousers, mock.CategoryCushions, mock.CategoryLamps, mock.CategoryVases} {
		item := p.TopMatch(mock.Filter{Category: category}.Apply(mock.Catalog()))
		out = append(out, favourite{Category: category, ranked: ranked{Item: item, Score: mock.Score(p.Affinity, item)}})
	}

	return out
}

// rank is the persona's view of the whole catalog, best first, ties broken by item id
// as in Persona.TopMatch.
func rank(p mock.Persona) []ranked {
	out := make([]ranked, 0, len(mock.Catalog()))
	for _, item := range mock.Catalog() {
		out = append(out, ranked{Item: item, Score: mock.Score(p.Affinity, item)})
	}

	slices.SortFunc(out, func(a, b ranked) int {
		if c := cmp.Compare(b.Score, a.Score); c != 0 {
			return c
		}

		return cmp.Compare(a.Item.ItemID, b.Item.ItemID)
	})

	return out
}

// steps reads a journey back into the products each event showed, looked up in the
// catalog by id for their pictures. In memory the params still hold mock.Item, so the
// ids survive DecodeParams; a decoded wire payload would need params.Item, see
// TestItemDecode.
func steps(t *testing.T, events []sesamy.Event[any]) []step {
	t.Helper()

	byID := map[string]mock.Item{}
	for _, item := range mock.Catalog() {
		byID[item.ItemID] = item
	}

	out := make([]step, 0, len(events))

	for _, e := range events {
		var p mock.ItemListParams // select_item shares its list and item fields
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
func month(t *testing.T, p mock.Persona) []visitRow {
	t.Helper()

	var out []visitRow

	for _, session := range mock.History(mock.Month(p)) {
		journey := steps(t, session.Events())

		// the product the visit ended on: the one it viewed from its section
		viewed := slices.IndexFunc(journey, func(s step) bool { return s.Name == sesamy.EventNameViewItem })
		require.GreaterOrEqual(t, viewed, 0)
		last := journey[viewed]
		require.NotEmpty(t, last.Items)
		bought := slices.ContainsFunc(journey, func(s step) bool { return s.Name == sesamy.EventNameAddToCart })

		landing := journey[0]
		rail := mock.Filter{Category: last.Items[0].Category()}.Apply(landing.Items)

		out = append(out, visitRow{
			At:         session.Start(),
			Session:    session.ID(),
			Bought:     bought,
			Item:       last.Items[0],
			Signals:    landing.Signals,
			Rank:       1 + slices.IndexFunc(landing.Items, func(i mock.Item) bool { return i.ItemID == last.Items[0].ItemID }),
			RailRank:   1 + slices.IndexFunc(rail, func(i mock.Item) bool { return i.ItemID == last.Items[0].ItemID }),
			LandingTop: landing.Items[0],
		})
	}

	return out
}
