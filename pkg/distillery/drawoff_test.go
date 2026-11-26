package distillery_test

import (
	"testing"

	"github.com/foomo/schaumamoi/pkg/distillery"
	"github.com/foomo/sesamy-go/pkg/sesamy"
	"github.com/stretchr/testify/assert"
)

// The draw-off tests use the plainest still there is: the envelope is the batch number,
// and the only infuser numbers each event in arrival order, so what a still holds reads
// as the list of numbers left.
type (
	numbered = distillery.Infused[any, int]
	numRule  = distillery.Rule[any, int]
)

// rules builds the rules for the numbered still.
var rules distillery.Rules[any, int]

// numberedStill numbers every event it keeps, from 1, and draws off what policy returns
// after every feed.
func numberedStill(policy func(batch int) numRule) *distillery.Still[int, any, int] {
	n := 0

	return distillery.New(distillery.Config[int, any, int]{
		Infusers: []distillery.Infuser[int, any, int]{
			func(int, sesamy.Event[any], int) int { n++; return n },
		},
		DrawOff: policy,
	})
}

// numbers is what the still holds, by number.
func numbers(still *distillery.Still[int, any, int]) []int {
	return still.Distill(func(events []numbered) []int {
		out := make([]int, 0, len(events))
		for _, e := range events {
			out = append(out, e.Infusion)
		}

		return out
	})
}

// feed feeds one event per name, as one batch.
func feed(still *distillery.Still[int, any, int], batch int, names ...sesamy.EventName) {
	events := make([]sesamy.Event[any], 0, len(names))
	for _, name := range names {
		events = append(events, sesamy.Event[any]{Name: name})
	}

	still.Feed(batch, events...)
}

const (
	view = sesamy.EventNameViewItem
	cart = sesamy.EventNameAddToCart
)

func even(e numbered) bool { return e.Infusion%2 == 0 }

func TestDrawOffKeepsOrder(t *testing.T) {
	t.Parallel()

	still := numberedStill(nil)
	feed(still, 1, view, view, view, view, view, view)

	assert.Equal(t, 3, still.DrawOff(rules.Where(even)))
	assert.Equal(t, []int{1, 3, 5}, numbers(still), "what remains keeps its arrival order")
	assert.Equal(t, 3, still.Len())
	assert.Zero(t, still.DrawOff(rules.Where(even)), "nothing left to draw off")
	assert.Zero(t, still.DrawOff(nil), "a nil rule draws off nothing")
}

func TestRules(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		rule numRule
		want []int
	}{
		"KeepLast keeps the newest":       {rules.KeepLast(2), []int{5, 6}},
		"KeepLast beyond the length":      {rules.KeepLast(10), []int{1, 2, 3, 4, 5, 6}},
		"Named":                           {rules.Named(cart), []int{1, 2, 4, 5}},
		"Any is either":                   {rules.Any(rules.Named(cart), rules.Where(even)), []int{1, 5}},
		"All narrows one rule by another": {rules.All(rules.Named(cart), rules.Where(even)), []int{1, 2, 3, 4, 5}},
		"Not keeps what it selects":       {rules.Not(rules.Named(cart)), []int{3, 6}},
		"empty Any draws off nothing":     {rules.Any(), []int{1, 2, 3, 4, 5, 6}},
		"empty All draws off nothing":     {rules.All(), []int{1, 2, 3, 4, 5, 6}},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			still := numberedStill(nil)
			feed(still, 1, view, view, cart, view, view, cart)
			still.DrawOff(tt.rule)

			assert.Equal(t, tt.want, numbers(still))
		})
	}
}

// TestPositionsDoNotDependOnRuleOrder pins that every rule of a composition sees the
// positions as they were before the draw-off began: keeping the newest three and drawing
// off the carts is the same whichever comes first.
func TestPositionsDoNotDependOnRuleOrder(t *testing.T) {
	t.Parallel()

	carts := rules.Named(cart)
	last := rules.KeepLast(3)

	for name, rule := range map[string]numRule{
		"carts first": rules.Any(carts, last),
		"last first":  rules.Any(last, carts),
	} {
		still := numberedStill(nil)
		feed(still, 1, view, view, view, cart, view, cart)
		still.DrawOff(rule)

		assert.Equal(t, []int{5}, numbers(still), name)
	}
}

// TestStandingDrawOff pins the standing policy: it runs after every feed, handed that
// batch's envelope.
func TestStandingDrawOff(t *testing.T) {
	t.Parallel()

	var seen []int

	still := numberedStill(func(batch int) numRule {
		seen = append(seen, batch)

		return rules.KeepLast(batch)
	})

	feed(still, 3, view, view, view, view)
	assert.Equal(t, []int{2, 3, 4}, numbers(still), "batch 3 keeps three")

	feed(still, 1, view)
	assert.Equal(t, []int{5}, numbers(still), "batch 1 keeps one")
	assert.Equal(t, []int{3, 1}, seen, "once per feed, with that batch's envelope")
}

// TestDrawOffDoesNotRewriteADistilledSlice pins that drawing off replaces the still's
// events rather than compacting them in place: a recipe that held on to its slice,
// against the rules, still sees what it was handed.
func TestDrawOffDoesNotRewriteADistilledSlice(t *testing.T) {
	t.Parallel()

	still := numberedStill(nil)
	feed(still, 1, view, view, view, view)

	held := still.Distill(func(events []numbered) []numbered { return events })
	still.DrawOff(rules.Where(func(e numbered) bool { return e.Infusion == 1 }))

	got := make([]int, 0, len(held))
	for _, e := range held {
		got = append(got, e.Infusion)
	}

	assert.Equal(t, []int{1, 2, 3, 4}, got)
	assert.Equal(t, []int{2, 3, 4}, numbers(still))
}
