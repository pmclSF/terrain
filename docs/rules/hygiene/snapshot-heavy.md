# terrain/hygiene/snapshot-heavy — Snapshot-Heavy Test

> Auto-generated stub. Edit anything below the marker; the generator preserves it.

**Type:** `snapshotHeavyTest`  
**Domain:** quality  
**Default severity:** low  
**Lifecycle status:** stable  
**Gating tier:** observability

## Summary

Test files over-rely on snapshot assertions, reducing defect specificity.

## Remediation

Supplement snapshots with targeted assertions on critical behavior.

## Promotion plan

Observability-tier. Gate-tier promotion requires a broader corpus and a framing-stable threshold (e.g. snap>=2 AND ratio>=0.3) plus an explicit user-facing policy declaration.

## Evidence sources

- `structural-pattern`

## Confidence range

Confidence interval: 0.50–0.75.

<!-- docs-gen: end stub. Hand-authored content below this line is preserved across regenerations. -->
