# TER-AI-108 — Eval-Flagged Hallucination Share

> Auto-generated stub. Edit anything below the marker; the generator preserves it.

**Type:** `aiHallucinationRate`  
**Domain:** ai  
**Default severity:** high  
**Status:** stable

## Summary

The eval framework's own hallucination metadata reports a share of cases above the project-configured threshold (default 5%). Terrain reads this from the framework output (Promptfoo / DeepEval / Ragas) — Terrain does not judge hallucinations directly.

## Remediation

Investigate the underlying eval-flagged cases; tighten retrieval or grounding before merging. If you disagree with the eval framework's classification, fix the eval scenario or raise the threshold (with a documented justification).

## Evidence sources

- `runtime`

## Confidence range

Detector confidence is bracketed at [0.80, 0.95] (heuristic in 0.2; calibration in 0.3).

<!-- docs-gen: end stub. Hand-authored content below this line is preserved across regenerations. -->
