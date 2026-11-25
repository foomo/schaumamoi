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

Besides checking the code, the tests write the [Reports](/reports/mock) of this site:
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
| `pkg/corpus` | a corpus of events: filtered and decorated once on arrival, read by any number of derivers — see [The corpus](/guide/corpus) |
| `pkg/affinity` | the corpus that emits a shopper's personalization vector — see [The vector](/guide/vector) |
| `pkg/mock` | deterministic GA4 traffic from a two-department shop — see [The shop](/guide/shop) and [Events on the wire](/guide/events) |
| `pkg/internal/docs` | the templates the tests render their reports with — see [Documentation](/guide/documentation) |
