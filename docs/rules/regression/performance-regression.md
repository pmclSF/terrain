# `terrain/regression/performance-regression`

<!-- docs-gen: end stub. Hand-authored content below this line is preserved across regenerations. -->

## 1. Summary

An ML model's performance metric (accuracy / F1 / AUC / RMSE / etc.) regressed past the configured threshold from baseline to current. Same shape as `eval-regression` but applied to classical ML metrics rather than LLM rubric scores.

## 2. Severity & status

- **Tier:** stable
- **Default severity:** high (critical at ≥25% drop)
- **Stable since:** v0.2.0

## 3. What this catches

- sklearn classifier accuracy dropped from 0.91 to 0.78 after a feature change
- XGBoost AUC regressed past 5% after hyperparameter tuning
- Time-series RMSE increased past threshold across a retraining cycle

## 4. Why this matters

Classical ML systems regress the same way LLM systems do — a feature change, a hyperparameter shift, a data refresh can move metrics. The regression rule for ML mirrors the one for LLM evals so adopters get the same CI gate semantics whether their model is GPT-4 or a sklearn random forest.

## 5. Detection mechanism

- **Approach:** identical to `regression/eval-regression`. The distinction is the rule ID — `eval-regression` is used by promptfoo / deepeval / ragas (LLM evals); `performance-regression` is used by Great Expectations and custom classical-ML reporters that surface accuracy / F1 / AUC / RMSE through the EvalRun shape.
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

Default threshold and severity ladder stable from v0.2.0.

## 11. Related rules

- `terrain/regression/eval-regression` — sibling, applied to LLM evals
- `terrain/data/leakage-suspected` — flags train/test contamination that can mask regressions
