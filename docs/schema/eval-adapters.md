# Eval Adapter Schema Contract

Documents the canonical shape every eval-framework adapter
(Promptfoo, DeepEval, Ragas, Gauntlet) emits into Terrain's
normalized `EvalRunResult` envelope.

This is the contract downstream detectors (aiCostRegression,
aiHallucinationRate, aiRetrievalRegression) consume. Adapter
authors should fill these fields whenever the upstream framework
provides them, and emit an `IngestionDiagnostic` when a field
falls back to a default.

## Top-level envelope: `EvalRunResult`

```jsonc
{
  // Source adapter ID. Lowercased canonical form.
  // Stability: Stable.
  "framework": "promptfoo",

  // Framework's identifier for this run. Empty when not supplied.
  // Stability: Stable.
  "runId": "eval-abc123",

  // RFC3339 UTC timestamp. Zero value when the framework didn't
  // expose one. Surface as a `default-applied` IngestionDiagnostic
  // when present-but-unparseable.
  // Stability: Stable.
  "createdAt": "2026-04-30T12:00:00Z",

  // One entry per (test, prompt, provider) combination. May be
  // empty when the framework only provides aggregate stats.
  // Stability: Stable.
  "cases": [ /* EvalCase, see below */ ],

  // Run summary. Either lifted from the framework's own stats
  // block or computed by the adapter. When computed, an
  // IngestionDiagnostic with kind="computed" must be emitted.
  // Stability: Stable.
  "aggregates": { /* EvalAggregates, see below */ },

  // Per-field fallback record. Empty when the upstream output
  // populated every expected field.
  // Stability: Stable.
  "diagnostics": [ /* IngestionDiagnostic, see below */ ]
}
```

## `EvalCase` — per-row result

```jsonc
{
  // Stable identifier within the run. Empty when not supplied;
  // downstream code falls back to positional ordering.
  // Stability: Stable.
  "caseId": "row-1",

  // Human-readable label. Stability: Stable.
  "description": "happy path",

  // Framework-specific provider ID (e.g. "openai:gpt-4-0613").
  // Used by aiModelDeprecationRisk and the report renderer.
  // Stability: Stable.
  "provider": "openai:gpt-4-0613",

  // Prompt's user-facing label. Empty when the framework only
  // attached prompt content.
  // Stability: Stable.
  "promptLabel": "system + user",

  // Whether the case passed. Adapters MAY synthesize this from
  // metric scores (e.g. Ragas vote on quality axes >= 0.5).
  // Stability: Stable.
  "success": true,

  // Per-case score in [0.0, 1.0] when available. yes/no maps
  // to {0.0, 1.0}.
  // Stability: Stable.
  "score": 1.0,

  // Wall-clock latency in milliseconds. Zero when not recorded.
  // Stability: Stable.
  "latencyMs": 850,

  // Token usage + cost for this case. Stability: Stable.
  "tokenUsage": { /* TokenUsage, see below */ },

  // Framework-specific scoring axes (faithfulness,
  // context_relevance, answer_relevancy, etc.). Adapters pass
  // through verbatim with lowercase normalization. Detectors
  // look for specific keys in this map.
  // Stability: Stable.
  "namedScores": {
    "faithfulness": 0.92,
    "context_relevance": 0.85
  },

  // Framework's diagnostic string when the case failed.
  // Stability: Stable.
  "failureReason": "expected 'paris', got 'wrong'"
}
```

## `EvalAggregates` — run summary

```jsonc
{
  // Three buckets mirroring Promptfoo's stats:
  //   - successes: assertion passed
  //   - failures:  assertion failed
  //   - errors:    runtime problem (provider rejection, network
  //                timeout) prevented scoring
  // Stability: Stable.
  "successes": 100,
  "failures": 12,
  "errors": 3,

  // Run-level total across all cases. Stability: Stable.
  "tokenUsage": { /* TokenUsage */ }
}
```

## `TokenUsage` — token + cost data

```jsonc
{
  // Per-bucket integer counts. Zero when not recorded.
  "prompt": 50,
  "completion": 30,
  "total": 80,

  // USD cost. Zero when the upstream output omits it; adapters
  // emit a "missing" diagnostic for aggregates.tokenUsage.cost
  // because aiCostRegression silently no-ops on zero cost data.
  "cost": 0.0024
}
```

## `IngestionDiagnostic` — fallback record

When an adapter falls back on a default, computes a missing
field, or coerces a near-shape, it appends one entry to
`EvalRunResult.diagnostics`.

```jsonc
{
  // Dotted JSON path within EvalRunResult.
  // Examples:
  //   "aggregates.{successes,failures,errors}"
  //   "aggregates.tokenUsage.cost"
  //   "cases[].metricsData"
  //   "createdAt"
  "field": "aggregates.tokenUsage.cost",

  // One of:
  //   "missing"          — upstream omitted the field
  //   "computed"         — adapter computed from other inputs
  //   "default-applied"  — upstream value was malformed
  //   "coerced"          — near-shape accepted (e.g. int→float)
  "kind": "missing",

  // One-sentence adopter-facing reason.
  "detail": "Promptfoo output has no cost data — aiCostRegression will be a no-op for this run"
}
```

Diagnostics surface in `terrain ai run` text output (under
"Ingestion diagnostics (N):") and in the JSON envelope so adopters
auditing a gating decision can see exactly which fields fell back.

## Stability commitment

This document is the public contract for adapter authors and
downstream detectors. Field-level stability tiers follow
[`FIELD_TIERS.md`](FIELD_TIERS.md):

- All fields named "Stability: Stable" above are part of the
  long-lived schema. Removal requires a major version bump and
  a migration window.
- New optional fields may be added in minor releases.
- The `IngestionDiagnostic.kind` enum may be extended with new
  values in minor releases; consumers should treat unknown kinds
  as a fallback (informational, not blocking).

## Adapter authoring checklist

When adding a new adapter to `internal/airun/`:

1. **Parse the framework's canonical output format.** Reject
   payloads that don't parse rather than silently producing
   empty results.
2. **Populate every Stable field that the framework provides.**
   When a field doesn't apply, leave it at zero value (don't
   substitute placeholder strings).
3. **Emit one IngestionDiagnostic per fallback.** The detector
   tier depends on this for data lineage; a silent fallback
   misleads gate decisions.
4. **Add conformance fixtures** under `internal/airun/conformance/`
   covering at least the canonical shape and one upstream-version
   shape. The conformance test suite locks parser semantics.
5. **Lock new diagnostics with a unit test.** Pattern:
   `TestParse<framework>_DiagnosticsOn<scenario>` asserting the
   expected `(kind, field)` pair appears in `Diagnostics`.

## See also

- [`internal/airun/eval_result.go`](../../internal/airun/eval_result.go) — Go type definitions
- [`docs/integrations/promptfoo.md`](../integrations/promptfoo.md) — Promptfoo-specific notes
- [`docs/integrations/deepeval.md`](../integrations/deepeval.md) — DeepEval-specific notes
- [`docs/integrations/ragas.md`](../integrations/ragas.md) — Ragas-specific notes
- [`docs/user-guides/ai-eval-onboarding.md`](../user-guides/ai-eval-onboarding.md) — adopter-facing onboarding
