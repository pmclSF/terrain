# terrain/hygiene/no-assertions — Assertion-Free Test

> Auto-generated stub. Edit anything below the marker; the generator preserves it.

**Type:** `assertionFreeTest`  
**Domain:** quality  
**Default severity:** medium  
**Status:** stable

## Summary

Test files contain test function signatures but no detectable assertions.

## Remediation

Add assertions to validate behavior — tests without assertions verify nothing.

## Promotion plan

Observability-tier. Gate-tier requires a multi-dialect assertion oracle, a path-role gate that excludes conftest/fixtures/commented-out tests, and framing-stable evaluation.

## Evidence sources

- `structural-pattern`

## Confidence range

Confidence interval: 0.75–0.90.

<!-- docs-gen: end stub. Hand-authored content below this line is preserved across regenerations. -->
