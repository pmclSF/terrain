# `terrain/performance/token-cost-budget` *(preview)*

<!-- docs-gen: end stub. Hand-authored content below this line is preserved across regenerations. -->

## 1. Summary

The total token cost of an eval run exceeds the configured ceiling, indicating prompt or context bloat.

## 2. Severity & status

- **Tier:** preview
- **Default severity:** off (opt-in)
- **Status:** preview — pending validation

## 3. What this catches

- An eval run whose cumulative token spend grew 3× between baseline and current
- A retrieval pipeline that started returning 20× the context per call
- A prompt that ballooned silently across multiple PRs

## 5. Detection mechanism

Read eval-framework reported token usage (promptfoo `tokenUsage`, deepeval `tokensConsumed`, equivalents). Fire when current run cost exceeds configured ceiling OR exceeds baseline by configured percentage.

## 6. Worked example

```
warning[terrain/performance/token-cost-budget]: run cost $4.20 exceeds budget $1.50
  --> current.json
   = docs: https://github.com/pmclSF/terrain/blob/main/docs/rules/performance/token-cost-budget
```

## 9. Reproducibility

```bash
terrain test --selector performance/token-cost-budget
```
