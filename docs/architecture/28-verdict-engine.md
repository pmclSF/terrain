# 28 — Verdict engine (aipipeline)

> **Status:** stable (0.2). Reachable via `terrain ai findings`.
> **Source of truth:** `internal/aipipeline` (core), `internal/aipipeline/stages` (per-stage logic), `internal/aipiperun` (production runner), `cmd/terrain-pipeline` (validation harness).
> **Companion docs:** `19-ai-scenario-and-eval-model.md` (the surface/eval model the verdict engine flags against), `14-evidence-scoring-and-confidence-model.md` (the general evidence-scoring pattern this is a specialization of).

The verdict engine is the calibrated AI-surface detector that ships in 0.2. It replaces the regex-only path that produced the recall-anchored AI detectors with a typed-evidence pipeline whose output is a `Finding` carrying confidence, severity, cohort, an evidence chain, and an optional fix scaffold.

This doc is the design reference. For numbers and release context see `CHANGELOG.md` ("Verdict engine — calibrated AI-surface findings with cross-file evidence"). For day-to-day use see the `terrain ai findings` section in `README.md`.

## Why a verdict engine

Pre-0.2 AI surface detection was a single-pass regex scan: if a file imported `openai` and called `.chat.completions.create`, flag it. Two problems:

1. **Single-file blindness.** The flagged file may have its eval coverage in a *sibling* file — the regex doesn't see the sibling, so it labels a covered surface as uncovered. On the labeled corpus this was the dominant FP class (≈70%).
2. **No confidence.** Every emission was treated as equally likely to be a real eval gap. The user couldn't choose between observability (broad surfacing) and gate (high-precision) without rebuilding the detector.

The verdict engine solves both by making the detector compositional: every signal is a typed atom with a calibrated weight, and a single composer turns the atom set into a sigmoid confidence the posture threshold can cut against.

## The pipeline shape

```
candidate ──► path-prefilter ──► regex-fastscan ──► ast-confirm ──► cross-file-scope ──► change-scope ──► composer ──► finding
                  │                  │                  │                  │                  │
              negative atoms     lexical atoms     structural atoms   scope atoms        scope atoms
              (drops noisy)     (SDK signatures)   (AST call sites)  (sibling evals)   (per-PR diff)
```

A `Candidate` is one (file, rule) tuple. Five stages run in order; any stage can short-circuit with `Continue: false` to drop the candidate before composition. The composer reads the accumulated atoms and the calibration table, computes `confidence = sigmoid(baseRate + Σ weight_i)`, and applies the posture threshold (`observability ≥ 0.40`, `gate ≥ 0.80`).

### Stage 1: path-prefilter

Hard-drops examples/tests/cookbooks. Soft-negative for `providers/`, `adapters/`, `connectors/`, `integrations/`, `readers/`, `loaders/`, `writers/`, suffix-style wrappers (`*_client`, `*_provider`, `*_wrapper`), and known utility filenames (`base.py`, `types.py`, `errors.py`, etc.). Strong negative atoms here give the composer a wide gate against framework/library code.

### Stage 2: regex-fastscan

Multi-language SDK anchor detection. For each known LLM/ML pair (OpenAI, Anthropic, LangChain, LlamaIndex, LangGraph, HuggingFace, Google GenAI, OpenAI-compat for Replicate/Cohere/Mistral/Groq/Together/Fireworks, plus the ML-training families), the stage requires *both* the import anchor and a verb (`chat.completions.create`, `.messages.create`, `.invoke`, `.fit(X, y)`, ...). Matching only the import emits a negative `regex.import_without_call` atom — the regex-derived version of the "no real call" gate.

The stage also emits the production-context atoms used by the training detector (`regex.production_ml_sdk`, `regex.scheduling_decorator`, `regex.model_registry_register`) and the framework-integration negatives (`regex.multi_framework` when 3+ ML frameworks co-import, `path.framework_integration` for `/integrations/llms/...` paths).

### Stage 3: ast-confirm

Tree-sitter parse for Python/JS/TS/Go/Java. When the regex stage flagged an LLM anchor, the AST stage either confirms with `ast.bound_call` (resolved call site, line number captured) or emits the strong negative `ast.no_call_despite_regex` when the regex matched but no call site is reachable. This is the precision-stack's main negative gate.

ML-training detection deliberately skips the AST verifier — sklearn/keras/pytorch training calls don't have a clean AST signature and the negative gate would suppress every training TP if scoped to them.

The `ast.bound_call` atom is capped at one per file even when AST resolves many call sites. Multiple sites in the same file are correlated evidence; compounding them linearly drowns out negatives on provider-wrapper files (which can have 10+ internal calls).

### Stage 4: cross-file-scope

The production `FSResolver` (in `internal/aipipeline/stages/cross_file_fs.go`) walks the candidate's directory and package looking for eval-framework imports — pytest, vitest, deepeval, ragas, promptfoo, mlflow, wandb, tensorboard, langsmith, trulens, jest, mocha, ava, playwright. When a sibling or package-mate fires, the candidate gets `scope.sibling_has_eval` (-1.8) or `scope.package_has_eval` (-1.4). This is the lever that handles the FP-eval-elsewhere class.

The resolver caches per-directory scan results as a *set of marker-bearing basenames* rather than a single bool. Caching just the bool was a real bug: the first candidate's self-exclusion poisoned every subsequent candidate's lookup in the same directory.

`findPackageRoot` walks upward from the candidate looking for a package marker (`package.json` for JS/TS, `__init__.py` chain for Python) and is bounded by the repo root via `pathInside` — without that bound it escaped to `/` on synthetic single-file repos and walked the entire filesystem.

### Stage 5: change-scope

Per-PR diff intersection. When a `DiffContext` is attached to the candidate, the stage emits `scope.diff_touched_file` (+0.8) for any matching path and `scope.diff_touched_line` (+1.4) when an atom's line span lands inside the diff. A helper `AddPRRemediation` lets the caller signal that the PR *itself* added the missing artifact (e.g. an evals/qa.yaml dropped in the same change) — that fires the strong negative `scope.diff_added_pr_evidence` (-1.5) so the verdict is suppressed.

In observability mode without a PR, the candidate's `Diff` field is nil and this stage emits no atoms. The verdict relies on the prior stages.

## Atom typing

Every atom is one of:

| Kind | Meaning | Examples |
|---|---|---|
| `lexical` | Regex co-occurrence / call-site shape | `regex.openai.import`, `regex.sklearn_train.call` |
| `structural` | AST-derived | `ast.bound_call`, `ast.module_level_call` |
| `topological` | Cross-file relationships | `topo.imported_by_app_module` (planned) |
| `scope` | Per-PR / per-file scope | `scope.diff_touched_line`, `scope.sibling_has_eval` |
| `shape` | Repo-level | `shape.is_application`, `shape.is_library` |
| `negative` | Suppressing signal | `path.examples`, `wrapper.class.match`, `ast.no_call_despite_regex` |

Each atom carries a `RuleID` (dotted namespace), a `Weight` (log-odds; the composer may override via calibration), a `Source` (which stage emitted), and a `Span` (line + snippet for evidence rendering).

## Composer + calibration

`Composer.Compose` returns:

```
confidence = sigmoid(baseRate(cohort, rule) + Σ atomWeight(cohort, rule, atom))
severity   = severityForRule × confidenceModulation
```

Calibration is a four-level lookup with `(cohort, rule)` specificity, falling back through `(cohort, "*")` → `("*", rule)` → `("*", "*")`. The `"*"` row holds the universal weights; per-cohort and per-rule overrides handle the cases where signal direction is rule-specific (e.g. production-context atoms are positive for `ai.train.missing_tracker`, neutral for `ai.surface.missing_eval`).

Per-cohort base rates encode "how likely is *anything* a TP in this cohort." The 0.2 corpus showed app-shape (RAG-app, agent-app, ai-feature-in-app) and library-sdk had nearly-identical natural TP rates (~2%), so the library-sdk base rate was corrected from -4.5 to -3.5 (matching unknown). This recovered ~13 TPs that the prior calibration suppressed.

A `TestCalibrationCoversAllRegexAtoms` regression test in `internal/aipipeline/stages` enumerates every atom ID the regex stage emits and asserts the calibration table has an explicit entry. This catches the bug class fixed mid-session: original calibration keys were `regex.sklearn.train` but the regex stage emitted `regex.sklearn_train.call` — overrides never resolved and the hand-tuned weights were dead code.

## Production-context training detector

The `ai.train.missing_tracker` rule at face value flags every `sklearn.fit(X, y)`. The labeled corpus shows that's noise (~2% precision: most flagged rows are tutorials, kaggle exports, research code). The detector is now *gated* on the production-context atoms:

- `regex.production_ml_sdk` (+1.8): imports of `sagemaker`, `azureml`, `vertexai`, `bentoml`, `kserve`, `torchserve`, `triton`, `mlflow.deployments`. Deliberately excludes Ray and Metaflow — those show up in research code too.
- `regex.scheduling_decorator` (+1.5): `@airflow.task`, `@prefect.flow`, `@dagster.asset`, `@ray.remote` (namespaced), plus bare `@task` / `@flow` / `@asset` when the corresponding framework is imported in the same file.
- `regex.model_registry_register` (+1.2): `mlflow.register_model`, `mlflow.log_model`, `bentoml.save_model`, `wandb.Artifact(..., type='model')`. Bare names like `register_model(` are deliberately *not* matched — they false-fire on framework source code that defines methods with that name (xgboost, RayDP, shorttext).

Without one of these signals, a sklearn-shaped training file stays at log-odds -3.5 + 0.4 + 1.5 = -1.6 → confidence 0.17, below the observability threshold. With production_ml_sdk it lifts to confidence 0.48 and emits.

The architectural goal: flag *production* training without tracking; leave research and tutorial code alone. The labeled corpus has only 1 TP for `ai.train.missing_tracker` so the empirical validation of this gating is deferred to the corpus expansion that targets the 0.3 cycle.

## Posture thresholds

| Posture | Threshold | Use case |
|---|---|---|
| `observability` | confidence ≥ 0.40 | Default. Wide surfacing — anything that clears the floor is shown. |
| `gate` | confidence ≥ 0.80 | Strict CI gate. Only high-confidence findings fail the build. |

Both are calibrated against the 0.2 corpus. `observability` corresponds to the 13% precision / 55% recall point; `gate` corresponds to ~17% precision / 46% recall (best-F1 territory). Threshold values are configurable per (posture, rule) in the calibration table when a rule needs a different operating point.

## Fix scaffolds

When a finding emits, the composer attaches a per-rule fix scaffold via `fixscaffold.Registry`:

- `ai.surface.missing_eval` → Promptfoo eval YAML, DeepEval pytest, or a metric-tracker pytest skeleton, language-appropriate.
- `ai.train.missing_tracker` → mlflow / wandb tracker wrapping the existing training call.
- `ai.prompt_file_missing_eval` → a Promptfoo `tests:` block referencing the prompt path.

The scaffold is a string + target path; the caller (terrain CLI today, IDE extension future) decides where to drop the file.

## Operation modes

### Validation (`cmd/terrain-pipeline`)

Replays the labeled corpus through the pipeline and reports precision/recall/F1 plus per-cohort and per-rule breakdowns. Six subcommands cover the full diagnostic workflow:

- `validate` — overall report at a fixed threshold.
- `debug` — per-row inspection with `--filter missed-tps | emitted-fps | all`.
- `tune` — threshold sweep with `--by-cohort`.
- `atoms` — per-atom marginal TP rate, lift, calibration weight, and alignment ("POS confirmed", "NEG MISALIGNED").
- `cv` — k-fold cross-validation with Wilson 95% CI.
- `fit` — k-fold logistic-regression refit via batch gradient descent (honest out-of-sample check).

### Production (`cmd/terrain ai findings`)

Walks the repo root via `internal/aipiperun.RunRepo`, attaches the production `FSResolver`, and emits findings. Defaults to `ai.surface.missing_eval` at observability posture; flag surface allows `--rule`, `--posture`, `--json`.

## Extension points

- **New rule.** Add the rule ID to `Calibration.Severities` and `BaseRates`. Wire any new positive/negative atoms in the regex or AST stage. Add a fix scaffold to `internal/aipipeline/fixscaffold`. Run `terrain-pipeline atoms` to verify weights are aligned with empirical lift.
- **New SDK.** Add a `ctxPair` entry to `regex_fastscan.go` (name, anchor regex, verb regex). The `TestCalibrationCoversAllRegexAtoms` test enforces matching calibration entries for `<name>.import` and `<name>.call`.
- **New cohort.** Extend `cohort.go` with the detection signals and add a per-cohort base-rate row. Cohorts are discovered once per repo by `DetectCohortFromDir` and applied to every candidate in that run.
- **External calibration.** `LoadCalibration(path)` reads a JSON file; production deployments can ship a per-tenant calibration without rebuilding the binary.

## Known limits

- The labeled corpus has 52 TPs total (29 at observability threshold). Confidence intervals on the headline precision are wide; the Wilson 95% CI lower bound (11%) is still 4× the path-only baseline (2.72%), but tighter claims require the 0.3 corpus expansion.
- The production-context training reshape is empirically unvalidated for the same reason — 1 TP for `ai.train.missing_tracker`.
- Cross-file Stage 4 is single-pass: it scans the candidate's directory and the package directory once each. Deeper cross-package analysis (e.g. "this prompt template is referenced by these three services") is 0.3+ work.
- No suppression model. Repeat false positives need `.terrain/suppressions.yaml` (planned for 0.3). For now, raise the posture threshold or scope to specific rules.
