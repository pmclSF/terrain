# Terrain for AI / ML teams

If your repo has prompts, agents, RAG pipelines, or eval scenarios,
Terrain treats them as first-class testable surfaces. Same structural
model as conventional tests, plus a dedicated AI signal domain that
catches the bugs a non-AI tool would miss.

## What Terrain catches that's specific to AI

| Signal | Severity | What it flags |
|---|---|---|
| `aiHardcodedAPIKey` | Critical | API key embedded in eval YAML / agent config / source. Eight providers covered (OpenAI, Anthropic, Google, AWS, GitHub, HuggingFace, Slack, Stripe). |
| `aiPromptInjectionRisk` | High | User-controlled input concatenated into a prompt without escaping. Pattern detector in 0.2; AST-precise taint flow in 0.3. |
| `aiSafetyEvalMissing` *(planned)* | High | Agent / prompt with no eval scenario covering safety (jailbreak, harm, leak). |
| `aiToolWithoutSandbox` *(planned)* | High | Destructive agent tool (delete / exec / drop) with no approval gate or sandbox. |
| `aiNonDeterministicEval` | Medium | Eval config without `temperature: 0` / seed pinned; CI comparisons get noisy. |
| `aiModelDeprecationRisk` | Medium | Floating model tag (`gpt-4`, `claude-3-opus`) or sunset model (`text-davinci-003`). Dated variants (`gpt-4-0613`) are the safe form. |
| `aiPromptVersioning` *(planned)* | Medium | Prompt content changed without a version bump in metadata or filename. |
| `aiCostRegression` *(planned)* | Medium | Prompt change increases token count >25% vs baseline. |
| `aiHallucinationRate` *(planned)* | High | Eval reports fabricated outputs above project threshold. |
| `aiFewShotContamination` *(planned)* | Medium | Few-shot examples overlap with the eval test set. |
| `aiEmbeddingModelChange` *(planned)* | Medium | Embedding model swap with no RAG re-evaluation. |
| `aiRetrievalRegression` *(planned)* | High | Context relevance / nDCG / coverage dropped vs baseline. |

*(planned)* signals reserve names and shapes; detectors land before
0.2 close. See `docs/signals/manifest.json` for the full status.

## What Terrain does NOT do for AI

- **Doesn't run evals.** Use Promptfoo, DeepEval, Ragas, or your own
  runner. Terrain analyses the eval *structure* (what surfaces are
  covered, which scenarios exist, where the gaps are) and ingests the
  *artifacts* those tools produce.
- **Doesn't sanitise prompts.** It tells you when concatenation looks
  injection-shaped; the fix lives in your prompt-template library.
- **Doesn't manage secrets.** It finds API keys you committed; rotate
  on the provider's console and move them to env vars / a vault.

## Surface inventory

`terrain ai list --json` outputs every detected AI surface in your
repo with:

- `tier` — `prompt` / `context` / `dataset` / `tool` / `scenario` /
  `agent`
- `path` and `line`
- `framework` — `promptfoo` / `deepeval` / `ragas` / `custom`
- `confidence` — how certain Terrain is about the classification
- `coveredBy` — eval scenarios that exercise this surface

Useful for:

- Finding orphaned prompts (no scenario covers them).
- Auditing the surface your agent definition actually exposes.
- Deciding what to add to the eval suite next.

## A typical workflow

```bash
# 1. What AI surfaces do we have?
terrain ai list --root . --json > .terrain/ai-inventory.json

# 2. What's the test coverage shape?
terrain analyze --root . --detail 2

# 3. Gauntlet (bring eval results in for the analysis to see).
promptfoo eval --output promptfoo-results.json
terrain ai run --eval-input promptfoo-results.json --root .

# 4. PR check.
terrain impact --base origin/main --root .  # changed AI surfaces
terrain ai gate --root . --policy strict     # block on Critical AI signals
```

## Suggested CI hookup

```yaml
name: terrain-ai
on:
  pull_request:

jobs:
  ai-gate:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v6
        with:
          fetch-depth: 0
      - run: |
          curl -L https://github.com/pmclSF/terrain/releases/latest/download/terrain_linux_amd64.tar.gz \
            | tar -xz
          ./terrain ai list --root . --json > ai-inventory.json
          # Fail the build on aiHardcodedAPIKey or aiPromptInjectionRisk.
          ./terrain ai gate --root . --policy strict
      - uses: actions/upload-artifact@v4
        with:
          name: ai-inventory
          path: ai-inventory.json
```

## Eval framework integrations (0.2)

- **Promptfoo** — parse run results (accuracy, latency, cost per case).
- **DeepEval** — parse artifact format; extract custom metric values.
- **Ragas** — ingest `retrieval_score`, `faithfulness`, `answer_relevancy`.
- Plugin architecture for community adapters lands in
  `internal/airun/plugin.go` during 0.2.

## Where to go next

- `docs/rules/ai/` — per-signal documentation.
- `docs/severity-rubric.md` — clauses that justify each severity.
- `docs/calibration/CORPUS.md` — labelled fixtures that drive
  per-detector precision/recall.
