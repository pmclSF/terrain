# terrain/deps/drift-caret-policy — Caret-Range Dependency Drift

> Auto-generated stub. Edit anything below the marker; the generator preserves it.

**Type:** `depsDriftRisk-caret-policy`  
**Domain:** quality  
**Default severity:** low  
**Status:** planned

## Summary

Dependencies use caret-range specs (`^x.y.z`) in an ecosystem where caret semantics let minor versions drift silently (npm, Poetry, and Cargo each interpret caret differently). The runtime version can change without a manifest edit.

## Remediation

Adopt a stricter pinning policy (tilde, exact, or commit-pinned) where minor-version drift would silently affect runtime behavior.

## Promotion plan

Preview status. One half of the dependency-drift split (the other is the strict-pin counterpart). Promotes to stable when broader validation confirms regression-PR lift on deps-bump PRs.

## Evidence sources

- `structural-pattern`

## Confidence range

Detector confidence is bracketed at [0.55, 0.80] (heuristic today; calibrated against a labeled corpus over time).

<!-- docs-gen: end stub. Hand-authored content below this line is preserved across regenerations. -->
