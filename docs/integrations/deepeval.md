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

DeepEval result persistence depends on how the suite is executed and
configured. For CI, configure DeepEval to write a JSON run artifact
(for example with `DEEPEVAL_RESULTS_FOLDER`, or with your Python
evaluation code) and pass that generated file to Terrain:

```bash
# Example: run DeepEval, then point Terrain at the generated JSON file
DEEPEVAL_RESULTS_FOLDER=out/deepeval deepeval test run tests/eval
terrain analyze --deepeval-results out/deepeval/<generated-run>.json

# Custom location from your own wrapper/API code
terrain analyze --deepeval-results path/to/deepeval-out.json
```

For multiple suites, pass a comma-separated list:

```bash
terrain analyze \
  --deepeval-results out/deepeval-rag.json,out/deepeval-agent.json
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
`2099-04-30 12:00:00` shape, and microsecond-fractional without
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

DeepEval-shaped calibration fixtures live in `tests/calibration/`. To contribute a new one, the format is:

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
| Detectors don't see metric scores | Metric name has internal space and isn't in the lowercase+underscore form — adapter normalizes this automatically |
| Wrong run cross-attributed | RunID empty; populate `testRunId` or `runId` in the DeepEval config |
