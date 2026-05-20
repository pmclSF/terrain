# terrain/prompt-quality/prompt-bloat — Prompt Bloat

> Auto-generated stub. Edit anything below the marker; the generator preserves it.

**Type:** `promptBloat`  
**Domain:** ai  
**Default severity:** low  
**Status:** experimental

## Promotion plan

0.2 — fires when prompt token count exceeds configured budget. Calibrated against dogfood corpus before promotion.

## Evidence sources

- `structural-pattern`

## Confidence range

Detector confidence is bracketed at [0.70, 0.85] (heuristic today; calibrated against a labeled corpus over time).

<!-- docs-gen: end stub. Hand-authored content below this line is preserved across regenerations. -->

## 1. Summary

A prompt template's token count exceeds the configured budget, inflating per-call cost and latency.

## 2. Severity & status

- **Tier:** preview
- **Default severity:** off (opt-in via `terrain.yaml`)
- **Status:** preview — pending validation
- **Graduation criteria:** false-positive rate ≤ 5% (Wilson 95% lower bound) on dogfood corpus; recall ≥ 80%.

## 3. What this catches

- A prompt that grew from 500 → 5000 tokens through repeated editing
- A prompt that embeds the full schema dump rather than referencing it
- A few-shot block that ballooned to 20+ examples

## 5. Detection mechanism

Token-counts the prompt template's content (BPE estimate or framework-reported count) and fires when ≥ configured budget. Budget defaults to 2000 tokens; adopters tune via `terrain.yaml`.

## 6. Worked example

```
warning[terrain/prompt-quality/prompt-bloat]: prompt "summarize" is 4823 tokens (budget 2000)
  --> prompts/summarize.txt
   = docs: https://github.com/pmclSF/terrain/blob/main/docs/rules/prompt-quality/prompt-bloat
```

## 9. Reproducibility

```bash
terrain test --selector prompt-quality/prompt-bloat
```

Preview rules ship default-off; enable in `terrain.yaml` to surface findings.
