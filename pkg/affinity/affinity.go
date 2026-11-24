// Package affinity is the corpus that turns a shopper's events into their personalization
// vector: one sparse personalization.Map of taste, learned from what the shopper selects,
// views and buys.
//
// It is a corpus.Corpus with a fixed setup: a filter that admits only choices, a chain of
// decorators that folds each choice into its contribution to taste, and derivers that
// read the vector back — flat, decayed by time, or faded per event with a reset on a
// change of intent (Fade). The shop ranks its listings with the vector this corpus emits;
// there is no second learner.
package affinity

import (
	"math"
	"time"

	"github.com/foomo/schaumamoi/pkg/corpus"
	"github.com/foomo/schaumamoi/pkg/personalization"
	"github.com/foomo/sesamy-go/pkg/sesamy"
)

type (
	// Envelope is what the routing stage knows about a batch beyond its events.
	Envelope struct {
		At      time.Time
		Session string
		// Personalization is whether the shopper consented to personalization storage.
		Personalization bool
	}

	// Decoration is what the corpus keeps of each admitted event: its contribution to
	// taste — the attractors of the items it carries, weighted by how deliberate the
	// choice is — and, copied off the envelope, when and in which session it happened.
	//
	// The taste is one Map for everything the shopper touches, whatever the category: an
	// item only carries the dimensions that describe it, so categories need no handling
	// of their own here. How much each dimension counts in a category is the scorer's
	// business, not the corpus's.
	Decoration struct {
		At       time.Time
		Session  string
		Affinity personalization.Map
	}

	Corpus         = corpus.Corpus[Envelope, any, Decoration]
	Decorated      = corpus.DecoratedEvent[any, Decoration]
	Filter         = corpus.EventFilter[Envelope, any]
	Decorator      = corpus.EventDecorator[Envelope, any, Decoration]
	Deriver[S any] = corpus.StateDeriver[any, Decoration, S]
)

// Weights is how much a choice says about taste: selecting and viewing a product are
// deliberate, putting it in the cart the strongest choice of all. They are the only
// events the corpus admits — see Choices.
var Weights = map[sesamy.EventName]personalization.Value{
	sesamy.EventNameSelectItem: 1,
	sesamy.EventNameViewItem:   1,
	sesamy.EventNameAddToCart:  3,
}

// Choices is the corpus's own filter: it admits what the shopper did — selected, viewed,
// put in the cart — and nothing else. It filters positively, for the events it learns
// from: listings are where products are offered, what the shop chose to show says
// nothing about the shopper, and a new kind of event stays out until there is a use for
// it.
func Choices(_ Envelope, e sesamy.Event[any]) bool {
	_, ok := Weights[e.Name]

	return ok
}

// Decorators is the chain every admitted event is folded through, in order.
var Decorators = []Decorator{StampTime, StampSession, AddAttractors, WeighByIntent}

// New is an affinity corpus: Choices, narrowed further by filter if one is given — the
// consent filter, say. A filter can only narrow what Choices admits, never widen it.
func New(filter Filter) *Corpus {
	admit := Choices
	if filter != nil {
		admit = func(env Envelope, e sesamy.Event[any]) bool { return Choices(env, e) && filter(env, e) }
	}

	return corpus.New(corpus.Config[Envelope, any, Decoration]{Filter: admit, Decorators: Decorators})
}

// StampTime and StampSession copy the envelope's time and session onto the event. The
// event itself carries neither, and the corpus keeps the decoration, not the envelope.
func StampTime(env Envelope, _ sesamy.Event[any], d Decoration) Decoration {
	d.At = env.At

	return d
}

func StampSession(env Envelope, _ sesamy.Event[any], d Decoration) Decoration {
	d.Session = env.Session

	return d
}

// items is the one thing every choice carries: items, each with its attractor. Decoding
// only the attractor keeps the corpus independent of any item type.
type items struct {
	Items []struct {
		Attractor personalization.Map `json:"attractor"`
	} `json:"items"`
}

// AddAttractors adds the attractor of every item the event carries to the decoration. An
// event without decodable items leaves the decoration as it was.
func AddAttractors(_ Envelope, e sesamy.Event[any], d Decoration) Decoration {
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

// WeighByIntent scales the decoration by the Weights of the event.
func WeighByIntent(_ Envelope, e sesamy.Event[any], d Decoration) Decoration {
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
func Flat(events []Decorated) personalization.Map {
	out := personalization.Map{}
	for _, e := range events {
		for dimension, v := range e.Decoration.Affinity {
			out[dimension] += v
		}
	}

	return out
}

// Decayed is the taste with recency by time: a contribution halves every halfLife it is
// older than now. Now is handed in rather than read off the clock, so deriving stays a
// pure read. An event from after now counts in full rather than growing.
func Decayed(now time.Time, halfLife time.Duration) Deriver[personalization.Map] {
	return func(events []Decorated) personalization.Map {
		out := personalization.Map{}
		for _, e := range events {
			w := personalization.Value(math.Exp2(-float64(max(now.Sub(e.Decoration.At), 0)) / float64(halfLife)))
			for dimension, v := range e.Decoration.Affinity {
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
