# Great Expectations + Terrain

How to wire up [Great Expectations](https://greatexpectations.io/)
so Terrain ingests Validation Result JSON.

## What Terrain does with Great Expectations data

When you pass `--great-expectations-results <file.json>` to
`terrain analyze`, Terrain parses the file into an `EvalRunEnvelope`
inside the snapshot. Each Great Expectations expectation becomes one
eval case:

- `success: true` maps to score `1.0`
- `success: false` maps to score `0.0`
- expectation type plus column name becomes the stable case ID
- `unexpected_count`, `unexpected_percent`, and exception details
  become the failure reason when present

This gives Terrain the same normalized `EvalRuns` envelope used by the
other eval adapters. Axis-specific detectors still require their own
metadata: cost regression needs per-case cost, retrieval regression
needs retrieval-quality named scores, and hallucination-rate detection
needs hallucination or grounding metadata. Great Expectations results
primarily provide pass/fail validation evidence in 0.4.0.

## Wiring it up

Run your Great Expectations checkpoint or validation code and save the
Validation Result JSON:

```python
import json

result = checkpoint.run()
with open("great-expectations-results.json", "w", encoding="utf-8") as f:
    json.dump(result.to_json_dict(), f, indent=2)
```

Then run Terrain:

```bash
terrain analyze --great-expectations-results great-expectations-results.json
```

For multiple suites, pass a comma-separated list:

```bash
terrain analyze \
  --great-expectations-results data-contracts.json,training-split.json
```

## Schema shapes accepted

Terrain accepts Great Expectations Validation Result JSON with:

- top-level `results[]`
- per-result `expectation_config.expectation_type`
- top-level `statistics.evaluated_expectations`,
  `successful_expectations`, and `unsuccessful_expectations` when
  present
- optional `meta.great_expectations_version`
- optional `meta.run_id.run_time`

If the statistics block is absent or empty, Terrain computes aggregate
success and failure counts from the per-expectation rows.

## Baseline comparison

Use the same baseline flow as other eval adapters: keep
`terrain analyze --write-snapshot` in CI, or pass
`--baseline path/to/old.json` explicitly when comparing a current run
against a previous snapshot.

## Troubleshooting

| Symptom | Cause |
|---------|-------|
| `ge: parse ...` | File is not valid JSON or is not a Great Expectations Validation Result |
| No per-column case ID | The expectation did not include `kwargs.column`; Terrain falls back to expectation type |
| Aggregate count is zero | The `statistics` block was missing or empty; Terrain computes counts from `results[]` |
