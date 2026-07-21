# terrain/security/insecure-deserialization — Insecure Deserialization

> Auto-generated stub. Edit anything below the marker; the generator preserves it.

**Type:** `insecureDeserialization`  
**Domain:** ai  
**Default severity:** critical  
**Lifecycle status:** experimental  
**Gating tier:** gate

## Summary

A call into an unsafe deserialization primitive (pickle.load, torch.load without weights_only=True, joblib.load, yaml.load without SafeLoader, dill.load, marshal.load) executes arbitrary code on untrusted input.

## Remediation

Switch to a safe format (JSON, msgpack, safetensors, ONNX). When the primitive is unavoidable, declare the explicit safe option (weights_only=True for torch.load, Loader=SafeLoader for yaml.load).

## Evidence sources

- `structural-pattern`

## Confidence range

Confidence interval: 0.90–0.99.

<!-- docs-gen: end stub. Hand-authored content below this line is preserved across regenerations. -->

## 1. Summary

A call into an unsafe deserialization primitive — `pickle.load`, `torch.load` without `weights_only=True`, `joblib.load`, `yaml.load` without `SafeLoader`, `dill.load`, or `marshal.load` — executes arbitrary code on untrusted input.

## 2. Severity & status

Experimental — off by default; enable in `terrain.yaml`.

- **Default severity:** critical
- **Configurable via `terrain.yaml`:** yes — see [configuration.md](../../configuration.md)

## 3. What this catches

- `pickle.load(open(path, "rb"))` reading a model checkpoint from a configurable path
- `torch.load(path)` without the `weights_only=True` flag added in PyTorch 2.0
- `joblib.load(path)` (joblib is pickle-backed)
- `yaml.load(stream)` without `Loader=yaml.SafeLoader`
- `dill.loads(payload)` / `marshal.load(f)` in any code path

## 4. Why this matters

Pickle deserialization runs arbitrary Python code at load time. If an adversary controls the bytes — uploaded checkpoint, HuggingFace Hub artifact, S3 path with public write, model registry that wasn't authenticated — the deserialization call gives them code execution on the loading process. The pattern is the leading-cause CVE class for ML systems: every major ML framework has had at least one pickle-related CVE in the last three years.

The rule fires at the source-level call site rather than at runtime because the detection target is "this code reads adversary-controllable bytes through an unsafe primitive." A static signal is sufficient — the static fact is the operationally-actionable one.

## 5. Detection mechanism

- **Approach:** Python AST walk for `call` nodes whose function text matches a known unsafe-deserialization primitive.
- **Languages supported:** Python.
- **Recognized primitives:** `pickle.load(s)`, `cPickle.load`, `dill.load(s)`, `joblib.load`, `torch.load`, `yaml.load`, `marshal.load(s)`. Qualified forms (`pkg.pickle.load`) match via suffix.
- **Suppression rules:**
  - `torch.load(path, weights_only=True)` → suppressed (PyTorch ≥2.0)
  - `yaml.load(stream, Loader=yaml.SafeLoader)` or `Loader=SafeLoader` / `CSafeLoader` → suppressed
- **Edge cases handled:** suppression is statically resolvable — adopters who pass the safe option literally get the suppression; runtime-only safety options (e.g., a variable that holds `weights_only=True`) aren't recognized.
- **Edge cases not handled:** non-Python deserialization (`Marshal.load` in Ruby, `JSON.parse` reviver functions in JS).

## 6. Worked example

```
critical[terrain/security/insecure-deserialization]: pickle.load deserializes arbitrary code
  --> src/loader.py:5
   = primitive: pickle.load
   = help:      Replace with a safe format (JSON, msgpack, or safetensors). When pickle is unavoidable, sandbox the deserialization and authenticate the source.
   = docs:      https://github.com/pmclSF/terrain/blob/main/docs/rules/security/insecure-deserialization
```

**Before:**

```python
import pickle
def load_model(path):
    with open(path, "rb") as f:
        return pickle.load(f)
```

**After:**

```python
import safetensors.torch
def load_model(path):
    return safetensors.torch.load_file(path)
```

Or for unavoidable PyTorch loads:

```python
import torch
def load_model(path):
    return torch.load(path, weights_only=True)
```

## 7. Configuration

**Downgrade to high** (already-mitigated environment):

```yaml
rules:
  security/insecure-deserialization: high
```

**Ignore safe-loader paths** (artifacts produced internally that bypass authentication):

```yaml
ignore:
  rules:
    security/insecure-deserialization:
      - "src/internal_loaders/**"
```

## 8. False-positive characterization

- **Calls that read from a hardcoded, content-addressed local path** — technically the rule fires, but the practical risk is zero. Mitigation: ignore the specific file.
- **Test fixtures that build pickle fixtures intentionally** — `pickle.dumps` doesn't fire (it's the read side that's dangerous), but tests sometimes round-trip through `pickle.loads` deliberately. Suppress with a per-test ignore.
- **Suppression by runtime variable** — `torch.load(path, weights_only=safe_flag)` where `safe_flag` is True at runtime isn't recognized; the rule fires because static analysis can't prove the value. Mitigation: inline the constant.

## 9. Reproducibility

```bash
terrain test --selector security/insecure-deserialization
```

## 10. Stability commitment

Rule ID, severity, and recognized-primitive list are stable. Adding new primitives (`pickle5.load`, etc.) is additive and not deprecation-cycled.

## 11. Related rules

- `terrain/security/pii-in-eval` — sister security rule, fires on PII in eval datasets
- `terrain/hygiene/model-fixture-unpinned` — saved-model artifacts without content-addressed pin; same family as the "load from untrusted source" concern
- `terrain/reproducibility/version-floating` — moving pickle-format versions across pickle library upgrades are another reproducibility risk
