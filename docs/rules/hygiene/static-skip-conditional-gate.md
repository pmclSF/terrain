# terrain/hygiene/static-skip-conditional-gate — Static Skipped Test — Conditional Gate

> Auto-generated stub. Edit anything below the marker; the generator preserves it.

**Type:** `staticSkippedTest-conditional-gate`  
**Domain:** quality  
**Default severity:** low  
**Status:** experimental

## Summary

A test is statically marked as skipped, but the skip is wrapped by an environment, feature-flag, or platform predicate. The skip is intentional and gated.

## Remediation

No remediation required when the gate is correct. Audit the gate periodically; CI should run the test on platforms where the gate is false.

## Promotion plan

Ships behind the static_skipped_test_split mechanism in shadow mode. Preserves the 39% of staticSkippedTest TPs that an A3 narrowing would otherwise drop.

## Evidence sources

- `structural-pattern`

## Confidence range

Detector confidence is bracketed at [0.75, 0.90] (heuristic today; calibrated against a labeled corpus over time).

<!-- docs-gen: end stub. Hand-authored content below this line is preserved across regenerations. -->
