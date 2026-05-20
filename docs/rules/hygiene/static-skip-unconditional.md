# terrain/hygiene/static-skip-unconditional — Static Skipped Test — Unconditional

> Auto-generated stub. Edit anything below the marker; the generator preserves it.

**Type:** `staticSkippedTest-unconditional`  
**Domain:** quality  
**Default severity:** low  
**Status:** experimental

## Summary

A test is statically marked as skipped without any surrounding environment / feature-flag gate. The skip is permanent until the marker is removed.

## Remediation

Re-enable, replace, or delete the test. Add a comment explaining why if the skip should persist.

## Promotion plan

Ships behind the static_skipped_test_split mechanism in shadow mode. Promotes to stable when the split's per-mechanism recall + frozen-suite gates clear.

## Evidence sources

- `structural-pattern`

## Confidence range

Detector confidence is bracketed at [0.90, 0.95] (heuristic today; calibrated against a labeled corpus over time).

<!-- docs-gen: end stub. Hand-authored content below this line is preserved across regenerations. -->
