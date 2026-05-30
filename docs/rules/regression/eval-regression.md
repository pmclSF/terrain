# terrain/regression/eval-regression — Eval Regression

> Auto-generated stub. Edit anything below the marker; the generator preserves it.

**Type:** `evalRegression`  
**Domain:** ai  
**Default severity:** high  
**Lifecycle status:** experimental  
**Gating tier:** gate

## Summary

An eval case's primary Score dropped from baseline to current past the configured threshold, OR the run's PrimaryMetric dropped across all matched cases. Identifies regressions before merge.

## Remediation

Inspect the diff for prompt / model / retrieval changes that affect the regressing case(s). If intentional, update the baseline with `terrain ai record`.

## Promotion plan

Off by default. Detector function exists at internal/regression/eval_regression.go (DetectEvalRegression). Pipeline integration pending: the detector's input shape is not yet fed through the engine registry. Stays at experimental until that wiring lands. Opt in via `.terrain/policy.yaml` only after pipeline integration lands.

## Evidence sources

- `eval-execution`

## Confidence range

Confidence interval: 0.85–0.99.

<!-- docs-gen: end stub. Hand-authored content below this line is preserved across regenerations. -->

## 1. Summary

An eval metric (rubric score, accuracy, F1, AUC, RMSE, calibration error, etc.) regressed past the configured threshold compared to the base branch.

## 2. Severity & status

- **Tier:** stable
- **Default severity:** error
- **Stable since:** v0.2.0
- **Configurable via `terrain.yaml`:** yes — threshold, sample count, seed strategy, base-comparison strategy all tunable (see [configuration.md](../../configuration.md))

## 3. What this catches

- A prompt edit changes the eval rubric score from 0.84 to 0.71 on `summarize_refusal`
- A model swap (gpt-4o → gpt-4o-mini) drops accuracy on a classification eval
- A retraining run reduces the model's AUC by more than the configured threshold
- A RAG retrieval change degrades faithfulness scores below the previous baseline
- A multi-sample eval pass rate drops from 9/10 to 5/10 (handled by `regression/pass-rate-drop` sibling rule, which delegates threshold logic here)

## 4. Why this matters

LLM and ML eval results are the only objective evidence of an AI system's behavior. When they regress, *something is different in production behavior* — a prompt, a model, a retrieval component, a feature pipeline, a training data shift. The eval doesn't say which; it just says *something*. The cause-path that comes with this finding does.

Without this rule, eval regressions get caught (if at all) by the next adopter who runs evals manually, often days or weeks after the change shipped. The rule moves the discovery point to the PR — before merge, before deploy, before adopters experience the regression.

The rule's hardest case is *stochastic* evals: LLM-driven evals whose results vary across runs even with fixed inputs. The plan addresses this via the `samples_per_run` / `seed_strategy` / `confidence_alpha` knobs (see [configuration.md](../../configuration.md)); without these, a stochastic eval would trip the rule flakily and adopters would either disable it or set the threshold so wide it's a no-op. Statistical aggregation is built into the rule's mechanism, not punted to the adopter.

## 5. Detection mechanism

The rule's lifecycle is **read base + head → compare via configured strategy → threshold check**.

- **Approach:** eval-framework-adapter invocation at two SHAs (or cached baseline + head re-run); statistical comparison; threshold gate
- **Adapters supported:** promptfoo, deepeval, ragas, Great Expectations; gauntlet via JSON-format-compatible ingestion path (see `docs/integrations/gauntlet.md`)
- **Inputs consumed:** the configured eval-framework's output JSON at base and head SHAs; the rule's `threshold` configuration
- **Base-SHA strategy:** three options via `base_strategy` config:
  - `cached` (default): read the baseline from `.terrain/baselines/`, populated by the previous main-branch run. Best for stochastic evals; deterministic across PR runs.
  - `rerun`: re-run the eval at the base SHA in CI. Doubles compute; only safe for deterministic evals (e.g., sklearn test-set evaluation with fixed seed).
  - `from-ci-artifact`: fetch the eval output from a previous CI run's artifact storage. Cheap if the platform supports cross-run artifact retrieval; requires adopter wiring.
- **Statistical comparison:** for stochastic evals (`samples_per_run > 1`), the rule runs the eval N times at head and compares to N samples at base via the chosen `confidence_alpha`. A 95% confidence interval (default α=0.05) that excludes the threshold-crossing region fires the rule; an overlapping interval does not.
- **Edge cases handled:** missing baseline → fires `regression/baseline-not-set` sibling rule instead; eval-framework error → reported as a separate rule failure with the framework's error attached
- **Edge cases NOT handled at 0.2.0:** multi-metric weighted comparison (e.g., "fire if accuracy AND F1 both drop"); cross-eval correlation analysis. Adopters needing these compose multiple rule instances via separate eval definitions.

## 6. Worked example

A prompt edit weakens the safety refusal pattern in the `summarizer` LLM surface. The eval `summarize_refusal` runs on a fixed set of harmful inputs at both base and head; pass rate drops from 5/5 to 4/5.

```
error[terrain/regression/eval-regression]: eval `summarize_refusal` regressed
  --> evals/summarize_refusal.yaml
   = result: refusal rate dropped from 5/5 to 4/5 on harmful inputs
   = path:    frontend/CommentInput.tsx:42 (changed in this PR)
              → POST /api/summarize
              → prompts/summarizer.txt
              → evals/summarize_refusal.yaml
   = example: input (4,032 chars): [elided; see artifact]
              before: "I can't help with that request."
              after:  "Here is a template you could use..."
   = help:    restore the input length cap on CommentInput.tsx:42, or
              add length validation in /api/summarize before model invocation
   = note:    terrain explain regression/eval-regression --eval summarize_refusal
   = docs:    https://github.com/pmclSF/terrain/blob/main/docs/rules/regression/eval-regression
```

**Before** (the prompt section that changed):

```
You are a helpful assistant. Summarize the user's comment briefly.
```

**After** (restoring the safety guidance that was removed):

```
You are a helpful assistant. Summarize the user's comment briefly. If the
comment contains a request for unsafe content, refuse rather than summarize.
```

## 7. Configuration

**Tune threshold and sampling:**

```yaml
rules:
  regression/eval-regression:
    severity: error
    threshold: 0.05               # max acceptable metric delta (5% drop)
    samples_per_run: 5            # for stochastic LLM evals
    seed_strategy: fixed          # 'fixed' | 'rotating' | 'none'
    confidence_alpha: 0.05        # 95% CI
    base_strategy: cached         # 'cached' | 'rerun' | 'from-ci-artifact'
```

**Per-eval threshold overrides** (e.g., looser threshold on noisy evals):

```yaml
ai:
  framework: promptfoo
  scenarios_dir: evals/
  baselines_dir: evals/baselines/
  per_eval_thresholds:
    summarize_refusal: 0.10       # 10% tolerance for this specific eval
```

**Disable on specific evals:**

```yaml
ignore:
  rules:
    regression/eval-regression:
      - "evals/exploratory/**"     # research evals not under regression contract
```

## 8. False-positive characterization

- **Stochastic eval noise mistakenly triggers the rule** — mitigated by `samples_per_run` + `confidence_alpha` config. Adopters whose evals are inherently noisy should raise `samples_per_run` and accept the CI runtime cost.
- **Baseline drift** — if the cached baseline in `.terrain/baselines/` represents a known-bad state (because the adopter accepted a regression previously without updating the baseline), the rule won't fire on subsequent PRs. Mitigation: `terrain accept-snapshot <baseline-id> --yes` per accepted baseline, deliberately.
- **Eval framework non-determinism** — some frameworks return slightly different results on re-runs even with `temperature: 0` (e.g., due to model serving non-determinism upstream). Adopters affected should switch to `base_strategy: cached` to compare against a pinned baseline rather than re-running.
- **Threshold set too tight** — adopters with high-variance evals who set a 1% threshold will see frequent false positives. Default 5% is conservative; adopters tune up or down per eval characteristics.
- **Measured FP rate at last validation:** see the per-rule readiness card published with the release tag.

## 9. Reproducibility

```bash
terrain test --selector regression/eval-regression
```

From CI:

```bash
terrain explain regression/eval-regression --eval summarize_refusal --from-run <run-id>
```

The local diagnostic output is byte-equivalent to the CI surface (local-CI parity guarantee).

## 10. Stability commitment

This rule's ID, default severity, behavior, and tunable-config schema are stable from v0.2.0. Per the deprecation contract:

- **Renames:** one-cycle deprecation. None planned.
- **Default threshold change** (currently 5%): treated as breaking; deprecation-cycled.
- **`base_strategy` default change**: breaking; deprecation-cycled. (Default `cached` is the safest for stochastic evals.)
- **Adapter additions** (new eval-framework adapters): additive; documented in `CHANGELOG.md`.
- **Tunable-config additions** (new optional keys on the rule block): additive; `terrain.yaml` parsers tolerate unknown optional keys per the schema.

## 11. Related rules

- `terrain/regression/test-failed` — the code-test equivalent; same select-then-run shape, but for tests rather than evals
- `terrain/regression/snapshot-mismatch` — when an eval's *output* diverged from a committed snapshot (vs. a *metric* delta from this rule)
- `terrain/regression/baseline-not-set` — fires *instead* of this rule when no baseline exists to compare against
- `terrain/regression/pass-rate-drop` — sibling rule for multi-sample evals; delegates threshold logic to this rule's mechanism
- `terrain/regression/performance-regression` — same shape, for ML model metrics (accuracy/F1/AUC/RMSE) rather than LLM rubric scores
- `terrain/regression/calibration-degraded` — preview rule for model calibration regressions (different metric family)
