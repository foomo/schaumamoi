# The shop

`pkg/internal/mock` is the shop the tests run on. It is internal: test fixtures for this
module, free to change, not an API. It generates traffic for a shop with two departments,
women's fashion and home decoration: a persona browses the fashion categories, opens and
buys something it likes, and moves on to home decoration — every listing ranked by what
the shop has learned.
Four GA4 events — `view_item_list`, `select_item`, `view_item`, `add_to_cart`.

## Catalog and personas

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

## Journeys

```go
mock.Journey(mock.PersonaFestiveSofia)      // mock.Session, converts
mock.BrowseJourney(mock.PersonaCasualLena)  // browses and leaves
```

A visit reaches the wire the way a real client sends it: not as one envelope, but as a
`mock.Session`, one request per event. Each request carries its own `timestamp_micros`
and an `engagement_time_msec` of the time since the previous request, and all share the
visit's session id. The requests follow each other at a shopper's pace, seeded so the
fixtures don't churn:

| After | Next request |
|---|---|
| a listing | 5–25 s (scanning it) |
| a selection | 20–75 s (looking at the product) |
| a product view | 15–60 s |
| an add to cart | 5–15 s |

`Session.Events()` gives a visit's events in order, and `Start()` and `ID()` give its
start time and session id.

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

A returning shopper is a history: one session per visit, oldest first. Each session
starts at its scheduled time, and its session id is derived from that time, the way GA4
does:

```go
mock.History(mock.Month(mock.PersonaBusinessKate))  // []mock.Session, 7 visits
```

`Month` spans the four weeks up to `mock.BaseTime`, mostly browsing, converting twice.
The shopper's corpus carries over from visit to visit, the way a shop recognises a
returning client.
The persona is set per `Visit`, so the intent can change within one history: overwrite
the persona of some visits and the shopper wants something else from then on.

## Images

`pkg/internal/mock/testdata/images` holds an SVG product shot for every dress and pair of
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

