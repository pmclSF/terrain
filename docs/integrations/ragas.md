# Ragas + Terrain

How to wire up [Ragas](https://github.com/explodinggradients/ragas)
so Terrain ingests its eval output.

## What Terrain does with Ragas data

When you pass `--ragas-results <file.json>` to `terrain analyze`,
Terrain parses the file into an `EvalRunEnvelope` inside the
snapshot. The retrieval-quality named scores Ragas produces
(`faithfulness`, `context_precision`, `context_recall`,
`answer_relevancy`, `context_relevance`, etc.) feed
`aiRetrievalRegression` directly via the same EvalRunEnvelope
plumbing that Promptfoo and DeepEval use.

## Wiring it up

Ragas results come from the Python evaluation API. Export to JSON:

```python
from ragas import evaluate
from datasets import Dataset

result = evaluate(
    dataset=ds,
    metrics=[faithfulness, context_precision, answer_relevancy],
)
result.to_pandas().to_json("ragas-out.json", orient="records")
```

Then run Terrain:

```bash
terrain analyze --ragas-results ragas-out.json
```

## Schema shapes accepted

Three Ragas output shapes are accepted (in priority order):

1. `evaluation_results` — modern Ragas (≥0.1.0) emits this when
   evaluating with the high-level API
2. `results` — legacy Ragas envelope
3. `scores` — what `result.to_pandas().to_json(...)` produces when
   the DataFrame is exported with `orient="records"` and a `scores`
   wrapper

The adapter merges all three into a single internal row list.

## Metric name normalization

Ragas metric keys are normalized before matching against the
quality-score whitelist:

- lowercased
- hyphens → underscores
- internal spaces → underscores
- leading `eval_` prefix stripped (used by some
  `ragas-evaluate-helpers` shapes)
- trailing `_score` stripped

So `Context Precision`, `context-precision`, `eval_context_precision`,
and `context_precision_score` all map to the same canonical key.

## Cost data

Ragas DataFrame exports include `total_tokens`, `prompt_tokens`,
and `total_cost` columns. Terrain treats these as ancillary
numeric fields that don't vote on success/failure (only the
quality axes do). They feed `aiCostRegression` via
`TokenUsage.Cost` when present.

## Calibration fixtures

Ragas-shaped fixtures haven't shipped in the 0.2 corpus. The 0.3
corpus expansion adds at least one Ragas + one DeepEval fixture
to lock in adapter behavior. Contribute via the format described
in [`promptfoo.md`](promptfoo.md#calibration-fixtures).

## Troubleshooting

| Symptom | Cause |
|---------|-------|
| `ragas payload has no results, evaluation_results, or scores` | File doesn't have any of the three accepted top-level keys |
| Score with internal space is dropped | Pre-0.2 the normaliser only stripped hyphens; updated since |
| `aiCostRegression` doesn't fire | Confirm `total_cost` is present per-row in the DataFrame export |
| Quality-only rows treated as Successes despite zero score | Ragas rows that contain only ancillary numerics (cost, tokens) but no quality axes don't count toward success votes |
