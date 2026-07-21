# terrain/regression/performance-regression — Performance Regression

> Auto-generated stub. Edit anything below the marker; the generator preserves it.

**Type:** `performanceRegression`  
**Domain:** ai  
**Default severity:** high  
**Lifecycle status:** experimental  
**Gating tier:** gate

## Summary

An ML model's performance metric (accuracy / F1 / AUC / RMSE / etc.) regressed past the configured threshold from baseline to current. Same shape as eval-regression but applied to classical ML metrics rather than LLM rubric scores.

## Remediation

Inspect the diff for training data / hyperparameter / feature changes. If the regression is intentional, update the baseline.

## Evidence sources

- `eval-execution`

## Confidence range

Confidence interval: 0.90–0.99.

<!-- docs-gen: end stub. Hand-authored content below this line is preserved across regenerations. -->

## 1. Summary

An ML model's performance metric (accuracy / F1 / AUC / RMSE / etc.) regressed past the configured threshold from baseline to current. Same shape as `eval-regression` but applied to classical ML metrics rather than LLM rubric scores.

## 2. Severity & status

Experimental — off by default; enable in `terrain.yaml`. Default severity: high.

## 3. What this catches

- sklearn classifier accuracy dropped from 0.91 to 0.78 after a feature change
- XGBoost AUC regressed past 5% after hyperparameter tuning
- Time-series RMSE increased past threshold across a retraining cycle

## 4. Why this matters

Classical ML systems regress the same way LLM systems do — a feature change, a hyperparameter shift, a data refresh can move metrics. The regression rule for ML mirrors the one for LLM evals so adopters get the same CI gate semantics whether their model is GPT-4 or a sklearn random forest.

## 5. Detection mechanism

- **Approach:** identical to `regression/eval-regression`. The distinction is the rule ID — `eval-regression` is used for LLM eval scores; `performance-regression` is used for Great Expectations and custom classical-ML reporters that surface accuracy / F1 / AUC / RMSE through the eval-run shape.
- **Inputs parsed:** Great Expectations Validation Result JSON can enter the snapshot via `--great-expectations-results`.
- **Inputs:** EvalRun.Stats.PrimaryMetric and per-case Score (Score holds the chosen metric).

## 6. Worked example

```
error[terrain/regression/performance-regression]: accuracy regressed
  --> ml_results.json
   = baseline: 0.910 → current: 0.780 (delta 0.130)
   = help:     Inspect the diff for training data / hyperparameter / feature changes.
   = docs:     https://github.com/pmclSF/terrain/blob/main/docs/rules/regression/performance-regression
```

## 7. Configuration

```yaml
rules:
  regression/performance-regression:
    threshold: 0.02  # tighter
```

## 9. Reproducibility

```bash
terrain test --selector regression/performance-regression
```

## 10. Stability commitment

The rule ID is reserved and stable. Threshold behavior and configuration remain experimental.

## 11. Related rules

- `terrain/regression/eval-regression` — sibling, applied to LLM evals
- `terrain/data/leakage-suspected` — flags train/test contamination that can mask regressions
