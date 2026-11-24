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

func (m Map) Score(other Map) Value                        // sparse dot product
func (m Map) WeightedScore(other, weights Map) Value       // … seen through a context
```

A map only names the dimensions it cares about; a dimension missing from either side
contributes nothing. `WeightedScore` adds a third map, the weights of a context: each
shared dimension counts `weights[d]` times, and a dimension the weights don't name
counts for nothing. Both sum in sorted dimension order: float addition is not
associative and Go randomises map iteration, so summing in map order would wobble in the
last bits and flip the ranking of near-tied products.

## Mock events

`pkg/mock` generates traffic for a shop with two departments, women's fashion and home
decoration: a persona browses the fashion categories, opens and buys something it
likes, and moves on to home decoration — every listing ranked by what the shop has
learned.
Four GA4 events — `view_item_list`, `select_item`, `view_item`, `add_to_cart`.

### Catalog and personas

54 products, each with an attractor vector and a description saying the same thing in
prose: 18 dresses and 18 trousers, six of each per occasion, and 18 home decoration
pieces, six per category. Several products in each section suit each persona, so a
shopper has real alternatives to choose between. Names are shop-style, an invented
model name plus what the product is (`Halden Concrete Pendant`, `Odette Tulle
Ballgown`); the description, not the name, says what taste it serves. They sit in a
small category tree, and each listing's id is its path:

| Category | List id | Items |
|---|---|---|
| All Products | `all-products` | all 54, both departments |
| Women's Fashion | `womens-fashion` | dresses and trousers |
| Dresses | `womens-fashion/dresses` | `DRESS_01` … `DRESS_18` |
| Trousers | `womens-fashion/trousers` | `TROUSERS_01` … `TROUSERS_18` |
| Home Decoration | `home-decoration` | cushions, lamps and vases |
| Cushions | `home-decoration/cushions` | six of `HOME_01` … `HOME_18` |
| Lamps | `home-decoration/lamps` | six of `HOME_01` … `HOME_18` |
| Vases | `home-decoration/vases` | six of `HOME_01` … `HOME_18` |

The GA4 category path carries the whole branch, e.g.
`Women's Fashion > Trousers > Business > Ankle`. Dresses come in mini, midi and maxi, and
trousers in cropped, ankle and full; home pieces stop at the category,
`Home Decoration > Lamps`.

Categories are where products are offered: entry points to browse and choose from, not
a signal about the shopper. What the shopper selects, views and buys is the signal, and
one `personalization.Map` carries it across every category. The Map is sparse: an item
carries only the dimensions that describe it. Dresses and trousers speak largely the
same dimensions, so taste carries between them. Home decoration shares some (modern,
classic, minimalistic, bohemian, opulent, romantic, retro, edgy, bold, colourful), never
uses the fashion-only ones (business, festive, casual, feminine, structured) and adds
its own (cozy, natural, industrial). So what the shop learns from clothes reaches the
home department only through what the two share, and the rest is learned from home picks.

How much each part of the vector counts is defined per category (`pkg/mock/weights.go`).
Each department states what it looks at: fashion leads with the occasion, home with how
a room should feel (cozy, natural, industrial). Each category overrides only where it
differs. Dresses weigh femininity and romance up and tailoring down, trousers the
reverse. Cushions are about comfort and texture, lamps about industrial, modern or
classic design, vases about colour, material and statement. Every product is scored
with its own category's weights (`mock.Score`): by the shop when it ranks, by the
personas when they choose, and by the corpus when it reads a category. A category's
weights are a complete statement (anything they don't name counts for nothing, and a
test keeps every dimension its products carry named). They're scaled to average 1, so
they set emphasis rather than scale, and a listing that mixes categories still ranks on
one scale.

| Category | Leads with |
|---|---|
| Dresses | occasion · feminine · romantic |
| Trousers | occasion · structured · edgy |
| Cushions | cozy · natural |
| Lamps | industrial · modern · classic |
| Vases | colourful · natural · bold |

```go
mock.Catalog()         // []mock.Item, all 54
mock.Fashion()         // dresses then trousers
mock.HomeDecoration()  // cushions, lamps, vases
mock.Personas()        // Kate, Sofia, Lena, Mira
```

The personas are chosen to discriminate. Kate and Mira both dress for business but sit
at opposite ends of classic and modern, so occasion alone never decides the outcome.
Both rails compete in one ranking, each product scored with its own category's weights,
so each persona's top match is a different product, and three of them are trousers.
Kate's is a pair of trousers because the trouser weights count tailoring more than the
dress weights do:

| Persona | | Top match |
|---|---|---|
| Kate | conservative lawyer | Temple Pleated Trousers |
| Sofia | event host, glamour | Odette Tulle Ballgown |
| Lena | designer, dresses down | Haven Joggers |
| Mira | architect, contemporary business | Grid Cigarette Trousers |

### Journeys

```go
mock.Journey(mock.PersonaFestiveSofia)      // mpv2.Payload[any], converts
mock.BrowseJourney(mock.PersonaCasualLena)  // browses and leaves
```

A journey crosses the rails before it decides:

```
womens-fashion → select
  → womens-fashion/trousers → select
  → womens-fashion/dresses → select
  → womens-fashion (narrowed to the persona's occasion) → select
  → view → add to cart
  → womens-fashion (back to the top, ranked by everything learned) → select
  → home-decoration → select
  → home-decoration/<category of that pick> → select → view
  → all-products (the shop's final ranking, everything learned)
```

Every visit ends on the listing of all products, without another click: the shop's
whole ranking for this shopper, across both departments. Its order is checked against
a profile replayed from the visit's own events, and read per category: in every
category the shopper chose from, that category's first product is one the persona
likes.

Every listing but the last is followed by a `select_item` carrying its list id. A
shopper doesn't read a listing to the end and take the optimum: it scans the first
`ScanDepth` (6) positions, considers what it likes well enough — at least `GoodEnough`
(80%) of its favourite's score in that listing — and picks one, the earlier positions
more likely. If nothing near the top suits it, it scrolls on to its favourite. The pick
is seeded, so the traffic stays reproducible. So a shopper does not always open the
first product, nor from a listing not yet ranked for it necessarily its best match,
and what it views and buys is what it opened. A browsing visit takes the same route
and leaves after the view.

Because the shop learns from what was opened, a pick that is not the favourite feeds
back into the ranking: Sofia lands on the sequin cocktail dress on her first visit and
the shop shows it first for a while, until she finds her way to the tulle ballgown.

Every listing is personalized, but by the shop, not the persona. The shop never sees a
persona and keeps no learner of its own. Each shopper's events go into an affinity
corpus (`pkg/affinity`) as they happen, and every listing is ranked with the
personalization vector that corpus emits at that moment. A first visit, with no events
yet, lands on the catalog order, and each listing after it is ranked with one more
pick. Every visit ends back on the top-level department, ranked by all the visit has
shown: the closing look. A test replays each journey's own events into a fresh corpus
and checks every listing's order against the vector it emits. The persona only decides
what the shopper picks; how well the listings anticipate it is up to the vector.

Nothing is hardcoded per persona: the occasion comes from the persona's strongest
occasion dimension and the product from its highest affinity score. Change a persona's
vector and its journey follows.

A returning shopper is a history: one payload per visit, oldest first, each stamped with
its session's start time and a session id derived from it, the way GA4 does:

```go
mock.History(mock.Month(mock.PersonaBusinessKate))  // []mpv2.Payload[any], 7 visits
```

`Month` spans the four weeks up to `mock.BaseTime`, mostly browsing, converting twice.
The shopper's corpus carries over from visit to visit, the way a shop recognises a
returning client.
The persona is set per `Visit`, so the intent can change within one history: overwrite
the persona of some visits and the shopper wants something else from then on.

### The personalization vector

The vector the shop ranks with is `affinity.Fade.Vector`, derived from the shopper's
choices (selections, views and carts; listings never enter the corpus). Recency is
counted in events, not time: a shopper who comes back after three weeks is not a
stranger. Three rules, applied in event order:

- every event fades everything before it by a factor (0.95: about 60% of the vector
  survives a visit);
- only the last 1000 events count at all, far more than the mock data reach;
- a strong change of intent forgets faster. When the last 3 choices point clearly away
  from the rest of the vector (cosine below 0.5, over the dimensions both name), the
  rest collapses to a fifth. Moving into a new category's own dimensions (clothes to
  cushions) is no change of intent and never triggers it.

`TestIntentPicksUpQuickly` checks this on long histories (a visit every two days, the
intent changing every six visits, every direction of change between the personas):

- **Strong change** to a taste that doesn't overlap the old one: the corpus reads it on
  the first visit, and the shop serves it by the second.
- **Gentle change** between overlapping tastes (Mira and Lena are both minimal and
  modern): the vector only fades into it, within three or four visits.

With the per-event fade alone, strong changes take as long as gentle ones. With no fade
at all, which was the shop's old behaviour, the shop stops following new intents once
the history is long enough. The corpus report's "Changing intent" section shows it
visit by visit.

### Filtering and ranking

```go
shopper := affinity.New(nil)            // one corpus per shopper
shopper.Add(envelope, events...)         // their events so far
profile := mock.Profile{
    Taste:   shopper.Derive(affinity.DefaultFade.Vector),
    Signals: shopper.Len(),
}

f := mock.SectionFilter(mock.PersonaBusinessKate).Personalize(profile)
// Filter{Category: "Women's Fashion", Occasion: "business",
//        Sort: mock.SortAffinity, Profile: profile}

f.Apply(mock.Catalog())  // business dresses and trousers, best match for the profile first
f.Query()                // "occasion=business&sort=affinity"
```

Facets reach the wire as their own fields, so aggregating over them is a field lookup
rather than string parsing. `taste_signals` is how many picks the ranking was learned
from, so a neutral ranking shows as 0:

```json
"params": {
  "item_list_id": "womens-fashion",
  "filter_category": "Women's Fashion",
  "filter_occasion": "business",
  "sort_by": "affinity",
  "personalized": true,
  "taste_signals": 3,
  "result_count": 8,
  "items": [...]
}
```

### `mock.Item`

`mock.Item` nests `params.Item` and adds two fields: the attractor — the vector
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

`pkg/mock/testdata/images` holds an SVG product shot for every dress and pair of
trousers and every home piece (`dress_01.svg`, `trousers_01.svg`, `home_01.svg` …) and
an avatar for every persona (`persona_kate.svg` …), drawn by `Item.SVG` and
`Persona.SVG` and regenerated with the fixtures.

Like the journeys, nothing is drawn per id. A dress hangs on a shoulder hanger: its hem
comes from its length, its waist, flare, sleeves and neckline from its attractor.
Trousers hang from a clip hanger: the leg length comes from the length facet, the leg
width from the attractor (palazzo wide, cigarette tapered). Colour comes from the
variant, and the fabric finish (sequins, velvet, silk, tulle, print, check, brocade,
denim, pleats, pockets) from the material the description names. Home pieces stand on
their own: the shape comes from the category, the form from the copy (a pendant hangs,
a ginger jar has its lid) and so does the finish (tassels, cable knit, a pleated shade,
swirled glass). A persona's hairstyle and accessories come from its taste vector, and it
wears the colour of its top-match product. Change the data and the picture follows.

### Reports

The tests also write two HTML pages for checking the concept by eye. They show every
product and persona by its generated image:

```bash
go test ./pkg/mock ./pkg/corpus
open pkg/mock/testdata/report.html    # listings, personas, rankings, journeys, months, catalog
open pkg/corpus/testdata/report.html  # what the corpus reads back: sessions, decay, drift, phases, consent
```

They are deterministic but git-ignored: they're test output, not fixtures.
