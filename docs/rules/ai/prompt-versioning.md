# terrain/ai/prompt-versioning — Prompt Versioning

> Auto-generated stub. Edit anything below the marker; the generator preserves it.

**Type:** `aiPromptVersioning`  
**Domain:** ai  
**Default severity:** medium  
**Status:** stable

## Summary

Prompt-kind surface ships without a recognizable version marker (filename suffix, inline `version:` field, or `# version:` comment). Future content changes will silently drift; consumers can't detect the change.

## Remediation

Add a `version:` field, a `_v<N>` filename suffix, or a `# version: ...` comment so downstream consumers can detect content drift.

## Promotion plan

Stays at observability tier until a labeled-sample baseline confirms the detector's precision. Any subsequent precision lift must be structurally motivated; the rule stays at observability otherwise.

## Evidence sources

- `structural-pattern`

## Confidence range

Confidence interval: 0.75–0.92.

<!-- docs-gen: end stub. Hand-authored content below this line is preserved across regenerations. -->
