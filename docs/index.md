---
layout: home

hero:
  name: schaumamoi
  text: Distill GA4 events into per-user state
  tagline: Feed each user's events into a still, from a JetStream stream or a database replay. Cut what doesn't matter, infuse what you know, draw off what's stale, and distill as many states as you have recipes. Proven against a shop full of generated traffic.
  image:
    src: /logo.png
    alt: 'schaumamoi — a Woipertinger in a Bavarian hat reading a ticker tape: "Schau ma moi"'
  actions:
    - theme: brand
      text: Get Started
      link: /guide/getting-started
    - theme: alt
      text: The Distillery
      link: /guide/distillery
    - theme: alt
      text: See the Reports
      link: /reports/catalog

features:
  - title: A still per user
    details: Feed it events as they arrive. The cut keeps what matters, the infusers add what's known about each event, once, on the way in. What stays is ready for any question.
    link: /guide/distillery
  - title: Distilled by recipes
    details: A recipe reduces a still to a state of its own type, as a pure read, as often as you like. One user's events answer taste, funnels and phases without being fed twice.
    link: /guide/distillery
  - title: Drawn off by rules
    details: Forget by age, by session, by kind or by count, as a standing policy after every feed or on request. Rules are small and compose with Any, All and Not.
    link: /guide/distillery#drawing-off
  - title: A vector that follows intent
    details: The affinity still distills a shopper's taste. It fades per choice and lets go of the past when the intent changes, picked up on the first visit for a strong change.
    link: /guide/vector
  - title: Visits and phases
    details: Recipes for any stamped still group events into visits and cut a history into phases of steady taste, recovering who the shopper was, and when.
    link: /guide/visits
  - title: A shop to test against
    details: Deterministic GA4 traffic from a two-department shop. Personas with hidden taste browse, choose and buy, request by request, while the shop ranks every listing by what the still distills.
    link: /guide/test-shop/
---

## How it works

Events come in from wherever they live: a NATS JetStream stream as they happen, or a
replay from a database. Each user gets a **still**, and every event is worked on once,
on arrival:

| | |
|---|---|
| **feed** | events go into the user's still, a batch at a time, in arrival order |
| **cut** | what isn't worth keeping is cut, and gone |
| **infuse** | what is known about each kept event (its time, its session, its taste) is infused into it |
| **distill** | a recipe reduces the still to a state, as often as you like, with any recipe |
| **draw off** | what the still should no longer hold is drawn off, by composable rules |

A shopper's month, fed into the affinity still and distilled two ways. This is the
package's [runnable example](https://github.com/foomo/schaumamoi/blob/main/pkg/distillery/stills/affinity/example_test.go),
output included:

```go
still := affinity.New(nil)
for _, visit := range month {
    for _, request := range visit {
        still.Feed(affinity.Envelope(request), request.Events...) // a sesamy-go payload, as it comes
    }
}

vector := still.Distill(affinity.DefaultFade.Vector)                     // the taste
phases := still.Distill(affinity.Phases(affinity.DefaultPhaseThreshold)) // who, and when

still.DrawOff(affinity.Rules.InSession(month[0].ID()))                   // forget a visit

// fed 7 visits, kept 65 choices
// the vector leads with classic, the month is 1 phase
// forgetting the first visit draws off 9 choices
```

## Reports from the tests

The [Reports](/reports/catalog) are written by the test suite: every page is computed from
the real code on every run, every product and persona shown by its image, every claim
next to the test that checks it.

- [The catalog](/reports/catalog), [the personas](/reports/personas), [the shop](/reports/shop)
  and [the shoppers](/reports/shoppers): the test shop, package by package
- [The distillery](/reports/distillery): how a still works, what it distills from the
  traffic, and what drawing off does to it
- [The vector](/reports/affinity): how the personalization vector fades and follows a
  changing intent

*Schau ma moi*: Bavarian for "let's have a look". Or, at a distillery, a taste.
