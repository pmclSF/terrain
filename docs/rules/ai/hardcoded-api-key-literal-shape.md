# terrain/ai/hardcoded-api-key-literal-shape — Hard-Coded API Key — Literal Shape

> Auto-generated stub. Edit anything below the marker; the generator preserves it.

**Type:** `aiHardcodedAPIKey-literal-shape`  
**Domain:** ai  
**Default severity:** high  
**Status:** experimental

## Summary

An API-key-shaped string literal (e.g. AKIA-prefix, sk-prefix, ghp_-prefix) appears in an eval, prompt, or agent definition file. The structural half of the cycle-1 aiHardcodedAPIKey detector — preserved at observability tier so the literal-shape capability stays available while the secret-scanner-coverage split lands.

## Remediation

Move the secret to an environment variable or secrets store and reference it via the runner's secret-resolution path.

## Promotion plan

Promotes to stable once secret-scanner-coverage-degraded (the other half of this split) is wired into CI integration as the gate-tier counterpart.

## Evidence sources

- `structural-pattern`

## Confidence range

Detector confidence is bracketed at [0.75, 0.90] (heuristic today; calibrated against a labeled corpus over time).

<!-- docs-gen: end stub. Hand-authored content below this line is preserved across regenerations. -->
