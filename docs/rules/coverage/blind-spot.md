# TER-QUAL-006 — Coverage Blind Spot

> Auto-generated stub. Edit anything below the marker; the generator preserves it.

**Type:** `coverageBlindSpot`  
**Domain:** quality  
**Default severity:** medium  
**Status:** stable

## Summary

Code units appear unprotected or weakly protected by current coverage mix.

## Remediation

Add unit/integration tests where only broad or indirect coverage exists.

## Evidence sources

- `coverage`
- `graph-traversal`

## Confidence range

Detector confidence is bracketed at [0.50, 0.80] (heuristic in 0.2; calibration in 0.3).

<!-- docs-gen: end stub. Hand-authored content below this line is preserved across regenerations. -->
