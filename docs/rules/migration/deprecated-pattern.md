# terrain/migration/deprecated-pattern — Deprecated Test Pattern

> Auto-generated stub. Edit anything below the marker; the generator preserves it.

**Type:** `deprecatedTestPattern`  
**Domain:** migration  
**Default severity:** low  
**Lifecycle status:** stable  
**Gating tier:** observability

## Summary

Deprecated test patterns increase migration and maintenance risk.

## Remediation

Replace deprecated APIs with supported alternatives.

## Promotion plan

Observability-tier. Drop the enzyme sub-rule, refresh the trigger set, and add a scope gate distinguishing jest.setTimeout from bare setTimeout in a test body.

## Evidence sources

- `structural-pattern`

## Confidence range

Confidence interval: 0.70–0.90.

<!-- docs-gen: end stub. Hand-authored content below this line is preserved across regenerations. -->
