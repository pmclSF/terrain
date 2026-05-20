# terrain/coverage/missing-baseline — Missing Coverage Baseline

> Auto-generated stub. Edit anything below the marker; the generator preserves it.

**Type:** `missingBaseline`  
**Domain:** ai  
**Default severity:** medium  
**Status:** stable

## Summary

The repository has eval surfaces but no `.terrain/baselines/` directory exists. Eval regression detection is disabled at the coverage layer.

## Remediation

Run `terrain ai record` to create the baseline directory. Commit `.terrain/baselines/latest.json` so subsequent PRs compare against it.

## Evidence sources

- `static`

## Confidence range

Detector confidence is bracketed at [1.00, 1.00] (heuristic today; calibrated against a labeled corpus over time).

<!-- docs-gen: end stub. Hand-authored content below this line is preserved across regenerations. -->

## 1. Summary

The repository has eval surfaces (prompts / models / retrievers) but no `.terrain/baselines/` directory exists. Eval regression detection is disabled at the coverage layer.

## 2. Severity & status

- **Tier:** stable
- **Default severity:** medium
- **Stable since:** v0.2.0

## 3. What this catches

- A repo that wires up promptfoo + Terrain but never calls `terrain ai record`
- A `.gitignore` rule that excludes `.terrain/baselines/` (so PRs never see the baseline)
- A first-time setup of Terrain on a repo with existing AI surfaces

## 4. Why this matters

The structural twin of `regression/baseline-not-set`: surfaces that should have baselines don't. This is the gate-level check that fires before any eval runs; `regression/baseline-not-set` fires after the eval runs and finds no comparator.

## 5. Detection mechanism

- **Approach:** filesystem check — does `.terrain/baselines/` exist with at least one file? — combined with surface count > 0.
- **Inputs:** repo root path + TestSuiteSnapshot.CodeSurfaces.

## 6. Worked example

```
warning[terrain/coverage/missing-baseline]: AI surfaces detected but .terrain/baselines/ is empty
  --> .
   = help: Run `terrain ai record` to create the baseline directory. Commit the result.
   = docs: https://github.com/pmclSF/terrain/blob/main/docs/rules/coverage/missing-baseline
```

## 7. Configuration

```yaml
rules:
  coverage/missing-baseline: low
```

## 9. Reproducibility

```bash
terrain test --selector coverage/missing-baseline
```

## 10. Stability commitment

Rule ID and severity stable from v0.2.0.

## 11. Related rules

- `terrain/regression/baseline-not-set` — the runtime sibling
- `terrain/coverage/no-eval` — adjacent: surfaces without any eval at all
