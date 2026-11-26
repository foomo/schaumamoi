package distillery

import (
	"slices"

	"github.com/foomo/sesamy-go/pkg/sesamy"
)

// Any draws off an event if any of the rules does. With no rules it draws off nothing.
func Any[Params any, Infusion any](rules ...Rule[Params, Infusion]) Rule[Params, Infusion] {
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
func All[Params any, Infusion any](rules ...Rule[Params, Infusion]) Rule[Params, Infusion] {
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
func Not[Params any, Infusion any](rule Rule[Params, Infusion]) Rule[Params, Infusion] {
	return func(e Infused[Params, Infusion], fromNewest int) bool {
		return !rule(e, fromNewest)
	}
}

// KeepLast draws off everything but the newest n events: the bound on how much a still
// holds, whatever else its rules say.
func KeepLast[Params any, Infusion any](n int) Rule[Params, Infusion] {
	return func(_ Infused[Params, Infusion], fromNewest int) bool {
		return fromNewest >= n
	}
}

// Named draws off the events with one of the given names.
func Named[Params any, Infusion any](names ...sesamy.EventName) Rule[Params, Infusion] {
	return func(e Infused[Params, Infusion], _ int) bool {
		return slices.Contains(names, e.Event.Name)
	}
}

// Where draws off the events pred selects: the way in for anything the infusion knows —
// a time, a session — which only the setup that infused it can read.
func Where[Params any, Infusion any](pred func(Infused[Params, Infusion]) bool) Rule[Params, Infusion] {
	return func(e Infused[Params, Infusion], _ int) bool {
		return pred(e)
	}
}
