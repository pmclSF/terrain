# terrain/ai/cost-regression — Prompt Token-Cost Regression

> Auto-generated stub. Edit anything below the marker; the generator preserves it.

**Type:** `aiCostRegression`  
**Domain:** ai  
**Default severity:** medium  
**Status:** stable

## Summary

A prompt change increases the token count by more than 25% versus the recorded baseline.

## Remediation

Investigate the change for unintended bloat; bump the baseline if the increase is intentional.

## Evidence sources

- `runtime`

## Confidence range

Detector confidence is bracketed at [0.85, 0.95] (heuristic today; calibrated against a labeled corpus over time).

<!-- docs-gen: end stub. Hand-authored content below this line is preserved across regenerations. -->
