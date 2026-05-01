# TER-AI-001 — Eval Failure

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

Detector lands in 0.2 with eval-framework metric ingestion.

## Evidence sources

- `runtime`

## Confidence range

Detector confidence is bracketed at [0.90, 1.00] (heuristic in 0.2; calibration in 0.3).

<!-- docs-gen: end stub. Hand-authored content below this line is preserved across regenerations. -->
