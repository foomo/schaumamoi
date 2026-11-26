package distillery

import (
	"slices"

	"github.com/foomo/sesamy-go/pkg/sesamy"
)

// Rules builds the rules for a still with the given Params and Infusion. It holds
// nothing: its only job is to fix the type arguments once, which Go cannot infer from a
// rule's own arguments, so a still package declares a single value —
//
//	var Rules distillery.Rules[any, Infusion]
//
// and writes Rules.KeepLast(1000), not the type arguments on every rule it builds.
// A package with rules of its own embeds Rules in a type of its own, as stamp does.
type Rules[Params any, Infusion any] struct{}

// Any draws off an event if any of the rules does. With no rules it draws off nothing.
func (Rules[Params, Infusion]) Any(rules ...Rule[Params, Infusion]) Rule[Params, Infusion] {
	return func(e Infused[Params, Infusion], fromNewest int) bool {
		for _, rule := range rules {
			if rule(e, fromNewest) {
				return true
			}
		}

		return false
	}
}

// All draws off an event only if every rule does: the way to narrow one rule by
// another — views, but only those older than two weeks. With no rules it draws off
// nothing, so an empty All is as harmless as an empty Any.
func (Rules[Params, Infusion]) All(rules ...Rule[Params, Infusion]) Rule[Params, Infusion] {
	return func(e Infused[Params, Infusion], fromNewest int) bool {
		for _, rule := range rules {
			if !rule(e, fromNewest) {
				return false
			}
		}

		return len(rules) > 0
	}
}

// Not draws off what the rule would keep, and keeps what it would draw off.
func (Rules[Params, Infusion]) Not(rule Rule[Params, Infusion]) Rule[Params, Infusion] {
	return func(e Infused[Params, Infusion], fromNewest int) bool {
		return !rule(e, fromNewest)
	}
}

// KeepLast draws off everything but the newest n events: the bound on how much a still
// holds, whatever else its rules say.
func (Rules[Params, Infusion]) KeepLast(n int) Rule[Params, Infusion] {
	return func(_ Infused[Params, Infusion], fromNewest int) bool {
		return fromNewest >= n
	}
}

// Named draws off the events with one of the given names.
func (Rules[Params, Infusion]) Named(names ...sesamy.EventName) Rule[Params, Infusion] {
	return func(e Infused[Params, Infusion], _ int) bool {
		return slices.Contains(names, e.Event.Name)
	}
}

// Where draws off the events pred selects: the way in for anything the infusion knows,
// which only the setup that infused it can read.
func (Rules[Params, Infusion]) Where(pred func(Infused[Params, Infusion]) bool) Rule[Params, Infusion] {
	return func(e Infused[Params, Infusion], _ int) bool {
		return pred(e)
	}
}
