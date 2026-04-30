# TER-AI-110 — Embedding Model Swap Without Re-Evaluation

> Auto-generated stub. Edit anything below the marker; the generator preserves it.

**Type:** `aiEmbeddingModelChange`  
**Domain:** ai  
**Default severity:** medium  
**Status:** planned

## Summary

An embedding model name or version changed in the codebase without a corresponding RAG re-evaluation in the gauntlet.

## Remediation

Run the RAG eval suite against the new embeddings; commit the updated baseline.

## Promotion plan

0.2 — git-blame detector. Promotes to stable once the embedding-aware surface scanner ships.

## Evidence sources

- `structural-pattern`
- `graph-traversal`

## Confidence range

Detector confidence is bracketed at [0.85, 0.95] (heuristic in 0.2; calibration in 0.3).

<!-- docs-gen: end stub. Hand-authored content below this line is preserved across regenerations. -->
