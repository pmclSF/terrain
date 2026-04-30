# TER-AI-108 — Hallucination Rate Above Threshold

> Auto-generated stub. Edit anything below the marker; the generator preserves it.

**Type:** `aiHallucinationRate`  
**Domain:** ai  
**Default severity:** high  
**Status:** planned

## Summary

An eval reports fabricated outputs at a rate above the project-configured threshold (default 5%).

## Remediation

Investigate failing scenarios; tighten retrieval or grounding before merging.

## Promotion plan

0.2 — depends on the Promptfoo / DeepEval / Ragas adapter shipping.

## Evidence sources

- `runtime`

## Confidence range

Detector confidence is bracketed at [0.80, 0.95] (heuristic in 0.2; calibration in 0.3).

<!-- docs-gen: end stub. Hand-authored content below this line is preserved across regenerations. -->
