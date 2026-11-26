# Events on the wire

`pkg/internal/mock/wire` is how the traffic reaches a collector: Measurement Protocol v2
envelopes, and visits as the requests a client sends. This page is about what is on the
wire: the item type the events carry, the envelope around them, and the fixtures they
are written to.

## `catalog.Item`

`catalog.Item` nests `params.Item` and adds two fields: the attractor — the vector
describing what taste a product appeals to — and the description. The attractor is
enriched asynchronously in a JSON-only environment, so it stays a plain nested object
with no encoding contortions:

```go
type Item struct {
    params.Item

    Attractor   personalization.Map `json:"attractor,omitempty" gtag:"-"`
    Description string              `json:"-" gtag:"-"`
}
```

The two are deliberately asymmetric about the wire. The attractor travels, because the
enrichment stage downstream consumes it. The description does not: GA4 has no item
description param, and shop copy in every line item of every event is payload no
consumer reads. It exists to make Go-side output analyzable — a ranking or a failing
assertion reads as dresses rather than as `DRESS_07`:

```
ranked for Mira, 41 year old architect, business wear but strictly contemporary:
  1. Grid Cigarette Trousers  Slim black cigarette trousers with a sharp crease and
                              nothing else — modern, minimal, a little severe.
  2. Monolith Column Dress    Severe black column, architectural seams and hard
                              edges — contemporary business wear for a minimalist.
```

The description is documentation of the vector, not an input to it; the two are kept in
step by hand. Because it never reaches the wire, an item decoded off a fixture has none
— join it on from `catalog.All()`. `TestItemDescriptionStaysOffTheWire` pins that.

The embedded field carries no struct tag, and the two decoders treat that differently:

- **`encoding/json`** inlines an anonymous struct field on its own, so the GA4 fields sit
  flat beside the attractor. This is the enrichment stage's path.
- **`sesamy.Decode`** uses mapstructure, which does *not* inline an untagged embedded
  field. `Event.DecodeParams` into `catalog.Item` returns the attractor but leaves the GA4
  fields zeroed — silently. Decode collected events into `params.Item`-based types
  instead, e.g. `params.AddToCart[params.Item]`.

`TestItemDecode` pins all three behaviours, including the silent one.

Because items are `catalog.Item` rather than `params.Item`, the events are built with
`sesamy.NewEvent` — the stock `event.New*` constructors pin the item type.

## Envelope

```go
shopper.Journey(persona.Kate,
    wire.WithClientID("123.456"),
    wire.WithSessionID("789"),
    wire.WithConsentDenied(),
    wire.WithTimestamp(time.Now()),
)
```

`wire.Payload(events, opts...)` wraps events in one envelope; a `wire.Session` is a
visit as the client sends it, one envelope per event, with `Events()`, `Start()` and
`ID()` to read it back.

Three properties the mocks guarantee, because they are what makes the traffic usable as
aggregation input:

- **A listing is its filter's output.** Items come from `Filter.Apply`, never
  hand-picked, so a fixture cannot report items that contradict the facets it claims.
- **Totals agree with line items.** `Value` is computed from the items, never hardcoded.
- **Output is deterministic.** Fixed ids, `wire.BaseTime`, and order-stable ranking.
  (`mpv2.NewPayload` stamps `time.Now()` and is deliberately unused.)

## Fixtures

`pkg/internal/mock/testdata` holds each mock rendered as MPv2 wire-format JSON, for testing
decoding and the pipeline against real bytes:

```bash
go generate ./...     # regenerate; a no-op unless a mock changed
go test ./... -update # same, via the golden test
```

The fixtures are covered by a golden test, so they cannot drift from the constructors.

