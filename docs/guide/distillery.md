# The distillery

GA4 events reach schaumamoi from a stream (NATS JetStream) or as a replay from a
database. `pkg/distillery` is the step in between: per user, it takes the events in,
works on them once, and keeps them ready for any number of states to be distilled from
them: a taste vector, a funnel, the phases of a history.

| Word | What happens | API |
|---|---|---|
| **feed** | events go into the user's still, a batch at a time, in arrival order | `Still.Feed(envelope, events...)` |
| **cut** | what isn't worth keeping is cut, and gone | `Config.Cut` |
| **infuse** | what is known about each kept event is infused into it, once | `Config.Infusers` |
| **distill** | a recipe reduces the infused events to a state of its own type | `Still.Distill(recipe)` |

```go
still := distillery.New(distillery.Config[Envelope, Params, Infusion]{
    Cut:      keep,      // which events are kept at all
    Infusers: infusers,  // what is infused into each one, in order
})

still.Feed(envelope, events...)    // cut and infuse, once, on arrival
spirit := still.Distill(recipe)    // a pure read, as often as you like
```

- **Cut and infused once.** `Feed` runs the cut and then the infusers on every event
  as it arrives, and keeps the result. Distilling never infuses again, no matter how
  often or how late it runs.
- **One still, many recipes.** A recipe reduces the infused events to whatever state it
  describes, and the state's type belongs to the recipe. One user's events can answer
  unrelated questions (taste, a funnel, phases of a history) without being fed twice.
  Unlike a real run, distilling uses nothing up.
- **The envelope exists on the way in only.** Each batch comes with an envelope: what
  the routing stage knows beyond the events, such as time, session and consent. The cut
  and the infusers see it, but the still doesn't keep it. Whatever a recipe needs from
  it, an infuser copies into the infusion.
- **Anything relative to now belongs in a recipe.** An infusion is fixed on the way in,
  so recency (decay, fading) is computed when distilling, with "now" handed in rather
  than read off the clock.

## The affinity still

`pkg/affinity` is the still the shop uses: a fixed setup on top of `pkg/distillery`.

- **Cut:** it keeps only a shopper's choices (`select_item`, `view_item`,
  `add_to_cart`), never listings. Listings are what the shop offered, not what the
  shopper chose.
- **Infusers:** they stamp each choice's time and session, sum the attractors of its
  items into one taste map, and weigh it by intent: a selection or view counts once, a
  cart three times.
- **Recipes:** `Flat` sums everything, `Decayed` fades by time, and `Fade.Vector`
  fades per choice. `Fade.Vector` is [the personalization vector](/guide/vector) the
  shop ranks every listing with.

[The distillery](/reports/distillery) report draws this setup from the code and traces
one visit through it, request by request.
