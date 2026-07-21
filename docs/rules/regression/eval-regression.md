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

## Evidence sources

- `eval-execution`

## Confidence range

Confidence interval: 0.85–0.99.

<!-- docs-gen: end stub. Hand-authored content below this line is preserved across regenerations. -->

## 1. Summary

An eval metric (rubric score, accuracy, F1, AUC, RMSE, calibration error, etc.) regressed past the configured threshold compared to the base branch.

## 2. Severity & status

Experimental — off by default; enable in `terrain.yaml`. Default severity: high. Terrain ingests eval artifacts into eval runs, and the eval-data-aware AI detectors consume that data.

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

The rule lifecycle is **read baseline + current eval run → compare by case/run metric → threshold check**.

- **Artifact inputs:** promptfoo (`--promptfoo-results`), deepeval (`--deepeval-results`), ragas (`--ragas-results`), Great Expectations (`--great-expectations-results`); gauntlet via the JSON-format-compatible ingestion path.
- **Approach:** compare current and baseline eval-run records case-by-case, then run-level when case IDs do not match.
- **Base strategy:** cached snapshot by default, with rerun and CI-artifact strategies available.
- **Edge cases not handled:** multi-metric weighted comparison, stochastic confidence intervals, and cross-eval correlation analysis.

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

This rule's ID is reserved and stable. Default severity, behavior, and tunable-config schema remain experimental. Changes follow the normal deprecation contract:

- **Renames:** one-cycle deprecation. None planned.
- **Default threshold change:** treated as breaking.
- **`base_strategy` default change:** treated as breaking.
- **Adapter additions** (new eval-framework adapters): additive; documented in `CHANGELOG.md`.
- **Tunable-config additions** (new optional keys on the rule block): additive; `terrain.yaml` parsers tolerate unknown optional keys per the schema.

## 11. Related rules

- `terrain/regression/test-failed` — the code-test equivalent; same select-then-run shape, but for tests rather than evals
- `terrain/regression/snapshot-mismatch` — when an eval's *output* diverged from a committed snapshot (vs. a *metric* delta from this rule)
- `terrain/regression/baseline-not-set` — fires *instead* of this rule when no baseline exists to compare against
- `terrain/regression/pass-rate-drop` — sibling rule for multi-sample evals; delegates threshold logic to this rule's mechanism
- `terrain/regression/performance-regression` — same shape, for ML model metrics (accuracy/F1/AUC/RMSE) rather than LLM rubric scores
- `terrain/regression/calibration-degraded` — experimental rule for model calibration regressions (different metric family)
