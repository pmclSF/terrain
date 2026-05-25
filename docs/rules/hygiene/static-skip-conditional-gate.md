# terrain/hygiene/static-skip-conditional-gate — Conditionally-Skipped Test (informational)

> Auto-generated stub. Edit anything below the marker; the generator preserves it.

**Type:** `staticSkippedTest-conditional-gate`  
**Domain:** quality  
**Default severity:** low  
**Status:** planned

## Summary

A test is statically marked as skipped, but the skip is wrapped by an environment, feature-flag, or platform predicate. The skip is intentional and gated by code. This finding is informational — no action is required unless the gate condition itself is wrong.

## Remediation

Audit the gate condition periodically. CI should run the test on platforms or branches where the gate evaluates to false.

## Promotion plan

Preview status. Promotes to stable when broader validation confirms the conditional-gate variant preserves the intentional-skip true positives that a narrower predicate would drop.

## Evidence sources

- `structural-pattern`

## Confidence range

Detector confidence is bracketed at [0.75, 0.90] (heuristic today; calibrated against a labeled corpus over time).

<!-- docs-gen: end stub. Hand-authored content below this line is preserved across regenerations. -->
