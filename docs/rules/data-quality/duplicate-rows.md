# terrain/data-quality/duplicate-rows — Duplicate Eval Rows

> Auto-generated stub. Edit anything below the marker; the generator preserves it.

**Type:** `duplicateEvalRows`  
**Domain:** ai  
**Default severity:** medium  
**Status:** experimental

## Promotion plan

Fires when an eval dataset has more than 5% duplicate input rows.

## Evidence sources

- `structural-pattern`

## Confidence range

Detector confidence is bracketed at [0.90, 0.99] (heuristic today; calibrated against a labeled corpus over time).

<!-- docs-gen: end stub. Hand-authored content below this line is preserved across regenerations. -->

## 1. Summary

An eval dataset has more than 5% duplicate input rows, inflating apparent coverage without adding distinct test cases.

## 2. Severity & status

- **Tier:** preview
- **Default severity:** off (opt-in)
- **Status:** preview — pending validation

## 3. What this catches

- An eval CSV with 1000 rows where 200 share an identical input field
- A JSONL where the same question appears under multiple test-case names
- A copy-paste expansion of a single seed case across hundreds of rows

## 5. Detection mechanism

For each eval data file, hash the canonical input field per row and count collisions. Fires when collision rate exceeds 5%.

## 6. Worked example

```
warning[terrain/data-quality/duplicate-rows]: eval dataset has 23.0% duplicate inputs
  --> evals/qa.jsonl
   = docs: https://github.com/pmclSF/terrain/blob/main/docs/rules/data-quality/duplicate-rows
```

## 9. Reproducibility

```bash
terrain test --selector data-quality/duplicate-rows
```
