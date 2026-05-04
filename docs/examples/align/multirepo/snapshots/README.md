# Per-repo snapshot fixtures

The Track 6 manifest format supports two ways to feed a repo into
the cross-repo aggregator:

1. **`path:` only.** The aggregator walks each repo and produces a
   fresh snapshot during the portfolio run. Convenient for small
   portfolios; gets expensive for large ones since every aggregator
   run re-walks every repo.

2. **`snapshotPath:` set.** The aggregator loads a previously
   written snapshot JSON instead of walking. Adopters who run
   `terrain analyze --write-snapshot` per-repo on their own
   schedule (e.g. nightly CI per-repo) hand the aggregator the
   pre-computed snapshots. Cheaper, and consistent across
   aggregator runs.

For this example we ship the manifest with `path:` only — every
repo gets walked fresh. Real portfolios with > 5 repos should
adopt the snapshot-path pattern.

## Future fixture shape

When the aggregator lands in 0.2.x and a runnable demo becomes
useful, this directory will hold:

```
snapshots/
├── README.md            ← this file
├── web-app.json         ← saved snapshot from `web-app` repo
├── api-service.json     ← saved snapshot from `api-service` repo
└── legacy-portal.json   ← saved snapshot from `legacy-portal` repo
```

…and the manifest's repo entries gain `snapshotPath: snapshots/<name>.json`
fields. We don't ship these snapshots today because:

- The schema is still settling within the 0.2.x window; freezing a
  snapshot now would create maintenance churn as fields stabilize
- The aggregator binary doesn't exist yet to consume them

When both conditions clear, this fixture lights up end-to-end.
