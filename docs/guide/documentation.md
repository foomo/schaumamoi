# Documentation

This site is a [VitePress](https://vitepress.dev) site in `docs/`, set up like the
other foomo projects' documentation: pages written by hand, and **exhibits** the tests
generate, shown on those pages.

## Exhibits from the tests

An exhibit is what a test shows of the code it checks: the shop's traffic, what the
distillery distills, the vector's fading, computed from the real code on every run.
Each package's `TestExhibit` hands its data to `pkg/internal/docs`, which renders the
exhibit's template (`templates/<name>.html`) into a fragment, `docs/.exhibits/<name>.md`,
and writes the product and persona images next to it, to `docs/.exhibits/images`.

A page shows an exhibit by including its fragment, with VitePress's
`<!--@include: …-->`. The pages are committed like any other: a page of its own under
`docs/exhibits/`, or a guide page that opens with its exhibit. `docs.Exhibits` names
each exhibit and the page that shows it, because an included fragment's image paths
resolve against that page.

| Exhibit | What it shows |
|---|---|
| [Catalog](/exhibits/catalog) | the category tree, the dimensions and weights, every product with its copy and attractor |
| [Personas](/exhibits/personas) | each persona's taste and favourites, how it chooses, and how it ranks the whole catalog |
| [Shop](/exhibits/shop) | every listing as a new shopper sees it, and ranked by what one visit taught the shop |
| [Shoppers](/exhibits/shoppers) | journeys step by step, months of visits, and the shop's final rankings |
| [Distillery](/exhibits/distillery) | how a still works, and what it distills: sessions, months, drift, detour, consent |
| [Affinity vector](/exhibits/affinity) | how the personalization vector fades and follows a changing intent (visit by visit, as `TestIntentPicksUpQuickly` checks it), against a flat sum and a decay by time |

The fragments are git-ignored. They are test output, deterministic but rebuilt on every
run, so they never go stale in version control.

The exhibit styles and the chart hover live in the VitePress theme
(`docs/.vitepress/theme/exhibit.css` and `charts.ts`). The line charts are drawn as SVG
by `pkg/internal/docs/chart.go`, with a crosshair readout and a table view.

## Building and publishing

```shell
mise run docs        # go test, then vitepress build into docs/.vitepress/dist
mise run docs:dev    # go test, then vitepress dev
```

The `docs` workflow builds the site on pull requests and pushes to `main`, and deploys
it to GitHub Pages on `main`.
