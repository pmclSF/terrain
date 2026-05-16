# Per-rule readiness cards

The harness's public output. Committed alongside every release tag.

## Format

One file per stable rule per release: `harness/readiness/v<release>/<rule-id-flat>.md`. The rule ID is flattened (e.g., `regression/eval-regression` → `regression-eval-regression.md`) for filesystem-friendliness.

Per-card structure per `docs/HARNESS.md`:

```markdown
# `terrain/<category>/<rule-name>` — v<release> readiness card

| Bar | Target | Measured | Pass |
|---|---|---|---|
| LB-1 diagnostic completeness | doc page exists; worked example matches output | ✓ | ✓ |
| LB-2a triage decision P75 | ≤60s | <measured> | <pass> |
| LB-2b fix-direction P75 | ≤Ns (category-tuned) | <measured> | <pass> |
| LB-4 reproduction parity | byte-equivalent CI ↔ CLI | <result> | <pass> |
| LB-5 FP rate Wilson 95% LB | ≤5% per-repo | per-repo: <values> | <pass> |
| LB-6 recall on seeded-failure | ≥90% per-repo | per-repo: <values> | <pass> |
| LB-7 renderer conformance | dorny + mikepenz + GitLab | <result> | <pass> |
| LB-9 runtime budget | ≤60s per-PR on 50k-file repo | <measured> | <pass> |

Stable since: v<X>
Last validated: v<X> (<date>)
Panel session: <date> (N=5 per repo)

Notes:
- <any partial-pass conditions, insufficient-data flags, or contextual notes>
```

## Cadence

- New card per stable rule at each release tag
- Preview rules get partial cards (no LB-5 / LB-6 target binding, marked "preview — pending validation")
- Cards from prior releases stay in their release directory; not overwritten

## Status at 0.2.0 dev

`v0.2.0/` is empty. Cards are generated at release time by the runner; landing them is the final Tier 4 deliverable.
