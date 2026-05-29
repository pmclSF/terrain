# terrain/quality/tests-only-mocks — Tests Only Mocks

> Auto-generated stub. Edit anything below the marker; the generator preserves it.

**Type:** `testsOnlyMocks`  
**Domain:** quality  
**Default severity:** medium  
**Lifecycle status:** stable  
**Gating tier:** observability

## Summary

Test files contain mock setup but zero assertions, verifying wiring only.

## Remediation

Add assertions on outputs, state changes, or side effects to validate real behavior.

## Promotion plan

Observability tier. Rebuild target is a repo-aggregate posture metric with multi-dialect assertion counting.

## Evidence sources

- `structural-pattern`

## Confidence range

Confidence interval: 0.70–0.90.

<!-- docs-gen: end stub. Hand-authored content below this line is preserved across regenerations. -->
