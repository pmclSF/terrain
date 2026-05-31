# Per-mechanism recall harnesses

When a rule is split into multiple detection mechanisms (regex + AST + def-following + barrel resolution + ...), each mechanism is a separate code path with its own precision/recall tradeoffs. The recall harness measures:

- **Per-mechanism recall:** what fraction of golden TPs this mechanism alone catches (against the subset of TPs hinted at it).
- **Union recall:** what fraction of golden TPs *all* mechanisms together catch.

A CI run loads the harness, runs terrain against the validation set, and compares the findings against the golden TPs. A drop past `union_min_recall` fails the build.

## File layout

One YAML per rule that has (or will have) multiple mechanisms:

```
harness/recall-harnesses/
  _README.md                            # this file
  <rule-id>.yaml                        # populated as rule splits land
```

Rules that have a single mechanism do not need a harness file.

## Schema

```yaml
schema_version: 1
rule_id: untestedExport
union_min_recall: 0.85         # all mechanisms together must catch ≥85% of golden TPs

mechanisms:
  - id: barrel-export
    description: re-export resolver
    min_recall: 0.4            # of TPs hinted "barrel-export", catch ≥40%
  - id: scope-classifier
    description: confirms the export is in a public scope
    min_recall: 0.3

golden_tps:
  - repo: github.com/example/app
    file: src/lib/index.ts
    line: 12
    mechanism_hint: barrel-export
```

## Adding TPs

Adding a golden TP requires its own PR with reviewer sign-off. Never grow the golden set to "fix" a recall failure — fix the mechanism instead.

## Running locally

```bash
go test ./internal/recallharness/...
```
