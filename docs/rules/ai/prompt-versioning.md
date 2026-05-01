# TER-AI-101 — Prompt Versioning

> Auto-generated stub. Edit anything below the marker; the generator preserves it.

**Type:** `aiPromptVersioning`  
**Domain:** ai  
**Default severity:** medium  
**Status:** stable

## Summary

Prompt-kind surface ships without a recognisable version marker (filename suffix, inline `version:` field, or `# version:` comment). Future content changes will silently drift; consumers can't detect the change.

## Remediation

Add a `version:` field, a `_v<N>` filename suffix, or a `# version: ...` comment so downstream consumers can detect content drift.

## Evidence sources

- `structural-pattern`

## Confidence range

Detector confidence is bracketed at [0.75, 0.92] (heuristic in 0.2; calibration in 0.3).

<!-- docs-gen: end stub. Hand-authored content below this line is preserved across regenerations. -->
