// Package distillery prepares a per-user GA4 event stream so that any number of states
// can be distilled from it.
//
// Events reach it from a stream (NATS JetStream) or as a replay from a database. Each
// user gets a Still, and the work on an event happens once, when it is fed in:
//
//   - Feed: events go in, in batches, in arrival order.
//   - Cut: what is not worth keeping is cut and gone.
//   - Infuse: the infusers add what is known about each kept event — its time, its
//     session, its taste — into the event's Infusion.
//   - Distill: a Recipe reduces the infused events to a state of its own type. Unlike a
//     real run, distilling uses nothing up: it is a pure read, as often as you like and
//     with any recipe, so one user's events answer as many different questions as there
//     are recipes.
//
// A batch is the events of one call to Feed. Each batch comes with an Envelope: whatever
// the routing stage upstream knows about it beyond the events themselves — when it was
// recorded, which session it belongs to, what the user consented to. The envelope type
// is the caller's, like the event params and the infusion.
//
// The envelope exists on the way in only. The cut and the infusers see it; the still
// does not keep it. Whatever a recipe will need from it — a time to decay by, a session
// to window by — an infuser folds into the infusion, so the infusion is the single record
// of what is known about an event.
package distillery

import "github.com/foomo/sesamy-go/pkg/sesamy"

type (
	// Infused is a kept event together with the infusion folded out of it.
	Infused[Params any, Infusion any] struct {
		Event    sesamy.Event[Params]
		Infusion Infusion
	}

	// Cut decides whether an event is kept in the still at all. It sees the envelope of
	// the batch the event arrived in, so it can cut by where an event came from — a
	// session without consent, say — as well as by what it is.
	Cut[Envelope any, Params any] func(Envelope, sesamy.Event[Params]) (keep bool)

	// Infuser folds one event into the infusion accumulated for it so far. It sees that
	// event and the envelope of its batch, and nothing else, so it cannot fail on anything
	// but its own inputs: an infuser that cannot decode absorbs the failure and returns
	// the infusion unchanged.
	Infuser[Envelope any, Params any, Infusion any] func(Envelope, sesamy.Event[Params], Infusion) Infusion

	// Recipe reduces the infused events to a state — the spirit — of its own type. It is
	// handed the still's own slice and must not modify it: a still is distilled
	// repeatedly, and a recipe that mutates corrupts every later call.
	//
	// Anything relative to the present, such as decay, belongs here and nowhere earlier:
	// an infusion is fixed on the way in, so a decayed one would be stale by the next
	// distill. A recipe that needs "now" should be handed it rather than read the clock,
	// so that distilling stays a pure read.
	Recipe[Params any, Infusion any, Spirit any] func([]Infused[Params, Infusion]) Spirit

	// Config is how a still works on what it is fed: what it keeps, and what it infuses.
	// Both fields are optional — a nil Cut keeps everything, and no Infusers leaves every
	// infusion at its zero value.
	Config[Envelope any, Params any, Infusion any] struct {
		// Cut keeps only relevant events. It is lossy by design: what it cuts is gone, so
		// it should cut by kind of event or by where it came from, never by time — a
		// window distilled later would be computing over a still that was already cut.
		Cut Cut[Envelope, Params]
		// Infusers run in order on every kept event, each handed the infusion the
		// previous one returned, starting from the zero value.
		Infusers []Infuser[Envelope, Params, Infusion]
	}

	// Still is one user's kept, infused events, in arrival order.
	Still[Envelope any, Params any, Infusion any] struct {
		events []Infused[Params, Infusion]
		config Config[Envelope, Params, Infusion]
	}
)

// New sets up a still that works on what it is fed as config says.
func New[Envelope any, Params any, Infusion any](config Config[Envelope, Params, Infusion]) *Still[Envelope, Params, Infusion] {
	return &Still[Envelope, Params, Infusion]{
		config: config,
	}
}

// Feed takes one batch: it cuts what is not worth keeping and infuses each kept event as
// it arrives. Infusing here rather than when distilling is what keeps Distill pure: an
// event goes through the infusers exactly once, no matter how often — or how late — the
// still is distilled.
func (s *Still[Envelope, Params, Infusion]) Feed(envelope Envelope, events ...sesamy.Event[Params]) {
	for _, e := range events {
		if s.config.Cut != nil && !s.config.Cut(envelope, e) {
			continue
		}

		var infusion Infusion
		for _, infuse := range s.config.Infusers {
			infusion = infuse(envelope, e, infusion)
		}

		s.events = append(s.events, Infused[Params, Infusion]{
			Event:    e,
			Infusion: infusion,
		})
	}
}

// Len reports how many events the still holds.
func (s *Still[Envelope, Params, Infusion]) Len() int {
	return len(s.events)
}

// Distill reduces the still to whatever spirit the recipe describes. The spirit's type
// belongs to the recipe rather than to the still, so one still answers questions of
// unrelated types without being fed twice.
func (s *Still[Envelope, Params, Infusion]) Distill[Spirit any](recipe Recipe[Params, Infusion, Spirit]) Spirit {
	return recipe(s.events)
}
