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

Gate-tier requires: (1) regex-floor lift to 70%+ precision, (2) A3 path-role gate to exclude conftest/fixtures/commented-out, (3) framing test flip <15%.

## Evidence sources

- `structural-pattern`

## Confidence range

Detector confidence is bracketed at [0.75, 0.90] (heuristic today; calibrated against a labeled corpus over time).

<!-- docs-gen: end stub. Hand-authored content below this line is preserved across regenerations. -->
