package stamp_test

import (
	"testing"
	"time"

	"github.com/foomo/schaumamoi/pkg/distillery"
	"github.com/foomo/schaumamoi/pkg/distillery/stamp"
	"github.com/foomo/sesamy-go/pkg/sesamy"
	"github.com/stretchr/testify/assert"
)

type (
	// envelope knows its stamp by a method, the way a caller's envelope type does.
	envelope struct {
		at      time.Time
		session string
	}

	// infusion embeds the stamp next to something of its own.
	infusion struct {
		stamp.Stamp

		Note string
	}
)

func (e envelope) Stamped() stamp.Stamp { return stamp.Stamp{At: e.at, Session: e.session} }

var start = time.Date(2026, time.September, 1, 10, 0, 0, 0, time.UTC)

// rules is how a still package offers its rules: one value with the type arguments fixed.
var rules stamp.Rules[any, infusion]

func still() *distillery.Still[envelope, any, infusion] {
	return distillery.New(distillery.Config[envelope, any, infusion]{
		Infusers: []distillery.Infuser[envelope, any, infusion]{
			func(_ envelope, e sesamy.Event[any], i infusion) infusion { i.Note = string(e.Name); return i },
			stamp.Infuse[envelope, any, infusion],
		},
	})
}

func held(s *distillery.Still[envelope, any, infusion]) []infusion {
	return s.Distill(func(events []distillery.Infused[any, infusion]) []infusion {
		out := make([]infusion, 0, len(events))
		for _, e := range events {
			out = append(out, e.Infusion)
		}

		return out
	})
}

func TestInfuseCopiesTheEnvelopesStamp(t *testing.T) {
	t.Parallel()

	s := still()
	s.Feed(envelope{at: start, session: "a"}, sesamy.Event[any]{Name: sesamy.EventNameViewItem})

	assert.Equal(t, []infusion{{Stamp: stamp.Stamp{At: start, Session: "a"}, Note: "view_item"}}, held(s),
		"the stamp is copied and what the infuser before it wrote stays")
}

func TestOlderThan(t *testing.T) {
	t.Parallel()

	s := still()
	for i := range 4 {
		s.Feed(envelope{at: start.Add(time.Duration(i) * 24 * time.Hour), session: "a"}, sesamy.Event[any]{Name: sesamy.EventNameViewItem})
	}

	s.Feed(envelope{session: "a"}, sesamy.Event[any]{Name: sesamy.EventNameViewItem}) // no time

	assert.Equal(t, 3, s.DrawOff(rules.OlderThan(start.Add(3*24*time.Hour), 36*time.Hour)),
		"the first two days, and the event without a time")

	var days []int
	for _, i := range held(s) {
		days = append(days, i.At.Day())
	}

	assert.Equal(t, []int{3, 4}, days)
}

func TestInSession(t *testing.T) {
	t.Parallel()

	s := still()
	for _, session := range []string{"a", "b", "a"} {
		s.Feed(envelope{at: start, session: session}, sesamy.Event[any]{Name: sesamy.EventNameViewItem})
	}

	assert.Equal(t, 2, s.DrawOff(rules.InSession("a")))
	assert.Equal(t, "b", held(s)[0].Session)
}
