# terrain/ai/prompt-versioning — Prompt Versioning

> Auto-generated stub. Edit anything below the marker; the generator preserves it.

**Type:** `aiPromptVersioning`  
**Domain:** ai  
**Default severity:** medium  
**Status:** stable

## Summary

Prompt-kind surface ships without a recognisable version marker (filename suffix, inline `version:` field, or `# version:` comment). Future content changes will silently drift; consumers can't detect the change.

## Remediation

Add a `version:` field, a `_v<N>` filename suffix, or a `# version: ...` comment so downstream consumers can detect content drift.

## Promotion plan

Measurement-phase per P2.13: stays at observability tier until baseline n≥150 stratified sample completes. Any lift mechanism from that sample must be structurally motivated or defer to n≥500.

## Evidence sources

- `structural-pattern`

## Confidence range

Detector confidence is bracketed at [0.75, 0.92] (heuristic today; calibrated against a labeled corpus over time).

<!-- docs-gen: end stub. Hand-authored content below this line is preserved across regenerations. -->
