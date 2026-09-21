package mock

import (
	"strings"

	"github.com/foomo/schaumamoi/pkg/personalization"
	"github.com/foomo/sesamy-go/pkg/event/params"
)

// Item is a catalog product: the GA4 item, plus the attractor vector describing what
// taste it appeals to, plus the shop copy describing the same thing in prose. The
// attractor is enriched asynchronously, in a JSON-only environment, so it can stay a
// nested object with no encoding contortions.
//
// The two added fields are deliberately asymmetric about the wire. The attractor
// travels, because the enrichment stage on the other end is what consumes it. The
// description does not: GA4 has no item description param, and shipping shop copy with
// every line item of every event would be payload no consumer reads.
//
// The embedded params.Item carries no struct tag. encoding/json inlines an anonymous
// struct field on its own, so the GA4 fields sit flat alongside the attractor. Note the
// consequence for the other decoder: sesamy.Decode uses mapstructure, which does not
// inline an untagged embedded field, so Event.DecodeParams into this type returns the
// attractor but leaves the GA4 fields zeroed. Decode collected events into
// params.Item-based types instead; this type is for the enrichment stage.
type Item struct {
	params.Item

	Attractor personalization.Map `json:"attractor,omitempty" gtag:"-"`

	// Description is the dress in prose, written to say what the attractor says. It is
	// catalog-only and stays off both wires — json and gtag are separate tag keys, so
	// it opts out of each. It exists to make output analyzable on the Go side: an
	// item printed by a ranking or a failing assertion reads as a dress rather than as
	// DRESS_07, without anyone having to interpret a vector.
	Description string `json:"-" gtag:"-"`
}

// Occasion and Length read the facets back out of the GA4 category path, which is where
// dress writes them: Dresses > <Occasion> > <Length>. They are accessors rather than
// fields on purpose — item_category2 and item_category3 already carry this, and putting
// it on the wire a second time would be two sources of truth that can drift.
func (i Item) Occasion() string { return strings.ToLower(i.ItemCategory2) }
func (i Item) Length() string   { return strings.ToLower(i.ItemCategory3) }
