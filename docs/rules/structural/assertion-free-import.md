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

Gate-tier requires: (1) A1 multi-dialect assertion oracle + path-role test gate, (2) cross-file inherited-assertion resolution, (3) framing test flip <15%.

## Evidence sources

- `graph-traversal`
- `structural-pattern`

## Confidence range

Detector confidence is bracketed at [0.80, 0.95] (heuristic today; calibrated against a labeled corpus over time).

<!-- docs-gen: end stub. Hand-authored content below this line is preserved across regenerations. -->
