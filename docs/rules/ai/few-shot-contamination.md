# TER-AI-109 — Few-Shot Contamination

> Auto-generated stub. Edit anything below the marker; the generator preserves it.

**Type:** `aiFewShotContamination`  
**Domain:** ai  
**Default severity:** medium  
**Status:** planned

## Summary

Few-shot examples in a prompt overlap with the eval test set, inflating reported scores.

## Remediation

De-duplicate or hold out the contaminated examples; re-run the eval.

## Promotion plan

0.2 — string-similarity detector across the prompt-surface inventory.

## Evidence sources

- `graph-traversal`

## Confidence range

Detector confidence is bracketed at [0.70, 0.90] (heuristic in 0.2; calibration in 0.3).

<!-- docs-gen: end stub. Hand-authored content below this line is preserved across regenerations. -->
