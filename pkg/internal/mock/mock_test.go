package mock_test

import (
	"cmp"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/foomo/gostandards/iso4217"
	"github.com/foomo/schaumamoi/pkg/affinity"
	"github.com/foomo/schaumamoi/pkg/internal/mock"
	"github.com/foomo/schaumamoi/pkg/personalization"
	"github.com/foomo/sesamy-go/pkg/encoding/mpv2"
	"github.com/foomo/sesamy-go/pkg/event/params"
	"github.com/foomo/sesamy-go/pkg/sesamy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var update = flag.Bool("update", false, "rewrite the testdata fixtures")

func itemIDs(items []mock.Item) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, item.ItemID)
	}

	return out
}

// TestFixtures keeps the committed fixtures in step with the constructors that produce
// them. Run with -update to rewrite them.
func TestFixtures(t *testing.T) {
	t.Parallel()

	fixtures := mock.Fixtures()

	for _, name := range mock.FixtureNames() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			body, err := json.MarshalIndent(fixtures[name], "", "  ")
			require.NoError(t, err)

			body = append(body, '\n')
			path := filepath.Join(mock.FixtureDir, name)

			if *update {
				require.NoError(t, os.WriteFile(path, body, 0o600))

				return
			}

			want, err := os.ReadFile(path)
			require.NoError(t, err, "fixture missing, run `go generate ./...`")
			assert.JSONEq(t, string(want), string(body))
		})
	}
}

// TestDeterministic guards the property the fixtures depend on: building the same mock
// twice must produce identical bytes. Both the affinity sort and the sparse dot product
// have to be order-stable for this to hold.
func TestDeterministic(t *testing.T) {
	t.Parallel()

	for _, p := range mock.Personas() {
		first, err := json.Marshal(mock.Journey(p))
		require.NoError(t, err)

		for range 20 {
			again, err := json.Marshal(mock.Journey(p))
			require.NoError(t, err)
			assert.JSONEq(t, string(first), string(again), p.Name)
		}
	}
}

// TestItemDecode pins how the two decoders treat mock.Item's embedded params.Item.
// They differ, and the difference decides which type you decode a collected event into.
func TestItemDecode(t *testing.T) {
	t.Parallel()

	body, err := os.ReadFile(filepath.Join(mock.FixtureDir, "add_to_cart.mpv2.json"))
	require.NoError(t, err)

	// encoding/json inlines an anonymous struct field by itself, so the GA4 fields sit
	// flat next to the attractor. This is the enrichment stage's path.
	t.Run("json", func(t *testing.T) {
		t.Parallel()

		var payload mpv2.Payload[params.AddToCart[mock.Item]]

		require.NoError(t, json.Unmarshal(body, &payload))
		require.Len(t, payload.Events, 1)
		require.Len(t, payload.Events[0].Params.Items, 1)

		item := payload.Events[0].Params.Items[0]
		assert.NotEmpty(t, item.ItemID, "embedded params.Item must inline on the wire")
		assert.NotEmpty(t, item.ItemName)
		assert.NotZero(t, item.Price)
		assert.NotEmpty(t, item.ItemCategory2)
	})

	// sesamy.Decode uses mapstructure, which does not inline an untagged embedded field.
	// Collected events are therefore decoded into params.Item-based types.
	t.Run("decode_params", func(t *testing.T) {
		t.Parallel()

		var payload mpv2.Payload[any]

		require.NoError(t, json.Unmarshal(body, &payload))
		require.Len(t, payload.Events, 1)

		var p params.AddToCart[params.Item]

		require.NoError(t, payload.Events[0].DecodeParams(&p))
		require.Len(t, p.Items, 1)

		item := p.Items[0]
		assert.NotEmpty(t, item.ItemID)
		assert.NotEmpty(t, item.ItemName)
		assert.NotZero(t, item.Price)
		assert.NotEmpty(t, item.ItemCategory2)
	})

	// The trap worth pinning: the same call with mock.Item silently returns zeroed GA4
	// fields rather than failing, because mapstructure looks for a nested "Item" key.
	t.Run("decode_params_into_mock_item_loses_ga4_fields", func(t *testing.T) {
		t.Parallel()

		var payload mpv2.Payload[any]

		require.NoError(t, json.Unmarshal(body, &payload))

		var p params.AddToCart[mock.Item]

		require.NoError(t, payload.Events[0].DecodeParams(&p))
		require.Len(t, p.Items, 1)

		assert.Empty(t, p.Items[0].ItemID, "use params.Item for collected events")
	})
}

// TestItemDescriptionStaysOffTheWire pins the other half of mock.Item's encoding
// contract. The attractor travels because something downstream consumes it; the
// description is catalog-only, and shipping it would put shop copy in every line item of
// every event for no reader. Checking the rendered payloads rather than the struct tag
// catches it whichever way it might leak.
func TestItemDescriptionStaysOffTheWire(t *testing.T) {
	t.Parallel()

	fixtures := mock.Fixtures()

	for _, name := range mock.FixtureNames() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			body, err := json.Marshal(fixtures[name])
			require.NoError(t, err)

			assert.NotContains(t, string(body), `"description"`)

			for _, item := range mock.Catalog() {
				assert.NotContains(t, string(body), item.Description, item.ItemID)
			}
		})
	}

	// The consequence for the enrichment stage: an item decoded back off the wire has no
	// description, so it has to be joined on from the catalog like any other shop data.
	t.Run("decoded_item_has_no_description", func(t *testing.T) {
		t.Parallel()

		body, err := os.ReadFile(filepath.Join(mock.FixtureDir, "add_to_cart.mpv2.json"))
		require.NoError(t, err)

		var payload mpv2.Payload[params.AddToCart[mock.Item]]

		require.NoError(t, json.Unmarshal(body, &payload))
		require.Len(t, payload.Events, 1)
		require.Len(t, payload.Events[0].Params.Items, 1)

		item := payload.Events[0].Params.Items[0]
		require.NotEmpty(t, item.ItemID)
		assert.Empty(t, item.Description, "join the description on from the catalog")
	})
}

// TestRoundTrip proves a fixture is consumable the way real traffic is: decoded off the
// wire straight into a typed sesamy-go envelope.
func TestRoundTrip(t *testing.T) {
	t.Parallel()

	body, err := os.ReadFile(filepath.Join(mock.FixtureDir, "add_to_cart.mpv2.json"))
	require.NoError(t, err)

	var payload mpv2.Payload[params.AddToCart[mock.Item]]

	require.NoError(t, json.Unmarshal(body, &payload))

	assert.Equal(t, mock.DefaultClientID, payload.ClientID)
	assert.Equal(t, mock.DefaultSessionID, payload.SessionID)
	assert.Equal(t, mock.BaseTime.UnixMicro(), payload.TimestampMicros)
	require.Len(t, payload.Events, 1)

	e := payload.Events[0]
	assert.Equal(t, sesamy.EventNameAddToCart, e.Name)
	assert.Equal(t, iso4217.CodeEUR, e.Params.Currency)
	assert.NotZero(t, e.Params.Value)
}

// TestValueMatchesItems is the property that makes these mocks usable as aggregation
// input: an event must never report a total that contradicts its own line items.
func TestValueMatchesItems(t *testing.T) {
	t.Parallel()

	catalog := mock.Catalog()
	e := mock.AddToCart(catalog[0], catalog[4])

	var want float64
	for _, item := range e.Params.Items {
		want += (item.Price - item.Discount) * item.Quantity
	}

	assert.InDelta(t, want, e.Params.Value, 0.01)
}

// TestCatalog pins the shape of the catalog the rest of the fixtures rest on.
func TestCatalog(t *testing.T) {
	t.Parallel()

	catalog := mock.Catalog()
	require.Len(t, catalog, 54)

	byRail := map[string]int{}
	lengths := map[string][]string{
		mock.CategoryDresses:  {mock.LengthMini, mock.LengthMidi, mock.LengthMaxi},
		mock.CategoryTrousers: {mock.LengthCropped, mock.LengthAnkle, mock.LengthFull},
		mock.CategoryCushions: {""},
		mock.CategoryLamps:    {""},
		mock.CategoryVases:    {""},
	}
	department := map[string]string{
		mock.CategoryDresses: mock.CategoryWomensFashion, mock.CategoryTrousers: mock.CategoryWomensFashion,
		mock.CategoryCushions: mock.CategoryHomeDecoration, mock.CategoryLamps: mock.CategoryHomeDecoration, mock.CategoryVases: mock.CategoryHomeDecoration,
	}

	for _, item := range catalog {
		assert.NotEmpty(t, item.ItemID)
		assert.NotEmpty(t, item.ItemName)
		assert.NotEmpty(t, item.Attractor, "every product needs a taste vector")
		assert.NotEmpty(t, item.Description, "every product needs a description")
		assert.Equal(t, department[item.Category()], item.ItemCategory, item.ItemID)
		assert.Contains(t, lengths[item.Category()], item.Length(), "%s has a length its category does not cut", item.ItemID)
		assert.NotZero(t, item.Price)
		byRail[item.Category()+" "+item.Occasion()]++
	}

	// six of each occasion on each fashion rail, six of each home category
	assert.Equal(t, map[string]int{
		"Dresses business": 6, "Dresses festive": 6, "Dresses casual": 6,
		"Trousers business": 6, "Trousers festive": 6, "Trousers casual": 6,
		"Cushions ": 6, "Lamps ": 6, "Vases ": 6,
	}, byRail)
}

// TestHomeSpeaksItsOwnDimensions pins how the Map separates the departments: a home item
// never carries a fashion-only dimension, so what the shop learns from clothes reaches
// the home department only through the dimensions the two share — and every home item
// shares some, or fashion taste could not rank it at all.
func TestHomeSpeaksItsOwnDimensions(t *testing.T) {
	t.Parallel()

	fashionOnly := []personalization.Dimension{
		mock.PersoDimensionBusiness, mock.PersoDimensionFestive, mock.PersoDimensionCasual,
		mock.PersoDimensionFeminine, mock.PersoDimensionStructured,
	}
	homeOnly := []personalization.Dimension{mock.PersoDimensionCozy, mock.PersoDimensionNatural, mock.PersoDimensionIndustrial}

	fashion := personalization.Map{}
	for _, item := range mock.Fashion() {
		for d := range item.Attractor {
			fashion[d] = 1
		}

		for _, d := range homeOnly {
			assert.NotContains(t, item.Attractor, d, "%s carries the home-only %s", item.ItemID, d)
		}
	}

	for _, item := range mock.HomeDecoration() {
		for _, d := range fashionOnly {
			assert.NotContains(t, item.Attractor, d, "%s carries the fashion-only %s", item.ItemID, d)
		}

		shared := 0
		for d := range item.Attractor {
			if _, ok := fashion[d]; ok {
				shared++
			}
		}

		assert.NotZero(t, shared, "%s shares no dimension with fashion", item.ItemID)
	}
}

// TestCategoryTree pins the listings the tree gives: the department lists everything,
// each category its own rail, and each list id is the category's path through the tree.
func TestCategoryTree(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		filter mock.Filter
		listID string
		want   []mock.Item
	}{
		{mock.CategoryFilter, "womens-fashion", mock.Fashion()},
		{mock.DressesFilter, "womens-fashion/dresses", mock.Dresses()},
		{mock.TrousersFilter, "womens-fashion/trousers", mock.Trousers()},
		{mock.HomeFilter, "home-decoration", mock.HomeDecoration()},
		{mock.CushionsFilter, "home-decoration/cushions", inCategory(mock.HomeDecoration(), mock.CategoryCushions)},
		{mock.LampsFilter, "home-decoration/lamps", inCategory(mock.HomeDecoration(), mock.CategoryLamps)},
		{mock.VasesFilter, "home-decoration/vases", inCategory(mock.HomeDecoration(), mock.CategoryVases)},
	} {
		t.Run(tt.listID, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.listID, tt.filter.ListID())
			assert.Equal(t, tt.filter.Category, tt.filter.ListName())
			assert.Equal(t, itemIDs(tt.want), itemIDs(tt.filter.Apply(mock.Catalog())))
		})
	}

	assert.Len(t, mock.Dresses(), 18)
	assert.Len(t, mock.Trousers(), 18)
}

// TestPersonaTopMatch is what keeps the corpus worth generating: four different tastes
// must reach four different products. Kate and Mira both dress for business, so occasion
// alone must not decide the outcome; and the categories compete, each scored with its own
// weights, so three of the four end up in trousers — Kate because the trouser weights
// count tailoring more than the dress weights do.
func TestPersonaTopMatch(t *testing.T) {
	t.Parallel()

	catalog := mock.Catalog()

	for _, tt := range []struct {
		persona  mock.Persona
		occasion string
		want     string
	}{
		{mock.PersonaBusinessKate, mock.OccasionBusiness, "TROUSERS_01"},
		{mock.PersonaFestiveSofia, mock.OccasionFestive, "DRESS_08"},
		{mock.PersonaCasualLena, mock.OccasionCasual, "TROUSERS_10"},
		{mock.PersonaModernMira, mock.OccasionBusiness, "TROUSERS_02"},
	} {
		t.Run(tt.persona.Name, func(t *testing.T) {
			t.Parallel()

			got := tt.persona.TopMatch(catalog)

			assert.Equal(t, tt.occasion, tt.persona.Occasion())
			assert.Equal(t, tt.want, got.ItemID,
				"%s, %s, matched %q: %s", tt.persona.Name, tt.persona.Description, got.ItemName, got.Description)
		})
	}

	seen := map[string]string{}

	for _, p := range mock.Personas() {
		match := p.TopMatch(catalog)
		assert.NotContains(t, seen, match.ItemID,
			"%s and %s share a top match, %q: %s", seen[match.ItemID], p.Name, match.ItemName, match.Description)
		seen[match.ItemID] = p.Name
	}
}

// TestAffinitySort covers the ranking itself: best match first, and stable across runs
// even when scores tie.
func TestAffinitySort(t *testing.T) {
	t.Parallel()

	// a profile that has learned Mira's taste exactly
	f := mock.SectionFilter(mock.PersonaModernMira).Personalize(mock.Profile{Taste: mock.PersonaModernMira.Affinity, Signals: 1})
	got := f.Apply(mock.Catalog())

	require.NotEmpty(t, got)

	// The ranking in readable form, so -v says what the order means rather than which
	// ids it happened to produce.
	t.Logf("ranked for %s, %s:", mock.PersonaModernMira.Name, mock.PersonaModernMira.Description)

	for i, item := range got {
		t.Logf("  %d. %-26s %s", i+1, item.ItemName, item.Description)
	}

	assert.Equal(t, "TROUSERS_02", got[0].ItemID, "best match should rank first")

	for _, item := range got {
		assert.Equal(t, mock.OccasionBusiness, item.Occasion())
	}

	want := itemIDs(got)
	for range 20 {
		assert.Equal(t, want, itemIDs(f.Apply(mock.Catalog())))
	}
}

// TestNeutralProfileKeepsCatalogOrder is where every shopper starts: a profile that has
// learned nothing scores every product the same, and the listing falls back to the
// catalog order rather than to anything arbitrary.
func TestNeutralProfileKeepsCatalogOrder(t *testing.T) {
	t.Parallel()

	got := mock.Filter{Category: mock.CategoryWomensFashion, Sort: mock.SortAffinity}.Apply(mock.Catalog())
	assert.Equal(t, itemIDs(mock.Fashion()), itemIDs(got))
}

// TestFilterApply pins that a listing is genuinely the output of its filter, rather than
// a hand-picked set that could contradict the facets it reports.
func TestFilterApply(t *testing.T) {
	t.Parallel()

	broad := mock.ViewItemList(mock.CategoryFilter)
	narrow := mock.ViewItemList(mock.NarrowedFilter)

	assert.Equal(t,
		itemIDs(mock.NarrowedFilter.Apply(mock.Catalog())),
		itemIDs(narrow.Params.Items),
	)

	assert.Equal(t, len(narrow.Params.Items), narrow.Params.ResultCount)
	assert.Subset(t, itemIDs(broad.Params.Items), itemIDs(narrow.Params.Items))
	assert.Less(t, len(narrow.Params.Items), len(broad.Params.Items),
		"the facets should actually narrow the listing")

	for _, item := range narrow.Params.Items {
		assert.Equal(t, mock.NarrowedFilter.Occasion, item.Occasion())
		assert.Equal(t, mock.NarrowedFilter.Length, item.Length())
	}
}

func TestFilterQuery(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "length=maxi&occasion=festive&sort=affinity", mock.NarrowedFilter.Query())
	assert.Equal(t, "sort=affinity", mock.CategoryFilter.Query())
	assert.Equal(t, "occasion=business&sort=affinity", mock.SectionFilter(mock.PersonaBusinessKate).Query())
}

// TestItemListParamsDecode covers the custom param struct: it must survive DecodeParams
// and carry the facets as fields rather than an opaque string.
func TestItemListParamsDecode(t *testing.T) {
	t.Parallel()

	body, err := os.ReadFile(filepath.Join(mock.FixtureDir, "view_item_list_personalized.mpv2.json"))
	require.NoError(t, err)

	var payload mpv2.Payload[any]

	require.NoError(t, json.Unmarshal(body, &payload))
	require.Len(t, payload.Events, 1)
	require.Equal(t, sesamy.EventNameViewItemList, payload.Events[0].Name)

	var p mock.ItemListParams

	require.NoError(t, payload.Events[0].DecodeParams(&p))

	assert.Equal(t, mock.CategoryFilter.ListID(), p.ItemListID)
	assert.NotEmpty(t, p.Items)
	assert.Equal(t, mock.CategoryWomensFashion, p.FilterCategory)
	assert.Equal(t, mock.OccasionBusiness, p.FilterOccasion)
	assert.Equal(t, mock.SortAffinity, p.SortBy)
	assert.True(t, p.Personalized)
	assert.Equal(t, 3, p.TasteSignals, "ranked by the three picks before it")
	assert.Equal(t, len(p.Items), p.ResultCount)
}

// TestJourneyIsMixed pins the shape journeys are built in: differently typed events —
// what sesamy-go hands downstream as sesamy.Event[any] — along a route that crosses both
// rails and comes back to the department.
func TestJourneyIsMixed(t *testing.T) {
	t.Parallel()

	names := make([]sesamy.EventName, 0)
	var lists []string

	for _, e := range mock.Journey(mock.PersonaBusinessKate).Events() {
		names = append(names, e.Name)

		if e.Name == sesamy.EventNameViewItemList {
			var p mock.ItemListParams
			require.NoError(t, e.DecodeParams(&p))
			lists = append(lists, p.ItemListID)
		}
	}

	assert.Equal(t, []string{
		"womens-fashion", "womens-fashion/trousers", "womens-fashion/dresses", "womens-fashion", "womens-fashion",
		"home-decoration", "home-decoration/vases", "all-products",
	}, lists)
	assert.Equal(t, []sesamy.EventName{
		sesamy.EventNameViewItemList, sesamy.EventNameSelectItem,
		sesamy.EventNameViewItemList, sesamy.EventNameSelectItem,
		sesamy.EventNameViewItemList, sesamy.EventNameSelectItem,
		sesamy.EventNameViewItemList, sesamy.EventNameSelectItem,
		sesamy.EventNameViewItem,
		sesamy.EventNameAddToCart,
		sesamy.EventNameViewItemList, sesamy.EventNameSelectItem,
		sesamy.EventNameViewItemList, sesamy.EventNameSelectItem,
		sesamy.EventNameViewItemList, sesamy.EventNameSelectItem,
		sesamy.EventNameViewItem,
		sesamy.EventNameViewItemList,
	}, names)
}

// TestEveryListIsInteractedWith pins that no listing is passed by: in every journey,
// converting or not, each view_item_list is followed by a select_item from that same
// list — the same list id, and an item the listing actually showed. The one exception is
// the last: a visit ends on the listing of all products, with nothing opened from it.
func TestEveryListIsInteractedWith(t *testing.T) {
	t.Parallel()

	for _, p := range mock.Personas() {
		for name, journey := range map[string]mock.Session{"journey": mock.Journey(p), "browse": mock.BrowseJourney(p)} {
			t.Run(p.Name+" "+name, func(t *testing.T) {
				t.Parallel()

				events := journey.Events()
				lists := 0

				for i, e := range events {
					if e.Name != sesamy.EventNameViewItemList {
						continue
					}

					lists++

					var list mock.ItemListParams
					require.NoError(t, e.DecodeParams(&list))

					if list.ItemListID == mock.AllProductsListID {
						assert.Equal(t, len(events)-1, i, "the visit ends on %s", list.ItemListID)

						continue
					}

					require.Less(t, i+1, len(events), "%s is the last event", list.ItemListID)
					require.Equal(t, sesamy.EventNameSelectItem, events[i+1].Name, "nothing selected from %s", list.ItemListID)

					var selected params.SelectItem[mock.Item]
					require.NoError(t, events[i+1].DecodeParams(&selected))
					assert.Equal(t, list.ItemListID, selected.ItemListID)
					require.Len(t, selected.Items, 1)
					assert.Contains(t, itemIDs(list.Items), selected.Items[0].ItemID, "selected from %s but not shown there", list.ItemListID)
				}

				assert.Equal(t, 8, lists)
			})
		}
	}
}

// TestJourneyIsPersonaDriven is the end to end version of TestPersonaTopMatch. A shopper
// does not always take its favourite, so the product in the cart need not be the
// persona's best match — but it is the one it opened from its section, and one it likes:
// within GoodEnough of its favourite there.
func TestJourneyIsPersonaDriven(t *testing.T) {
	t.Parallel()

	for _, p := range mock.Personas() {
		var added []mock.Item

		for _, e := range mock.Journey(p).Events() {
			if e.Name != sesamy.EventNameAddToCart {
				continue
			}

			var ap params.AddToCart[mock.Item]

			require.NoError(t, e.DecodeParams(&ap))
			added = append(added, ap.Items...)
		}

		require.Len(t, added, 1, p.Name)

		section := mock.SectionFilter(p).Apply(mock.Catalog())
		assert.Contains(t, itemIDs(section), added[0].ItemID, "%s bought outside its section", p.Name)
		assert.True(t, p.Likes(added[0], section), "%s, %s, bought %s, which it does not like", p.Name, p.Description, added[0].ItemID)
	}
}

// TestBrowseJourneyDoesNotConvert keeps the non-converting session honest.
func TestBrowseJourneyDoesNotConvert(t *testing.T) {
	t.Parallel()

	for _, e := range mock.BrowseJourney(mock.PersonaBusinessKate).Events() {
		assert.NotEqual(t, sesamy.EventNameAddToCart, e.Name)
	}
}

// TestMonthIsAMonth pins the shape of a history: one session per visit, oldest first,
// starting inside the thirty days up to BaseTime, each with a session id of its own.
func TestMonthIsAMonth(t *testing.T) {
	t.Parallel()

	visits := mock.Month(mock.PersonaBusinessKate)
	history := mock.History(visits, mock.WithUserID("U-KATE"))

	require.Len(t, history, len(visits))

	sessions := map[string]bool{}
	conversions := 0

	for i, session := range history {
		at := session.Start()

		assert.False(t, at.After(mock.BaseTime), "visit %d is after BaseTime", i)
		assert.False(t, at.Before(mock.BaseTime.Add(-30*mock.Day)), "visit %d is more than a month old", i)
		if i > 0 {
			assert.True(t, at.After(history[i-1].Start()), "visit %d is out of order", i)
		}

		for _, hit := range session {
			assert.Equal(t, "U-KATE", hit.UserID, "options reach every request of every visit")
		}

		assert.NotContains(t, sessions, session.ID(), "visit %d reuses a session", i)
		sessions[session.ID()] = true

		for _, e := range session.Events() {
			if e.Name == sesamy.EventNameAddToCart {
				conversions++
			}
		}
	}

	assert.Equal(t, mock.BaseTime, history[len(history)-1].Start(), "the last visit starts now")
	assert.Equal(t, 2, conversions)
}

// TestHistoryOrdersVisits keeps History safe to hand a hand-written schedule: whatever
// order the visits are listed in, the sessions come out oldest first, and each visit's
// persona drives its own session.
func TestHistoryOrdersVisits(t *testing.T) {
	t.Parallel()

	history := mock.History([]mock.Visit{
		{Persona: mock.PersonaBusinessKate, Ago: 0, Convert: true},
		{Persona: mock.PersonaFestiveSofia, Ago: 10 * mock.Day, Convert: true},
	})

	require.Len(t, history, 2)
	assert.True(t, history[0].Start().Before(history[1].Start()))

	for i, want := range []mock.Persona{mock.PersonaFestiveSofia, mock.PersonaBusinessKate} {
		var ap params.AddToCart[mock.Item]
		events := history[i].Events()
		cart := slices.IndexFunc(events, func(e sesamy.Event[any]) bool { return e.Name == sesamy.EventNameAddToCart })

		require.NotEqual(t, -1, cart, "visit %d did not convert", i)
		require.NoError(t, events[cart].DecodeParams(&ap))
		section := mock.SectionFilter(want).Apply(mock.Catalog())
		assert.Equal(t, want.Occasion(), ap.Items[0].Occasion(), "visit %d", i)
		assert.True(t, want.Likes(ap.Items[0], section), "visit %d: %s does not like %s", i, want.Name, ap.Items[0].ItemID)
	}
}

// listings returns the params of every view_item_list in events, in order.
func listings(t *testing.T, events []sesamy.Event[any]) []mock.ItemListParams {
	t.Helper()

	var out []mock.ItemListParams

	for _, e := range events {
		if e.Name != sesamy.EventNameViewItemList {
			continue
		}

		var p mock.ItemListParams
		require.NoError(t, e.DecodeParams(&p))
		out = append(out, p)
	}

	return out
}

// TestEveryListIsPersonalized pins that the shop never renders an unranked listing: in
// every fixture, journey and history, each view_item_list is sorted by affinity.
func TestEveryListIsPersonalized(t *testing.T) {
	t.Parallel()

	var events []sesamy.Event[any]
	for _, p := range mock.Personas() {
		events = append(events, mock.Journey(p).Events()...)
		events = append(events, mock.BrowseJourney(p).Events()...)

		for _, session := range mock.History(mock.Month(p)) {
			events = append(events, session.Events()...)
		}
	}

	for _, e := range []sesamy.Event[mock.ItemListParams]{
		mock.ViewItemList(mock.CategoryFilter), mock.ViewItemList(mock.DressesFilter),
		mock.ViewItemList(mock.TrousersFilter), mock.ViewItemList(mock.NarrowedFilter),
	} {
		events = append(events, e.AnyEvent())
	}

	all := listings(t, events)
	require.NotEmpty(t, all)

	for _, p := range all {
		assert.True(t, p.Personalized, p.ItemListID)
		assert.Equal(t, mock.SortAffinity, p.SortBy, p.ItemListID)
	}
}

// TestJourneyStartsNeutral is a first visit: the shop knows nothing, so the landing page
// is ranked by a neutral profile — catalog order — and every listing after it by one more
// pick than the last.
func TestJourneyStartsNeutral(t *testing.T) {
	t.Parallel()

	for _, p := range mock.Personas() {
		lists := listings(t, mock.Journey(p).Events())
		require.Len(t, lists, 8)

		assert.Equal(t, itemIDs(mock.Fashion()), itemIDs(lists[0].Items), "%s lands on the catalog order", p.Name)

		for i, list := range lists[:4] {
			assert.Equal(t, i, list.TasteSignals, "%s, listing %d (%s)", p.Name, i+1, list.ItemListID)
		}

		// four picks, the view and the cart
		assert.Equal(t, 6, lists[4].TasteSignals, "%s, closing listing", p.Name)
	}
}

// TestHistoryLearns is the shop getting to know a returning shopper. The profile carries
// from visit to visit, so after the first visit the landing page is no longer neutral —
// and the product it puts first is one the shopper likes, though the shop never saw the
// persona, only what the shopper picked. Not necessarily its favourite: the shopper does
// not always open its favourite, and the shop learns from what it does open.
func TestHistoryLearns(t *testing.T) {
	t.Parallel()

	for _, p := range mock.Personas() {
		t.Run(p.Name, func(t *testing.T) {
			t.Parallel()

			previous := -1

			for i, session := range mock.History(mock.Month(p)) {
				landing := listings(t, session.Events())[0]

				assert.Greater(t, landing.TasteSignals, previous, "visit %d learned nothing", i+1)
				previous = landing.TasteSignals

				if i == 0 {
					assert.Zero(t, landing.TasteSignals, "the first visit starts neutral")

					continue
				}

				assert.True(t, p.Likes(landing.Items[0], landing.Items), "visit %d: the landing page opens on %s, which %s does not like", i+1, landing.Items[0].ItemName, p.Name)
			}
		})
	}
}

// replayListings feeds events in order into a shopper's affinity corpus, as the shop
// does, and calls check with every listing and the vector it should have been ranked by:
// what the corpus emitted just before it. Hand it the same corpus for every visit of a
// history and the vector carries over, as it does in the shop.
func replayListings(t *testing.T, c *affinity.Corpus, events []sesamy.Event[any], check func(mock.ItemListParams, mock.Profile)) {
	t.Helper()

	for _, e := range events {
		if e.Name == sesamy.EventNameViewItemList {
			var p mock.ItemListParams
			require.NoError(t, e.DecodeParams(&p))
			check(p, mock.Profile{Taste: c.Derive(affinity.DefaultFade.Vector), Signals: c.Len()})
		}

		c.Add(affinity.Envelope{}, e)
	}
}

// assertSortedBy checks a listing against the profile replayed for it, without going
// through Filter.Apply: the signal count must match, the scores must never rise down the
// list, and the order must be the listing's products sorted by score, ties by item id.
func assertSortedBy(t *testing.T, list mock.ItemListParams, prof mock.Profile, label string) {
	t.Helper()

	assert.Equal(t, prof.Signals, list.TasteSignals, "%s: signals", label)

	for i := 1; i < len(list.Items); i++ {
		prev, cur := mock.Score(prof.Taste, list.Items[i-1]), mock.Score(prof.Taste, list.Items[i])
		assert.GreaterOrEqual(t, prev, cur, "%s: %s ranked above %s", label, list.Items[i-1].ItemID, list.Items[i].ItemID)
	}

	want := slices.Clone(mock.Filter{Category: list.FilterCategory, Occasion: list.FilterOccasion}.Apply(mock.Catalog()))
	slices.SortStableFunc(want, func(a, b mock.Item) int {
		if c := cmp.Compare(mock.Score(prof.Taste, b), mock.Score(prof.Taste, a)); c != 0 {
			return c
		}

		return cmp.Compare(a.ItemID, b.ItemID)
	})
	assert.Equal(t, itemIDs(want), itemIDs(list.Items), "%s: order", label)
}

// TestListingsAreSortedByWhatWasLearned verifies the sorting of every listing — the
// closing look at the department above all — against a profile rebuilt from the
// journey's own events, for single journeys and for whole histories, where the profile
// carries from visit to visit.
func TestListingsAreSortedByWhatWasLearned(t *testing.T) {
	t.Parallel()

	for _, p := range mock.Personas() {
		t.Run(p.Name, func(t *testing.T) {
			t.Parallel()

			for name, journey := range map[string]mock.Session{"journey": mock.Journey(p), "browse": mock.BrowseJourney(p)} {
				n := 0
				replayListings(t, affinity.New(nil), journey.Events(), func(list mock.ItemListParams, prof mock.Profile) {
					n++
					assertSortedBy(t, list, prof, fmt.Sprintf("%s listing %d (%s)", name, n, list.ItemListID))
				})
			}

			c := affinity.New(nil)
			for i, session := range mock.History(mock.Month(p)) {
				n := 0
				replayListings(t, c, session.Events(), func(list mock.ItemListParams, prof mock.Profile) {
					n++
					assertSortedBy(t, list, prof, fmt.Sprintf("visit %d listing %d (%s)", i+1, n, list.ItemListID))
				})
			}
		})
	}
}

// TestJourneyClosesOnTheDepartment pins the end of every visit: back to the top-level
// department, unnarrowed, ranked by everything the visit taught the shop — and what comes
// first there is something the shopper likes.
func TestJourneyClosesOnTheDepartment(t *testing.T) {
	t.Parallel()

	for _, p := range mock.Personas() {
		for name, journey := range map[string]mock.Session{"journey": mock.Journey(p), "browse": mock.BrowseJourney(p)} {
			lists := listings(t, journey.Events())
			closing := lists[4] // the fashion route, then the closing look

			assert.Equal(t, mock.CategoryFilter.ListID(), closing.ItemListID, "%s %s", p.Name, name)
			assert.Empty(t, closing.FilterOccasion, "%s %s: the closing look is unnarrowed", p.Name, name)
			assert.Len(t, closing.Items, len(mock.Fashion()))

			assert.True(t, p.Likes(closing.Items[0], closing.Items), "%s %s: the closing look opens on %s", p.Name, name, closing.Items[0].ItemName)
		}
	}
}

// chosenCategories are the categories of the products selected, viewed or put in the
// cart in events — the categories the shopper chose from rather than was only shown.
func chosenCategories(t *testing.T, events []sesamy.Event[any]) []string {
	t.Helper()

	var out []string

	for _, e := range events {
		if e.Name == sesamy.EventNameViewItemList {
			continue
		}

		var p mock.ItemListParams
		require.NoError(t, e.DecodeParams(&p))

		for _, item := range p.Items {
			if !slices.Contains(out, item.Category()) {
				out = append(out, item.Category())
			}
		}
	}

	return out
}

// TestJourneyEndsOnAllProducts reads the shop's final ranking of a visit: every visit
// ends on the top-level listing of all products, ranked by everything learned — its
// order is checked against a replayed profile by TestListingsAreSortedByWhatWasLearned.
// Read per category, it has to be right where the shopper chose: in every category it
// selected, viewed or bought from, that category's first product in the final ranking is
// one the persona likes. Not necessarily its favourite — the shop learns from what the
// shopper opened, and the shopper does not always open its favourite. The visit ends
// there: the listing is the last event.
func TestJourneyEndsOnAllProducts(t *testing.T) {
	t.Parallel()

	for _, p := range mock.Personas() {
		t.Run(p.Name, func(t *testing.T) {
			t.Parallel()

			visits := map[string][]sesamy.Event[any]{
				"journey": mock.Journey(p).Events(),
				"browse":  mock.BrowseJourney(p).Events(),
			}
			for i, session := range mock.History(mock.Month(p)) {
				visits[fmt.Sprintf("visit %d", i+1)] = session.Events()
			}

			for name, events := range visits {
				lists := listings(t, events)
				final := lists[len(lists)-1]

				require.Equal(t, mock.AllProductsListID, final.ItemListID, name)
				assert.Equal(t, sesamy.EventNameViewItemList, events[len(events)-1].Name, "%s ends on a listing", name)
				assert.Equal(t, mock.AllProductsListName, final.ItemListName, name)
				assert.Empty(t, final.FilterCategory, name)
				require.Len(t, final.Items, len(mock.Catalog()), name)

				for _, category := range chosenCategories(t, events) {
					first := mock.Filter{Category: category}.Apply(final.Items)[0]
					assert.True(t, p.Likes(first, inCategory(mock.Catalog(), category)), "%s, %s: %s comes first, which %s does not like", name, category, first.ItemName, p.Name)
				}
			}
		})
	}
}

// inCategory keeps the items of one category, in their order.
func inCategory(items []mock.Item, category string) []mock.Item {
	var out []mock.Item

	for _, item := range items {
		if item.Category() == category {
			out = append(out, item)
		}
	}

	return out
}

// TestVisitsAreSentRequestByRequest pins how a visit reaches the wire: as a client sends
// it, one request per event, not one envelope for the whole visit. The requests of a visit
// share its session id and client, follow each other in time at the pace a shopper takes
// — seconds to scan a listing, longer to look at a product — and each reports as its
// engagement time the time since the request before.
func TestVisitsAreSentRequestByRequest(t *testing.T) {
	t.Parallel()

	paces := map[sesamy.EventName][2]time.Duration{
		sesamy.EventNameViewItemList: {5 * time.Second, 25 * time.Second},
		sesamy.EventNameSelectItem:   {20 * time.Second, 75 * time.Second},
		sesamy.EventNameViewItem:     {15 * time.Second, 60 * time.Second},
		sesamy.EventNameAddToCart:    {5 * time.Second, 15 * time.Second},
	}

	for _, p := range mock.Personas() {
		visits := map[string]mock.Session{"journey": mock.Journey(p), "browse": mock.BrowseJourney(p)}
		for i, session := range mock.History(mock.Month(p)) {
			visits[fmt.Sprintf("visit %d", i+1)] = session
		}

		for name, session := range visits {
			label := p.Name + " " + name
			require.Greater(t, len(session), 10, label)

			for i, hit := range session {
				require.Len(t, hit.Events, 1, "%s: request %d carries more than one event", label, i+1)
				assert.Equal(t, session.ID(), hit.SessionID, label)
				assert.Equal(t, session[0].ClientID, hit.ClientID, label)

				if i == 0 {
					assert.Equal(t, int64(1000), hit.EngagementTimeMSec, label)

					continue
				}

				gap := time.Duration(hit.TimestampMicros-session[i-1].TimestampMicros) * time.Microsecond
				pace := paces[session[i-1].Events[0].Name]

				assert.Equal(t, gap.Milliseconds(), hit.EngagementTimeMSec, "%s: request %d", label, i+1)
				assert.GreaterOrEqual(t, gap, pace[0], "%s: request %d after %s", label, i+1, session[i-1].Events[0].Name)
				assert.LessOrEqual(t, gap, pace[1], "%s: request %d after %s", label, i+1, session[i-1].Events[0].Name)
			}
		}
	}
}
