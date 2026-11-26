# Visits and phases

`pkg/visits` reads a still visit by visit. It is a package of recipes, not a still of its
own: it works on any still whose infusion is stamped (see
[drawing off](/guide/distillery#drawing-off) for `stamp`), whatever else the infusion
holds.

A **visit** is one session's run of events. It is the unit taste is judged in: a single
event is too small to say much, and a whole month too large to say when the taste
changed.

```go
visits.Of(events)                       // []visits.Visit: Session, From, To, Events
visits.Phases(threshold, taste)         // a recipe: []visits.Phase: From, To, Visits, Taste

still.Distill(affinity.Phases(affinity.DefaultPhaseThreshold))  // the affinity still's taste
```

## Visits

`Of` groups a still's events by session. Events arrive in order and a session's events
arrive together, so a change of session opens a new visit, including a session that
comes back after another. A visit runs from its first event's time to its last's.

## Phases

`Phases` merges consecutive visits into **phases of steady taste**. A visit joins the
current phase if its taste is at least `threshold` similar (`personalization.Map.Cosine`)
to the phase's taste so far, and opens a new phase otherwise. A visit that says nothing
about taste joins whatever phase it falls in. The caller hands in the taste to judge by,
so the recipe never needs to know what else an infusion holds.

Phases are **contiguous**. A shopper who drifts away and comes back gets three phases,
not two: the return is news about when the taste held, which a merge would lose.

Unlike decay, which only answers "who now", phases recover who the shopper was, and
when.

## The threshold

`affinity.Phases` judges each visit by the taste its choices add up to, with
`affinity.DefaultPhaseThreshold`, 0.9. Across every persona pair of the test shop, a
visit within a phase scores at least 0.94, and the first visit after a drift at most
0.63 (Mira to Lena, who share a minimal, modern taste, is the close call). 0.9 sits in
the gap, nearer its top. The within-phase side is the tight one: a shopper doesn't open
the same products every visit, so even one taste's visits differ.

Phases need sessions. A still stamped with its time but not its session is one
undivided visit, with nothing to split (`TestPhasesNeedSessions`). The
[distillery report](/reports/distillery) shows phases for steady months, every drift,
a detour and a consent cut.
