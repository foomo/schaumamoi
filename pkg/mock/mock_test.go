package mock_test

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/foomo/gostandards/iso4217"
	"github.com/foomo/schaumamoi/pkg/mock"
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
	require.Len(t, catalog, 12)

	byOccasion := map[string]int{}

	for _, item := range catalog {
		assert.NotEmpty(t, item.ItemID)
		assert.NotEmpty(t, item.ItemName)
		assert.NotEmpty(t, item.Attractor, "every dress needs a taste vector")
		assert.NotEmpty(t, item.Description, "every dress needs a description")
		assert.Equal(t, mock.CategoryDresses, item.ItemCategory)
		assert.NotZero(t, item.Price)
		byOccasion[item.Occasion()]++
	}

	assert.Equal(t, map[string]int{
		mock.OccasionBusiness: 4,
		mock.OccasionFestive:  4,
		mock.OccasionCasual:   4,
	}, byOccasion)
}

// TestPersonaTopMatch is what keeps the corpus worth generating: four different tastes
// must reach four different dresses. Kate and Mira both dress for business, so occasion
// alone must not decide the outcome.
func TestPersonaTopMatch(t *testing.T) {
	t.Parallel()

	catalog := mock.Catalog()

	for _, tt := range []struct {
		persona  mock.Persona
		occasion string
		want     string
	}{
		{mock.PersonaBusinessKate, mock.OccasionBusiness, "DRESS_01"},
		{mock.PersonaFestiveSofia, mock.OccasionFestive, "DRESS_08"},
		{mock.PersonaCasualLena, mock.OccasionCasual, "DRESS_10"},
		{mock.PersonaModernMira, mock.OccasionBusiness, "DRESS_03"},
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

	f := mock.PersonalizedFilter(mock.PersonaModernMira)
	got := f.Apply(mock.Catalog())

	require.NotEmpty(t, got)

	// The ranking in readable form, so -v says what the order means rather than which
	// ids it happened to produce.
	t.Logf("ranked for %s, %s:", mock.PersonaModernMira.Name, mock.PersonaModernMira.Description)

	for i, item := range got {
		t.Logf("  %d. %-26s %s", i+1, item.ItemName, item.Description)
	}

	assert.Equal(t, "DRESS_03", got[0].ItemID, "best match should rank first")

	for _, item := range got {
		assert.Equal(t, mock.OccasionBusiness, item.Occasion())
	}

	want := itemIDs(got)
	for range 20 {
		assert.Equal(t, want, itemIDs(f.Apply(mock.Catalog())))
	}
}

// TestAffinitySortWithoutPersona keeps SortAffinity harmless when no persona is set.
func TestAffinitySortWithoutPersona(t *testing.T) {
	t.Parallel()

	got := mock.Filter{Category: mock.CategoryDresses, Sort: mock.SortAffinity}.Apply(mock.Catalog())
	assert.Equal(t, itemIDs(mock.Catalog()), itemIDs(got))
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

	assert.Equal(t, "length=maxi&occasion=festive", mock.NarrowedFilter.Query())
	assert.Empty(t, mock.CategoryFilter.Query())
	assert.Equal(t,
		"occasion=business&persona=Kate&sort=affinity",
		mock.PersonalizedFilter(mock.PersonaBusinessKate).Query(),
	)
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
	assert.Equal(t, mock.CategoryDresses, p.FilterCategory)
	assert.Equal(t, mock.OccasionBusiness, p.FilterOccasion)
	assert.Equal(t, mock.SortAffinity, p.SortBy)
	assert.True(t, p.Personalized)
	assert.Equal(t, mock.PersonaBusinessKate.Name, p.Persona)
	assert.Equal(t, len(p.Items), p.ResultCount)
}

// TestJourneyIsMixed pins the shape journeys are built in: one envelope carrying several
// differently typed events, which is what sesamy-go hands downstream.
func TestJourneyIsMixed(t *testing.T) {
	t.Parallel()

	names := make([]sesamy.EventName, 0)
	for _, e := range mock.Journey(mock.PersonaBusinessKate).Events {
		names = append(names, e.Name)
	}

	assert.Equal(t, []sesamy.EventName{
		sesamy.EventNameViewItemList,
		sesamy.EventNameViewItemList,
		sesamy.EventNameViewItemList,
		sesamy.EventNameSelectItem,
		sesamy.EventNameViewItem,
		sesamy.EventNameAddToCart,
	}, names)
}

// TestJourneyIsPersonaDriven is the end to end version of TestPersonaTopMatch: the dress
// that ends up in the cart must be the one the persona's vector picked.
func TestJourneyIsPersonaDriven(t *testing.T) {
	t.Parallel()

	seen := map[string]string{}

	for _, p := range mock.Personas() {
		var added []mock.Item

		for _, e := range mock.Journey(p).Events {
			if e.Name != sesamy.EventNameAddToCart {
				continue
			}

			var ap params.AddToCart[mock.Item]

			require.NoError(t, e.DecodeParams(&ap))
			added = append(added, ap.Items...)
		}

		// The catalog item rather than the decoded one: descriptions are catalog-only,
		// so the item that came back off the wire has none.
		want := p.TopMatch(mock.Catalog())

		require.Len(t, added, 1, p.Name)
		assert.Equal(t, want.ItemID, added[0].ItemID,
			"%s, %s, should have bought %q: %s", p.Name, p.Description, want.ItemName, want.Description)
		assert.NotContains(t, seen, added[0].ItemID,
			"%s and %s bought the same dress, %s", seen[added[0].ItemID], p.Name, added[0].ItemID)
		seen[added[0].ItemID] = p.Name
	}
}

// TestBrowseJourneyDoesNotConvert keeps the non-converting session honest.
func TestBrowseJourneyDoesNotConvert(t *testing.T) {
	t.Parallel()

	for _, e := range mock.BrowseJourney(mock.PersonaBusinessKate).Events {
		assert.NotEqual(t, sesamy.EventNameAddToCart, e.Name)
	}
}

// TestMonthIsAMonth pins the shape of a history: one session per visit, oldest first,
// inside the thirty days up to BaseTime, each with a session id of its own.
func TestMonthIsAMonth(t *testing.T) {
	t.Parallel()

	visits := mock.Month(mock.PersonaBusinessKate)
	history := mock.History(visits, mock.WithUserID("U-KATE"))

	require.Len(t, history, len(visits))

	sessions := map[string]bool{}
	conversions := 0

	for i, payload := range history {
		at := time.UnixMicro(payload.TimestampMicros)

		assert.False(t, at.After(mock.BaseTime), "visit %d is after BaseTime", i)
		assert.False(t, at.Before(mock.BaseTime.Add(-30*mock.Day)), "visit %d is more than a month old", i)
		if i > 0 {
			assert.Greater(t, payload.TimestampMicros, history[i-1].TimestampMicros, "visit %d is out of order", i)
		}

		assert.Equal(t, "U-KATE", payload.UserID, "options reach every visit")
		assert.NotContains(t, sessions, payload.SessionID, "visit %d reuses a session", i)
		sessions[payload.SessionID] = true

		for _, e := range payload.Events {
			if e.Name == sesamy.EventNameAddToCart {
				conversions++
			}
		}
	}

	assert.Equal(t, mock.BaseTime.UnixMicro(), history[len(history)-1].TimestampMicros, "the month ends now")
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
	assert.Less(t, history[0].TimestampMicros, history[1].TimestampMicros)

	for i, want := range []mock.Persona{mock.PersonaFestiveSofia, mock.PersonaBusinessKate} {
		var ap params.AddToCart[mock.Item]
		last := history[i].Events[len(history[i].Events)-1]

		require.Equal(t, sesamy.EventNameAddToCart, last.Name)
		require.NoError(t, last.DecodeParams(&ap))
		assert.Equal(t, want.TopMatch(mock.Catalog()).ItemID, ap.Items[0].ItemID, "visit %d", i)
	}
}
