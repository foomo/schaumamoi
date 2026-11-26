package shopper_test

import (
	"cmp"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/foomo/schaumamoi/pkg/distillery/stills/affinity"
	"github.com/foomo/schaumamoi/pkg/internal/mock/catalog"
	"github.com/foomo/schaumamoi/pkg/internal/mock/persona"
	"github.com/foomo/schaumamoi/pkg/internal/mock/shop"
	"github.com/foomo/schaumamoi/pkg/internal/mock/shopper"
	"github.com/foomo/schaumamoi/pkg/internal/mock/wire"
	"github.com/foomo/sesamy-go/pkg/event/params"
	"github.com/foomo/sesamy-go/pkg/sesamy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func itemIDs(items []catalog.Item) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, item.ItemID)
	}

	return out
}

// TestJourneyIsMixed pins the shape journeys are built in: differently typed events —
// what sesamy-go hands downstream as sesamy.Event[any] — along a route that crosses both
// rails and comes back to the department.
func TestJourneyIsMixed(t *testing.T) {
	t.Parallel()

	names := make([]sesamy.EventName, 0)
	var lists []string

	for _, e := range shopper.Journey(persona.Kate).Events() {
		names = append(names, e.Name)

		if e.Name == sesamy.EventNameViewItemList {
			var p shop.ItemListParams
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

	for _, p := range persona.All() {
		for name, journey := range map[string]wire.Session{"journey": shopper.Journey(p), "browse": shopper.BrowseJourney(p)} {
			t.Run(p.Name+" "+name, func(t *testing.T) {
				t.Parallel()

				events := journey.Events()
				lists := 0

				for i, e := range events {
					if e.Name != sesamy.EventNameViewItemList {
						continue
					}

					lists++

					var list shop.ItemListParams
					require.NoError(t, e.DecodeParams(&list))

					if list.ItemListID == shop.AllProductsListID {
						assert.Equal(t, len(events)-1, i, "the visit ends on %s", list.ItemListID)

						continue
					}

					require.Less(t, i+1, len(events), "%s is the last event", list.ItemListID)
					require.Equal(t, sesamy.EventNameSelectItem, events[i+1].Name, "nothing selected from %s", list.ItemListID)

					var selected params.SelectItem[catalog.Item]
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

	for _, p := range persona.All() {
		var added []catalog.Item

		for _, e := range shopper.Journey(p).Events() {
			if e.Name != sesamy.EventNameAddToCart {
				continue
			}

			var ap params.AddToCart[catalog.Item]

			require.NoError(t, e.DecodeParams(&ap))
			added = append(added, ap.Items...)
		}

		require.Len(t, added, 1, p.Name)

		section := shopper.SectionFilter(p).Apply(catalog.All())
		assert.Contains(t, itemIDs(section), added[0].ItemID, "%s bought outside its section", p.Name)
		assert.True(t, p.Likes(added[0], section), "%s, %s, bought %s, which it does not like", p.Name, p.Description, added[0].ItemID)
	}
}

// TestBrowseJourneyDoesNotConvert keeps the non-converting session honest.
func TestBrowseJourneyDoesNotConvert(t *testing.T) {
	t.Parallel()

	for _, e := range shopper.BrowseJourney(persona.Kate).Events() {
		assert.NotEqual(t, sesamy.EventNameAddToCart, e.Name)
	}
}

// TestMonthIsAMonth pins the shape of a history: one session per visit, oldest first,
// starting inside the thirty days up to BaseTime, each with a session id of its own.
func TestMonthIsAMonth(t *testing.T) {
	t.Parallel()

	visits := shopper.Month(persona.Kate)
	history := shopper.History(visits, wire.WithUserID("U-KATE"))

	require.Len(t, history, len(visits))

	sessions := map[string]bool{}
	conversions := 0

	for i, session := range history {
		at := session.Start()

		assert.False(t, at.After(wire.BaseTime), "visit %d is after BaseTime", i)
		assert.False(t, at.Before(wire.BaseTime.Add(-30*shopper.Day)), "visit %d is more than a month old", i)
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

	assert.Equal(t, wire.BaseTime, history[len(history)-1].Start(), "the last visit starts now")
	assert.Equal(t, 2, conversions)
}

// TestHistoryOrdersVisits keeps History safe to hand a hand-written schedule: whatever
// order the visits are listed in, the sessions come out oldest first, and each visit's
// persona drives its own session.
func TestHistoryOrdersVisits(t *testing.T) {
	t.Parallel()

	history := shopper.History([]shopper.Visit{
		{Persona: persona.Kate, Ago: 0, Convert: true},
		{Persona: persona.Sofia, Ago: 10 * shopper.Day, Convert: true},
	})

	require.Len(t, history, 2)
	assert.True(t, history[0].Start().Before(history[1].Start()))

	for i, want := range []persona.Persona{persona.Sofia, persona.Kate} {
		var ap params.AddToCart[catalog.Item]
		events := history[i].Events()
		cart := slices.IndexFunc(events, func(e sesamy.Event[any]) bool { return e.Name == sesamy.EventNameAddToCart })

		require.NotEqual(t, -1, cart, "visit %d did not convert", i)
		require.NoError(t, events[cart].DecodeParams(&ap))
		section := shopper.SectionFilter(want).Apply(catalog.All())
		assert.Equal(t, want.Occasion(), ap.Items[0].Occasion(), "visit %d", i)
		assert.True(t, want.Likes(ap.Items[0], section), "visit %d: %s does not like %s", i, want.Name, ap.Items[0].ItemID)
	}
}

// listings returns the params of every view_item_list in events, in order.
func listings(t *testing.T, events []sesamy.Event[any]) []shop.ItemListParams {
	t.Helper()

	var out []shop.ItemListParams

	for _, e := range events {
		if e.Name != sesamy.EventNameViewItemList {
			continue
		}

		var p shop.ItemListParams
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
	for _, p := range persona.All() {
		events = append(events, shopper.Journey(p).Events()...)
		events = append(events, shopper.BrowseJourney(p).Events()...)

		for _, session := range shopper.History(shopper.Month(p)) {
			events = append(events, session.Events()...)
		}
	}

	for _, e := range []sesamy.Event[shop.ItemListParams]{
		shop.ViewItemList(shop.CategoryFilter), shop.ViewItemList(shop.DressesFilter),
		shop.ViewItemList(shop.TrousersFilter), shop.ViewItemList(shop.NarrowedFilter),
	} {
		events = append(events, e.AnyEvent())
	}

	all := listings(t, events)
	require.NotEmpty(t, all)

	for _, p := range all {
		assert.True(t, p.Personalized, p.ItemListID)
		assert.Equal(t, shop.SortAffinity, p.SortBy, p.ItemListID)
	}
}

// TestJourneyStartsNeutral is a first visit: the shop knows nothing, so the landing page
// is ranked by a neutral profile — catalog order — and every listing after it by one more
// pick than the last.
func TestJourneyStartsNeutral(t *testing.T) {
	t.Parallel()

	for _, p := range persona.All() {
		lists := listings(t, shopper.Journey(p).Events())
		require.Len(t, lists, 8)

		assert.Equal(t, itemIDs(catalog.Fashion()), itemIDs(lists[0].Items), "%s lands on the catalog order", p.Name)

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

	for _, p := range persona.All() {
		t.Run(p.Name, func(t *testing.T) {
			t.Parallel()

			previous := -1

			for i, session := range shopper.History(shopper.Month(p)) {
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

// replayListings feeds events in order into a shopper's affinity still, as the shop
// does, and calls check with every listing and the vector it should have been ranked by:
// what the still distilled just before it. Hand it the same still for every visit of a
// history and the vector carries over, as it does in the shop.
func replayListings(t *testing.T, c *affinity.Still, events []sesamy.Event[any], check func(shop.ItemListParams, shop.Profile)) {
	t.Helper()

	for _, e := range events {
		if e.Name == sesamy.EventNameViewItemList {
			var p shop.ItemListParams
			require.NoError(t, e.DecodeParams(&p))
			check(p, shop.Profile{Taste: c.Distill(affinity.DefaultFade.Vector), Signals: c.Len()})
		}

		c.Feed(affinity.Envelope{}, e)
	}
}

// assertSortedBy checks a listing against the profile replayed for it, without going
// through Filter.Apply: the signal count must match, the scores must never rise down the
// list, and the order must be the listing's products sorted by score, ties by item id.
func assertSortedBy(t *testing.T, list shop.ItemListParams, prof shop.Profile, label string) {
	t.Helper()

	assert.Equal(t, prof.Signals, list.TasteSignals, "%s: signals", label)

	for i := 1; i < len(list.Items); i++ {
		prev, cur := catalog.Score(prof.Taste, list.Items[i-1]), catalog.Score(prof.Taste, list.Items[i])
		assert.GreaterOrEqual(t, prev, cur, "%s: %s ranked above %s", label, list.Items[i-1].ItemID, list.Items[i].ItemID)
	}

	want := slices.Clone(shop.Filter{Category: list.FilterCategory, Occasion: list.FilterOccasion}.Apply(catalog.All()))
	slices.SortStableFunc(want, func(a, b catalog.Item) int {
		if c := cmp.Compare(catalog.Score(prof.Taste, b), catalog.Score(prof.Taste, a)); c != 0 {
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

	for _, p := range persona.All() {
		t.Run(p.Name, func(t *testing.T) {
			t.Parallel()

			for name, journey := range map[string]wire.Session{"journey": shopper.Journey(p), "browse": shopper.BrowseJourney(p)} {
				n := 0
				replayListings(t, affinity.New(nil), journey.Events(), func(list shop.ItemListParams, prof shop.Profile) {
					n++
					assertSortedBy(t, list, prof, fmt.Sprintf("%s listing %d (%s)", name, n, list.ItemListID))
				})
			}

			c := affinity.New(nil)
			for i, session := range shopper.History(shopper.Month(p)) {
				n := 0
				replayListings(t, c, session.Events(), func(list shop.ItemListParams, prof shop.Profile) {
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

	for _, p := range persona.All() {
		for name, journey := range map[string]wire.Session{"journey": shopper.Journey(p), "browse": shopper.BrowseJourney(p)} {
			lists := listings(t, journey.Events())
			closing := lists[4] // the fashion route, then the closing look

			assert.Equal(t, shop.CategoryFilter.ListID(), closing.ItemListID, "%s %s", p.Name, name)
			assert.Empty(t, closing.FilterOccasion, "%s %s: the closing look is unnarrowed", p.Name, name)
			assert.Len(t, closing.Items, len(catalog.Fashion()))

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

		var p shop.ItemListParams
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

	for _, p := range persona.All() {
		t.Run(p.Name, func(t *testing.T) {
			t.Parallel()

			visits := map[string][]sesamy.Event[any]{
				"journey": shopper.Journey(p).Events(),
				"browse":  shopper.BrowseJourney(p).Events(),
			}
			for i, session := range shopper.History(shopper.Month(p)) {
				visits[fmt.Sprintf("visit %d", i+1)] = session.Events()
			}

			for name, events := range visits {
				lists := listings(t, events)
				final := lists[len(lists)-1]

				require.Equal(t, shop.AllProductsListID, final.ItemListID, name)
				assert.Equal(t, sesamy.EventNameViewItemList, events[len(events)-1].Name, "%s ends on a listing", name)
				assert.Equal(t, shop.AllProductsListName, final.ItemListName, name)
				assert.Empty(t, final.FilterCategory, name)
				require.Len(t, final.Items, len(catalog.All()), name)

				for _, category := range chosenCategories(t, events) {
					first := shop.Filter{Category: category}.Apply(final.Items)[0]
					assert.True(t, p.Likes(first, inCategory(catalog.All(), category)), "%s, %s: %s comes first, which %s does not like", name, category, first.ItemName, p.Name)
				}
			}
		})
	}
}

// inCategory keeps the items of one category, in their order.
func inCategory(items []catalog.Item, category string) []catalog.Item {
	var out []catalog.Item

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

	for _, p := range persona.All() {
		visits := map[string]wire.Session{"journey": shopper.Journey(p), "browse": shopper.BrowseJourney(p)}
		for i, session := range shopper.History(shopper.Month(p)) {
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
