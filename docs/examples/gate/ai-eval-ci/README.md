# Promptfoo + Terrain — end-to-end CI walkthrough

> **What this example shows.** A working GitHub Actions pipeline
> that runs Promptfoo against your prompts on every pull request,
> feeds the results into Terrain's PR gate, and posts a single
> unified comment summarizing structural test coverage *and* AI eval
> regressions in the same shape.

This is the canonical Tier-1 deliverable for the Gate pillar's AI
side. Adopters who copy this directory into their own repo (with
minor edits) get a working CI gate in under 30 minutes.

## What you get

After wiring this in:

1. Every PR gets a single Terrain comment (no separate "AI bot"
   noise) covering:
   - Coverage gaps in changed code
   - Recommended tests
   - AI Risk Review with three tier-tagged sub-stanzas (Inventory /
     Hygiene / Regression — see
     [`docs/product/ai-risk-tiers.md`](../../../product/ai-risk-tiers.md))
2. A `--fail-on critical` gate blocks merges when:
   - A new untested AI surface is introduced (Inventory)
   - An eval-flagged hallucination regression appears (Regression)
3. Hygiene findings (prompt-injection structural patterns,
   hardcoded keys) are visible but **not blocking** in the
   recommended config — opt-in once you've measured precision in
   your own repo.

## Files in this example

```
docs/examples/gate/ai-eval-ci/
├── README.md                  ← you are here
├── github-action.yml          ← drop-in workflow
├── promptfoo.config.yaml      ← Promptfoo config skeleton
├── prompts/                   ← prompt templates Promptfoo runs against
│   └── refund-eligibility.txt
└── evals/                     ← Promptfoo test scenarios
    └── refund-eligibility.yaml
```

## Step-by-step

### 1. Install Terrain

In your repo (one-time):

```bash
brew install pmclSF/terrain/mapterrain
# or
go install github.com/pmclSF/terrain/cmd/terrain@latest
# or
npm install -g mapterrain          # Node 22+ required, see README
```

Verify:

```bash
terrain --version
```

### 2. Install Promptfoo

```bash
npm install -g promptfoo
```

Verify:

```bash
promptfoo --version
```

### 3. Copy the workflow

Drop `github-action.yml` into `.github/workflows/terrain.yml` in
your repo. Edit:

- `OPENAI_API_KEY` → use your own secret name if different
- The `prompts/` and `evals/` paths if you stage them elsewhere

### 4. Establish a baseline

Before the first gating PR, capture a baseline so the
`--new-findings-only` flag has something to compare against:

```bash
# Run Promptfoo locally against main:
git checkout main
promptfoo eval --output .terrain/promptfoo-baseline.json

# Run Terrain analyze and stash the snapshot as the baseline:
terrain analyze --write-snapshot .terrain/snapshots/baseline.json \
    --promptfoo-results .terrain/promptfoo-baseline.json

git add .terrain/snapshots/baseline.json
git commit -m "ci: terrain baseline for PR gate"
```

The workflow's `--baseline` flag points at this committed
snapshot. Refresh it whenever the eval scoreboard shifts
materially (new model, prompt rewrite, eval-set expansion) and
commit the refresh as its own PR.

### 5. Open a PR

The first PR after wiring this in produces a Terrain comment with
the unified shape. Coverage gaps, recommended tests, and AI Risk
Review all appear in one card.

If the change introduces a new prompt or model surface, you'll see
the Inventory sub-stanza light up. If the change shifts
hallucination rate or cost beyond the baseline, you'll see the
Regression sub-stanza. Hygiene findings appear but don't block
the merge in the recommended config.

## Why we recommend this exact shape

There are many ways to wire eval results into CI. We landed on
this shape after the launch-readiness review for two specific
reasons:

1. **One comment, not two.** Adopters running Promptfoo
   independently of Terrain end up with two PR bots talking past
   each other — eval pass/fail in one comment, structural coverage
   in another. The unified shape uses Promptfoo's output as raw
   material for *Terrain's* comment; only one bot speaks per PR.
2. **Baseline-aware default.** `--new-findings-only --baseline`
   means established repos with existing AI debt don't brick on
   day one. Adopters who turn on `--fail-on critical` after a
   month of Inventory cleanup never see an avalanche of legacy
   findings.

The "warn-only by default" trust ladder
([`docs/product/trust-ladder.md`](../../../product/trust-ladder.md))
is the alternative for adopters who want to see findings before
committing to a blocking gate. This walkthrough assumes
`--fail-on critical` is the destination; adjust the template if
you're earlier on the ladder.

## What this example does NOT do

Documented up front so adopters don't infer a contract that
isn't there:

- **Doesn't run your tests.** Promptfoo runs its tests; Terrain
  reads the output. We never invoke your test runner.
- **Doesn't ship API keys.** `OPENAI_API_KEY` is consumed by
  Promptfoo (a child process Terrain optionally spawns). Terrain
  does not read, log, or proxy the key. See
  [`docs/product/ai-trust-boundary.md`](../../../product/ai-trust-boundary.md).
- **Doesn't sandbox eval execution.** Sandboxing is on the 0.3
  roadmap. If your prompts include tool-call shapes that touch
  filesystem or network, the sandbox is the eval framework's
  responsibility in 0.2.
- **Doesn't replace Lakera / Guardrails.** Those are
  request-time AI safety services. Terrain solves the structural
  / pre-deploy / inventory side. Both can coexist.

## Troubleshooting

### "Eval results JSON not found"

Confirm Promptfoo's `--output` path matches the
`--promptfoo-results` path in the Terrain step. The workflow
uses `out/promptfoo.json` for both; if you change one, change
both.

### "Terrain reports unfamiliar Promptfoo shape"

Track 7.2 surfaces a single warning when Promptfoo's output shape
doesn't match v3 or v4. The warning text names the specific drift.
Most often it means Promptfoo got upgraded and the export shape
shifted; file an issue with the Terrain version + Promptfoo
version and we'll add a conformance fixture.

### "PR comment is missing the AI Risk Review section"

The AI section only renders when Terrain detects AI surfaces in
the changed files OR has eval results to summarize. If both are
absent, no AI section. To verify Terrain found your AI surfaces:

```bash
terrain ai list
```

## Related reading

- [`docs/examples/gate/github-action.yml`](../github-action.yml) —
  the canonical Terrain CI workflow (without AI eval wiring)
- [`docs/product/ai-risk-tiers.md`](../../../product/ai-risk-tiers.md) —
  why the AI section has three sub-stanzas
- [`docs/product/unified-pr-comment.md`](../../../product/unified-pr-comment.md) —
  the visual contract this comment commits to
- [`docs/integrations/promptfoo.md`](../../../integrations/promptfoo.md) —
  full Promptfoo integration notes
