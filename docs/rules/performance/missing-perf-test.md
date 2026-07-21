# terrain/performance/missing-perf-test — Missing Performance Test

> Auto-generated stub. Edit anything below the marker; the generator preserves it.

**Type:** `missingPerfTest`  
**Domain:** quality  
**Default severity:** low  
**Lifecycle status:** experimental  
**Gating tier:** observability

## Summary

A latency-critical AI surface (prompt / retrieval / agent / model / handler / route) has no benchmark or load test exercising it. Latency or throughput regressions ship silently.

## Remediation

Add a benchmark under benchmarks/ or perf/ that records P50 / P95 latency for the surface.

## Evidence sources

- `graph-traversal`

## Confidence range

Confidence interval: 0.70–0.85.

<!-- docs-gen: end stub. Hand-authored content below this line is preserved across regenerations. -->

## 1. Summary

A latency-critical AI surface (prompt / retrieval / agent / model / handler / route) has no benchmark or load test exercising it.

## 2. Severity & status

Experimental — off by default; enable in `terrain.yaml`.

## 3. What this catches

- A prompt template with no benchmark recording token-count / latency
- A retrieval pipeline with no load test against the vector store
- An agent flow whose multi-turn latency isn't measured
- A model checkpoint loaded in production with no throughput benchmark
- An HTTP handler with no load test (k6, locust, etc.)

## 4. Why this matters

Latency / throughput regressions ship silently when no benchmark guards the path. A prompt that grows from 500 to 5000 tokens may not break any test, but the inference cost and P95 latency change materially. The rule's purpose is to make the gap visible so adopters know where to add benchmarks deliberately.

## 5. Detection mechanism

- **Approach:** graph traversal. For each surface kind in the latency-critical set, check whether any test reaching the surface is in a benchmark-shaped path.
- **Latency-critical surface kinds:** SurfacePrompt, SurfaceRetrieval, SurfaceAgent, SurfaceModel, SurfaceHandler, SurfaceRoute.
- **Benchmark-test paths:** `/bench/`, `/benchmarks/`, `/perf/`, `/performance/`, `/load/`, `/loadtest/`, `/__benchmarks__/`.
- **Silence rule:** if the repo has no benchmark tests anywhere, the rule stays silent — otherwise the first run would flag every surface and overwhelm adopters.

## 6. Worked example

```
info[terrain/performance/missing-perf-test]: AI surface "summarize_prompt" has no benchmark
  --> prompts/summarize.txt
   = help: Add a benchmark under benchmarks/ that records P50/P95 latency.
   = docs: https://github.com/pmclSF/terrain/blob/main/docs/rules/performance/missing-perf-test
```

## 7. Configuration

```yaml
rules:
  performance/missing-perf-test: warning
ignore:
  rules:
    performance/missing-perf-test:
      - "experiments/**"
```

## 9. Reproducibility

```bash
terrain test --selector performance/missing-perf-test
```

## 10. Related rules

- `terrain/regression/performance-regression` — fires when a benchmark exists and the metric regressed
- `terrain/coverage/no-eval` — adjacent: surfaces without any eval coverage
