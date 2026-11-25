package shop

import (
	"github.com/foomo/schaumamoi/pkg/personalization"
)

// Profile is what the shop ranks a listing with: the shopper's personalization vector as
// the affinity corpus emits it from their events so far, and how many events went into
// it.
//
// The shop never sees a persona, and it keeps no learner of its own. A shopper starts out
// with no events — a neutral vector, under which every product scores the same — and
// every listing is ranked by whatever the corpus has made of the shopper by then. The
// persona only drives what the shopper picks; how well the listings anticipate those
// picks is up to the vector.
type Profile struct {
	Taste   personalization.Map
	Signals int
}

// Neutral reports whether the shop knows nothing about the shopper yet.
func (p Profile) Neutral() bool {
	return p.Signals == 0
}
