# terrain/prompt-quality/prompt-without-temperature — Prompt Without Temperature

> Auto-generated stub. Edit anything below the marker; the generator preserves it.

**Type:** `promptWithoutTemperature`  
**Domain:** ai  
**Default severity:** low  
**Lifecycle status:** experimental  
**Gating tier:** observability

## Promotion plan

Fires when an LLM SDK call has no temperature kwarg. Defaults differ across SDKs; an explicit value is reproducibility-critical.

## Evidence sources

- `structural-pattern`

## Confidence range

Confidence interval: 0.85–0.95.

<!-- docs-gen: end stub. Hand-authored content below this line is preserved across regenerations. -->

## 1. Summary

An LLM SDK call lacks an explicit `temperature` argument. Default temperatures differ across SDKs and versions; pinning the value is reproducibility-critical for evals.

## 2. Severity & status

- **Tier:** preview
- **Default severity:** off (opt-in via `terrain.yaml`)
- **Status:** preview — pending validation
- **Graduation criteria:** false-positive rate ≤ 5%.

## 3. What this catches

- `client.chat.completions.create(model="gpt-4o", messages=[...])` (no temperature)
- `client.messages.create(model="claude-opus-4-7", messages=[...])` (no temperature)
- `chain.invoke(...)` where the bound model defaults the temperature

## 5. Detection mechanism

AST-level inspection of LLM SDK call sites (consuming the AICallSite records from `internal/aidetect/`). Fires when the call has no `temperature=` kwarg and isn't using a streaming method that pins it implicitly.

## 6. Worked example

```
warning[terrain/prompt-quality/prompt-without-temperature]: LLM call has no explicit temperature
  --> api/summarize.py:12
   = docs: https://github.com/pmclSF/terrain/blob/main/docs/rules/prompt-quality/prompt-without-temperature
```

## 9. Reproducibility

```bash
terrain test --selector prompt-quality/prompt-without-temperature
```
