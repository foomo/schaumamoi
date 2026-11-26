# Getting Started

schaumamoi distills GA4 events, as produced by
[foomo/sesamy-go](https://github.com/foomo/sesamy-go), into per-user state. Events come
from a NATS JetStream stream or a database replay. Each user's events are fed into a
**still**, worked on once on arrival, and distilled by any number of **recipes**: a
shopper's personalization vector, the phases of their history, whatever you need next.
The [distillery](/guide/distillery) explains the words; the rest of this page gets you
running.

## Requirements

The toolchain is pinned in `.mise.toml` and installed with [mise](https://mise.jdx.dev):
Go for the code, Bun for this documentation site, actionlint for the workflows.

```shell
mise install
```

## A first still

The affinity still's example feeds a shopper's month of test traffic into a still,
distills it two ways, and draws off one visit. It's a test, so its output is checked:

```shell
go test -run '^Example$' -v ./pkg/distillery/stills/affinity
```

Read it in
[`example_test.go`](https://github.com/foomo/schaumamoi/blob/main/pkg/distillery/stills/affinity/example_test.go),
then [the distillery](/guide/distillery) for what each step does.

## Run the tests

```shell
go test ./...
```

Besides checking the code, the tests write the [exhibits](/guide/documentation) of this site:
pages computed from the real code on every run, next to the tests that check them.

## Build the documentation

```shell
mise run docs        # generate the exhibits and build the site into docs/.vitepress/dist
mise run docs:dev    # generate the exhibits and serve the site with live reload
```

## Packages

| Package | What it does |
|---|---|
| `pkg/personalization` | the sparse taste vector: `Vector`, `Score`, `WeightedScore`, `Cosine` — see [Personalization](/guide/personalization) |
| `pkg/distillery` | a still per user: fed events from a stream or a replay, cut and infused once on arrival, distilled by any number of recipes — see [Distillery](/guide/distillery) |
| `pkg/distillery/stamp` | time and session, shared by every still: the stamp infuser and the rules on it — see [Drawing off](/guide/distillery#drawing-off) |
| `pkg/distillery/stills/affinity` | the still that distills a shopper's personalization vector — see [Affinity vector](/guide/vector) |
| `pkg/distillery/recipes/visits` | recipes that read any stamped still visit by visit: visits, and phases of steady taste — see [Visits and phases](/guide/visits) |
| `pkg/internal/mock` | the test shop: deterministic GA4 traffic from two departments, in `catalog`, `persona`, `shop`, `journeys` and `wire` — see [Test shop](/guide/test-shop/) |
| `pkg/internal/docs` | the templates the tests render their exhibits with — see [Documentation](/guide/documentation) |
