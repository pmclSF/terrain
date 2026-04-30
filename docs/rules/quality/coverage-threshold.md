# TER-QUAL-007 — Coverage Threshold Break

> Auto-generated stub. Edit anything below the marker; the generator preserves it.

**Type:** `coverageThresholdBreak`  
**Domain:** quality  
**Default severity:** medium  
**Status:** stable

## Summary

Measured coverage falls below configured thresholds.

## Remediation

Target low-coverage, high-risk areas and raise meaningful coverage first.

## Promotion plan

Severity flips at hard 100%-gap boundary; smooth gradient lands in 0.3 per docs/scoring-rubric.md.

## Evidence sources

- `coverage`

## Confidence range

Detector confidence is bracketed at [0.90, 0.99] (heuristic in 0.2; calibration in 0.3).

<!-- docs-gen: end stub. Hand-authored content below this line is preserved across regenerations. -->
