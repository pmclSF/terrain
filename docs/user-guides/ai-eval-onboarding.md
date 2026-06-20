# AI Eval Onboarding — first 10 minutes

The audit's `ai_execution_gating.P4` finding called this out: users
new to AI evals don't know whether to run Promptfoo / DeepEval /
Ragas / Great Expectations first, or just point Terrain at a repo.
This doc answers that in three steps.

## TL;DR

```bash
# 1. Confirm Terrain sees your AI surfaces:
terrain ai list

# 2. Preview what Terrain would execute:
terrain ai run --dry-run --base main

# 3. Run the impacted eval scenarios and gate on the result:
terrain ai run --base main --timeout 10m
```

That's it. Terrain detects eval scenarios, builds the supported framework
command, runs it unless `--dry-run` is set, ingests the structured output
when the adapter supports it, and records the run artifact.

## What Terrain does — and doesn't — do

| Action                                  | Terrain does this | You do this |
|-----------------------------------------|:-:|:-:|
| Detect AI surfaces (prompts / agents / tools / retrieval) | ✓ | |
| Detect eval scenarios + framework configs | ✓ | |
| Execute supported eval framework command | ✓ | |
| Install/configure eval framework dependencies | | ✓ |
| Read structured eval framework output | ✓ | |
| Compute gate decision (BLOCKED / WARN / PASS) | ✓ | |
| Surface per-input ingestion diagnostics | ✓ | |

Detail: see [`SECURITY-DATA-HANDLING.md`](../../SECURITY-DATA-HANDLING.md).

## Step 1 — Confirm Terrain sees your AI surfaces

```bash
terrain ai list
```

Expected output (your numbers will differ):

```
Terrain AI Inventory
============================================================

| Component          | Count |
|--------------------|-------|
| Scenarios          |    17 |
| Capabilities       |    17 |
| Prompts            |     3 |
| Contexts           |     6 |
| Tool Definitions   |     1 |
| Retrieval / RAG    |     4 |
| Eval Files         |    64 |
| Frameworks         |     2 |
| Missing coverage   |    14 |

Capabilities
------------------------------------------------------------
  <list of detected capabilities, one per line>
```

For the JSON shape and a per-capability breakdown, run
`terrain ai list --json`.

**If output says "No AI surfaces detected":** Terrain hasn't found
any prompt / agent / tool / retrieval code. Check that your AI code
uses the patterns Terrain recognizes — for example
[prompt-injection-risk.md](../rules/ai/prompt-injection-risk.md)
documents the canonical AI surface shapes the detectors look for.
If you expected detection, run `terrain ai doctor` for diagnostic output.

## Step 2 — Preview or run the eval command

Use dry-run first to verify the detected framework and command:

```bash
terrain ai run --dry-run --base main
```

Then run the impacted scenarios. `--base` is a git ref used for
impact-based scenario selection; `--full` runs every detected scenario.
`--timeout` bounds the eval framework child process.

```bash
terrain ai run --base main --timeout 10m
terrain ai run --full --timeout 10m
```

Terrain currently recognizes Promptfoo, DeepEval, Ragas, LangSmith,
and generic test-runner fallbacks. During `terrain ai run`, Promptfoo
output is written to a Terrain-owned temp file and parsed through the
Promptfoo adapter. Other framework commands may still run even when
they do not expose a structured-output flag Terrain can parse yet; use
the `terrain analyze --deepeval-results` / `--ragas-results` /
`--great-expectations-results` artifact flags for those JSON outputs.

Run the framework directly when debugging the framework itself or when
you need a custom wrapper.

### Promptfoo

```bash
npx promptfoo eval --output promptfoo-results.json
```

### DeepEval

```bash
DEEPEVAL_RESULTS_FOLDER=out/deepeval deepeval test run tests/eval
terrain analyze --deepeval-results out/deepeval/<generated-run>.json
```

### Ragas

```bash
python eval_ragas.py  # writes ragas-results.json via result.to_pandas().to_json(...)
terrain analyze --ragas-results ragas-results.json
```

### Great Expectations

```bash
python validate_data.py  # writes Great Expectations Validation Result JSON
terrain analyze --great-expectations-results great-expectations-results.json
```

Terrain's artifact-ingestion flags accept each adapter's canonical
output format. Adapter version detection + warn-on-shape-drift surfaces
upstream changes when they affect parsing.

## Step 3 — Record and compare baselines

```bash
# First known-good state — record the scenario inventory baseline:
terrain ai record

# Later — compare the current scenario inventory and metrics:
terrain ai baseline compare
```

Terrain renders a hero verdict block:

```
────────────────────────────────────────────────────────────
  [✗ BLOCKED]  3 AI eval signals — block merge
────────────────────────────────────────────────────────────

Reason: 3 high-severity AI signals
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

**Q: Can Terrain run Promptfoo for me?** Yes. `terrain ai run`
invokes detected supported framework commands unless `--dry-run` is
set. This is bounded execution, not sandboxing: use `--timeout`, run in
CI, and keep framework dependencies/config under your normal repo
controls. Sandboxed eval execution remains future work.

**Q: What if I use a custom eval framework?** The artifact-ingestion
adapter set covers Promptfoo, DeepEval, Ragas, Great Expectations, and
Gauntlet today.
Custom frameworks: write a small wrapper that emits one of those
formats, or open an issue describing the shape.

**Q: Is the gate decision audited?** Every run records an
`.terrain/artifacts/ai-run-latest.json` artifact with the command,
selected/skipped scenarios, content hashes, verdict, and diagnostics
that informed it. The exit code (0 / 4 / 6) plus the JSON envelope are
the canonical machine-readable surface.

## Next steps

- [docs/integrations/promptfoo.md](../integrations/promptfoo.md) — Promptfoo-specific setup
- [docs/integrations/deepeval.md](../integrations/deepeval.md) — DeepEval-specific setup
- [docs/integrations/ragas.md](../integrations/ragas.md) — Ragas-specific setup
- [docs/integrations/great-expectations.md](../integrations/great-expectations.md) — Great Expectations-specific setup
- [`SECURITY-DATA-HANDLING.md`](../../SECURITY-DATA-HANDLING.md) — what Terrain executes vs. parses
