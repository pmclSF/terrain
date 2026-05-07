# AI Eval Onboarding — first 10 minutes

The audit's `ai_execution_gating.P4` finding called this out: users
new to AI evals don't know whether to run Promptfoo / DeepEval /
Ragas first, or just point Terrain at a repo. This doc answers that
in three steps.

## TL;DR

```bash
# 1. Confirm Terrain sees your AI surfaces:
terrain ai list

# 2. Run your eval framework directly (Terrain does NOT execute them):
npx promptfoo eval --output results.json     # or deepeval / ragas

# 3. Hand the result to Terrain for gating:
terrain ai run --baseline .terrain/snapshots/baseline.json
```

That's it. Terrain reads your eval framework's output; it does not
run the framework on your behalf.

## What Terrain does — and doesn't — do

| Action                                  | Terrain does this | You do this |
|-----------------------------------------|:-:|:-:|
| Detect AI surfaces (prompts / agents / tools / retrieval) | ✓ | |
| Detect eval scenarios + framework configs | ✓ | |
| Execute eval framework (Promptfoo etc.) | | ✓ |
| Read eval framework output | ✓ | |
| Compute gate decision (BLOCKED / WARN / PASS) | ✓ | |
| Surface per-input ingestion diagnostics | ✓ | |

Detail: see [docs/product/ai-trust-boundary.md](../product/ai-trust-boundary.md).

## Step 1 — Confirm Terrain sees your AI surfaces

```bash
terrain ai list
```

Expected output:

```
AI Inventory
============
Components found
  AI Surfaces        12
  Eval Scenarios      5
  Eval Files          3
  Frameworks          1

Frameworks
  promptfoo            via promptfooconfig.yaml (95%)
```

**If output says "No AI surfaces detected":** Terrain hasn't found
any prompt / agent / tool / retrieval code. Check that your AI code
uses the patterns Terrain recognizes (see
[docs/rules/ai/](../rules/ai/) for the per-detector docs). If you
expected detection, run `terrain ai doctor` for diagnostic output.

## Step 2 — Run your eval framework yourself

Terrain does **not** execute Promptfoo / DeepEval / Ragas. You run
them directly, asking each to write its structured output to a file
Terrain can read.

### Promptfoo

```bash
npx promptfoo eval --output promptfoo-results.json
```

### DeepEval

```bash
deepeval test run tests/eval --export deepeval-results.json
```

### Ragas

```bash
python -m ragas evaluate --output ragas-results.json
```

Terrain accepts each adapter's canonical output format. Adapter
version detection + warn-on-shape-drift surfaces upstream changes
when they affect parsing.

## Step 3 — Hand the result to Terrain for gating

```bash
# First time — record this run as the baseline:
terrain ai run --record-baseline

# Subsequent runs — gate against the baseline:
terrain ai run --baseline .terrain/snapshots/baseline.json
```

Terrain renders a hero verdict block:

```
────────────────────────────────────────────────────────────
  [✗ BLOCKED]  3 AI eval signals — block merge
────────────────────────────────────────────────────────────

Reason: 3 high-severity findings introduced vs. baseline
...
```

Verdicts:
- **BLOCKED** — high-severity AI signal introduced; CI should fail
- **WARN**    — medium-severity flag; review recommended
- **PASS**    — gate clear

## Step 4 — Audit data lineage

When Terrain's gate decision rests on adapter-defaulted fields
(missing cost data, computed aggregates, etc.), you'll see a block
like this:

```
Ingestion diagnostics (2):
  [computed] aggregates.{successes,failures,errors} — stats block missing or all-zero; aggregates summed from per-case rows
  [missing] aggregates.tokenUsage.cost — Promptfoo output has no cost data — aiCostRegression will be a no-op for this run
```

This tells you when the gate decision is leaning on inferred data
vs. authoritative upstream fields. If a diagnostic surprises you,
fix it upstream (e.g. enable cost tracking in your Promptfoo config).

## CI integration

The recommended GitHub Action template is at
[docs/examples/gate/github-action.yml](../examples/gate/github-action.yml).
Walk-through with reproducible fixture: [docs/examples/gate/ai-eval-ci](../examples/gate/ai-eval-ci/).

## Common questions

**Q: Can Terrain run Promptfoo for me?** No — Terrain reads eval
output, doesn't execute eval frameworks. Sandboxed eval execution
is on the 0.3 roadmap.

**Q: What if I use a custom eval framework?** The adapter set
covers Promptfoo, DeepEval, Ragas, and Gauntlet today. Custom
frameworks: write a small wrapper that emits one of those formats,
or open an issue describing the shape.

**Q: Is the gate decision audited?** Every run records an
`.terrain/conversion-history/` JSONL entry with the verdict and
the diagnostics that informed it. The exit code (0 / 4 / 6) plus
the JSON envelope are the canonical machine-readable surface.

## Next steps

- [docs/integrations/promptfoo.md](../integrations/promptfoo.md) — Promptfoo-specific setup
- [docs/integrations/deepeval.md](../integrations/deepeval.md) — DeepEval-specific setup
- [docs/integrations/ragas.md](../integrations/ragas.md) — Ragas-specific setup
- [docs/product/ai-trust-boundary.md](../product/ai-trust-boundary.md) — what Terrain executes vs. parses
