# Documentation

This site is a [VitePress](https://vitepress.dev) site in `docs/`, set up like the
other foomo projects' documentation: guides written by hand, and **Reports** the
tests generate.

## Reports from the tests

Each package's `TestReport` computes its page from the real code (the shop's traffic,
the corpus's readings, the vector's fading) and hands the data to `pkg/internal/docs`.
That package renders the page's template (`templates/<name>.html`, wrapped in
`templates/page.md`) into `docs/reports/<name>.md`, and writes the product and persona
images next to them, to `docs/reports/images`.

| Report | What it shows |
|---|---|
| [The shop](/reports/mock) | listings, personas, rankings, journeys, months, final rankings, catalog |
| [The corpus](/reports/corpus) | how the corpus works, and what it reads back: sessions, months, drift, detour, consent |
| [The vector](/reports/affinity) | how the personalization vector fades and follows a changing intent (visit by visit, as `TestIntentPicksUpQuickly` checks it), against a flat sum and a decay by time |

The generated pages are git-ignored. They are test output, deterministic but rebuilt
on every run, so they never go stale in version control.

The report styles and the chart hover live in the VitePress theme
(`docs/.vitepress/theme/report.css` and `charts.ts`). The line charts are drawn as SVG
by `pkg/internal/docs/chart.go`, with a crosshair readout and a table view.

## Building and publishing

```shell
mise run docs        # go test, then vitepress build into docs/.vitepress/dist
mise run docs:dev    # go test, then vitepress dev
```

The `docs` workflow builds the site on pull requests and pushes to `main`, and deploys
it to GitHub Pages on `main`.
