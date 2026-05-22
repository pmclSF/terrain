# Regression suites

Each shared-infrastructure module (barrel resolver, scope classifier,
ASCG, eval-harness recognizer, fixture/source classifier, surface-literal
presence gate, intra-repo def-following, runtime-config recognizer)
ships with a frozen suite of true positives from its consumer detectors.
A change that drops more than `max_tp_loss` frozen TPs blocks the
module's PR.

## File layout

One YAML per shared module. Phase 1 baseline has no populated suites yet;
suites land with each Phase 2 module PR.

## Schema

```yaml
schema_version: 1
module: A7-barrel-resolver
max_tp_loss: 10            # symmetric ≥10%-or-≥5 rule
consumer_detectors:        # documentary
  - untestedExport
  - orphanedTestFile
frozen_tps:
  - rule_id: untestedExport
    repo: github.com/example/app
    file: src/util/parse.ts
    line: 8
    note: optional explanation
  - rule_id: orphanedTestFile
    repo: github.com/example/other
    file: tests/billing/order_test.py
```

## When to add a suite

A Phase-2 module PR adds its own suite as part of the same change:

1. Identify every consumer detector whose recall depends on the new module.
2. Collect each consumer's current TPs from labeled validation data.
3. Hand-resolve which TPs the new module is supposed to preserve.
4. Write the YAML and commit it alongside the module.
5. CI runs `regressionsuite.Check` against the head SHA's findings.
   A regression past `max_tp_loss` blocks the merge.

## Running locally

```bash
go test ./internal/regressionsuite/...
```
