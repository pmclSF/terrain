# TER-AI-105 — Non-Deterministic Eval Configuration

> Auto-generated stub. Edit anything below the marker; the generator preserves it.

**Type:** `aiNonDeterministicEval`  
**Domain:** ai  
**Default severity:** medium  
**Status:** stable

## Summary

An LLM eval runs without temperature pinned to 0 or a deterministic seed, so re-runs produce noisy comparisons.

## Remediation

Pin temperature: 0 and a seed in the eval config, or document the non-determinism budget.

## Evidence sources

- `structural-pattern`

## Confidence range

Detector confidence is bracketed at [0.90, 0.98] (heuristic in 0.2; calibration in 0.3).

<!-- docs-gen: end stub. Hand-authored content below this line is preserved across regenerations. -->

# TER-AI-105 — Non-Deterministic Eval Configuration

**Type:** `aiNonDeterministicEval`
**Domain:** AI
**Default severity:** Medium
**Severity clauses:** [`sev-medium-003`](../../severity-rubric.md)
**Status:** stable (0.2)

## What it detects

The detector parses YAML and JSON config files in the snapshot whose
path looks like an eval / agent / prompt config (`eval`, `promptfoo`,
`deepeval`, `ragas`, `agent`, `prompt`, `.terrain/`) and inspects the
parsed tree for the determinism knobs.

It fires in two situations:

1. **`temperature` set to a non-zero value.** The eval will produce
   different scores on identical inputs across runs; comparisons in CI
   become noisy.
2. **`temperature` missing while a `model` is declared.** Default
   sampling for most providers is non-deterministic.

The detector does not check `seed` separately because seed support
varies by provider — checking it alongside temperature would produce
spurious findings on providers that don't honour it.

## Why it's Medium

Per `sev-medium-003`. Non-deterministic evals are not broken — they
just produce noisy CI signal. Teams that explicitly want stochastic
sampling can keep doing it with a documented exemption; the cost of
the warning is low.

## What you should do

```yaml
provider:
  name: openai
  model: gpt-4-0613
  temperature: 0      # pin determinism
  seed: 42            # optional, provider-dependent
```

Or, if you intentionally want stochastic sampling (rare for CI evals,
common for production live traffic), document the budget alongside
the scenario:

```yaml
# ACCEPTED: live-traffic eval; see docs/runbook/temperature-budget.md
provider:
  temperature: 0.7
```

## Why it might be a false positive

- The config inspects a non-LLM provider that doesn't have a
  `temperature` knob. File a fixture under `tests/calibration/` with
  `expectedAbsent: aiNonDeterministicEval` so we add the provider's
  shape to the allowlist.
- The repo has many YAML files unrelated to AI evals. The detector
  only inspects files whose path contains an AI-config marker; if
  yours uses a non-standard path, document it.

## Known limitations (0.2)

- Only YAML and JSON. Python config files (`pyproject.toml [tool.eval]`,
  `pytest.ini`-style) are not inspected today.
- Does not check `top_p` (provider-specific; usually paired with
  temperature anyway).
