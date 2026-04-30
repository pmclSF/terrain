# TER-AI-111 — Retrieval Quality Regression

> Auto-generated stub. Edit anything below the marker; the generator preserves it.

**Type:** `aiRetrievalRegression`  
**Domain:** ai  
**Default severity:** high  
**Status:** stable

## Summary

Context relevance, nDCG, or coverage dropped versus the recorded baseline.

## Remediation

Investigate the regression; revert the offending change or re-tune retrieval before merging.

## Evidence sources

- `runtime`

## Confidence range

Detector confidence is bracketed at [0.85, 0.95] (heuristic in 0.2; calibration in 0.3).

<!-- docs-gen: end stub. Hand-authored content below this line is preserved across regenerations. -->
