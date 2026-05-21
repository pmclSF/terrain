# terrain/migration/deprecated-pattern — Deprecated Test Pattern

> Auto-generated stub. Edit anything below the marker; the generator preserves it.

**Type:** `deprecatedTestPattern`  
**Domain:** migration  
**Default severity:** low  
**Status:** stable

## Summary

Deprecated test patterns increase migration and maintenance risk.

## Remediation

Replace deprecated APIs with supported alternatives.

## Promotion plan

Observability-tier. Drop the enzyme sub-rule, refresh the trigger set, and add a scope gate distinguishing jest.setTimeout from bare setTimeout in a test body.

## Evidence sources

- `structural-pattern`

## Confidence range

Detector confidence is bracketed at [0.70, 0.90] (heuristic today; calibrated against a labeled corpus over time).

<!-- docs-gen: end stub. Hand-authored content below this line is preserved across regenerations. -->
