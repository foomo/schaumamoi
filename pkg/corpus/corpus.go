// Package corpus folds a routed, per-user GA4 event stream into domain state.
//
// A Corpus holds events; it does not answer any one question about them. Events are
// admitted and decorated exactly once on arrival; deriving state is a pure read, so one
// user's stream answers as many different questions as the caller has derivers.
//
// Events are assumed to arrive in order, in batches: a batch is the events of one call to
// Add. Each batch comes with an Envelope: whatever the routing stage upstream knows about
// it beyond the events themselves — when it was recorded, which session it belongs to,
// what the user consented to. The envelope type is the caller's, like the event params
// and the decoration.
//
// The envelope exists at ingest only. The filter and the decorators see it; the corpus
// does not keep it. Whatever a deriver will need from it — a time to decay by, a session
// to window by — a decorator folds into the decoration, so the decoration is the single
// record of what is known about an event.
package corpus

import "github.com/foomo/sesamy-go/pkg/sesamy"

type (
	// DecoratedEvent is an admitted event together with the decoration folded out of it.
	DecoratedEvent[Params any, Decoration any] struct {
		Event      sesamy.Event[Params]
		Decoration Decoration
	}

	// EventFilter decides whether an event is admitted to a corpus at all. It sees the
	// envelope of the batch the event arrived in, so it can drop by where an event came
	// from — a session without consent, say — as well as by what it is.
	EventFilter[Envelope any, Params any] func(Envelope, sesamy.Event[Params]) (admit bool)

	// EventDecorator folds one event into the decoration accumulated for it so far. It
	// sees that event and the envelope of its batch, and nothing else, so it cannot fail
	// on anything but its own inputs: a decorator that cannot decode absorbs the failure
	// and returns the decoration unchanged.
	EventDecorator[Envelope any, Params any, Decoration any] func(Envelope, sesamy.Event[Params], Decoration) Decoration

	// StateDeriver reduces the decorated events to a domain state. It is handed the
	// corpus's own slice and must not modify it — state is derived repeatedly, and a
	// deriver that mutates corrupts every later call.
	//
	// Anything relative to the present, such as decay, belongs here and nowhere earlier:
	// a decoration is frozen on arrival, so a decayed one would be stale by the next
	// derive. A deriver that needs "now" should be handed it rather than read the clock,
	// so that deriving stays a pure read.
	StateDeriver[Params any, Decoration any, DomainState any] func([]DecoratedEvent[Params, Decoration]) DomainState

	// Config is the ingest policy: what is admitted, and what is computed about it on the
	// way in. Both fields are optional — a nil Filter admits everything, and no
	// Decorators leaves every decoration at its zero value.
	Config[Envelope any, Params any, Decoration any] struct {
		// Filter admits only relevant events. It is lossy by design: what it drops is
		// gone, so it should narrow by kind of event or by where it came from, never by
		// time — a window derived later would be computing over a corpus that was
		// already cut.
		Filter EventFilter[Envelope, Params]
		// Decorators run in order on every admitted event, each handed the decoration
		// the previous one returned, starting from the zero value.
		Decorators []EventDecorator[Envelope, Params, Decoration]
	}

	// Corpus is one user's admitted, decorated events, in arrival order.
	Corpus[Envelope any, Params any, Decoration any] struct {
		events []DecoratedEvent[Params, Decoration]
		config Config[Envelope, Params, Decoration]
	}
)

func New[Envelope any, Params any, Decoration any](config Config[Envelope, Params, Decoration]) *Corpus[Envelope, Params, Decoration] {
	return &Corpus[Envelope, Params, Decoration]{
		config: config,
	}
}

// Add takes one batch, keeps the events the filter admits, and decorates each one as
// it arrives. Decorating here rather than at derive time is what keeps Derive pure: an
// event is folded through the decorators exactly once, no matter how often — or how
// late — state is derived.
func (c *Corpus[Envelope, Params, Decoration]) Add(envelope Envelope, events ...sesamy.Event[Params]) {
	for _, e := range events {
		if c.config.Filter != nil && !c.config.Filter(envelope, e) {
			continue
		}

		var decoration Decoration
		for _, decorate := range c.config.Decorators {
			decoration = decorate(envelope, e, decoration)
		}

		c.events = append(c.events, DecoratedEvent[Params, Decoration]{
			Event:      e,
			Decoration: decoration,
		})
	}
}

// Len reports how many events the filter has admitted.
func (c *Corpus[Envelope, Params, Decoration]) Len() int {
	return len(c.events)
}

// Derive reduces the corpus to whatever domain state the deriver describes. The state
// type belongs to the deriver rather than to the corpus, so one corpus answers questions
// of unrelated types without being ingested twice.
func (c *Corpus[Envelope, Params, Decoration]) Derive[DomainState any](derive StateDeriver[Params, Decoration, DomainState]) DomainState {
	return derive(c.events)
}
