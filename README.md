<p align="center">
  <img src="docs/public/logo.png" alt="schaumamoi — a Woipertinger in a Bavarian hat reading a ticker tape: &quot;Schau ma moi&quot;" width="320">
</p>

# schaumamoi

event aggregation, metering and analysis

Processes GA4 event data as produced by [`foomo/sesamy-go`](https://github.com/foomo/sesamy-go):
it folds a shopper's events into a personalization vector, and tests that against
deterministic traffic from a mock shop.

**Documentation: [foomo.github.io/schaumamoi](https://foomo.github.io/schaumamoi/)**

## Requirements

The toolchain (Go, Bun for the docs, actionlint) is pinned in `.mise.toml`:

```bash
mise install
```

## Usage

```bash
go test ./...        # run the tests; they also write the documentation's reports
mise run docs        # build the documentation site into docs/.vitepress/dist
mise run docs:dev    # serve it locally with live reload
```

## Packages

| Package | |
|---|---|
| `pkg/personalization` | the sparse taste vector and its (weighted) score |
| `pkg/corpus` | a corpus of events: filtered and decorated once, read by any number of derivers |
| `pkg/affinity` | the corpus that emits a shopper's personalization vector |
| `pkg/mock` | deterministic GA4 traffic from a two-department shop |

How they work, with reports computed from the code by the tests, is in the
[documentation](https://foomo.github.io/schaumamoi/); its sources are in [`docs/`](docs/).
