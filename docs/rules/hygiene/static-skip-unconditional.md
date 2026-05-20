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

Preview status. Promotes to stable when calibration data confirms the unconditional / conditional-gate split preserves true positives without inflating false positives.

## Evidence sources

- `structural-pattern`

## Confidence range

Detector confidence is bracketed at [0.90, 0.95] (heuristic today; calibrated against a labeled corpus over time).

<!-- docs-gen: end stub. Hand-authored content below this line is preserved across regenerations. -->
