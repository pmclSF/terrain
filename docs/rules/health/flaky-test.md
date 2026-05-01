# TER-HEALTH-002 — Flaky Test

> Auto-generated stub. Edit anything below the marker; the generator preserves it.

**Type:** `flakyTest`  
**Domain:** health  
**Default severity:** medium  
**Status:** stable

## Summary

Tests exhibit inconsistent pass/fail behavior across runs.

## Remediation

Stabilize timing, shared state, and external dependency handling.

## Promotion plan

Today's detector is retry-based, not statistical failure-rate. Statistical detection lands in 0.3 with the calibration corpus.

## Evidence sources

- `runtime`

## Confidence range

Detector confidence is bracketed at [0.70, 0.85] (heuristic in 0.2; calibration in 0.3).

<!-- docs-gen: end stub. Hand-authored content below this line is preserved across regenerations. -->
