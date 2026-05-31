# terrain/structural/uncovered-ai-surface — Uncovered AI Surface

> Auto-generated stub. Edit anything below the marker; the generator preserves it.

**Type:** `uncoveredAISurface`  
**Domain:** ai  
**Default severity:** high  
**Lifecycle status:** experimental  
**Gating tier:** observability

## Summary

AI surfaces (prompts, tools, datasets) have zero test or scenario coverage.

## Remediation

Add eval scenarios that exercise this AI surface — untested prompts and tools can change behavior silently.

## Promotion plan

Coverage attribution depends on .terrain/terrain.yaml scenario declarations; precision/recall calibrated in 0.2 against the AI fixture corpus.

## Evidence sources

- `graph-traversal`
- `structural-pattern`

## Confidence range

Confidence interval: 0.70–0.90.

<!-- docs-gen: end stub. Hand-authored content below this line is preserved across regenerations. -->
