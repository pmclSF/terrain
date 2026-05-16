# `terrain/prompt-quality/prompt-bloat` *(preview)*

<!-- docs-gen: end stub. Hand-authored content below this line is preserved across regenerations. -->

## 1. Summary

A prompt template's token count exceeds the configured budget, inflating per-call cost and latency.

## 2. Severity & status

- **Tier:** preview
- **Default severity:** off (opt-in via `terrain.yaml`)
- **Status:** preview — pending validation
- **Graduation criteria:** LB-5 ≤ 5% (Wilson 95% lower bound) on dogfood corpus; LB-6 recall ≥ 80%.

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
   = docs: https://terrain.dev/rules/prompt-quality/prompt-bloat
```

## 9. Reproducibility

```bash
terrain test --selector prompt-quality/prompt-bloat
```

Preview rules ship default-off; enable in `terrain.yaml` to surface findings.
