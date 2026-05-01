# TER-STRUCT-006 — Assertion-Free Import

> Auto-generated stub. Edit anything below the marker; the generator preserves it.

**Type:** `assertionFreeImport`  
**Domain:** structure  
**Default severity:** high  
**Status:** stable

## Summary

Test files import production code but contain zero assertions — exercising code without verifying behavior.

## Remediation

Add assertions to validate behavior or remove tests that verify nothing.

## Evidence sources

- `graph-traversal`
- `structural-pattern`

## Confidence range

Detector confidence is bracketed at [0.80, 0.95] (heuristic in 0.2; calibration in 0.3).

<!-- docs-gen: end stub. Hand-authored content below this line is preserved across regenerations. -->
