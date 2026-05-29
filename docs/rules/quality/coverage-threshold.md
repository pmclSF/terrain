# terrain/quality/coverage-threshold — Coverage Threshold Break

> Auto-generated stub. Edit anything below the marker; the generator preserves it.

**Type:** `coverageThresholdBreak`  
**Domain:** quality  
**Default severity:** medium  
**Lifecycle status:** stable  
**Gating tier:** gate

## Summary

Measured coverage falls below configured thresholds.

## Remediation

Target low-coverage, high-risk areas and raise meaningful coverage first.

## Promotion plan

Severity flips at hard 100%-gap boundary; a smooth gradient lands in a future release per docs/scoring-rubric.md.

## Evidence sources

- `coverage`

## Confidence range

Confidence interval: 0.90–0.99.

<!-- docs-gen: end stub. Hand-authored content below this line is preserved across regenerations. -->
