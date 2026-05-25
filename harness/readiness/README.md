# Per-rule readiness cards

Per-rule quality cards, committed alongside every release tag.

## Format

One file per stable rule per release: `harness/readiness/v<release>/<rule-id-flat>.md`. The rule ID is flattened (e.g., `regression/eval-regression` → `regression-eval-regression.md`) for filesystem-friendliness.

```markdown
# `terrain/<category>/<rule-name>` — v<release> readiness card

| Bar | Target | Measured | Pass |
|---|---|---|---|
| Diagnostic completeness | doc page exists; worked example matches output | ✓ | ✓ |
| Triage decision P75 | ≤60s | <measured> | <pass> |
| Fix-direction P75 | ≤Ns (category-tuned) | <measured> | <pass> |
| Reproduction parity | byte-equivalent CI ↔ CLI | <result> | <pass> |
| FP rate | ≤5% per-repo | per-repo: <values> | <pass> |
| Recall on seeded-failure | ≥90% per-repo | per-repo: <values> | <pass> |
| Renderer conformance | dorny + mikepenz + GitLab | <result> | <pass> |
| Runtime budget | ≤60s per-PR on 50k-file repo | <measured> | <pass> |

Stable since: v<X>
Last validated: v<X> (<date>)

Notes:
- <any partial-pass conditions, insufficient-data flags, or contextual notes>
```

## Cadence

- New card per stable rule at each release tag
- Preview rules get partial cards (no FP / recall target binding, marked "preview — pending validation")
- Cards from prior releases stay in their release directory; not overwritten
