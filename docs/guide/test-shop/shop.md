# The shop

`pkg/internal/mock/shop` is the shop's side of the traffic: the listings it renders,
and the GA4 events reporting them — `view_item_list`, `select_item`, `view_item`,
`add_to_cart`. It ranks by what a shopper has done, never by who the shopper is: the
package does not import a persona. The [shop report](/reports/shop) shows every
listing, and the same department ranked for four shoppers after one visit each.

## Listings

A listing is a `shop.Filter` over the catalog: the category being browsed, whatever the
shopper narrowed it down by, and how it is ranked. Each listing's id is its path in the
category tree:

| Listing | List id | Filter |
|---|---|---|
| All Products | `all-products` | `shop.AllProductsFilter` |
| Women's Fashion | `womens-fashion` | `shop.CategoryFilter` |
| Dresses | `womens-fashion/dresses` | `shop.DressesFilter` |
| Trousers | `womens-fashion/trousers` | `shop.TrousersFilter` |
| Home Decoration | `home-decoration` | `shop.HomeFilter` |
| Cushions | `home-decoration/cushions` | `shop.CushionsFilter` |
| Lamps | `home-decoration/lamps` | `shop.LampsFilter` |
| Vases | `home-decoration/vases` | `shop.VasesFilter` |

Items always come from `Filter.Apply`, never hand-picked, so a fixture cannot report
items that contradict the facets it claims.

## Ranking by what was learned

Every listing is ranked by a `shop.Profile`: the shopper's personalization vector as the
affinity still distills from their events so far, and how many events went into it.
The shop keeps no learner of its own. A shopper with no events has a neutral profile,
under which every product scores the same, so a first listing comes in catalog order.

```go
c := affinity.New(nil)                 // one still per shopper
c.Feed(envelope, events...)            // their events so far
profile := shop.Profile{
    Taste:   c.Distill(affinity.DefaultFade.Vector),
    Signals: c.Len(),
}

f := shop.CategoryFilter.Personalize(profile)
// Filter{Category: "Women's Fashion", Sort: shop.SortAffinity, Profile: profile}

f.Apply(catalog.All())  // dresses and trousers, best match for the profile first
f.Query()               // "sort=affinity"
```

Products are ranked by `catalog.Score` in their own category's weights, so a listing
that mixes categories (a department, all products) still ranks on one scale. Equal
scores fall back to the item id, so the order is reproducible.

## Events

`shop.ViewItemList(f)` reports a listing with the filter that produced it. Facets
reach the wire as their own fields, so aggregating over them is a field lookup rather
than string parsing. `taste_signals` is how many picks the ranking was learned from, so
a neutral ranking shows as 0:

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

`shop.SelectItem(f, item)` carries the list id of the listing it was picked from;
`shop.ViewItem` and `shop.AddToCart` follow. Totals are computed from the line items,
never hardcoded. The items are `catalog.Item`, which is why the events are built with
`sesamy.NewEvent`: the stock `event.New*` constructors pin the item type to
`params.Item` (see [Events on the wire](./wire#catalog-item)).
