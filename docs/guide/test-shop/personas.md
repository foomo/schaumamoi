# Personas

`pkg/internal/mock/persona` is the four shoppers the traffic is generated for, each a
taste vector over the [catalog's dimensions](./catalog#dimensions):

```go
persona.All()   // Kate, Sofia, Lena, Mira
persona.Kate    // persona.Persona{Name, Description, Affinity}
```

The [personas exhibit](/exhibits/personas) shows each one's taste, favourite in every
category, and ranking of the whole catalog.

## Chosen to discriminate

Kate and Mira both dress for business but sit at opposite ends of classic and modern,
so occasion alone never decides the outcome. Both fashion rails compete in one ranking,
each product scored with its own category's weights, so each persona's top match is a
different product, and three of them are trousers. Kate's is a pair of trousers because
the trouser weights count tailoring more than the dress weights do:

| Persona | | Top match |
|---|---|---|
| Kate | conservative lawyer | Temple Pleated Trousers |
| Sofia | event host, glamour | Odette Tulle Ballgown |
| Lena | designer, dresses down | Haven Joggers |
| Mira | architect, contemporary business | Grid Cigarette Trousers |

`Persona.Occasion()` is the section of the shop a persona browses: its strongest
occasion dimension, not a field set per persona. `Persona.TopMatch(items)` is its
favourite among items, by `catalog.Score`.

## How a persona chooses

A persona decides what a shopper picks, and nothing else: the shop never sees it. A
shopper doesn't read a listing to the end and take the optimum. `Persona.Choose` scans
the first `ScanDepth` (6) positions, considers what it likes well enough (at least
`GoodEnough`, 80%, of its favourite's score in that listing) and picks one, the earlier
positions more likely: position *n* weighs 1/*n*. If nothing near the top suits it, it
scrolls on to its favourite. The pick is seeded, so the traffic stays reproducible.

So a shopper does not always open the first product, nor, from a listing not yet ranked
for it, necessarily its best match. Because the shop learns from what was opened, a pick
that is not the favourite feeds back into the ranking: Sofia lands on the sequin
cocktail dress on her first visit and the shop shows it first for a while, until she
finds her way to the tulle ballgown.

## Avatars

`Persona.SVG()` draws an avatar (`persona_kate.svg` …) next to the product images. A
persona's hairstyle and accessories come from its taste vector, and it wears the colour
of its top-match product.
