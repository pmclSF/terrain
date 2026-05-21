# terrain/hygiene/weak-assertion — Weak Assertion

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

Observability-tier. Gate-tier promotion requires an explicit user-declared policy threshold and framing-stable evaluation.

## Evidence sources

- `structural-pattern`

## Confidence range

Detector confidence is bracketed at [0.40, 0.80] (heuristic today; calibrated against a labeled corpus over time).

<!-- docs-gen: end stub. Hand-authored content below this line is preserved across regenerations. -->
