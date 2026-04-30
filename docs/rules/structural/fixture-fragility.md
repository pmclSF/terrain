# TER-STRUCT-005 — Fixture Fragility Hotspot

> Auto-generated stub. Edit anything below the marker; the generator preserves it.

**Type:** `fixtureFragilityHotspot`  
**Domain:** structure  
**Default severity:** medium  
**Status:** stable

## Summary

Fixtures depended on by many tests, where a single change cascades widely.

## Remediation

Extract smaller, focused fixtures to reduce cascading test failures.

## Evidence sources

- `graph-traversal`

## Confidence range

Detector confidence is bracketed at [0.70, 0.90] (heuristic in 0.2; calibration in 0.3).

<!-- docs-gen: end stub. Hand-authored content below this line is preserved across regenerations. -->
