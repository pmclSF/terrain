# terrain/deps/drift-caret-policy — Dependency Drift — Caret Policy

> Auto-generated stub. Edit anything below the marker; the generator preserves it.

**Type:** `depsDriftRisk-caret-policy`  
**Domain:** quality  
**Default severity:** low  
**Status:** experimental

## Summary

Dependencies use caret-range specs (`^x.y.z`) under an ecosystem whose caret semantics make minor-version drift opaque (npm vs Poetry vs Cargo treat caret differently).

## Remediation

Adopt a stricter pinning policy (tilde, exact, or commit-pinned) where minor-version drift would silently affect runtime behavior.

## Promotion plan

Gated by the deps_drift_risk_split mechanism; ships in shadow mode until the mechanism's regression suite clears live activation. Half of the deps-drift split.

## Evidence sources

- `structural-pattern`

## Confidence range

Detector confidence is bracketed at [0.55, 0.80] (heuristic today; calibrated against a labeled corpus over time).

<!-- docs-gen: end stub. Hand-authored content below this line is preserved across regenerations. -->
