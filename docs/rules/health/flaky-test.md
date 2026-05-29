# terrain/health/flaky-test — Flaky Test

> Auto-generated stub. Edit anything below the marker; the generator preserves it.

**Type:** `flakyTest`  
**Domain:** health  
**Default severity:** medium  
**Lifecycle status:** stable  
**Gating tier:** observability

## Summary

Tests exhibit inconsistent pass/fail behavior across runs.

## Remediation

Stabilize timing, shared state, and external dependency handling.

## Promotion plan

Today's detector is retry-based, not statistical failure-rate. Statistical detection lands in a future release.

## Evidence sources

- `runtime`

## Confidence range

Confidence interval: 0.70–0.85.

<!-- docs-gen: end stub. Hand-authored content below this line is preserved across regenerations. -->
