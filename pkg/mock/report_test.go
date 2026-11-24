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
	"github.com/foomo/schaumamoi/pkg/internal/report"
	"github.com/foomo/schaumamoi/pkg/mock"
	"github.com/foomo/schaumamoi/pkg/personalization"
	"github.com/foomo/sesamy-go/pkg/sesamy"
	"github.com/stretchr/testify/require"
)

// reportPath is where TestReport writes the mock report, relative to this package.
var reportPath = filepath.Join(mock.FixtureDir, "report.html")

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
			Journey:    steps(t, mock.Journey(p).Events),
			Month:      month(t, p),
			Final: []finalRanking{
				final(t, p, "after one journey", mock.Journey(p).Events, affinity.New(nil)),
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

	require.NoError(t, report.Write(reportPath, "schaumamoi · mock report", mockReport, map[string]any{
		"Listings": listings,
		"Personas": personas,
		"Catalog":  mock.Catalog(),
		"BaseTime": mock.BaseTime,
	}, nil))

	for _, path := range imagePaths() {
		require.FileExists(t, filepath.Join(mock.FixtureDir, path), "the report shows an image that was never generated, run `go generate ./...`")
	}

	t.Logf("wrote %s", reportPath)
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

	for _, payload := range history[:len(history)-1] {
		replayListings(t, c, payload.Events, func(mock.ItemListParams, mock.Profile) {})
	}

	return final(t, p, "after a month", history[len(history)-1].Events, c)
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

	for _, payload := range mock.History(mock.Month(p)) {
		journey := steps(t, payload.Events)
		last := journey[len(journey)-1]
		require.NotEmpty(t, last.Items)

		landing := journey[0]
		rail := mock.Filter{Category: last.Items[0].Category()}.Apply(landing.Items)

		out = append(out, visitRow{
			At:         time.UnixMicro(payload.TimestampMicros),
			Session:    payload.SessionID,
			Bought:     last.Name == sesamy.EventNameAddToCart,
			Item:       last.Items[0],
			Signals:    landing.Signals,
			Rank:       1 + slices.IndexFunc(landing.Items, func(i mock.Item) bool { return i.ItemID == last.Items[0].ItemID }),
			RailRank:   1 + slices.IndexFunc(rail, func(i mock.Item) bool { return i.ItemID == last.Items[0].ItemID }),
			LandingTop: landing.Items[0],
		})
	}

	return out
}

func imagePaths() []string {
	paths := make([]string, 0, len(mock.Images()))
	for path := range mock.Images() {
		paths = append(paths, path)
	}

	slices.Sort(paths)

	return paths
}

const mockReport = `
<h1>Mock report</h1>
<p class="lede">Generated by <code>TestReport</code> in <code>pkg/mock</code>. Every picture is drawn from the
data it illustrates, so a product that looks wrong for its persona points at a vector, not at the drawing.
Visits are relative to BaseTime, {{date .BaseTime}}.</p>
<nav><a href="#listings">Listings</a><a href="#personas">Personas</a><a href="#rankings">Rankings</a><a href="#journeys">Journeys</a><a href="#months">Months</a><a href="#final">Final ranking</a><a href="#catalog">Catalog</a></nav>

<h2 id="listings">Listings</h2>
<p class="note">The category tree as a shopper browses it: two departments, each listing its categories' products, each
category its own. The list id is the category's path. These are as a new shopper sees them, ranked by a neutral profile.</p>
{{range .Listings}}
  <h3 style="margin-top:16px">{{.Name}} <span class="muted">· <code>{{.ID}}</code> · {{len .Items}} items</span></h3>
  <div class="row">{{range .Items}}<div class="thumb"><img src="{{.ImagePath}}" alt="{{.ItemName}}" title="{{.ItemID}} · {{.Description}}"><b>{{.ItemName}}</b>€{{.Price}}</div>{{end}}</div>
{{end}}

<h2 id="personas">Personas</h2>
<p class="note">Each persona's taste vector, the occasion it browses (its strongest occasion dimension), and its favourite
in every category. Fashion and home share some dimensions and not others: the home-only ones (cozy, natural,
industrial) come from the personas' own vectors, the shared ones carry over from what they wear.</p>
<div class="grid">
{{range .Personas}}
  <div class="card">
    <img class="avatar" src="{{.Persona.ImagePath}}" alt="{{.Persona.Name}}">
    <h3 style="text-align:center">{{.Persona.Name}}</h3>
    <p class="meta" style="text-align:center">{{.Persona.Description}} · browses <b>{{.Occasion}}</b></p>
    {{taste .Persona.Affinity}}
    <p class="meta" style="margin-top:12px">favourite per category — the overall top match outlined</p>
    <div class="row">{{$top := .Top.Item.ItemID}}{{range .Favourites}}<div class="thumb{{if eq .Item.ItemID $top}} top{{end}}"><img src="{{.Item.ImagePath}}" alt="{{.Item.ItemName}}" title="{{.Item.ItemID}} · {{.Item.Description}}"><b>{{.Item.ItemName}}</b>{{.Category}} · {{score .Score}}</div>{{end}}</div>
  </div>
{{end}}
</div>

<h2 id="rankings">Rankings</h2>
<p class="note">The whole catalog, dresses and trousers together, ordered by each persona's score (dot product of taste and attractor), best first.
The front of each row should look like the persona; the back like their opposite.</p>
<div class="card">
{{range .Personas}}
  <div class="row">
    <div class="who"><img src="{{.Persona.ImagePath}}" alt="{{.Persona.Name}}"><b>{{.Persona.Name}}</b></div>
    {{range $i, $r := .Ranking}}
      <div class="thumb{{if eq $i 0}} top{{end}}"><img src="{{$r.Item.ImagePath}}" alt="{{$r.Item.ItemName}}" title="{{$r.Item.ItemID}} · {{$r.Item.Description}}"><b>{{plusOne $i}}. {{$r.Item.ItemName}}</b>{{score $r.Score}}</div>
    {{end}}
  </div>
{{end}}
</div>

<h2 id="journeys">Journeys</h2>
<p class="note">The converting journey of each persona, with the list id of every listing and of every selection made from
one: the department, the trousers, the dresses, and back to the department narrowed to the persona's occasion; a view
and add to cart of what it opened there; a closing look at the department; then home decoration — the department, ranked only
by what carries over from fashion through shared dimensions, and the category of the pick there — and last a look at all
products, with nothing opened. On every other listing the persona opens something it likes from the first few positions:
not always the first, not always its favourite.</p>
{{range .Personas}}
  <div class="row card">
    <div class="who"><img src="{{.Persona.ImagePath}}" alt="{{.Persona.Name}}"><b>{{.Persona.Name}}</b></div>
    {{range $i, $s := .Journey}}
      {{if $i}}<div class="arrow">→</div>{{end}}
      <div class="step"><h4>{{$s.Name}}</h4>{{with $s.ListID}}<div class="meta"><code>{{.}}</code>{{with $s.Facets}} · {{.}}{{end}}</div>{{end}}<div class="thumbs">
        {{range $s.Items}}<img {{if eq (len $s.Items) 1}}class="big" {{end}}src="{{.ImagePath}}" alt="{{.ItemName}}" title="{{.ItemName}} ({{.ItemID}})">{{end}}
      </div></div>
    {{end}}
  </div>
{{end}}

<h2 id="months">Months</h2>
<p class="note">Each persona's <code>mock.Month</code>: seven visits over four weeks, mostly browsing, buying twice. Every
visit ends on the same product because the persona is steady; the corpus report shows what happens when it is not.
The line under each visit is the shop learning: how many interactions the landing page was ranked by — none on the first visit
— and where the product the shopper ends on ranked there, overall and on its own rail. Where it is not first overall, the
thumbnail beside it is what the shop put first instead.</p>
{{range .Personas}}
  <div class="row card">
    <div class="who"><img src="{{.Persona.ImagePath}}" alt="{{.Persona.Name}}"><b>{{.Persona.Name}}</b></div>
    {{range .Month}}
      <div class="thumb{{if .Bought}} top{{end}}"><img src="{{.Item.ImagePath}}" alt="{{.Item.ItemName}}" title="{{.Item.ItemID}}"><b>{{.Item.ItemName}}</b>{{if .Bought}}bought{{else}}viewed{{end}} · {{date .At}}<br>{{if .Signals}}{{.Signals}} interactions{{else}}neutral{{end}} · #{{.Rank}}{{if ne .Rank 1}} (#{{.RailRank}} on rail){{end}}</div>
      {{if ne .Rank 1}}<div class="thumb" style="opacity:.75"><img src="{{.LandingTop.ImagePath}}" alt="{{.LandingTop.ItemName}}" title="{{.LandingTop.ItemID}}"><b>{{.LandingTop.ItemName}}</b>shop's #1</div>{{end}}
    {{end}}
  </div>
{{end}}

<h2 id="final">Final ranking</h2>
<p class="note">Every visit ends on the top-level listing of all products, ranked by everything the shop has learned — its
whole view of the shopper, across both departments. Shown after a single converting journey and after the last visit of
a month. Under each product: its place and its score under the learned profile. Outlined: the persona's favourite in
each category — read per category, each should come first among its category's products wherever the shopper chose.</p>
{{range .Personas}}
  {{$p := .Persona}}
  {{range .Final}}
  <div class="card" style="margin-top:12px">
    <div class="row">
      <div class="who"><img src="{{$p.ImagePath}}" alt="{{$p.Name}}"><b>{{$p.Name}}</b><br><span class="muted">{{.Label}}<br>{{.Signals}} interactions</span></div>
      {{range .Items}}<div class="thumb{{if .Favourite}} top{{end}}"><img src="{{.Item.ImagePath}}" alt="{{.Item.ItemName}}" title="{{.Item.ItemID}} · {{.Item.Category}}"><b>#{{.Rank}} {{.Item.ItemName}}</b>{{score .Score}}</div>{{end}}
    </div>
  </div>
  {{end}}
{{end}}

<h2 id="catalog">Catalog</h2>
<p class="note">Every product with its facets, copy and attractor vector. The picture, the description and the bars should
all say the same thing.</p>
<div class="grid">
{{range .Catalog}}
  <div class="card">
    <img class="product" src="{{.ImagePath}}" alt="{{.ItemName}}">
    <h3 style="margin-top:10px">{{.ItemName}}</h3>
    <p class="meta">{{.ItemID}} · {{.Category}} · {{.ItemVariant}} · {{.Occasion}} · {{.Length}} · €{{.Price}} · {{.ItemBrand}}</p>
    <p style="margin:0">{{.Description}}</p>
    {{taste .Attractor}}
  </div>
{{end}}
</div>
`
