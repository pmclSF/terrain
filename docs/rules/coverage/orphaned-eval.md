# `terrain/coverage/orphaned-eval` *(preview)*

<!-- docs-gen: end stub. Hand-authored content below this line is preserved across regenerations. -->

## 1. Summary

An Eval has no `CoveredSurfaceIDs` — it references no AI surface. The eval runs but contributes nothing to surface coverage tracking.

## 2. Severity & status

- **Tier:** preview
- **Default severity:** off (opt-in)
- **Status:** preview — pending validation

## 3. What this catches

- An eval scenario configured in YAML with no `coveredSurfaceIds` field set
- An auto-derived eval whose surface inference produced no matches
- An eval moved from its original location, breaking surface attribution

## 5. Detection mechanism

Walk every Eval and check `CoveredSurfaceIDs`. Fire when empty.

## 6. Worked example

```
info[terrain/coverage/orphaned-eval]: eval "summarize_v2" references no AI surface
  --> evals/summarize_v2.yaml
   = docs: https://terrain.dev/rules/coverage/orphaned-eval
```

## 9. Reproducibility

```bash
terrain test --selector coverage/orphaned-eval
```
