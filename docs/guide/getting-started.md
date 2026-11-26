# Getting Started

schaumamoi processes GA4 event data as produced by
[foomo/sesamy-go](https://github.com/foomo/sesamy-go): it folds a shopper's events into
a personalization vector, and tests that against deterministic traffic from a mock shop.

## Requirements

The toolchain is pinned in `.mise.toml` and installed with [mise](https://mise.jdx.dev):
Go for the code, Bun for this documentation site, actionlint for the workflows.

```shell
mise install
```

## Run the tests

```shell
go test ./...
```

Besides checking the code, the tests write the [Reports](/reports/catalog) of this site:
pages computed from the real code on every run, next to the tests that check them.

## Build the documentation

```shell
mise run docs        # generate the reports and build the site into docs/.vitepress/dist
mise run docs:dev    # generate the reports and serve the site with live reload
```

## The packages

| Package | What it does |
|---|---|
| `pkg/personalization` | the sparse taste vector: `Map`, `Score`, `WeightedScore` — see [Personalization](/guide/personalization) |
| `pkg/distillery` | a still per user: fed events from a stream or a replay, cut and infused once on arrival, distilled by any number of recipes — see [The distillery](/guide/distillery) |
| `pkg/distillery/stamp` | time and session, shared by every still: the stamp infuser and the rules on it — see [Drawing off](/guide/distillery#drawing-off) |
| `pkg/distillery/stills/affinity` | the still that distills a shopper's personalization vector — see [The vector](/guide/vector) |
| `pkg/distillery/recipes/visits` | recipes that read any stamped still visit by visit: visits, and phases of steady taste — see [Visits and phases](/guide/visits) |
| `pkg/internal/mock` | the test shop: deterministic GA4 traffic from two departments, in `catalog`, `persona`, `shop`, `shopper` and `wire` — see [The test shop](/guide/test-shop/) |
| `pkg/internal/docs` | the templates the tests render their reports with — see [Documentation](/guide/documentation) |
