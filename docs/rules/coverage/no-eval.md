# terrain/coverage/no-eval — No Eval for AI Surface

> Auto-generated stub. Edit anything below the marker; the generator preserves it.

**Type:** `noEvalForAISurface`  
**Domain:** ai  
**Default severity:** high  
**Lifecycle status:** experimental  
**Gating tier:** gate

## Summary

An AI-typed CodeSurface (prompt / context / dataset / tool / retrieval / agent / eval_definition / model) has no Eval that claims to cover it. Model behavior can shift in production without any eval surfacing the regression.

## Remediation

Add an eval scenario that exercises the surface and asserts on its output / metric / shape.

## Evidence sources

- `graph-traversal`

## Confidence range

Confidence interval: 0.80–0.95.

<!-- docs-gen: end stub. Hand-authored content below this line is preserved across regenerations. -->

## 1. Summary

An AI-typed CodeSurface (prompt / context / dataset / tool / retrieval / agent / eval_definition / model) has no Eval that claims to cover it.

## 2. Status

Experimental — off by default; enable in terrain.yaml. Configurable via `terrain.yaml` — see [configuration.md](../../configuration.md).

## 3. What this catches

- A new prompt template added to `prompts/` with no eval scenario referencing it
- A saved `.pt` model in `models/` that's loaded in production code but never exercised by an eval
- A tool definition exported to an agent with no eval that uses it
- A retrieval / RAG component (chunker, embedder, reranker) with no eval validating its outputs
- A new dataset added under `data/` that's loaded in training but never used as an eval input

## 4. Why this matters

AI surfaces are the production system's behavioral substrate. Prompts, model weights, retrieval pipelines, tool definitions — these are the things that change model output without the surrounding application code changing. The same regression-driven safety net that exists for code (tests cover code; changes that break covered code get caught) needs to exist for AI surfaces (evals cover surfaces; changes that break covered surfaces get caught). When an AI surface has no covering eval, regressions reach production undetected. The rule fires on the *structural* fact that no Eval lists the surface in its `CoveredSurfaceIDs`, which is the static counterpart to "no test imports this code unit" — auditable, blame-free, and exactly the unit Terrain's CI gate enforces.

## 5. Detection mechanism

- **Approach:** Graph traversal over the snapshot's `Evals[].CoveredSurfaceIDs` index built once and reused across surfaces.
- **AI surface kinds covered:** SurfacePrompt, SurfaceContext, SurfaceDataset, SurfaceToolDef, SurfaceRetrieval, SurfaceAgent, SurfaceEvalDef, SurfaceModel.
- **Inputs consumed:** `TestSuiteSnapshot.CodeSurfaces` and `TestSuiteSnapshot.Evals`.
- **Edge cases handled:** non-AI surface kinds (function, method, handler, route, class, fixture) are skipped entirely.
- **Edge cases not handled:** transitive coverage — an eval that covers a downstream surface doesn't suppress the rule for an upstream surface even when the downstream's behavior depends on the upstream.

## 6. Worked example

```
error[terrain/coverage/no-eval]: AI surface "summarizer_v3.pt" (kind=model) has no eval coverage
  --> models/summarizer_v3.pt
   = help: Add an eval scenario that exercises "summarizer_v3.pt" and asserts on its output / metric / shape.
   = docs: https://github.com/pmclSF/terrain/blob/main/docs/rules/coverage/no-eval
```

**Before:** New `summarizer_v3.pt` checked in; production code starts loading it. No eval references it. Model behavior in production can drift across pulls of this checkpoint without any signal.

**After:** Add `evals/summarize_quality.yaml` that runs `summarize` against a fixed input set and asserts on rubric score. `CoveredSurfaceIDs` on that eval includes the model's surface ID.

## 7. Configuration

```yaml
rules:
  coverage/no-eval: warning   # downgrade if your team owns evals async to surface introduction
ignore:
  rules:
    coverage/no-eval:
      - "models/legacy/**"     # legacy artifacts pending removal
```

## 8. False-positive characterization

- **Eval declares coverage via folder convention but not in `CoveredSurfaceIDs`** — coverage is usually inferred from co-location; when it isn't, the eval's `terrain.yaml` declaration is the source of truth. Mitigation: list the surface in the eval's YAML.
- **Indirect coverage** (eval exercises a pipeline that internally invokes the surface) — not credited; explicit declaration is required.
- **Vendored / experimental surfaces** — ignore via path.

## 9. Reproducibility

```bash
terrain test --selector coverage/no-eval
```

## 11. Related rules

- `terrain/coverage/no-tests` — same shape for code units rather than AI surfaces
- `terrain/structural/uncovered-ai-surface` — sibling that uses different attribution heuristics
- `terrain/structural/phantom-eval` — fires when an eval CLAIMS coverage but the import graph doesn't support the claim
