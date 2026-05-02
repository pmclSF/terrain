# DeepEval + Terrain

How to wire up [DeepEval](https://github.com/confident-ai/deepeval)
so Terrain ingests its eval output.

## What Terrain does with DeepEval data

When you pass `--deepeval-results <file.json>` to `terrain analyze`,
Terrain parses the file into an `EvalRunEnvelope` inside the
snapshot. The same three eval-data-aware AI detectors that consume
Promptfoo data also consume DeepEval data:

- `aiCostRegression`
- `aiHallucinationRate`
- `aiRetrievalRegression`

## Wiring it up

DeepEval writes results to `~/.deepeval/test_results.json` by
default. Either point Terrain at that file, or write to a custom
location via DeepEval's Python API and pass the path explicitly:

```bash
# Default location
terrain analyze --deepeval-results ~/.deepeval/test_results.json

# Custom location
terrain analyze --deepeval-results path/to/deepeval-out.json
```

Multiple suites:

```bash
terrain analyze \
  --deepeval-results out/deepeval-rag.json \
  --deepeval-results out/deepeval-agent.json
```

## Schema versions accepted

The adapter handles two DeepEval shapes:

- **Older `testCases` shape** — `{"testRunId": ..., "testCases":
  [{"metricsData": [...]}]}`
- **Newer `runId` shape (1.x)** — `{"runId": ...}` (with
  `testCases` or `test_results`)

When `testRunId` is empty the adapter falls back to `runId` so
modern DeepEval JSON dumps still produce a usable `EvalRunEnvelope`.
Without this, baseline matching dropped into the framework-wide
first-match fallback and could cross-attribute runs.

`createdAt` accepts RFC3339, the older space-separated
`2026-04-30 12:00:00` shape, and microsecond-fractional without
timezone.

## Metric name normalization

DeepEval emits metric names in two shapes:

- snake_case (`answer_relevancy`)
- human-readable (`Answer Relevancy`)

Terrain lowercases and replaces internal spaces with underscores so
both shapes match the consumer detectors'
`retrievalScoreKeys` / `hallucinationGroundingKeys` whitelists.

## Baseline comparison

Same as Promptfoo: keep `--write-snapshot` in CI to populate the
implicit baseline, or pass `--baseline path/to/old.json` explicitly.

## Calibration fixtures

DeepEval-shaped fixtures haven't shipped in the 0.2 corpus (the
0.2-known-gaps doc tracks this for 0.3). If you can contribute one,
the format is:

```
tests/calibration/<your-fixture>/
├── labels.yaml             # expected signals
├── eval-runs/
│   └── deepeval.json       # your DeepEval output
└── (optional) baseline/
    └── eval-runs/
        └── deepeval.json   # the prior run for regression detectors
```

## Troubleshooting

| Symptom | Cause |
|---------|-------|
| `deepeval payload has no testCases` | File missing the `testCases` array; check DeepEval version |
| Detectors don't see metric scores | Metric name has internal space and isn't in the lowercase+underscore form — adapter handles this since 0.2 |
| Wrong run cross-attributed | RunID empty; populate `testRunId` or `runId` in the DeepEval config |
