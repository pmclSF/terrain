# terrain/ai/embedding-model-change — Embedding Model Swap Without Re-Evaluation

> Auto-generated stub. Edit anything below the marker; the generator preserves it.

**Type:** `aiEmbeddingModelChange`  
**Domain:** ai  
**Default severity:** medium  
**Status:** stable

## Summary

A repository references an embedding model in source code without a retrieval-shaped eval scenario, so a future model swap will silently change retrieval quality.

## Remediation

Add a retrieval eval scenario (Ragas, Promptfoo, or DeepEval) that exercises this surface so embedding swaps surface as a measurable regression.

## Promotion plan

Ships the static precondition (embedding referenced + no retrieval coverage) today. The cross-snapshot content-hash diff variant lands once snapshot fingerprints are recorded.

## Evidence sources

- `structural-pattern`

## Confidence range

Confidence interval: 0.70–0.88.

<!-- docs-gen: end stub. Hand-authored content below this line is preserved across regenerations. -->
