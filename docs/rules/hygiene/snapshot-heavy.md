# terrain/hygiene/snapshot-heavy — Snapshot-Heavy Test

> Auto-generated stub. Edit anything below the marker; the generator preserves it.

**Type:** `snapshotHeavyTest`  
**Domain:** quality  
**Default severity:** low  
**Status:** stable

## Summary

Test files over-rely on snapshot assertions, reducing defect specificity.

## Remediation

Supplement snapshots with targeted assertions on critical behavior.

## Promotion plan

Gate-tier promotion gated on: (1) n=200+ from ≥40 unique repos; (2) framing flip <15% under threshold-conjunction (snap≥2 AND ratio≥0.3); (3) explicit user-facing policy declaration.

## Evidence sources

- `structural-pattern`

## Confidence range

Detector confidence is bracketed at [0.50, 0.75] (heuristic today; calibrated against a labeled corpus over time).

<!-- docs-gen: end stub. Hand-authored content below this line is preserved across regenerations. -->
