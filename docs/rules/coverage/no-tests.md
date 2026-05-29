# terrain/coverage/no-tests — No Tests for Code Unit

> Auto-generated stub. Edit anything below the marker; the generator preserves it.

**Type:** `noTestsForCodeUnit`  
**Domain:** quality  
**Default severity:** medium  
**Status:** stable

## Summary

A code unit (exported function / method / class) exists in the codebase but no test in the snapshot's dependency graph covers it. Untested code reaches production undetected when changed.

## Remediation

Add a test that imports the code unit and exercises its observable behavior. The rule defaults to exported symbols only; configure `include_private: true` to widen coverage.

## Evidence sources

- `graph-traversal`

## Confidence range

Confidence interval: 0.85–0.95.

<!-- docs-gen: end stub. Hand-authored content below this line is preserved across regenerations. -->

## 1. Summary

A code unit exists in the codebase but no test in the snapshot's dependency graph covers it.

## 2. Severity & status

- **Tier:** stable
- **Default severity:** warning
- **Stable since:** v0.2.0
- **Configurable via `terrain.yaml`:** yes — severity adjustment and per-path ignores supported (see [configuration.md](../../configuration.md))

## 3. What this catches

- A new function added in this PR has no test referencing it via imports
- An existing public function in the codebase has zero tests covering it
- A public method on an exported class has no test invoking it
- A handler / route that's reachable from production code paths has no test exercising it (in which case the more specific sibling rule `coverage/no-integration-test` may also fire)
- A class with public methods has tests for one method but not others

## 4. Why this matters

Untested code is the asymmetric risk surface of every codebase. A bug in untested code reaches production undetected; a bug in tested code is *more likely* (not guaranteed) to be caught. The asymmetry matters because adopters routinely *think* they have higher coverage than they do — coverage tools count lines exercised but not lines actually asserted on, and what gets exercised in tests skews toward happy-path execution.

This rule operationalizes the question "what code in this repo has no asserting test referencing it?" via graph traversal: code units with no incoming `covered_by_test` edge in the unified graph. Unlike per-line coverage metrics, this is symbol-level (function / method) and reflects the actual import-graph relationship, not just execution coincidence.

The rule's category default is **warning**, not **error**, because:
- Some unmoderated code is intentionally not tested (e.g., generated code, vendored third-party code, scripts)
- New code in a PR that hasn't *yet* had a test written is a transient state, not a blocking error
- The rule is a *coverage* rule, not a *regression* rule — it describes state, not a change

Adopters who want stricter behavior (e.g., "new public function must ship with a test in the same PR") upgrade severity to `error` in `terrain.yaml`. The catalog ships the more permissive default to keep adopter trust at first ship.

## 5. Detection mechanism

The rule's lifecycle is **enumerate code units → query graph for covering tests → emit findings for the empty set**.

- **Approach:** graph traversal — code units (`internal/models/code_unit.go`) with no incoming `covered_by_test` edge
- **Languages supported:** Go, JS/TS, Python, Java (per Terrain's `internal/testcase/` AST coverage; depends on Java import-graph linkage landing in Tier 0 to be honest for Java code)
- **Inputs consumed:** the snapshot's `CodeUnits[]`; the `ImpactGraph`'s reverse adjacency (which tests reach this unit)
- **Edge type queried:** `EdgeBucketCoverage` (test → code unit) and `EdgeExactCoverage` (test → specific symbol); reverse direction
- **What counts as a "test":** any node classified as a test by `internal/testtype/`; unit / integration / e2e / component / smoke all count
- **What counts as a "code unit":** function / method / class as enumerated by per-language AST extraction; trivially-named items (`init`, `main`, generated identifiers) are excluded via the filter list in `internal/analysis/code_unit.go`
- **Edge cases handled:** generated code (e.g., `*_pb.go`, `dist/`, `__generated__/`) excluded via path-default ignore; private/unexported items excluded by default at first ship (configurable; some adopters want private-method coverage as well)
- **Edge cases NOT handled at 0.2.0:** symbol-level coverage for languages with limited AST extraction (Java's reflection-heavy code paths can hide covering tests behind dynamic dispatch). Falls back to file-level "test imports source file" linkage in those cases; findings may be noisier on such codebases.

## 6. Worked example

A PR adds a new function `processRefund` in `src/api/handlers/refund.py` but the corresponding `tests/api/test_refund.py` file is empty.

```
warning[terrain/coverage/no-tests]: code unit has no covering test
  --> src/api/handlers/refund.py:14
14 | def process_refund(order_id: str, amount: Decimal) -> RefundResult:
   |     ^^^^^^^^^^^^^^ no test imports or references this function
   |
   = help: add a test in tests/api/test_refund.py that imports and exercises `process_refund`
   = note: this rule is default-warning; upgrade to error in terrain.yaml if your team's policy is "no untested code merges"
   = docs: https://github.com/pmclSF/terrain/blob/main/docs/rules/coverage/no-tests
```

**Before** (the empty / nonexistent test file):

```python
# tests/api/test_refund.py
# (empty file or missing)
```

**After** (minimum acceptable test that satisfies the rule):

```python
# tests/api/test_refund.py
from src.api.handlers.refund import process_refund
from decimal import Decimal

def test_process_refund_returns_refund_result():
    result = process_refund("order-123", Decimal("50.00"))
    assert result is not None
```

(The minimum test just satisfies the rule; rigorous testing covers happy-path + error cases + edge cases — but that's beyond what this rule mandates. The rule's bar is "a test exists and references the function," not "the test is rigorous." A subsequent rule, `hygiene/no-assertions`, catches assertion-free tests.)

## 7. Configuration

**Upgrade to error** (stricter team policy — "no untested code merges"):

```yaml
rules:
  coverage/no-tests: error
```

**Per-path ignores** (generated code, scripts, vendored):

```yaml
ignore:
  rules:
    coverage/no-tests:
      - "scripts/**"
      - "vendor/**"
      - "**/__generated__/**"
      - "**/dist/**"
```

**Include private/unexported items** (off by default):

```yaml
rules:
  coverage/no-tests:
    severity: warning
    include_private: true
```

## 8. False-positive characterization

- **Code that's deliberately untested** (e.g., a thin glue function whose behavior is fully captured by integration tests of the system it composes) — handle via per-path ignore in `terrain.yaml`. Coverage rules deliberately bias toward completeness over precision — they describe state, not per-PR judgments.
- **Code tested via integration tests that don't import the function directly** — the function is exercised but no test's import graph reaches it. The rule fires correctly per its mechanism (no incoming "covers" edge). Mitigation: either add a unit test, or accept the warning if integration-test-only coverage is the adopter's policy. The `coverage/no-integration-test` rule provides the complementary view.
- **Generated code** — Terrain excludes common generated-path patterns (`__generated__/`, `*.pb.go`, etc.) by default; uncommon generators may need explicit per-path ignores.
- **Code that's only reachable via reflection / dynamic dispatch** — the import-graph cannot trace dynamic dispatch. Such code may show as "no tests" even when invoked by tests at runtime. Mitigation: declare the surface in `terrain.yaml` `surfaces:` to make Terrain aware of the dynamic reachability, or accept the warning.
- **Measured FP rate at last validation:** see the per-rule readiness card published with the release tag (`harness/readiness/v0.2.0/coverage-no-tests.md`). Coverage rules typically have higher FP/false-relevance rates than detection rules; the 5% target FP-rate may be relaxed for this category — see the readiness card for measured values.

## 9. Reproducibility

```bash
terrain test --selector coverage/no-tests
```

The rule fires on the entire repo's code units, not just changed ones. To scope to changes only:

```bash
terrain test --selector coverage/no-tests --base $(git merge-base HEAD main) --head HEAD
```

## 10. Stability commitment

This rule's ID, default severity (warning), and behavior are stable from v0.2.0. Per the deprecation contract:

- **Severity default change from warning to error** would be breaking; deprecation-cycled. None planned.
- **What counts as a "test"** is governed by `internal/testtype/`'s classification; expanding this (e.g., adding a new test-type taxonomy) is additive and reduces findings — not a breaking change.
- **What counts as a "code unit"** is governed by per-language AST extractors; expanding coverage (e.g., adding new language support) is additive and may increase findings on previously-unscanned files; this is treated as a precision improvement, documented in `CHANGELOG.md`, not deprecation-cycled.

## 11. Related rules

- `terrain/coverage/no-eval` — same shape, but for AI surfaces lacking eval coverage (vs. code units lacking test coverage)
- `terrain/coverage/no-integration-test` — narrower: code reachable from production paths but no integration test exercises it (a stricter coverage claim than `no-tests`)
- `terrain/coverage/blind-spot` — graph-shape complement: code with high in-degree from production but zero in-degree from tests (vs. zero-in-degree from any test, which this rule covers)
- `terrain/coverage/untested-export` — narrower: exported symbols specifically (vs. all code units)
- `terrain/hygiene/no-assertions` — what fires when a test exists but contains no assertions (so it satisfies this rule but doesn't actually test anything)
