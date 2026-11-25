# The corpus

`pkg/corpus` turns a per-user GA4 event stream into domain state. A corpus holds
events; it does not answer any one question about them.

```go
c := corpus.New(corpus.Config[Envelope, Params, Decoration]{
    Filter:     admit,      // which events get in at all
    Decorators: decorators, // what is computed about each one, in order
})

c.Add(envelope, events...)   // admit and decorate, once, on arrival
state := c.Derive(deriver)   // a pure read, as often as you like
```

- **Admitted and decorated once.** `Add` runs the filter and then the decorator chain
  on every event as it arrives, and keeps the result. Deriving never re-decorates, no
  matter how often or how late it runs.
- **Many questions, one corpus.** A deriver reduces the decorated events to whatever
  state it describes, and the state type belongs to the deriver. One user's stream can
  answer unrelated questions (taste, a funnel, phases of a history) without being
  ingested twice.
- **The envelope exists at ingest only.** Each batch comes with an envelope: what the
  routing stage knows beyond the events, such as time, session and consent. The filter
  and the decorators see it, but the corpus doesn't keep it. Whatever a deriver needs
  from it, a decorator copies into the decoration.
- **Anything relative to now belongs in a deriver.** A decoration is frozen on arrival,
  so recency (decay, fading) is computed when deriving, with "now" handed in rather
  than read off the clock.

## The affinity corpus

`pkg/affinity` is the corpus the shop uses: a fixed setup on top of `pkg/corpus`.

- **Filter:** it admits only a shopper's choices (`select_item`, `view_item`,
  `add_to_cart`), never listings. Listings are what the shop offered, not what the
  shopper chose.
- **Decorators:** they stamp each choice's time and session, sum the attractors of its
  items into one taste map, and weigh it by intent: a selection or view counts once, a
  cart three times.
- **Derivers:** `Flat` sums everything, `Decayed` fades by time, and `Fade.Vector`
  fades per choice. `Fade.Vector` is [the personalization vector](/guide/vector) the
  shop ranks every listing with.

[The corpus](/reports/corpus) report draws this pipeline from the code and traces one
visit through it, request by request.
