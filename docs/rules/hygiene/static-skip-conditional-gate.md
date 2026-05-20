# terrain/hygiene/static-skip-conditional-gate — Conditionally-Skipped Test (informational)

> Auto-generated stub. Edit anything below the marker; the generator preserves it.

**Type:** `staticSkippedTest-conditional-gate`  
**Domain:** quality  
**Default severity:** low  
**Status:** experimental

## Summary

A test is statically marked as skipped, but the skip is wrapped by an environment, feature-flag, or platform predicate. The skip is intentional and gated by code. This finding is informational — no action is required unless the gate condition itself is wrong.

## Remediation

Audit the gate condition periodically. CI should run the test on platforms or branches where the gate evaluates to false.

## Promotion plan

Gated by the static_skipped_test_split mechanism; ships in shadow mode until the mechanism's regression suite clears live activation.

## Evidence sources

- `structural-pattern`

## Confidence range

Detector confidence is bracketed at [0.75, 0.90] (heuristic today; calibrated against a labeled corpus over time).

<!-- docs-gen: end stub. Hand-authored content below this line is preserved across regenerations. -->
