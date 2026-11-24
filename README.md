# schaumamoi

event aggregation, metering and analysis

Processes GA4 event data as produced by [`foomo/sesamy-go`](https://github.com/foomo/sesamy-go).

## Requirements

Go >= 1.27 (pinned via [mise](https://mise.jdx.dev)):

```bash
mise install
```

## Personalization

`pkg/personalization` is a sparse vector space over taste dimensions:

```go
type (
    Dimension string
    Value     float32
    Map       map[Dimension]Value
)

func (m Map) Score(other Map) Value   // sparse dot product
```

A map only names the dimensions it cares about; a dimension missing from either side
contributes nothing. `Score` sums in sorted dimension order — float addition is not
associative and Go randomises map iteration, so summing in map order would wobble in the
last bits and flip the ranking of near-tied products.

## Mock events

`pkg/mock` generates traffic for a dress shop: a persona browses the catalog, narrows to
an occasion, has the listing ranked by its taste, then opens and buys its best match.
Four GA4 events — `view_item_list`, `select_item`, `view_item`, `add_to_cart`.

### Catalog and personas

Twelve dresses, four per occasion, each with an attractor vector and a description
saying the same thing in prose:

```go
mock.Catalog()   // []mock.Item, DRESS_01 … DRESS_12
mock.Personas()  // Kate, Sofia, Lena, Mira
```

The personas are chosen to discriminate. Kate and Mira both dress for business but sit
at opposite ends of classic and modern, so occasion alone never decides the outcome —
each persona's top match is a different dress:

| Persona | | Top match |
|---|---|---|
| Kate | conservative lawyer | Tailored Sheath Dress |
| Sofia | event host, glamour | Tulle Ballgown |
| Lena | designer, dresses down | Jersey T-Shirt Dress |
| Mira | architect, contemporary business | Architectural Column Dress |

### Journeys

```go
mock.Journey(mock.PersonaFestiveSofia)      // mpv2.Payload[any], converts
mock.BrowseJourney(mock.PersonaCasualLena)  // browses and leaves
```

Nothing is hardcoded per persona: the occasion comes from the persona's strongest
occasion dimension and the dress from its highest affinity score. Change a persona's
vector and its journey follows.

A returning shopper is a history: one payload per visit, oldest first, each stamped with
its session's start time and a session id derived from it, the way GA4 does:

```go
mock.History(mock.Month(mock.PersonaBusinessKate))  // []mpv2.Payload[any], 7 visits
```

`Month` spans the four weeks up to `mock.BaseTime`, mostly browsing, converting twice.
The persona is set per `Visit`, so taste can drift within one history — overwrite the
persona of the early visits and the shopper starts the month as someone else.

### Filtering and ranking

```go
f := mock.PersonalizedFilter(mock.PersonaBusinessKate)
// Filter{Category: "Dresses", Occasion: "business",
//        Sort: mock.SortAffinity, Persona: &kate}

f.Apply(mock.Catalog())  // matching dresses, best match first
f.Query()                // "occasion=business&persona=Kate&sort=affinity"
```

Facets reach the wire as their own fields, so aggregating over them is a field lookup
rather than string parsing:

```json
"params": {
  "item_list_id": "dresses",
  "filter_category": "Dresses",
  "filter_occasion": "business",
  "sort_by": "affinity",
  "personalized": true,
  "persona": "Kate",
  "result_count": 4,
  "items": [...]
}
```

### `mock.Item`

`mock.Item` nests `params.Item` and adds two fields: the attractor — the vector
describing what taste a dress appeals to — and the description. The attractor is
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
  1. Architectural Column Dress  Severe black column, architectural seams and hard
                                 edges — contemporary business wear for a minimalist.
  2. Tailored Sheath Dress       Sharply tailored navy sheath, classic structured
                                 line — conservative boardroom wear.
```

The description is documentation of the vector, not an input to it; the two are kept in
step by hand. Because it never reaches the wire, an item decoded off a fixture has none
— join it on from `Catalog()`. `TestItemDescriptionStaysOffTheWire` pins that.

The embedded field carries no struct tag, and the two decoders treat that differently:

- **`encoding/json`** inlines an anonymous struct field on its own, so the GA4 fields sit
  flat beside the attractor. This is the enrichment stage's path.
- **`sesamy.Decode`** uses mapstructure, which does *not* inline an untagged embedded
  field. `Event.DecodeParams` into `mock.Item` returns the attractor but leaves the GA4
  fields zeroed — silently. Decode collected events into `params.Item`-based types
  instead, e.g. `params.AddToCart[params.Item]`.

`TestItemDecode` pins all three behaviours, including the silent one.

Because items are `mock.Item` rather than `params.Item`, the events are built with
`sesamy.NewEvent` — the stock `event.New*` constructors pin the item type.

### Envelope

```go
mock.Journey(mock.PersonaBusinessKate,
    mock.WithClientID("123.456"),
    mock.WithSessionID("789"),
    mock.WithConsentDenied(),
    mock.WithTimestamp(time.Now()),
)
```

Three properties the mocks guarantee, because they are what makes the corpus usable as
aggregation input:

- **A listing is its filter's output.** Items come from `Filter.Apply`, never
  hand-picked, so a fixture cannot report items that contradict the facets it claims.
- **Totals agree with line items.** `Value` is computed from the items, never hardcoded.
- **Output is deterministic.** Fixed ids, `mock.BaseTime`, and order-stable ranking.
  (`mpv2.NewPayload` stamps `time.Now()` and is deliberately unused.)

### Fixtures

`pkg/mock/testdata` holds each mock rendered as MPv2 wire-format JSON, for testing
decoding and the pipeline against real bytes:

```bash
go generate ./...     # regenerate; a no-op unless a mock changed
go test ./... -update # same, via the golden test
```

The fixtures are covered by a golden test, so they cannot drift from the constructors.

### Images

`pkg/mock/testdata/images` holds an SVG product shot for every dress (`dress_01.svg` …)
and an avatar for every persona (`persona_kate.svg` …), drawn by `Item.SVG` and
`Persona.SVG` and regenerated with the fixtures.

Like the journeys, nothing is drawn per id. A dress's hem comes from its length, its
waist, flare, sleeves and neckline from its attractor, its colour from its variant and
its fabric finish (sequins, velvet, silk, tulle, print) from the material its description
names. A persona's hairstyle and accessories come from its taste vector, and it wears the
colour of its top-match dress. Change the data and the picture follows.
