// Package stamp is what every still needs to know about an event besides the event
// itself: when it happened and in which session, as the envelope it arrived in says.
//
// The envelope exists on the way in only, so the stamp has to be infused into each kept
// event to survive. Embed Stamp in an infusion and Infuse copies it there; the rules
// OlderThan and InSession, and any recipe that decays by age or groups by visit, read
// it back — whatever else the infusion holds.
package stamp

import (
	"time"

	"github.com/foomo/schaumamoi/pkg/distillery"
	"github.com/foomo/sesamy-go/pkg/sesamy"
)

type (
	// Stamp is when and in which session an event happened.
	Stamp struct {
		At      time.Time
		Session string
	}

	// Stamped is anything that knows its stamp: an envelope, by a method of its own, or an
	// infusion that embeds Stamp.
	Stamped interface {
		Stamped() Stamp
	}
)

// Stamped returns the stamp itself. Embedded in an infusion, it makes the infusion
// Stamped.
func (s Stamp) Stamped() Stamp { return s }

// Restamp replaces the stamp. Embedded in an infusion, it is promoted to a pointer to
// that infusion, which is how Infuse writes the stamp without knowing the infusion's
// type.
func (s *Stamp) Restamp(v Stamp) { *s = v }

// Infuse is the infuser that copies the envelope's stamp onto the event's infusion,
// leaving everything else in the infusion as it was. The event itself carries neither a
// time nor a session.
func Infuse[Envelope Stamped, Params any, Infusion any, PI interface {
	*Infusion
	Restamp(Stamp)
}](env Envelope, _ sesamy.Event[Params], i Infusion) Infusion {
	PI(&i).Restamp(env.Stamped())

	return i
}

// OlderThan draws off the events that happened more than age before now. Now is handed
// in, not read off the clock: as a standing policy it is the time of the batch being fed.
// An event without a time counts as infinitely old — it would weigh nothing when decayed
// either.
func OlderThan[Params any, Infusion Stamped](now time.Time, age time.Duration) distillery.Rule[Params, Infusion] {
	cutoff := now.Add(-age)

	return distillery.Where(func(e distillery.Infused[Params, Infusion]) bool {
		at := e.Infusion.Stamped().At

		return at.IsZero() || at.Before(cutoff)
	})
}

// InSession draws off the events of one session: a visit the user asked to have
// forgotten.
func InSession[Params any, Infusion Stamped](id string) distillery.Rule[Params, Infusion] {
	return distillery.Where(func(e distillery.Infused[Params, Infusion]) bool {
		return e.Infusion.Stamped().Session == id
	})
}
