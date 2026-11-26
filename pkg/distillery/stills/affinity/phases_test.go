package affinity_test

import (
	"testing"
	"time"

	"github.com/foomo/schaumamoi/pkg/distillery"
	"github.com/foomo/schaumamoi/pkg/distillery/stills/affinity"
	"github.com/foomo/schaumamoi/pkg/internal/mock/persona"
	"github.com/foomo/schaumamoi/pkg/internal/mock/shopper"
	"github.com/foomo/schaumamoi/pkg/internal/mock/wire"
	"github.com/foomo/sesamy-go/pkg/sesamy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The phase tests read whole months of the test shop's traffic through an affinity still.

// month feeds a history into a fresh affinity still, request by request.
func month(history []wire.Session) *affinity.Still {
	c := affinity.New(nil)
	for _, session := range history {
		add(c, session)
	}

	return c
}

// drifted is a month that starts with one taste and settles on another: the first three
// visits, up to two weeks ago and including the first purchase, are old's; the rest,
// ending with today's purchase, are recent's.
func drifted(old, recent persona.Persona) []wire.Session {
	visits := shopper.Month(recent)
	for i := range 3 {
		visits[i].Persona = old
	}

	return shopper.History(visits)
}

// phaseNames names the persona each phase is nearest to.
func phaseNames(ps []affinity.Phase) []string {
	out := make([]string, 0, len(ps))
	for _, p := range ps {
		out = append(out, nearestPersona(p.Taste).Name)
	}

	return out
}

// assertDuring checks that at falls within a visit: from its first request to its last.
// A phase starts and ends at choices, which happen during a visit, not at its start.
func assertDuring(t *testing.T, at time.Time, session wire.Session, msg string) {
	t.Helper()

	end := time.UnixMicro(session[len(session)-1].TimestampMicros).UTC()
	assert.False(t, at.Before(session.Start()), "%s: %s is before %s", msg, at, session.Start())
	assert.False(t, at.After(end), "%s: %s is after %s", msg, at, end)
}

var phases = affinity.Phases(affinity.DefaultPhaseThreshold)

// TestSteadyMonthIsOnePhase is the baseline: a shopper who never drifts has one phase,
// spanning the whole month, and it reads as them.
func TestSteadyMonthIsOnePhase(t *testing.T) {
	t.Parallel()

	for _, who := range persona.All() {
		t.Run(who.Name, func(t *testing.T) {
			t.Parallel()

			history := shopper.History(shopper.Month(who))
			got := month(history).Distill(phases)

			require.Len(t, got, 1)
			assert.Equal(t, []string{who.Name}, phaseNames(got))
			assertDuring(t, got[0].From, history[0], "the phase starts in the first visit")
			assertDuring(t, got[0].To, history[len(history)-1], "the phase ends in the last visit")
			assert.Len(t, got[0].Visits, len(shopper.Month(who)))
		})
	}
}

// TestDriftSplitsIntoPhases is what phases are for. A month that drifts from one taste to
// another must split exactly where it drifted — after the third visit, two weeks ago —
// and each side must read as the persona who was shopping then. Unlike decay, which only
// answers "who now", this recovers who the shopper was and when.
func TestDriftSplitsIntoPhases(t *testing.T) {
	t.Parallel()

	for _, old := range persona.All() {
		for _, recent := range persona.All() {
			if old.Name == recent.Name {
				continue
			}

			t.Run(old.Name+" then "+recent.Name, func(t *testing.T) {
				t.Parallel()

				history := drifted(old, recent)
				got := month(history).Distill(phases)

				require.Len(t, got, 2)
				assert.Equal(t, []string{old.Name, recent.Name}, phaseNames(got))
				assertDuring(t, got[0].To, history[2], "the old taste ends in the visit of the first purchase")
				assertDuring(t, got[1].From, history[3], "the new taste starts in the next visit")
				assert.Len(t, got[0].Visits, 3)
				assert.Len(t, got[1].Visits, 4)
			})
		}
	}
}

// TestReturningTasteIsANewPhase pins that phases are contiguous: Kate detours into
// Sofia's taste for two visits mid-month and comes back, which is three phases, not a
// Kate phase with a hole in it.
func TestReturningTasteIsANewPhase(t *testing.T) {
	t.Parallel()

	detour := shopper.Month(persona.Kate)
	detour[2].Persona = persona.Sofia
	detour[3].Persona = persona.Sofia

	assert.Equal(t, []string{"Kate", "Sofia", "Kate"}, phaseNames(month(shopper.History(detour)).Distill(phases)))
}

// TestPhasesNeedSessions shows the session stamp doing the work: the same drifted month
// stamped with its time but not its session is one undivided visit, nothing to split.
func TestPhasesNeedSessions(t *testing.T) {
	t.Parallel()

	timeOnly := func(env affinity.Envelope, _ sesamy.Event[any], d affinity.Infusion) affinity.Infusion {
		d.At = env.At

		return d
	}

	c := distillery.New(distillery.Config[affinity.Envelope, any, affinity.Infusion]{
		Cut:      affinity.Choices,
		Infusers: []affinity.Infuser{timeOnly, affinity.AddAttractors, affinity.WeighByIntent},
	})
	for _, session := range drifted(persona.Sofia, persona.Kate) {
		for _, hit := range session {
			c.Feed(affinity.Envelope{At: time.UnixMicro(hit.TimestampMicros).UTC(), Session: hit.SessionID}, hit.Events...)
		}
	}

	assert.Len(t, c.Distill(phases), 1)
}
