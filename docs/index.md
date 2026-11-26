---
layout: home

hero:
  name: schaumamoi
  text: Event aggregation, metering and analysis
  tagline: Fold GA4 events from foomo/sesamy-go into a shopper's personalization vector — and see it work against a shop full of generated traffic.
  image:
    src: /logo.png
    alt: 'schaumamoi — a Woipertinger in a Bavarian hat reading a ticker tape: "Schau ma moi"'
  actions:
    - theme: brand
      text: Get Started
      link: /guide/getting-started
    - theme: alt
      text: See the Reports
      link: /reports/catalog
    - theme: alt
      text: View on GitHub
      link: https://github.com/foomo/schaumamoi

features:
  - title: Taste as a sparse vector
    details: Shoppers and products live in one sparse space of taste dimensions. A product names only what describes it, so taste carries between categories through what they share — and each category weighs the dimensions it looks at.
    link: /guide/personalization
  - title: A distillery for events
    details: Events are fed in from a stream or a replay, cut and infused once on arrival, drawn off again by composable rules, and distilled by any number of recipes. The affinity still keeps a shopper's choices and distills their personalization vector.
    link: /guide/distillery
  - title: A vector that follows intent
    details: The vector fades per choice and lets go of the past when the shopper's intent changes — picked up on the first visit for a strong change, tested on long histories.
    link: /guide/vector
  - title: A shop to test against
    details: Deterministic GA4 traffic from a two-department shop — personas with hidden taste browse, choose and buy, request by request, while every listing is ranked with the vector the still distills.
    link: /guide/test-shop/
---

## Reports from the tests

The [Reports](/reports/catalog) are written by the test suite: every page is computed from
the real code on every run, every product and persona shown by its image, every claim
next to the test that checks it.

- [The catalog](/reports/catalog), [the personas](/reports/personas), [the shop](/reports/shop)
  and [the shoppers](/reports/shoppers): the test shop, package by package
- [The distillery](/reports/distillery): how a still works, and what it distills from the
  traffic
- [The vector](/reports/affinity): how the personalization vector fades and follows a
  changing intent
