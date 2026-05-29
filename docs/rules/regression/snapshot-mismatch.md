# terrain/regression/snapshot-mismatch — Snapshot Mismatch

> Auto-generated stub. Edit anything below the marker; the generator preserves it.

**Type:** `snapshotMismatch`  
**Domain:** ai  
**Default severity:** medium  
**Lifecycle status:** stable  
**Gating tier:** observability

## Summary

An eval case's recorded output snapshot diverged from baseline to current. Catches behavior changes the scalar score may not surface.

## Remediation

Inspect the diff for prompt / model / retrieval changes affecting the case. If the new output is correct, accept it via `terrain ai record`.

## Evidence sources

- `eval-execution`

## Confidence range

Confidence interval: 0.85–0.95.

<!-- docs-gen: end stub. Hand-authored content below this line is preserved across regenerations. -->

## 1. Summary

An eval case's recorded output snapshot (or framework failure reason) diverged from baseline to current. Catches behavior changes that scalar scores may not surface.

## 2. Severity & status

- **Tier:** stable
- **Default severity:** medium
- **Stable since:** v0.2.0

## 3. What this catches

- Model output text changed but the score is unchanged (different phrasing, same rubric grade)
- Framework's `gradingResult.reason` shifted between runs (different failure pattern)
- An eval that used to refuse harmful input now refuses with a different message

## 4. Why this matters

Scores can mask behavior changes: two outputs scoring 0.9 may behave differently in production. Snapshot comparison catches drift the metric misses.

## 5. Detection mechanism

- **Approach:** for each case matched by ID across baseline and current, compare `Reason` fields. When they diverge (and aren't both empty), fire.
- **Inputs:** EvalRun.Cases.Reason populated by adapters from `gradingResult.reason` / `failure_reason` / equivalent.
- **0.2.0 scope:** comparison uses the Reason proxy. Richer per-case output capture (full response text snapshots) is followup work that requires snapshot files alongside the eval results JSON.

## 6. Worked example

```
warning[terrain/regression/snapshot-mismatch]: case "safety" snapshot diverged
  --> current.json
   = baseline reason: "refused successfully"
   = current reason:  "responded instead of refusing"
   = help:            If the new output is correct, accept it via `terrain ai record`.
   = docs:            https://github.com/pmclSF/terrain/blob/main/docs/rules/regression/snapshot-mismatch
```

## 7. Configuration

```yaml
rules:
  regression/snapshot-mismatch: low
```

## 9. Reproducibility

```bash
terrain test --selector regression/snapshot-mismatch
```

## 10. Stability commitment

Rule ID and severity stable from v0.2.0. Richer snapshot comparison (full output text vs. just reason field) is additive.

## 11. Related rules

- `terrain/regression/eval-regression` — score-delta sibling
- `terrain/regression/pass-rate-drop` — discrete-outcome sibling
