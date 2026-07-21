# terrain/coverage/orphaned-eval — Orphaned Eval

> Auto-generated stub. Edit anything below the marker; the generator preserves it.

**Type:** `orphanedEval`  
**Domain:** ai  
**Default severity:** low  
**Lifecycle status:** experimental  
**Gating tier:** observability

## Evidence sources

- `graph-traversal`

## Confidence range

Confidence interval: 0.80–0.95.

<!-- docs-gen: end stub. Hand-authored content below this line is preserved across regenerations. -->

## 1. Summary

An Eval has no `CoveredSurfaceIDs` — it references no AI surface. The eval runs but contributes nothing to surface coverage tracking.

## 2. Severity & status

Experimental — off by default; enable in `terrain.yaml`.

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
   = docs: https://github.com/pmclSF/terrain/blob/main/docs/rules/coverage/orphaned-eval
```

## 9. Reproducibility

```bash
terrain test --selector coverage/orphaned-eval
```
