package wire

import (
	"time"

	"github.com/foomo/sesamy-go/pkg/encoding/mpv2"
	"github.com/foomo/sesamy-go/pkg/sesamy"
)

// Session is one visit as a client sends it: one request per event, in order, each with
// its own timestamp and all with the visit's session id. A real GA4 client does not hold
// a visit back and ship it in one envelope; it sends each interaction as it happens — the
// listing when the page loads, the click when the shopper clicks.
type Session []mpv2.Payload[any]

// Events are the visit's events in the order they were sent.
func (s Session) Events() []sesamy.Event[any] {
	var out []sesamy.Event[any]
	for _, p := range s {
		out = append(out, p.Events...)
	}

	return out
}

// Start is when the visit's first request was sent.
func (s Session) Start() time.Time {
	if len(s) == 0 {
		return time.Time{}
	}

	return time.UnixMicro(s[0].TimestampMicros).UTC()
}

// ID is the visit's session id, shared by all its requests.
func (s Session) ID() string {
	if len(s) == 0 {
		return ""
	}

	return s[0].SessionID
}

// WithConsent is the visit with every request carrying consent c: the decision is the
// shopper's for the whole visit, not per request.
func (s Session) WithConsent(c *mpv2.ConsentData) Session {
	out := make(Session, len(s))
	for i, p := range s {
		p.Consent = c
		out[i] = p
	}

	return out
}
