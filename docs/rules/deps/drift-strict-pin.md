# terrain/deps/drift-strict-pin — Dependency Drift — Strict-Pin

> Auto-generated stub. Edit anything below the marker; the generator preserves it.

**Type:** `depsDriftRisk-strict-pin`  
**Domain:** quality  
**Default severity:** medium  
**Status:** experimental

## Summary

Dependencies are declared without an explicit version anchor (bare name, `*`, `latest`, or unversioned URL). The resolver picks whatever happens to be available at install time.

## Remediation

Add an explicit version, version range, or lockfile-verification gate so installs are reproducible.

## Promotion plan

Gated by the deps_drift_risk_split mechanism; ships in shadow mode until the mechanism's regression suite clears live activation. Half of the deps-drift split.

## Evidence sources

- `structural-pattern`

## Confidence range

Detector confidence is bracketed at [0.70, 0.90] (heuristic today; calibrated against a labeled corpus over time).

<!-- docs-gen: end stub. Hand-authored content below this line is preserved across regenerations. -->
