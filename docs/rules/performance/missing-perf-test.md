# `terrain/performance/missing-perf-test`

<!-- docs-gen: end stub. Hand-authored content below this line is preserved across regenerations. -->

## 1. Summary

A latency-critical AI surface (prompt / retrieval / agent / model / handler / route) has no benchmark or load test exercising it.

## 2. Severity & status

- **Tier:** stable
- **Default severity:** low
- **Stable since:** v0.2.0

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
- **0.2.0 silence rule:** if the repo has no benchmark tests anywhere, the rule stays silent — otherwise the first run would flag every surface and overwhelm adopters.

## 6. Worked example

```
info[terrain/performance/missing-perf-test]: AI surface "summarize_prompt" has no benchmark
  --> prompts/summarize.txt
   = help: Add a benchmark under benchmarks/ that records P50/P95 latency.
   = docs: https://terrain.dev/rules/performance/missing-perf-test
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

## 10. Stability commitment

Latency-critical surface set and benchmark-path vocabulary are stable from v0.2.0.

## 11. Related rules

- `terrain/regression/performance-regression` — fires when a benchmark exists and the metric regressed
- `terrain/coverage/no-eval` — adjacent: surfaces without any eval coverage
