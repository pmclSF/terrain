# Regression suites

Each shared-infrastructure module ships with a frozen suite of true positives from its consumer detectors. A change that drops more than `max_tp_loss` frozen TPs blocks the module's PR.

## File layout

One YAML per shared module:

```
harness/regression-suites/
  _README.md
  <module-id>.yaml         # populated as modules land
```

## Schema

```yaml
schema_version: 1
module: <module-id>
max_tp_loss: 10            # symmetric ≥10%-or-≥5 rule
consumer_detectors:        # documentary
  - untestedExport
  - orphanedTestFile
frozen_tps:
  - rule_id: untestedExport
    repo: github.com/example/app
    file: src/util/parse.ts
    line: 8
```

## When to add a suite

A module PR adds its own suite as part of the same change:

1. Identify every consumer detector whose recall depends on the new module.
2. Collect each consumer's current TPs from labeled validation data.
3. Hand-resolve which TPs the new module is supposed to preserve.
4. Write the YAML and commit it alongside the module.
5. CI runs `regressionsuite.Check` against the head SHA's findings.

## Running locally

```bash
go test ./internal/regressionsuite/...
```
