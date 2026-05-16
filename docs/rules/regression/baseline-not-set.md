# `terrain/regression/baseline-not-set`

<!-- docs-gen: end stub. Hand-authored content below this line is preserved across regenerations. -->

## 1. Summary

An EvalRun exists for the current PR but no baseline is recorded. Eval-regression detection is disabled until a baseline exists.

## 2. Severity & status

- **Tier:** stable
- **Default severity:** medium
- **Stable since:** v0.2.0

## 3. What this catches

- A repo with promptfoo eval output but no `.terrain/baselines/latest.json`
- An eval framework just wired up with no baseline yet recorded
- A deleted / missing baseline file

## 4. Why this matters

Without a baseline, the `eval-regression`, `pass-rate-drop`, and `snapshot-mismatch` rules have nothing to compare against. Eval results show up in CI but can't fail the gate regardless of value. The rule surfaces the gap so adopters lock a baseline deliberately rather than living with silent disabled regression detection.

## 5. Detection mechanism

- **Approach:** check that the current EvalRun has cases AND the baseline EvalRun is nil or empty.
- **Inputs:** EvalRun records produced by `internal/evaladapter/`.
- **Fires:** once per current run when no comparable baseline exists.

## 6. Worked example

```
warning[terrain/regression/baseline-not-set]: eval run has 12 cases but no baseline is recorded
  --> current.json
   = help: Run `terrain ai record` on the current main-branch state to lock the baseline.
   = docs: https://terrain.dev/rules/regression/baseline-not-set
```

## 7. Configuration

```yaml
rules:
  regression/baseline-not-set: low
```

## 9. Reproducibility

```bash
terrain test --selector regression/baseline-not-set
```

## 10. Stability commitment

Rule ID and severity are stable from v0.2.0.

## 11. Related rules

- `terrain/regression/eval-regression` — what fires once a baseline exists
- `terrain/coverage/missing-baseline` — sibling check at the filesystem layer
