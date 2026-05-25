# terrain/deps/drift-strict-pin — Unpinned Dependency

> Auto-generated stub. Edit anything below the marker; the generator preserves it.

**Type:** `depsDriftRisk-strict-pin`  
**Domain:** quality  
**Default severity:** medium  
**Status:** planned

## Summary

One or more dependencies are declared without an explicit version anchor (bare name, `*`, `latest`, or unversioned URL). The resolver picks whatever happens to be available at install time, so installs are not reproducible across runs.

## Remediation

Add an explicit version, version range, or lockfile-verification gate so installs are reproducible.

## Promotion plan

Preview status. One half of the dependency-drift split (the other is the caret-policy / unpinned counterpart). Promotes to stable when broader validation confirms regression-PR lift on deps-bump PRs.

## Evidence sources

- `structural-pattern`

## Confidence range

Detector confidence is bracketed at [0.70, 0.90] (heuristic today; calibrated against a labeled corpus over time).

<!-- docs-gen: end stub. Hand-authored content below this line is preserved across regenerations. -->
