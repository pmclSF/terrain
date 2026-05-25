# terrain/retrieval-quality/no-rerank — Retrieval Without Rerank

> Auto-generated stub. Edit anything below the marker; the generator preserves it.

**Type:** `retrievalWithoutRerank`  
**Domain:** ai  
**Default severity:** low  
**Status:** experimental

## Promotion plan

Flags retrieval pipelines with top_k > 5 and no reranker.

## Evidence sources

- `structural-pattern`

## Confidence range

Confidence interval: 0.65–0.80.

<!-- docs-gen: end stub. Hand-authored content below this line is preserved across regenerations. -->

## 1. Summary

A retrieval pipeline returns `top_k > 5` documents without a reranking step. Recall is high but precision suffers from raw vector similarity ordering.

## 2. Severity & status

- **Tier:** preview
- **Default severity:** off (opt-in)
- **Status:** preview — pending validation

## 3. What this catches

- `retriever.invoke(query, k=10)` with no `BgeReranker` / `CohereRerank` / `MMR` step downstream
- A RAG chain that pipes the top 20 chunks straight to the LLM
- An eval that scores retrieval@k without a reranker between retrieval and generation

## 5. Detection mechanism

Inspect SurfaceRetrieval entries' associated config: top_k value and presence/absence of a rerank-shaped surface (`CrossEncoderRerank`, `CohereRerank`, `BgeReranker`, MMR / MaximalMarginalRelevance) in the same pipeline.

## 6. Worked example

```
info[terrain/retrieval-quality/no-rerank]: retrieval k=20 with no reranker
  --> rag/retriever.py:34
   = docs: https://github.com/pmclSF/terrain/blob/main/docs/rules/retrieval-quality/no-rerank
```

## 9. Reproducibility

```bash
terrain test --selector retrieval-quality/no-rerank
```
