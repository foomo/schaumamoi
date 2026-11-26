// Package affinity is the still that turns a shopper's events into their personalization
// vector: one sparse personalization.Map of taste, learned from what the shopper selects,
// views and buys.
//
// It is a distillery.Still with a fixed setup: a cut that keeps only choices, a chain of
// infusers that folds each choice into its contribution to taste, and recipes that
// read the vector back — flat, decayed by time, or faded per event with a reset on a
// change of intent (Fade). The shop ranks its listings with the vector this still distills;
// there is no second learner.
package affinity

import (
	"math"
	"time"

	"github.com/foomo/schaumamoi/pkg/distillery"
	"github.com/foomo/schaumamoi/pkg/distillery/stamp"
	"github.com/foomo/schaumamoi/pkg/personalization"
	"github.com/foomo/sesamy-go/pkg/encoding/mpv2"
	"github.com/foomo/sesamy-go/pkg/sesamy"
)

type (
	// Envelope is the Measurement Protocol envelope a batch arrived in, as sesamy-go
	// decodes it: an mpv2.Payload, whose events are the batch. A payload off a JetStream
	// stream or a database converts for free — affinity.Envelope(payload) — and nothing
	// in it is lost on the way in; Stamped and Personalization are the readings the still
	// needs.
	Envelope mpv2.Payload[any]

	// Infusion is what the still keeps of each kept event: its contribution to
	// taste — the attractors of the items it carries, weighted by how deliberate the
	// choice is — and, copied off the envelope, when and in which session it happened. The
	// stamp is embedded, so Infusion is stamp.Stamped and the shared rules and recipes
	// read it.
	//
	// The taste is one Map for everything the shopper touches, whatever the category: an
	// item only carries the dimensions that describe it, so categories need no handling
	// of their own here. How much each dimension counts in a category is the scorer's
	// business, not the still's.
	Infusion struct {
		stamp.Stamp

		Affinity personalization.Map
	}

	Still         = distillery.Still[Envelope, any, Infusion]
	Infused       = distillery.Infused[any, Infusion]
	Cut           = distillery.Cut[Envelope, any]
	Infuser       = distillery.Infuser[Envelope, any, Infusion]
	Recipe[S any] = distillery.Recipe[any, Infusion, S]
	Rule          = distillery.Rule[any, Infusion]
)

// Weights is how much a choice says about taste: selecting and viewing a product are
// deliberate, putting it in the cart the strongest choice of all. They are the only
// events the still keeps — see Choices.
var Weights = map[sesamy.EventName]personalization.Value{
	sesamy.EventNameSelectItem: 1,
	sesamy.EventNameViewItem:   1,
	sesamy.EventNameAddToCart:  3,
}

// Choices is the still's own cut: it keeps what the shopper did — selected, viewed,
// put in the cart — and nothing else. It cuts positively, keeping the events it learns
// from: listings are where products are offered, what the shop chose to show says
// nothing about the shopper, and a new kind of event stays out until there is a use for
// it.
func Choices(_ Envelope, e sesamy.Event[any]) bool {
	_, ok := Weights[e.Name]

	return ok
}

// Infusers is the chain every kept event is folded through, in order.
var Infusers = []Infuser{Stamp, AddAttractors, WeighByIntent}

// New is an affinity still: its cut is Choices, narrowed further by cut if one is given —
// a consent cut, say. A cut can only narrow what Choices keeps, never widen it. It
// holds the newest DefaultKeep choices: older ones are drawn off after every feed.
func New(cut Cut) *Still {
	keep := Choices
	if cut != nil {
		keep = func(env Envelope, e sesamy.Event[any]) bool { return Choices(env, e) && cut(env, e) }
	}

	return distillery.New(distillery.Config[Envelope, any, Infusion]{
		Cut:      keep,
		Infusers: Infusers,
		DrawOff:  func(Envelope) Rule { return Rules.KeepLast(DefaultKeep) },
	})
}

// Stamped is the envelope's stamp: when the batch was recorded (its timestamp_micros,
// the zero time if it has none) and in which session.
func (e Envelope) Stamped() stamp.Stamp {
	var at time.Time
	if e.TimestampMicros != 0 {
		at = time.UnixMicro(e.TimestampMicros).UTC()
	}

	return stamp.Stamp{At: at, Session: e.SessionID}
}

// Personalization is whether the shopper consented to personalization storage: granted
// explicitly in the envelope's consent. No consent data means no consent.
func (e Envelope) Personalization() bool {
	return e.Consent != nil && e.Consent.PersonalizationStorage != nil && *e.Consent.PersonalizationStorage == mpv2.ConsentGranted
}

// Stamp copies the envelope's time and session onto the choice. The event itself carries
// neither, and the still keeps the infusion, not the envelope.
func Stamp(env Envelope, e sesamy.Event[any], d Infusion) Infusion {
	return stamp.Infuse(env, e, d)
}

// items is the one thing every choice carries: items, each with its attractor. Decoding
// only the attractor keeps the still independent of any item type.
type items struct {
	Items []struct {
		Attractor personalization.Map `json:"attractor"`
	} `json:"items"`
}

// AddAttractors adds the attractor of every item the event carries to the infusion. An
// event without decodable items leaves the infusion as it was.
func AddAttractors(_ Envelope, e sesamy.Event[any], d Infusion) Infusion {
	var p items
	if err := e.DecodeParams(&p); err != nil {
		return d
	}

	out := clone(d.Affinity)
	for _, item := range p.Items {
		for dimension, v := range item.Attractor {
			out[dimension] += v
		}
	}

	d.Affinity = out

	return d
}

// WeighByIntent scales the infusion by the Weights of the event.
func WeighByIntent(_ Envelope, e sesamy.Event[any], d Infusion) Infusion {
	w := Weights[e.Name]

	out := make(personalization.Map, len(d.Affinity))
	for dimension, v := range d.Affinity {
		out[dimension] = v * w
	}

	d.Affinity = out

	return d
}

// Flat sums every contribution in full: the shopper's taste as if nothing faded. It sums
// in slice order — events arrive in order and Add appends, so the float sums are stable.
func Flat(events []Infused) personalization.Map {
	out := personalization.Map{}
	for _, e := range events {
		for dimension, v := range e.Infusion.Affinity {
			out[dimension] += v
		}
	}

	return out
}

// Decayed is the taste with recency by time: a contribution halves every halfLife it is
// older than now. Now is handed in rather than read off the clock, so distilling stays a
// pure read. An event from after now counts in full rather than growing.
func Decayed(now time.Time, halfLife time.Duration) Recipe[personalization.Map] {
	return func(events []Infused) personalization.Map {
		out := personalization.Map{}
		for _, e := range events {
			w := personalization.Value(math.Exp2(-float64(max(now.Sub(e.Infusion.At), 0)) / float64(halfLife)))
			for dimension, v := range e.Infusion.Affinity {
				out[dimension] += v * w
			}
		}

		return out
	}
}

func clone(m personalization.Map) personalization.Map {
	out := make(personalization.Map, len(m))
	for dimension, v := range m {
		out[dimension] = v
	}

	return out
}
