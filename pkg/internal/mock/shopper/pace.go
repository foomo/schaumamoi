package shopper

import (
	"time"

	"github.com/foomo/schaumamoi/pkg/internal/mock/internal/seed"
	"github.com/foomo/sesamy-go/pkg/sesamy"
)

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
// drawn from gaps by key, to the second.
func gap(after sesamy.EventName, key string) time.Duration {
	r, ok := gaps[after]
	if !ok {
		return 10 * time.Second
	}

	span := uint64((r[1] - r[0]) / time.Second)

	return r[0] + time.Duration(seed.Hash(key)%(span+1))*time.Second
}
