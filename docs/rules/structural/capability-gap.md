# terrain/structural/capability-gap — Capability Validation Gap

> Auto-generated stub. Edit anything below the marker; the generator preserves it.

**Type:** `capabilityValidationGap`  
**Domain:** ai  
**Default severity:** medium  
**Status:** experimental

## Summary

Inferred AI capabilities have no eval scenarios validating them.

## Remediation

Add eval scenarios that exercise this capability to ensure behavioral regression detection.

## Promotion plan

Capability inference is heuristic; promote to gate-tier once the AI taxonomy supports explicit capability tags and validated precision meets the gate-tier bar.

## Evidence sources

- `graph-traversal`
- `structural-pattern`

## Confidence range

Detector confidence is bracketed at [0.50, 0.80] (heuristic today; calibrated against a labeled corpus over time).

<!-- docs-gen: end stub. Hand-authored content below this line is preserved across regenerations. -->
