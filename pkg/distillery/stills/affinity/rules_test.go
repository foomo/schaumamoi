package affinity_test

import (
	"testing"
	"time"

	"github.com/foomo/schaumamoi/pkg/distillery"
	"github.com/foomo/schaumamoi/pkg/distillery/stills/affinity"
	"github.com/foomo/schaumamoi/pkg/internal/mock/catalog"
	"github.com/foomo/schaumamoi/pkg/internal/mock/shop"
	"github.com/foomo/sesamy-go/pkg/sesamy"
	"github.com/stretchr/testify/assert"
)

var (
	day   = 24 * time.Hour
	start = time.Date(2026, time.September, 1, 10, 0, 0, 0, time.UTC)
)

// choice is one choice event of the given name, on the catalog's first product.
func choice(name sesamy.EventName) sesamy.Event[any] {
	item := catalog.All()[0]

	switch name {
	case sesamy.EventNameViewItem:
		return shop.ViewItem(item).AnyEvent()
	case sesamy.EventNameAddToCart:
		return shop.AddToCart(item).AnyEvent()
	default:
		return shop.SelectItem(shop.CategoryFilter, item).AnyEvent()
	}
}

// kept is what a still holds, as "name@day session" per event, oldest first.
func kept(still *affinity.Still) []string {
	return still.Distill(func(events []affinity.Infused) []string {
		out := make([]string, 0, len(events))
		for _, e := range events {
			out = append(out, string(e.Event.Name)+"@"+e.Infusion.At.Format("02")+" "+e.Infusion.Session)
		}

		return out
	})
}

// TestKeepLastBoundsTheStill: an affinity still holds the newest DefaultKeep choices, and
// draws off the rest after every feed — the bound the fade used to apply on every read.
func TestKeepLastBoundsTheStill(t *testing.T) {
	t.Parallel()

	still := affinity.New(nil)
	for range affinity.DefaultKeep + 5 {
		still.Feed(affinity.Envelope{}, choice(sesamy.EventNameSelectItem))
	}

	assert.Equal(t, affinity.DefaultKeep, still.Len())
}

func TestOlderThan(t *testing.T) {
	t.Parallel()

	still := affinity.New(nil)
	for i := range 4 {
		still.Feed(affinity.Envelope{At: start.Add(time.Duration(i) * 7 * day), Session: "s"}, choice(sesamy.EventNameViewItem))
	}

	still.Feed(affinity.Envelope{}, choice(sesamy.EventNameViewItem)) // no time: infinitely old

	drawn := still.DrawOff(affinity.OlderThan(start.Add(21*day), 10*day))

	assert.Equal(t, 3, drawn, "the first two weeks and the choice without a time")
	assert.Equal(t, []string{"view_item@15 s", "view_item@22 s"}, kept(still))
}

func TestInSession(t *testing.T) {
	t.Parallel()

	still := affinity.New(nil)
	for _, session := range []string{"a", "b", "a", "c"} {
		still.Feed(affinity.Envelope{At: start, Session: session}, choice(sesamy.EventNameSelectItem))
	}

	assert.Equal(t, 2, still.DrawOff(affinity.InSession("a")))
	assert.Equal(t, []string{"select_item@01 b", "select_item@01 c"}, kept(still))
}

// TestSelectivePolicy is what composing rules is for: a standing policy that forgets
// views and selections after two weeks but keeps what went into the cart, judged against
// the time of each batch fed in.
func TestSelectivePolicy(t *testing.T) {
	t.Parallel()

	browsing := affinity.Named(sesamy.EventNameSelectItem, sesamy.EventNameViewItem)
	still := distillery.New(distillery.Config[affinity.Envelope, any, affinity.Infusion]{
		Cut:      affinity.Choices,
		Infusers: affinity.Infusers,
		DrawOff: func(env affinity.Envelope) affinity.Rule {
			return distillery.Any(
				affinity.KeepLast(affinity.DefaultKeep),
				distillery.All(browsing, affinity.OlderThan(env.At, 14*day)),
			)
		},
	})

	for i, name := range []sesamy.EventName{sesamy.EventNameSelectItem, sesamy.EventNameAddToCart, sesamy.EventNameViewItem} {
		still.Feed(affinity.Envelope{At: start.Add(time.Duration(i) * day), Session: "early"}, choice(name))
	}

	still.Feed(affinity.Envelope{At: start.Add(20 * day), Session: "late"}, choice(sesamy.EventNameViewItem))

	assert.Equal(t, []string{"add_to_cart@02 early", "view_item@21 late"}, kept(still), "the early browsing is drawn off, the cart stays")
}
