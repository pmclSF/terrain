# terrain/hygiene/mock-heavy — Mock-Heavy Test

> Auto-generated stub. Edit anything below the marker; the generator preserves it.

**Type:** `mockHeavyTest`  
**Domain:** quality  
**Default severity:** low  
**Status:** experimental

## Summary

Tests rely heavily on mocks and may miss integration-level regressions.

## Remediation

Replace brittle mocks with real collaborators where practical.

## Promotion plan

Underlying hypothesis empirically refuted (corpus lift 0.02x). Framing-instability confirmed. Rebuild requires a mock-classifier distinguishing module vs callback mocks. Defer or remove.

## Evidence sources

- `structural-pattern`

## Confidence range

Detector confidence is bracketed at [0.30, 0.50] (heuristic today; calibrated against a labeled corpus over time).

<!-- docs-gen: end stub. Hand-authored content below this line is preserved across regenerations. -->
