# Promptfoo + Terrain

How to wire up [Promptfoo](https://www.promptfoo.dev/) so Terrain
ingests its eval output and surfaces cost / hallucination / retrieval
regressions.

## What Terrain does with Promptfoo data

When you pass `--promptfoo-results <file.json>` to `terrain analyze`,
Terrain parses the file into an `EvalRunEnvelope` inside the
snapshot. Three eval-data-aware AI detectors then read those
envelopes:

| Detector | Triggers when |
|---|---|
| `aiCostRegression` | Per-case cost is above the configured floor + relative-delta threshold versus baseline |
| `aiHallucinationRate` | Hallucination-shaped failure rate (per `caseIsScoreable` + `caseLooksHallucinated`) exceeds the threshold |
| `aiRetrievalRegression` | Retrieval-quality score (context_precision / nDCG / coverage / faithfulness) dropped vs baseline |

If you don't pass `--promptfoo-results`, none of these fire — they
require runtime data.

## Wiring it up

Run Promptfoo with `--output` to write a JSON file:

```bash
promptfoo eval -c promptfoo.yaml --output promptfoo.json
```

Then run Terrain:

```bash
terrain analyze --promptfoo-results promptfoo.json
```

You can pass the flag multiple times for multiple suites:

```bash
terrain analyze \
  --promptfoo-results suite-rag.json \
  --promptfoo-results suite-classifier.json
```

## Baseline comparison

The cost and retrieval regression detectors need a baseline to
compare against. Two ways to supply one:

1. **Snapshot history** — keep `terrain analyze --write-snapshot` in
   CI. Terrain reads the most recent baseline from
   `.terrain/snapshots/latest.json`.
2. **Explicit baseline** — `terrain analyze --baseline path/to/old.json`
   uses the supplied snapshot as the comparison anchor.

## Schema versions accepted

The adapter handles both the v3 nested-results shape
(`{"results": {"results": [...], "stats": {...}}}`) and the v4+
flat-results shape (`{"results": [...]}` or `[...]`). Magnitude
detection on `createdAt` accepts both unix-millis and unix-seconds.

## Per-case cost

Promptfoo writes per-case cost to either:

- `r.response.tokenUsage.cost` (long-standing shape)
- `r.cost` (post-v0.91 modern shape)

Terrain reads the response-level field first and falls back to the
top-level field when the former is zero — both shapes work. If
neither is populated and only `stats.tokenUsage.cost` is set,
`aiCostRegression` will silently no-op (per-case data is required to
pair against the baseline).

## Errors vs failures

When a Promptfoo case crashes (provider timeout, schema parse
error, network) — represented in the JSON as a row-level `error`
string — Terrain routes that row into `Aggregates.Errors` rather
than `Aggregates.Failures`. This matters because
`aiHallucinationRate` excludes `Errors` from the denominator (you
shouldn't count infra crashes as hallucination cases).

## Calibration fixtures

Examples of valid Promptfoo input live under:

- `tests/calibration/eval-cost-regression/`
- `tests/calibration/eval-hallucination-rate/`

Drop your own JSON into `eval-runs/promptfoo.json` inside a fixture
directory plus a `labels.yaml` declaring the expected signals to
add a new fixture. See [`docs/contributing/writing-a-detector.md`](../contributing/writing-a-detector.md)
for the format.

## Troubleshooting

| Symptom | Cause |
|---------|-------|
| `promptfoo payload has no results array` | File is empty or has neither nested nor flat shape |
| `aiCostRegression` doesn't fire despite a real regression | Per-case cost not populated; aggregate-only is insufficient |
| `aiHallucinationRate` fires above expected rate | Confirm errored cases are routed via `error` field, not `failureReason` |
| Cross-attributed runs in repos with multiple suites | Set distinct `evalId` in each Promptfoo config so RunIDs don't collide |
