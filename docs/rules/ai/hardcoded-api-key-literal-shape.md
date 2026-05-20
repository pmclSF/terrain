# terrain/ai/hardcoded-api-key-literal-shape — Hard-Coded API Key — Literal Shape

> Auto-generated stub. Edit anything below the marker; the generator preserves it.

**Type:** `aiHardcodedAPIKey-literal-shape`  
**Domain:** ai  
**Default severity:** high  
**Status:** experimental

## Summary

An API-key-shaped string literal (e.g. AKIA-prefix, sk-prefix, ghp_-prefix) appears in an eval, prompt, or agent definition file. Detects the literal in source; pairs with secretScannerCoverageDegraded which flags the absence of a CI-side secret scanner.

## Remediation

Move the secret to an environment variable or secrets store and reference it via the runner's secret-resolution path.

## Promotion plan

Promotes to stable once secretScannerCoverageDegraded (the CI-coverage counterpart) ships at gate tier.

## Evidence sources

- `structural-pattern`

## Confidence range

Detector confidence is bracketed at [0.75, 0.90] (heuristic today; calibrated against a labeled corpus over time).

<!-- docs-gen: end stub. Hand-authored content below this line is preserved across regenerations. -->
