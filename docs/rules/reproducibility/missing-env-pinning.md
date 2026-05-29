# terrain/reproducibility/missing-env-pinning — Missing Env Pinning

> Auto-generated stub. Edit anything below the marker; the generator preserves it.

**Type:** `missingEnvPinning`  
**Domain:** quality  
**Default severity:** medium  
**Lifecycle status:** stable  
**Gating tier:** observability

## Summary

An environment-variable read in eval / inference code lacks a default value. The same code produces different behavior depending on which environment runs it.

## Remediation

Supply a default — os.environ.get(KEY, "<pinned-value>") — or fail fast with a clear error message when the variable is absent.

## Evidence sources

- `structural-pattern`

## Confidence range

Confidence interval: 0.70–0.90.

<!-- docs-gen: end stub. Hand-authored content below this line is preserved across regenerations. -->

## 1. Summary

An environment-variable read in eval / inference code lacks a default value. The same code produces different behavior depending on which environment runs it.

## 2. Severity & status

- **Tier:** stable
- **Default severity:** medium
- **Stable since:** v0.2.0
- **Configurable via `terrain.yaml`:** yes — see [configuration.md](../../configuration.md)

## 3. What this catches

- `os.environ["MODEL"]` in an eval file — raises `KeyError` if unset, but more importantly: the eval's result depends on the runtime value
- `os.environ.get("MODEL")` — returns `None` silently and the eval keeps running with bad inputs
- `os.getenv("MODEL")` (same shape, different module) — same risk
- A notebook in `experiments/` that reads `os.environ["DATASET_VERSION"]` to choose a fixture file

## 4. Why this matters

Eval and inference paths must produce the same output across runs to be reproducible. Reading from `os.environ` without a default makes the eval's behavior depend on whichever shell variable happens to be set in the calling environment — CI vs local vs production. A test that "works on my machine" because the developer's shell has `MODEL=gpt-4o-mini` but fails in CI because the CI runner doesn't is a classic shape; the diff doesn't show a difference, but the runtime behavior diverges.

The rule fires on eval / inference / training paths only — application config code is exempt because unset environment variables there are intentionally part of the configuration surface.

## 5. Detection mechanism

- **Approach:** Python AST walk.
- **Paths considered:** `/eval/`, `/evals/`, `/evaluations/`, `/__evals__/`, `/train/`, `/training/`, `/models/`, `/notebooks/`, `/experiments/` (same set as `terrain/reproducibility/no-seed`).
- **Read shapes recognized:**
  - `os.environ["KEY"]` and `environ["KEY"]` subscript form
  - `os.environ.get("KEY")` / `environ.get("KEY")` / `os.getenv("KEY")` / `getenv("KEY")` without default
- **Suppression:** the `get()` / `getenv()` calls suppress when a second positional argument or a `default="..."` kwarg is present.
- **Edge cases NOT handled at 0.2.0:** envs read into a config object that's later consulted; the rule only fires at the read site.

## 6. Worked example

```
warning[terrain/reproducibility/missing-env-pinning]: env "MODEL" read without default
  --> evals/inference.py:4
   = help: Supply a default — os.environ.get("MODEL", "<pinned-value>") — or declare the variable as required at the top of the file and fail fast with a clear error message.
   = docs: https://github.com/pmclSF/terrain/blob/main/docs/rules/reproducibility/missing-env-pinning
```

**Before:**

```python
import os
def get_model():
    return os.environ["MODEL"]
```

**After:**

```python
import os
def get_model():
    return os.environ.get("MODEL", "gpt-4o-mini")
```

## 7. Configuration

```yaml
rules:
  reproducibility/missing-env-pinning: low
ignore:
  rules:
    reproducibility/missing-env-pinning:
      - "evals/dev/**"
```

## 8. False-positive characterization

- **Env read in a settings module that's imported by the eval file** — the rule fires at the read site (the settings module), which is correct but may not match where the eval author wants to suppress. Mitigation: move the env read into the eval file with an explicit default, or ignore via path.
- **Env vars read via `os.environ.get` with a runtime-computed default** — typically a `dict.get(...)` lookup. Not suppressed (the rule needs a literal). Mitigation: inline the literal.
- **Measured FP rate at last validation:** see the per-rule readiness card.

## 9. Reproducibility

```bash
terrain test --selector reproducibility/missing-env-pinning
```

## 10. Stability commitment

Rule ID, severity, and the eval-path / read-shape vocabulary are stable from v0.2.0.

## 11. Related rules

- `terrain/reproducibility/version-floating` — same family for source dependencies
- `terrain/reproducibility/no-seed` — same family for stochastic libraries
- `terrain/hygiene/model-fixture-unpinned` — same family for saved model artifacts
