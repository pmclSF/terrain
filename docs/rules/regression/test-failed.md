# terrain/regression/test-failed — Impacted Test Failed

> Auto-generated stub. Edit anything below the marker; the generator preserves it.

**Type:** `testFailed`  
**Domain:** health  
**Default severity:** high  
**Lifecycle status:** experimental  
**Gating tier:** gate

## Summary

A test selected by impact analysis as relevant to the current change failed. The change broke something the test suite already protects.

## Remediation

Reproduce locally with `terrain test --selector regression/test-failed`. Fix the failure or, if the test is stale, update it deliberately.

## Promotion plan

Off by default. Detector function exists at internal/regression/test_failed.go (DetectTestFailed). Pipeline integration pending: the detector's input shape is not yet fed through the engine registry. Stays at experimental until that wiring lands. Opt in via `.terrain/policy.yaml` only after pipeline integration lands.

## Evidence sources

- `runtime`
- `graph-traversal`

## Confidence range

Confidence interval: 1.00–1.00.

<!-- docs-gen: end stub. Hand-authored content below this line is preserved across regenerations. -->

## 1. Summary

A test selected by impact analysis as relevant to the current change failed. The change broke something the test suite already protects.

## 2. Severity & status

- **Tier:** stable
- **Default severity:** high
- **Stable since:** v0.2.0

## 3. What this catches

- A unit test in the code unit a PR modifies fails when run on the head SHA
- An integration test whose imported code path is touched by the PR fails
- A test reachable via the cross-language API graph (Python backend exercised by a changed TS frontend) fails
- A test selected by directory-proximity fallback (no exact edge) fails

## 4. Why this matters

The foundational rule in the regression family: "tests we picked for this change actually ran and failed." If the gate's impact selection picks the right tests and they pass, every other regression rule fires on additional risk beyond what tests already cover. When `regression/test-failed` fires, that risk is already realized.

## 5. Detection mechanism

- **Approach:** Impact analysis selects tests touching the diff via `internal/impact/findImpactedTests`; the test runner executes them; JUnit XML output is parsed into per-test pass/fail; failures emit this signal.
- **Languages supported:** Go (`go test`), JS/TS (jest, vitest, mocha, playwright, cypress), Python (pytest, unittest), Java (JUnit 4, JUnit 5, TestNG).
- **0.2.0 implementation:** consumes existing JUnit ingestion. Per-case parameterized enumeration is followup work (template-level only at 0.2.0).

## 6. Worked example

```
error[terrain/regression/test-failed]: impacted test failed: test_summarize_refuses
  --> backend/tests/integration/test_summarize.py:42
   = cause path: frontend/CommentInput.tsx → POST /api/summarize → handle_summarize
   = failure:    AssertionError: expected refusal in response
   = help:       Restore the input length cap on CommentInput.tsx:42.
   = docs:       https://github.com/pmclSF/terrain/blob/main/docs/rules/regression/test-failed
```

## 7. Configuration

```yaml
rules:
  regression/test-failed:
    flaky_retry_count: 0  # default; raise carefully — retries can mask real failures
```

## 9. Reproducibility

```bash
terrain test --selector regression/test-failed
```

## 10. Stability commitment

Rule ID, severity, and the impact-then-run-then-surface lifecycle are stable from v0.2.0.

## 11. Related rules

- `terrain/regression/eval-regression` — the AI/eval equivalent
- `terrain/coverage/no-tests` — what fires when there's no test to select
- `terrain/coverage/no-integration-test` — when production paths lack integration coverage
