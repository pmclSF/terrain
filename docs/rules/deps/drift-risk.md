# terrain/deps/drift-risk — Dependency Drift Risk

> Auto-generated stub. Edit anything below the marker; the generator preserves it.

**Type:** `depsDriftRisk`  
**Domain:** quality  
**Default severity:** medium  
**Status:** experimental

## Summary

A dependency manifest has a high share of moving-target version specs (caret, tilde, *, latest), making the repo silently susceptible to upstream regressions.

## Remediation

Pin versions or add a lockfile-verification gate. Re-audit the manifest after pinning to confirm the moving-target share drops below the threshold.

## Promotion plan

Promotes to stable once the calibration corpus confirms regression-PR lift ≥ 1.5x on deps-bump PRs.

## Evidence sources

- `structural-pattern`

## Confidence range

Confidence interval: 0.55–0.85.

<!-- docs-gen: end stub. Hand-authored content below this line is preserved across regenerations. -->
