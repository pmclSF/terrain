# terrain/ai/hardcoded-api-key-literal-shape — Hard-Coded API Key in Source

> Auto-generated stub. Edit anything below the marker; the generator preserves it.

**Type:** `aiHardcodedAPIKey-literal-shape`  
**Domain:** ai  
**Default severity:** high  
**Status:** planned

## Summary

An API-key-shaped string appears verbatim in an eval, prompt, or agent definition file. Pairs with the CI-coverage counterpart, secretScannerCoverageDegraded.

## Remediation

Move the secret to an environment variable or secrets store and reference it via the runner's secret-resolution path.

## Promotion plan

Planned. Reserved signal type — the literal-shape half of the API-key split; detector not yet wired.

## Evidence sources

- `structural-pattern`

## Confidence range

Detector confidence is bracketed at [0.75, 0.90] (heuristic today; calibrated against a labeled corpus over time).

<!-- docs-gen: end stub. Hand-authored content below this line is preserved across regenerations. -->
