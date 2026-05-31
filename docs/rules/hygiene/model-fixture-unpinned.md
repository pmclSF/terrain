# terrain/hygiene/model-fixture-unpinned — Model Fixture Unpinned

> Auto-generated stub. Edit anything below the marker; the generator preserves it.

**Type:** `modelFixtureUnpinned`  
**Domain:** ai  
**Default severity:** high  
**Lifecycle status:** experimental  
**Gating tier:** gate

## Summary

A model-loading call (from_pretrained / torch.load / joblib.load / load_model / snapshot_download) uses a path or revision that isn't content-addressed. The underlying weights may change without a code edit, regressing eval scores silently.

## Remediation

Pin the load to a commit SHA (revision="<sha>" for HuggingFace), a version-suffixed filename (model_v3.0.pt), or a .safetensors-with-checksum format.

## Promotion plan

Off by default. Detector function exists at internal/hygiene/model_fixture_unpinned.go (DetectModelFixtureUnpinned). Pipeline integration pending: the detector's input shape is not yet fed through the engine registry. Stays at experimental until that wiring lands. Opt in via `.terrain/policy.yaml` only after pipeline integration lands.

## Evidence sources

- `structural-pattern`

## Confidence range

Confidence interval: 0.85–0.95.

<!-- docs-gen: end stub. Hand-authored content below this line is preserved across regenerations. -->

## 1. Summary

A model-loading call uses a path or revision that isn't content-addressed. The underlying weights may change without a code edit, regressing eval scores silently.

## 2. Severity & status

- **Tier:** stable
- **Default severity:** high
- **Stable since:** v0.2.0
- **Configurable via `terrain.yaml`:** yes — see [configuration.md](../../configuration.md)

## 3. What this catches

- `AutoModelForCausalLM.from_pretrained("mistralai/Mistral-7B-v0.1")` — no revision pin; HuggingFace Hub `main` branch can move
- `from_pretrained(..., revision="main")` — `main` is a branch head, not a commit
- `torch.load("model.pt")` — filename doesn't encode a version or hash; the file at that path can be replaced
- `joblib.load("classifier.joblib")` — same shape; sklearn pickle without versioning
- `huggingface_hub.snapshot_download("user/repo")` — no revision pin

## 4. Why this matters

Model weights are the AI system's behavioral substrate. Two engineers running the same code can get materially different production behavior if the underlying `.pt` or HuggingFace Hub artifact shifted between their two checkouts. The bug presents as "the eval used to pass — what changed?" with no code diff to explain it. The rule operationalizes the same discipline as `terrain/reproducibility/version-floating` does for source dependencies: changes that affect runtime behavior must be visible at the source level.

## 5. Detection mechanism

- **Approach:** Python AST walk for `call` nodes whose function text matches a known model-loading primitive.
- **Recognized loaders:**
  - `*.from_pretrained(...)` — HuggingFace family (transformers, sentence-transformers, diffusers)
  - `torch.load(path)` — PyTorch checkpoint
  - `joblib.load(path)` — sklearn pickle
  - `load_model(path)` — Keras / TensorFlow
  - `snapshot_download(...)` — `huggingface_hub`
- **Pinning suppression:**
  - HuggingFace: `revision="<value>"` kwarg where value is NOT `main` / `master` / `latest` / `head` (case-insensitive).
  - Path-based loaders: first positional argument is a string literal that looks pinned (has a 7+-hex segment, a `v<major>.<minor>` version, or ends in `.safetensors`).
- **Edge cases NOT handled at 0.2.0:** revision passed via a variable rather than a literal (the rule can't prove the variable's value statically); custom loader wrappers that the rule doesn't recognize.

## 6. Worked example

```
error[terrain/hygiene/model-fixture-unpinned]: *.from_pretrained loads a model without a content-addressed pin
  --> src/load.py:2
   = loader: *.from_pretrained
   = help:   Pass revision="<commit-sha-or-version>" to from_pretrained. Avoid revision="main" / "master" — they're branch heads.
   = docs:   https://github.com/pmclSF/terrain/blob/main/docs/rules/hygiene/model-fixture-unpinned
```

**Before:**

```python
from transformers import AutoModelForCausalLM
model = AutoModelForCausalLM.from_pretrained("mistralai/Mistral-7B-v0.1")
```

**After:**

```python
from transformers import AutoModelForCausalLM
model = AutoModelForCausalLM.from_pretrained(
    "mistralai/Mistral-7B-v0.1",
    revision="abc1234def5678",
)
```

## 7. Configuration

```yaml
rules:
  hygiene/model-fixture-unpinned: warning
ignore:
  rules:
    hygiene/model-fixture-unpinned:
      - "experiments/**"  # research code, pinning policy applied later
```

## 8. False-positive characterization

- **Internal model registry that bypasses HuggingFace conventions** — if the load goes through a wrapper the detector doesn't recognize, it doesn't fire (false negative side). Mitigation: extend the wrapper to call the recognized primitive directly, or accept.
- **Variable-bound revision** — `revision=cfg.model_revision` doesn't statically resolve; the rule fires. Mitigation: inline the literal at the call site, or ignore via path.
- **Measured FP rate at last validation:** see the per-rule readiness card.

## 9. Reproducibility

```bash
terrain test --selector hygiene/model-fixture-unpinned
```

## 10. Stability commitment

Rule ID, severity, recognized loader list, and pinning vocabulary are stable from v0.2.0. New loader patterns are additive and not deprecation-cycled.

## 11. Related rules

- `terrain/reproducibility/version-floating` — same family but for source dependencies
- `terrain/security/insecure-deserialization` — pickle / torch.load is also a security concern; this rule and that one usually fire together when both targets match
- `terrain/coverage/no-eval` — when the loaded model has no eval covering it
