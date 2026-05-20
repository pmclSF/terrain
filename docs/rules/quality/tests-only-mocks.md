# terrain/quality/tests-only-mocks — Tests Only Mocks

> Auto-generated stub. Edit anything below the marker; the generator preserves it.

**Type:** `testsOnlyMocks`  
**Domain:** quality  
**Default severity:** medium  
**Status:** stable

## Summary

Test files contain mock setup but zero assertions, verifying wiring only.

## Remediation

Add assertions on outputs, state changes, or side effects to validate real behavior.

## Promotion plan

Rebuild as repo-aggregate posture metric. Per-file detection blocked by assertion-counter blindness (caught only bare `assert`); A1 multi-dialect oracle would lift to ~55-70% but TP base rate is fundamental ceiling.

## Evidence sources

- `structural-pattern`

## Confidence range

Detector confidence is bracketed at [0.70, 0.90] (heuristic today; calibrated against a labeled corpus over time).

<!-- docs-gen: end stub. Hand-authored content below this line is preserved across regenerations. -->
