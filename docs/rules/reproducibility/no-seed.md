# terrain/reproducibility/no-seed — Missing Random Seed

> Auto-generated stub. Edit anything below the marker; the generator preserves it.

**Type:** `noSeed`  
**Domain:** ai  
**Default severity:** medium  
**Lifecycle status:** experimental  
**Gating tier:** observability

## Summary

Stochastic library call (np.random / torch / random / tf.random) in an eval or training file without a preceding seed call. Run-to-run results vary, masking real regressions.

## Remediation

Add a seed call at module scope or in a pytest fixture (np.random.seed(42), torch.manual_seed(42), or transformers.set_seed(42)).

## Evidence sources

- `structural-pattern`

## Confidence range

Confidence interval: 0.75–0.90.

<!-- docs-gen: end stub. Hand-authored content below this line is preserved across regenerations. -->

## 1. Summary

A stochastic library call (`np.random.*`, `torch.rand*`, `random.*`, `tf.random.*`) appears in an eval / training file without a preceding seed call. Run-to-run results vary, masking real regressions.

## 2. Severity & status

Experimental — off by default; enable in `terrain.yaml`.

- **Default severity:** medium
- **Configurable via `terrain.yaml`:** yes — see [configuration.md](../../configuration.md)

## 3. What this catches

- `np.random.rand(100)` in `evals/quality.py` without an earlier `np.random.seed(...)` in module scope
- `torch.manual_seed` missing before a training loop that uses `torch.randn`
- A pytest fixture that doesn't reset the RNG between cases
- HuggingFace `transformers.set_seed` covers numpy/torch/random simultaneously — only un-set-seeded files fire

## 4. Why this matters

Stochastic evals are the standard case in modern ML: bootstrap sampling for confidence intervals, Monte Carlo evaluation, LLM judge calls with temperature > 0. The result is reproducible only when the seed is set; without a seed, the same eval can produce a different score across CI runs of identical code, which:

1. Masks real regressions (the eval scored differently for unrelated reasons).
2. Trips the eval-regression rule's noise-tolerance heuristics.
3. Erodes adopter trust in the eval suite ("it failed for no reason").

The rule fires once per file at the first un-seeded call. It's a structural check — the rule has no view into whether the eval is "actually" stochastic at runtime, only whether the seed is set in the source.

## 5. Detection mechanism

- **Approach:** Python AST walk via tree-sitter, in-order traversal that tracks which libraries have been seeded as it visits nodes.
- **Languages supported:** Python.
- **Files considered:** any path containing `/eval/`, `/evals/`, `/evaluations/`, `/__evals__/`, `/train/`, `/training/`, `/models/`, `/notebooks/`, or `/experiments/`.
- **Seed primitives recognized:** `np.random.seed`, `numpy.random.seed`, `torch.manual_seed`, `torch.cuda.manual_seed*`, `random.seed`, `tf.random.set_seed`, `tensorflow.random.set_seed`, HuggingFace `set_seed` (covers numpy / torch / random).
- **Stochastic primitives recognized:** `np.random.*`, `numpy.random.*`, `torch.rand` / `torch.randn` / `torch.randint`, `random.random` / `random.choice` / `random.uniform`, `tf.random.*` / `tensorflow.random.*`.
- **Edge cases handled:** `transformers.set_seed()` is treated as setting numpy / torch / random simultaneously; one signal per file at the first un-seeded site.
- **Edge cases not handled:** seed calls inside pytest fixtures whose autouse scope covers the test function — flagged with a false positive; suppress via `# terrain:disable` or accept the warning.

## 6. Worked example

```
warning[terrain/reproducibility/no-seed]: stochastic call into "numpy" without preceding seed
  --> evals/quality.py:4
   = help: Add a seed call (np.random.seed(42)) at module scope or in a pytest fixture.
   = docs: https://github.com/pmclSF/terrain/blob/main/docs/rules/reproducibility/no-seed
```

**Before:**

```python
import numpy as np
def eval_quality():
    samples = np.random.rand(100)
    return samples.mean()
```

**After:**

```python
import numpy as np

np.random.seed(42)

def eval_quality():
    samples = np.random.rand(100)
    return samples.mean()
```

## 7. Configuration

```yaml
rules:
  reproducibility/no-seed: warning
ignore:
  rules:
    reproducibility/no-seed:
      - "experiments/**"  # research code, intentionally stochastic
```

## 8. False-positive characterization

- **Pytest autouse fixture seeds the RNG** — the rule doesn't trace fixture scope; mitigation is to add a module-scope seed redundantly, or accept.
- **Cryptographic randomness** (`secrets`, `os.urandom`) — not flagged; the rule's vocabulary deliberately excludes crypto sources.
- **Notebooks vs scripts** — notebooks under `/notebooks/` are flagged. Adopters who treat notebooks as exploratory ignore via path.

## 9. Reproducibility

```bash
terrain test --selector reproducibility/no-seed
```

## 10. Stability commitment

Rule ID, default severity, and library / path matchers are stable.

## 11. Related rules

- `terrain/reproducibility/version-floating` — unpinned dependencies, same family
- `terrain/reproducibility/missing-env-pinning` — unpinned environment variables
- `terrain/hygiene/eval-no-assertion` — sister rule for eval quality
