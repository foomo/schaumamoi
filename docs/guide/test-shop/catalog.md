# Catalog

`pkg/internal/mock/catalog` is what the test shop sells: 54 products in a small category
tree, each with an attractor vector and a description saying the same thing in prose.
Here is every one of them, as the tests see it.

<!-- written by the tests: TestExhibit renders it into docs/.exhibits/ -->
<!--@include: ../../.exhibits/catalog.md-->

## Products

There are 18 dresses and 18 pairs of trousers, six of each per occasion, and 18 home
decoration pieces, six per category. Several products in each section suit each
persona, so a shopper has real alternatives to choose between. Names are shop-style, an
invented model name plus what the product is (`Halden Concrete Pendant`, `Odette Tulle
Ballgown`); the description, not the name, says what taste it serves.

```go
catalog.All()             // []catalog.Item, all 54
catalog.Fashion()         // dresses then trousers
catalog.HomeDecoration()  // cushions, lamps, vases
catalog.Dresses()         // one category
```

## Category tree

The GA4 category path carries the whole branch, e.g.
`Women's Fashion > Trousers > Business > Ankle`. Dresses come in mini, midi and maxi,
and trousers in cropped, ankle and full; home pieces stop at the category,
`Home Decoration > Lamps`. `Item.Category`, `Item.Occasion` and `Item.Length` read the
facets back out of the path rather than storing them twice.

Categories are where products are offered: entry points to browse and choose from, not
a signal about the shopper. What the shopper selects, views and buys is the signal.
How the shop lists them is the [shop](./shop)'s business.

## Dimensions

One `personalization.Vector` carries taste across every category, in the dimensions of
`pkg/internal/mock/taste` (`taste.Business`, `taste.Cozy` …). The vector is sparse: an
item carries only the dimensions that describe it. Dresses and trousers speak largely
the same dimensions, so taste carries between them. Home decoration shares some
(modern, classic, minimalistic, bohemian, opulent, romantic, retro, edgy, bold,
colourful), never uses the fashion-only ones (business, festive, casual, feminine,
structured) and adds its own (cozy, natural, industrial). So what the shop learns from
clothes reaches the home department only through what the two share, and the rest is
learned from home picks.

## Weights and `Score`

Every product is scored in its own category's weights: `catalog.Score(taste, item)` is
the weighted dot product of a taste and the item's attractor under
`catalog.CategoryWeights(item.Category())`. It is the one scorer the shop ranks with
and the personas choose by. Each department states what it looks at, and each category
overrides only where it differs. A category then looks only at the part of the vector
space its own products speak: no cushion is industrial, so cushions don't weigh
industrial, whatever home decoration does. The exhibit above shows every department's
base weights and every category's own, next to its products;
[Personalization](../personalization#weights-per-category) explains how they are scaled.

## Images

Every product draws its own picture: `Item.SVG()`, written to
`pkg/internal/mock/testdata/images` (`dress_01.svg`, `trousers_01.svg`, `home_01.svg`
…) with the fixtures. Nothing is drawn per id. A dress hangs on a shoulder hanger: its
hem comes from its length, its waist, flare, sleeves and neckline from its attractor.
Trousers hang from a clip hanger: the leg length comes from the length facet, the leg
width from the attractor (palazzo wide, cigarette tapered). Colour comes from the
variant, and the fabric finish (sequins, velvet, silk, tulle, print, check, brocade,
denim, pleats, pockets) from the material the description names. Home pieces stand on
their own: the shape comes from the category, the form from the copy (a pendant hangs,
a ginger jar has its lid) and so does the finish (tassels, cable knit, a pleated shade,
swirled glass). Change the data and the picture follows.
