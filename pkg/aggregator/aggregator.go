// Package aggregator folds a routed, per-user GA4 event stream into domain state.
//
// A Pipeline is a corpus, not a question. Events are admitted and decorated exactly once
// on arrival; deriving state is a pure read, so one user's stream answers as many
// different questions as the caller has derivers.
//
// Events are assumed to arrive in order, in batches. Each batch comes with an Envelope:
// whatever the routing stage upstream knows about it beyond the events themselves — when
// it was recorded, which session it belongs to, what the user consented to. The envelope
// type is the caller's, like the payload and the decoration.
//
// The envelope exists at ingest only. The filter and the decorators see it; the corpus
// does not keep it. Whatever a deriver will need from it — a time to decay by, a session
// to window by — a decorator folds into the decoration, so the decoration is the single
// record of what is known about an event.
package aggregator

import "github.com/foomo/sesamy-go/pkg/sesamy"

type (
	// DecoratedEvent is an admitted event together with the decoration folded out of it.
	DecoratedEvent[Payload any, Decoration any] struct {
		Event      sesamy.Event[Payload]
		Decoration Decoration
	}

	// EventFilter decides whether an event is relevant to a pipeline at all. It sees the
	// envelope of the batch the event arrived in, so it can drop by where an event came
	// from — a session without consent, say — as well as by what it is.
	EventFilter[Envelope any, Payload any] func(Envelope, sesamy.Event[Payload]) (keep bool)

	// EventDecorator folds one event into the decoration accumulated for it so far. It
	// sees that event and the envelope of its batch, and nothing else, so it cannot fail
	// on anything but its own inputs: a decorator that cannot decode absorbs the failure
	// and returns the decoration unchanged.
	EventDecorator[Envelope any, Payload any, Decoration any] func(Envelope, sesamy.Event[Payload], Decoration) Decoration

	// StateDeriver reduces the decorated corpus to a domain state. It is handed the
	// pipeline's own slice and must not modify it — state is derived repeatedly, and a
	// deriver that mutates corrupts every later call.
	//
	// Anything relative to the present, such as decay, belongs here and nowhere earlier:
	// a decoration is frozen on arrival, so a decayed one would be stale by the next
	// derive. A deriver that needs "now" should be handed it rather than read the clock,
	// so that deriving stays a pure read.
	StateDeriver[Payload any, Decoration any, DomainState any] func([]DecoratedEvent[Payload, Decoration]) DomainState

	// PipelineConfig is the ingest policy: what gets in, and what is computed about it on
	// the way in. Both fields are optional — a nil PreFilter admits everything, and an
	// empty DecoratorChain leaves every decoration at its zero value.
	PipelineConfig[Envelope any, Payload any, Decoration any] struct {
		// PreFilter keeps only relevant events. It is lossy by design: what it drops is
		// gone, so it should narrow by kind of event or by where it came from, never by
		// time — a window derived later would be computing over a corpus that was
		// already cut.
		PreFilter      EventFilter[Envelope, Payload]
		DecoratorChain []EventDecorator[Envelope, Payload, Decoration]
	}

	Pipeline[Envelope any, Payload any, Decoration any] struct {
		events []DecoratedEvent[Payload, Decoration]
		config PipelineConfig[Envelope, Payload, Decoration]
	}
)

func NewPipeline[Envelope any, Payload any, Decoration any](config PipelineConfig[Envelope, Payload, Decoration]) *Pipeline[Envelope, Payload, Decoration] {
	return &Pipeline[Envelope, Payload, Decoration]{
		config: config,
	}
}

// Add admits the events of one batch that the filter keeps, and decorates each one as it
// arrives. Decorating here rather than at derive time is what keeps DeriveState pure: an
// event is folded through the chain exactly once, no matter how often — or how late —
// state is derived.
func (p *Pipeline[Envelope, Payload, Decoration]) Add(envelope Envelope, events ...sesamy.Event[Payload]) {
	for _, e := range events {
		if p.config.PreFilter != nil && !p.config.PreFilter(envelope, e) {
			continue
		}

		var decoration Decoration
		for _, decorate := range p.config.DecoratorChain {
			decoration = decorate(envelope, e, decoration)
		}

		p.events = append(p.events, DecoratedEvent[Payload, Decoration]{
			Event:      e,
			Decoration: decoration,
		})
	}
}

// Len reports how many events the filter has admitted.
func (p *Pipeline[Envelope, Payload, Decoration]) Len() int {
	return len(p.events)
}

// DeriveState reduces the corpus to whatever domain state the deriver describes. The
// state type belongs to the deriver rather than to the pipeline, so one corpus answers
// questions of unrelated types without being ingested twice.
func (p *Pipeline[Envelope, Payload, Decoration]) DeriveState[DomainState any](derive StateDeriver[Payload, Decoration, DomainState]) DomainState {
	return derive(p.events)
}
