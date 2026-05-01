# TER-STRUCT-001 — Uncovered AI Surface

> Auto-generated stub. Edit anything below the marker; the generator preserves it.

**Type:** `uncoveredAISurface`  
**Domain:** ai  
**Default severity:** high  
**Status:** experimental

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

Detector confidence is bracketed at [0.70, 0.90] (heuristic in 0.2; calibration in 0.3).

<!-- docs-gen: end stub. Hand-authored content below this line is preserved across regenerations. -->
