package mock_test

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/foomo/gostandards/iso4217"
	"github.com/foomo/schaumamoi/pkg/internal/mock"
	"github.com/foomo/schaumamoi/pkg/internal/mock/catalog"
	"github.com/foomo/schaumamoi/pkg/internal/mock/journeys"
	"github.com/foomo/schaumamoi/pkg/internal/mock/persona"
	"github.com/foomo/schaumamoi/pkg/internal/mock/shop"
	"github.com/foomo/schaumamoi/pkg/internal/mock/wire"
	"github.com/foomo/sesamy-go/pkg/encoding/mpv2"
	"github.com/foomo/sesamy-go/pkg/event/params"
	"github.com/foomo/sesamy-go/pkg/sesamy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var update = flag.Bool("update", false, "rewrite the testdata fixtures")

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

	for _, p := range persona.All() {
		first, err := json.Marshal(journeys.Buying(p))
		require.NoError(t, err)

		for range 20 {
			again, err := json.Marshal(journeys.Buying(p))
			require.NoError(t, err)
			assert.JSONEq(t, string(first), string(again), p.Name)
		}
	}
}

// TestItemDecode pins how the two decoders treat catalog.Item's embedded params.Item.
// They differ, and the difference decides which type you decode a collected event into.
func TestItemDecode(t *testing.T) {
	t.Parallel()

	body, err := os.ReadFile(filepath.Join(mock.FixtureDir, "add_to_cart.mpv2.json"))
	require.NoError(t, err)

	// encoding/json inlines an anonymous struct field by itself, so the GA4 fields sit
	// flat next to the attractor. This is the enrichment stage's path.
	t.Run("json", func(t *testing.T) {
		t.Parallel()

		var payload mpv2.Payload[params.AddToCart[catalog.Item]]

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

	// The trap worth pinning: the same call with catalog.Item silently returns zeroed GA4
	// fields rather than failing, because mapstructure looks for a nested "Item" key.
	t.Run("decode_params_into_mock_item_loses_ga4_fields", func(t *testing.T) {
		t.Parallel()

		var payload mpv2.Payload[any]

		require.NoError(t, json.Unmarshal(body, &payload))

		var p params.AddToCart[catalog.Item]

		require.NoError(t, payload.Events[0].DecodeParams(&p))
		require.Len(t, p.Items, 1)

		assert.Empty(t, p.Items[0].ItemID, "use params.Item for collected events")
	})
}

// TestItemDescriptionStaysOffTheWire pins the other half of catalog.Item's encoding
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

			for _, item := range catalog.All() {
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

		var payload mpv2.Payload[params.AddToCart[catalog.Item]]

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

	var payload mpv2.Payload[params.AddToCart[catalog.Item]]

	require.NoError(t, json.Unmarshal(body, &payload))

	assert.Equal(t, wire.DefaultClientID, payload.ClientID)
	assert.Equal(t, wire.DefaultSessionID, payload.SessionID)
	assert.Equal(t, wire.BaseTime.UnixMicro(), payload.TimestampMicros)
	require.Len(t, payload.Events, 1)

	e := payload.Events[0]
	assert.Equal(t, sesamy.EventNameAddToCart, e.Name)
	assert.Equal(t, iso4217.CodeEUR, e.Params.Currency)
	assert.NotZero(t, e.Params.Value)
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

	var p shop.ItemListParams

	require.NoError(t, payload.Events[0].DecodeParams(&p))

	assert.Equal(t, shop.CategoryFilter.ListID(), p.ItemListID)
	assert.NotEmpty(t, p.Items)
	assert.Equal(t, catalog.CategoryWomensFashion, p.FilterCategory)
	assert.Equal(t, catalog.OccasionBusiness, p.FilterOccasion)
	assert.Equal(t, shop.SortAffinity, p.SortBy)
	assert.True(t, p.Personalized)
	assert.Equal(t, 3, p.TasteSignals, "ranked by the three picks before it")
	assert.Equal(t, len(p.Items), p.ResultCount)
}
