# terrain/regression/pass-rate-drop — Eval Pass Rate Dropped

> Auto-generated stub. Edit anything below the marker; the generator preserves it.

**Type:** `passRateDrop`  
**Domain:** ai  
**Default severity:** high  
**Lifecycle status:** stable  
**Gating tier:** gate

## Summary

The success / total ratio across eval cases dropped past the configured threshold from baseline to current. Distinct from eval-regression (continuous score deltas) — fires on discrete pass/fail count deltas.

## Remediation

Inspect per-case eval-regression findings for cases that flipped from pass to fail. If intentional, update the baseline.

## Evidence sources

- `eval-execution`

## Confidence range

Confidence interval: 0.90–0.99.

<!-- docs-gen: end stub. Hand-authored content below this line is preserved across regenerations. -->

## 1. Summary

The success/total ratio across eval cases dropped from baseline to current past the configured threshold. Distinct from `eval-regression` (continuous score deltas) — this fires on discrete pass/fail count shifts.

## 2. Severity & status

- **Tier:** stable
- **Default severity:** high (critical when drop ≥ 25%)
- **Stable since:** v0.2.0

## 3. What this catches

- Baseline 10/10 → current 7/10 (3 cases flipped to failing)
- A multi-sample eval where pass count dropped from 9/10 to 5/10 between runs
- Eval suite stability degraded by a prompt edit even though average score didn't shift much

## 4. Why this matters

A run can have unchanged average scores but a different pass rate if the model's outputs became less consistent or if specific assertions started failing. Scoring averages mask binary-outcome regressions; pass rate surfaces them.

## 5. Detection mechanism

- **Approach:** compute `successes / total` for baseline and current. Fire when `baseline_pass_rate - current_pass_rate >= threshold` (default 0.05).
- **Severity ladder:** high for [threshold, 0.25); critical at ≥0.25 drop.
- **Inputs:** EvalRun.Stats from any of the four eval adapters.

## 6. Worked example

```
error[terrain/regression/pass-rate-drop]: eval pass rate regressed
  --> current.json
   = baseline: 10/10 (100.0%) → current: 7/10 (70.0%); delta 30.0%
   = help:     Inspect per-case eval-regression findings for the cases that flipped from pass to fail.
   = docs:     https://github.com/pmclSF/terrain/blob/main/docs/rules/regression/pass-rate-drop
```

## 7. Configuration

```yaml
rules:
  regression/pass-rate-drop:
    threshold: 0.10  # looser
```

## 9. Reproducibility

```bash
terrain test --selector regression/pass-rate-drop
```

## 10. Stability commitment

Default threshold (0.05) and severity ladder are stable from v0.2.0.

## 11. Related rules

- `terrain/regression/eval-regression` — score-delta sibling
- `terrain/regression/snapshot-mismatch` — output-shape sibling
