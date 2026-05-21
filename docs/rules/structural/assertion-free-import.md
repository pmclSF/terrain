# terrain/structural/assertion-free-import — Assertion-Free Import

> Auto-generated stub. Edit anything below the marker; the generator preserves it.

**Type:** `assertionFreeImport`  
**Domain:** structure  
**Default severity:** medium  
**Status:** stable

## Summary

Test files import production code but contain zero assertions — exercising code without verifying behavior.

## Remediation

Add assertions to validate behavior or remove tests that verify nothing.

## Promotion plan

Observability-tier. Gate-tier requires a multi-dialect assertion oracle, a path-role test gate, cross-file inherited-assertion resolution, and framing-stable evaluation.

## Evidence sources

- `graph-traversal`
- `structural-pattern`

## Confidence range

Detector confidence is bracketed at [0.80, 0.95] (heuristic today; calibrated against a labeled corpus over time).

<!-- docs-gen: end stub. Hand-authored content below this line is preserved across regenerations. -->
