# terrain/retrieval-quality/cold-store — Cold Vector Store

> Auto-generated stub. Edit anything below the marker; the generator preserves it.

**Type:** `coldVectorStore`  
**Domain:** ai  
**Default severity:** low  
**Status:** experimental

## Promotion plan

Fires when a vector store is initialized but no index-population call exists in the same module.

## Evidence sources

- `structural-pattern`

## Confidence range

Detector confidence is bracketed at [0.70, 0.85] (heuristic today; calibrated against a labeled corpus over time).

<!-- docs-gen: end stub. Hand-authored content below this line is preserved across regenerations. -->

## 1. Summary

A vector store is initialized but no index-population call exists in the same module, suggesting the store may be queried against empty content in production.

## 2. Severity & status

- **Tier:** preview
- **Default severity:** off (opt-in)
- **Status:** preview — pending validation

## 3. What this catches

- `Chroma()` / `Pinecone.from_existing_index(...)` constructed without a preceding `add_documents` / `upsert` call
- A FAISS index built in-memory but never `.write_index()`'d
- A pgvector connection with no INSERT path visible in the surrounding code

## 5. Detection mechanism

AST walk per module: find vector-store constructor calls (Chroma, Pinecone, Weaviate, FAISS, pgvector, Qdrant, Milvus). Cross-reference for population calls (`add_documents`, `add_texts`, `upsert`, `add`, `write_index`, INSERT). Fires when init exists but population doesn't.

## 6. Worked example

```
info[terrain/retrieval-quality/cold-store]: vector store initialized with no population call
  --> rag/store.py:22
   = docs: https://github.com/pmclSF/terrain/blob/main/docs/rules/retrieval-quality/cold-store
```

## 9. Reproducibility

```bash
terrain test --selector retrieval-quality/cold-store
```
