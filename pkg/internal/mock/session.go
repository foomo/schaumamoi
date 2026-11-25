package mock

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

// How long a shopper takes between two requests, by what the earlier one was. A listing
// is scanned before a click; a click opens a product, which is looked at before the
// shopper moves on; a product is considered before it goes into the cart. The ranges are
// what the mock draws from, seeded so the traffic stays reproducible.
var gaps = map[sesamy.EventName][2]time.Duration{
	sesamy.EventNameViewItemList: {5 * time.Second, 25 * time.Second},
	sesamy.EventNameSelectItem:   {20 * time.Second, 75 * time.Second},
	sesamy.EventNameViewItem:     {15 * time.Second, 60 * time.Second},
	sesamy.EventNameAddToCart:    {5 * time.Second, 15 * time.Second},
}

// firstEngagement is the engagement time of a visit's first request, which has nothing
// before it in the visit to measure from.
const firstEngagement = time.Second

// gap is the time between a request carrying an event named after and the next one,
// drawn from gaps by seed, to the second.
func gap(after sesamy.EventName, seed string) time.Duration {
	r, ok := gaps[after]
	if !ok {
		return 10 * time.Second
	}

	span := uint64((r[1] - r[0]) / time.Second)

	return r[0] + time.Duration(seedHash(seed)%(span+1))*time.Second
}
