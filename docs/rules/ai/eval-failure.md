# terrain/ai/eval-failure — Eval Failure

> Auto-generated stub. Edit anything below the marker; the generator preserves it.

**Type:** `evalFailure`  
**Domain:** ai  
**Default severity:** high  
**Status:** planned

## Summary

An AI eval scenario reported a hard failure.

## Remediation

Investigate the failing case in the eval framework's report and patch the prompt or guardrail.

## Promotion plan

Planned — generic per-case failure surfacing on top of airun eval ingestion. Today's per-case failures route through the specific aiHallucinationRate / aiCostRegression / aiRetrievalRegression detectors.

## Evidence sources

- `runtime`

## Confidence range

Detector confidence is bracketed at [0.90, 1.00] (heuristic today; calibrated against a labeled corpus over time).

<!-- docs-gen: end stub. Hand-authored content below this line is preserved across regenerations. -->
