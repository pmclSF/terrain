# terrain/ai/surface-missing-eval — AI/ML Surface Without Eval Coverage

> Auto-generated stub. Edit anything below the marker; the generator preserves it.

**Type:** `promptFileMissingEval`  
**Domain:** ai  
**Default severity:** medium  
**Status:** experimental

## Summary

An AI/ML surface (prompt, agent, tool definition, model context, or model artifact) has no eval scenario covering it. Across 2000 OSS AI/ML repos, 136 of every 137 detected surfaces have this gap — the dominant AI-testing failure mode.

## Remediation

Add an eval scenario (promptfoo / DeepEval / Ragas / framework-specific) that exercises this surface. Use `terrain ai list` to see other uncovered surfaces in the same repo and batch-fix.

## Promotion plan

Promotes to stable once calibration data confirms regression-PR lift on prompt-eval-gap findings.

## Evidence sources

- `graph-traversal`

## Confidence range

Confidence interval: 0.55–0.85.

<!-- docs-gen: end stub. Hand-authored content below this line is preserved across regenerations. -->
