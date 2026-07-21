# terrain/data-quality/schema-drift — Schema Drift

> Auto-generated stub. Edit anything below the marker; the generator preserves it.

**Type:** `schemaDrift`  
**Domain:** ai  
**Default severity:** high  
**Lifecycle status:** experimental  
**Gating tier:** gate

## Evidence sources

- `structural-pattern`

## Confidence range

Confidence interval: 0.85–0.95.

<!-- docs-gen: end stub. Hand-authored content below this line is preserved across regenerations. -->

## 1. Summary

A pipeline's output schema changed between baseline and current runs — column added, removed, or renamed; dtype shifted.

## 2. Severity & status

Experimental — off by default; enable in `terrain.yaml`.

## 3. What this catches

- A dbt model that renamed an output column without updating downstream tests
- A pandas pipeline that started returning `Decimal` instead of `float`
- A Great Expectations validation that drifted from baseline schema

## 5. Detection mechanism

Compare baseline EvalRun (or schema-validation artifact) against current. Recognize column-set diffs and dtype mismatches; fire when either differs.

## 6. Worked example

```
error[terrain/data-quality/schema-drift]: column "user_id" type changed from int64 to string
  --> pipelines/users.py
   = docs: https://github.com/pmclSF/terrain/blob/main/docs/rules/data-quality/schema-drift
```

## 9. Reproducibility

```bash
terrain test --selector data-quality/schema-drift
```
