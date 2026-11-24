package mock

import "github.com/foomo/schaumamoi/pkg/personalization"

// What one interaction teaches the shop about a shopper's taste. They mirror the weights
// a consumer of the traffic would use: opening or viewing a product is a deliberate
// choice, putting it in the cart the strongest one. Seeing a listing teaches nothing —
// the shop chose what was in it, not the shopper.
const (
	LearnSelect personalization.Value = 1
	LearnView   personalization.Value = 1
	LearnCart   personalization.Value = 3
)

// Profile is what the shop knows about a shopper: a taste vector learned from what they
// opened, viewed and bought, and how many such signals went into it.
//
// The shop never sees a persona. A shopper starts out with the zero Profile — a neutral
// vector, under which every product scores the same — and every listing is ranked by
// whatever the shop has learned by then. The persona only drives what the shopper picks;
// how well the listings anticipate those picks is up to the profile.
type Profile struct {
	Taste   personalization.Map
	Signals int
}

// Learn returns the profile after the shopper interacted with items, each interaction
// counting weight times the item's attractor. The receiver is left untouched, so a
// profile handed to a listing stays what it was when the listing was ranked.
func (p Profile) Learn(weight personalization.Value, items ...Item) Profile {
	taste := make(personalization.Map, len(p.Taste))
	for d, v := range p.Taste {
		taste[d] = v
	}

	for _, item := range items {
		for d, v := range item.Attractor {
			taste[d] += weight * v
		}
	}

	return Profile{Taste: taste, Signals: p.Signals + len(items)}
}

// Neutral reports whether the shop has learned nothing yet.
func (p Profile) Neutral() bool {
	return p.Signals == 0
}
