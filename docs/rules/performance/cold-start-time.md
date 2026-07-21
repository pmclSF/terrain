# terrain/performance/cold-start-time — Cold Start Time

> Auto-generated stub. Edit anything below the marker; the generator preserves it.

**Type:** `coldStartTime`  
**Domain:** ai  
**Default severity:** low  
**Lifecycle status:** experimental  
**Gating tier:** observability

## Evidence sources

- `runtime`

## Confidence range

Confidence interval: 0.80–0.95.

<!-- docs-gen: end stub. Hand-authored content below this line is preserved across regenerations. -->

## 1. Summary

The first request's latency in a fresh process exceeds the configured threshold (typically 2× P50), indicating cold-start cost that affects production.

## 2. Severity & status

Experimental — off by default; enable in `terrain.yaml`.

## 3. What this catches

- A model whose first inference call loads weights and takes 10× the warm latency
- A retrieval pipeline that builds the vector index on first call
- A FastAPI handler that imports torch lazily on first request

## 5. Detection mechanism

Compare first-call latency vs P50 of subsequent calls from runtime telemetry. Fire when ratio exceeds threshold (default 2.0).

## 6. Worked example

```
info[terrain/performance/cold-start-time]: cold-start latency 8.2× warm P50
  --> runtime/inference.py
   = docs: https://github.com/pmclSF/terrain/blob/main/docs/rules/performance/cold-start-time
```

## 9. Reproducibility

```bash
terrain test --selector performance/cold-start-time
```
