# Terrain's three AI detection shapes

Terrain ships three different shapes of AI/ML detection in 0.2. They are *complementary*, not competing. This doc explains what each one does, when to reach for it, and how they fit together.

| Shape | Command | Output |
|---|---|---|
| **Inventory** | `terrain ai list` | What AI surfaces exist + which eval scenarios cover them |
| **Verdict engine** | `terrain ai findings` | Calibrated *eval-gap* findings with confidence + evidence |
| **Catalog detectors** | `terrain analyze`, `terrain report pr` | Categorical AI quality checks (12 detectors) |

## When to reach for each

**Use `terrain ai list` when you want an inventory.** Which prompts exist. Which agents are defined. Which RAG pipelines were wired. Which eval scenarios attach to which surfaces. This is the structural picture. Auto-derives scenarios from eval test files and AI framework imports; surfaces them grouped by capability. Pair with `terrain impact` for "what evals matter for this PR."

**Use `terrain ai findings` when you want eval-gap verdicts.** "Should this file have an eval and doesn't?" Each finding carries:

- A confidence score (0.0–1.0) from the calibrated pipeline
- A severity tier (low / medium / high / critical)
- A cohort label (ai-feature-in-app / rag-app / agent-app / library-sdk / ...)
- A full evidence chain — atom-by-atom, with weights and source spans
- An optional fix scaffold (Promptfoo eval YAML, DeepEval pytest, mlflow/wandb tracker)

The engine has two postures:
- `observability` (confidence ≥ 0.40, default): wide surfacing. Anything that clears the floor shows up.
- `gate` (confidence ≥ 0.80): strict CI gate. Only high-confidence findings fail the build.

**Use the 12 catalog detectors when you want specific anti-pattern checks.** These fire on categorical conditions and ship via the regular `terrain analyze` / `terrain report pr` paths:

- `aiHardcodedAPIKey` — config leaks provider keys
- `aiNonDeterministicEval` — eval declares a model without `temperature: 0`
- `aiModelDeprecationRisk` — floating model tags, sunset variants
- `aiPromptInjectionRisk` *(experimental)* — user input concatenated into prompts
- `aiToolWithoutSandbox` — destructive agent tools without sandbox/approval
- `aiSafetyEvalMissing` — safety-critical surfaces without safety scenarios
- `aiHallucinationRate` — eval run hallucination rate above threshold
- `aiCostRegression` — paired-case avg cost rising vs baseline
- `aiRetrievalRegression` — retrieval scores dropping vs baseline
- `aiPromptVersioning` — prompts without version markers
- `aiFewShotContamination` *(experimental)* — few-shot overlap with eval inputs
- `aiEmbeddingModelChange` — embedding model in source without retrieval eval

Catalog detectors emit *categorical* findings ("this fires or doesn't"); the verdict engine emits *gradual* findings ("here's the confidence + evidence"). They look at different signals and answer different questions.

## How they fit together

```
                        ┌─────────────────────┐
                        │  terrain analyze    │
                        │  (full repo scan)   │
                        └──────────┬──────────┘
                                   │ feeds snapshot
                  ┌────────────────┼────────────────┐
                  ▼                ▼                ▼
        ┌──────────────────┐ ┌────────────┐ ┌──────────────────┐
        │  terrain ai list │ │  12 AI     │ │ terrain ai       │
        │  (inventory)     │ │  catalog   │ │ findings         │
        │                  │ │  detectors │ │ (verdict engine) │
        └──────────────────┘ └────────────┘ └──────────────────┘
              shapes              shapes           shapes
              surfaces+scenarios  categorical      calibrated
              for impact/run      anti-patterns    eval-gap
              workflow            (HIGH/MED/LOW)   findings
```

The snapshot pipeline is the source of truth that all three read from. They differ in what they emit.

## Typical CI workflow

```yaml
# .github/workflows/terrain-ai.yml (sketch)
- name: AI inventory + impact selection
  run: terrain ai list --json > ai-surfaces.json

- name: Categorical AI quality checks
  run: terrain report pr --fail-on=high   # fires the 12 catalog detectors

- name: Calibrated eval-gap gate
  run: terrain ai findings --posture=gate  # 17%+ precision at 0.80 threshold
```

Run all three in CI. They flag different problems. A finding in one doesn't shadow a finding in another.

## What changes in 0.3

The 12 catalog detectors and the verdict engine will partially converge: catalog detectors will produce typed atoms that feed into the verdict engine's composer, so a single `terrain ai findings` call can surface both shape-class findings (the catalog) and gradient findings (the verdict engine) in one chain.

Until then, run both — they're complementary.
