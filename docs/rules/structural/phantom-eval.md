# TER-STRUCT-002 — Phantom Eval Scenario

> Auto-generated stub. Edit anything below the marker; the generator preserves it.

**Type:** `phantomEvalScenario`  
**Domain:** ai  
**Default severity:** medium  
**Status:** experimental

## Summary

Eval scenarios claim to validate AI surfaces but have no import-graph path to those surfaces.

## Remediation

Verify the test file actually imports and exercises the target code, or correct the surface mapping.

## Promotion plan

Promote once .terrain/terrain.yaml scenario declarations are validated against the AI fixture corpus in 0.2. Today's traversal can miss surfaces declared by ID without a corresponding code path; calibration in 0.3 closes the gap.

## Evidence sources

- `graph-traversal`

## Confidence range

Detector confidence is bracketed at [0.60, 0.85] (heuristic in 0.2; calibration in 0.3).

<!-- docs-gen: end stub. Hand-authored content below this line is preserved across regenerations. -->
