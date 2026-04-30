# TER-STRUCT-007 — Capability Validation Gap

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

Capability inference is heuristic in 0.1.2; 0.2 introduces the AI taxonomy v2 with explicit capability tags so this signal can fire only on declared capabilities, eliminating false positives. Promote once precision >=0.8.

## Evidence sources

- `graph-traversal`
- `structural-pattern`

## Confidence range

Detector confidence is bracketed at [0.50, 0.80] (heuristic in 0.2; calibration in 0.3).

<!-- docs-gen: end stub. Hand-authored content below this line is preserved across regenerations. -->
