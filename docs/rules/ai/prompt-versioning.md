# TER-AI-101 — Prompt Changed Without Version Bump

> Auto-generated stub. Edit anything below the marker; the generator preserves it.

**Type:** `aiPromptVersioning`  
**Domain:** ai  
**Default severity:** medium  
**Status:** planned

## Summary

Prompt content changed between commits without a corresponding version field bump or filename version suffix.

## Remediation

Bump the prompt's version metadata or rename the file with a new version suffix before merging.

## Promotion plan

0.2 — git-blame-driven detector lands with the prompt-surface expansion.

## Evidence sources

- `structural-pattern`
- `graph-traversal`

## Confidence range

Detector confidence is bracketed at [0.85, 0.95] (heuristic in 0.2; calibration in 0.3).

<!-- docs-gen: end stub. Hand-authored content below this line is preserved across regenerations. -->
