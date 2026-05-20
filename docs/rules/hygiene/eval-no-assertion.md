# terrain/hygiene/eval-no-assertion — Eval Without Assertion

> Auto-generated stub. Edit anything below the marker; the generator preserves it.

**Type:** `evalNoAssertion`  
**Domain:** ai  
**Default severity:** high  
**Status:** stable

## Summary

An eval test function runs to completion without any assertion / score / metric call. The test cannot detect regressions because it accepts any model output.

## Remediation

Add an assert / score check that fails when the eval output deviates from expectations.

## Evidence sources

- `structural-pattern`

## Confidence range

Detector confidence is bracketed at [0.80, 0.95] (heuristic today; calibrated against a labeled corpus over time).

<!-- docs-gen: end stub. Hand-authored content below this line is preserved across regenerations. -->

## 1. Summary

An eval test function runs to completion without any assertion / score / metric call. The test accepts any model output and therefore cannot detect regressions.

## 2. Severity & status

- **Tier:** stable
- **Default severity:** high
- **Stable since:** v0.2.0
- **Configurable via `terrain.yaml`:** yes — severity downgrade and path-level ignores supported (see §7)

## 3. What this catches

- `def test_summarize():` that calls the model and `print`s the response but never asserts on it
- An eval that constructs a metric object but never calls `.score()` or compares against a baseline
- A pytest function under `evals/` whose body is `pass` or a comment placeholder
- A function that catches all exceptions silently and reports success regardless

## 4. Why this matters

Eval tests that run-but-don't-check are worse than absent tests: they consume CI time, produce green CI runs, and create the illusion of coverage where there is none. The pattern is common in teams shifting from "manually look at the output" to automated evals — the harness gets wired up, but the assertion step gets deferred and never lands.

The rule fires on the structural shape, not on the runtime result. A test that fails its assertions is correctly working; a test that has none fires regardless of how the model behaves.

## 5. Detection mechanism

- **Approach:** Python AST walk via tree-sitter — `function_definition` nodes named `test_*` or `eval_*` in files under `eval/` / `evals/` / `evaluations/` / `__evals__/` / `benchmarks/` paths.
- **Languages supported:** Python.
- **Inputs consumed:** source bytes of files in eval-like paths.
- **Assertion vocabulary recognized:** Python `assert` statements (AST-level), unittest `self.assert*` methods, pytest `pytest.fail` / `pytest.skip`, framework calls `evaluator.score`, `deepeval.assert_*`, `ragas.evaluate`, `expect(...)`, `.score`, `metric.*`, `should.*`, `.toBe` / `.toEqual` for cross-framework JS-style helpers, `promptfoo` markers, `assert_response`.
- **Edge cases handled:** assertions inside helper functions called from the test count (the text-match path catches them); raw `assert` statements caught at the AST level so string literals containing `assert` don't trip the suppression.
- **Edge cases NOT handled at 0.2.0:** Python tests outside the eval path conventions; non-Python eval tests (Jest, Vitest under `evals/`) are future work for a JS/TS detector.

## 6. Worked example

```
error[terrain/hygiene/eval-no-assertion]: eval test "test_summarize_quality" has no assertion
  --> evals/summarize_test.py:1
   = help: Add an assert / score check that fails when the eval output deviates from expectations.
   = docs: https://github.com/pmclSF/terrain/blob/main/docs/rules/hygiene/eval-no-assertion
```

**Before:**

```python
# evals/summarize_test.py
def test_summarize_quality():
    response = call_model("summarize this")
    print(response)
```

**After:**

```python
# evals/summarize_test.py
def test_summarize_quality():
    response = call_model("summarize this")
    assert response.score > 0.8, f"score regressed: {response.score}"
```

## 7. Configuration

**Downgrade to warning:**

```yaml
rules:
  hygiene/eval-no-assertion: warning
```

**Ignore specific eval paths** (legacy harnesses being phased out):

```yaml
ignore:
  rules:
    hygiene/eval-no-assertion:
      - "evals/legacy/**"
```

## 8. False-positive characterization

- **Tests that delegate to a `validate(...)` helper** — the helper does the assertion but the test body doesn't. Mitigation: inline an `assert validate(...)` so the test body carries the assertion shape, or accept the warning.
- **Tests under `eval/` that aren't actually evals** (utility / fixture builders named `test_*`) — rename them to a non-`test_*` prefix, or move them out of the eval path.
- **Snapshot-based evals** that compare output against a recorded file via a framework helper Terrain doesn't recognize — extend `assertionShapes` or downgrade the rule.
- **Measured FP rate at last validation:** see the per-rule readiness card published with the release tag.

## 9. Reproducibility

```bash
terrain test --selector hygiene/eval-no-assertion
```

## 10. Stability commitment

This rule's ID, default severity, and detection mechanism are stable from v0.2.0.

## 11. Related rules

- `terrain/hygiene/no-assertions` — the same shape for non-eval tests
- `terrain/hygiene/weak-assertion` — fires when the test has an assertion but it's trivially passing
- `terrain/coverage/no-eval` — when an AI surface has no eval test at all
