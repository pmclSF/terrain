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

Drop enzyme sub-rule; refresh trigger set; setTimeout sub-rule needs A3 scope gate (jest.setTimeout config vs bare setTimeout in test body). Path-role gate to exclude fuzzer/fixture/comment-only matches.

## Evidence sources

- `structural-pattern`

## Confidence range

Detector confidence is bracketed at [0.70, 0.90] (heuristic today; calibrated against a labeled corpus over time).

<!-- docs-gen: end stub. Hand-authored content below this line is preserved across regenerations. -->
