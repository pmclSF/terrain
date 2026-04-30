# TER-QUAL-002 — Weak Assertion

> Auto-generated stub. Edit anything below the marker; the generator preserves it.

**Type:** `weakAssertion`  
**Domain:** quality  
**Default severity:** medium  
**Status:** stable

## Summary

Tests use weak or low-density assertions, reducing defect-catching power.

## Remediation

Add behavior-focused assertions on outputs, state transitions, and side effects.

## Promotion plan

Detector is regex/density-based; AST-based semantic scoring lands in 0.3 alongside the calibration corpus.

## Evidence sources

- `structural-pattern`

## Confidence range

Detector confidence is bracketed at [0.40, 0.80] (heuristic in 0.2; calibration in 0.3).

<!-- docs-gen: end stub. Hand-authored content below this line is preserved across regenerations. -->
