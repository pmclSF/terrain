# terrain/data-quality/target-leakage — Target Leakage

> Auto-generated stub. Edit anything below the marker; the generator preserves it.

**Type:** `targetLeakage`  
**Domain:** ai  
**Default severity:** high  
**Lifecycle status:** experimental  
**Gating tier:** gate

## Evidence sources

- `structural-pattern`

## Confidence range

Confidence interval: 0.70–0.85.

<!-- docs-gen: end stub. Hand-authored content below this line is preserved across regenerations. -->

## 1. Summary

A feature column is derived from the target column, leaking the target into training and inflating reported accuracy.

## 2. Severity & status

Experimental — off by default; enable in `terrain.yaml`.

## 3. What this catches

- `X["target_lag1"] = y.shift(1)` after train/test split (lag of the target is a feature)
- `X["mean_target_by_category"] = X.groupby("category")["y"].transform("mean")` (target encoding without group-wise split)
- A derived column whose name pattern strongly suggests it's the target relabeled (e.g., `y_smoothed`, `target_encoded`)

## 5. Detection mechanism

AST walk + name-pattern matching: find DataFrame-column assignments whose RHS references the target column or whose LHS has target-shaped names. Combined with the missing-train-test-split's split detection, fires when leakage patterns appear in training code.

## 6. Worked example

```
error[terrain/data-quality/target-leakage]: feature column derived from target column
  --> training/features.py:42
   = docs: https://github.com/pmclSF/terrain/blob/main/docs/rules/data-quality/target-leakage
```

## 9. Reproducibility

```bash
terrain test --selector data-quality/target-leakage
```
