# terrain/ai/secret-scanner-coverage-degraded — No Secret Scanner in CI

> Auto-generated stub. Edit anything below the marker; the generator preserves it.

**Type:** `secretScannerCoverageDegraded`  
**Domain:** ai  
**Default severity:** medium  
**Status:** planned

## Summary

The repository configures or references AI surfaces that should be guarded by a secret scanner, but no secret-scanner CI integration (GitGuardian, GitHub secret scanning, gitleaks, trufflehog) is enabled. CI-coverage counterpart to aiHardcodedAPIKey-literal-shape.

## Remediation

Enable a secret scanner in CI and document its coverage in the project README. Re-audit periodically.

## Promotion plan

Planned. Reserved signal type for the CI-integration gap that pairs with the in-repo key-shape detector.

## Evidence sources

- `structural-pattern`

## Confidence range

Detector confidence is bracketed at [0.60, 0.85] (heuristic today; calibrated against a labeled corpus over time).

<!-- docs-gen: end stub. Hand-authored content below this line is preserved across regenerations. -->
