# Per-mechanism recall harnesses

When a rule is split into multiple detection mechanisms (regex + AST +
def-following + barrel resolution + ...), each mechanism is a separate code
path with its own precision/recall tradeoffs. The recall harness measures:

- **Per-mechanism recall:** what fraction of golden TPs this mechanism alone
  catches (against the subset of TPs hinted at it).
- **Union recall:** what fraction of golden TPs *all* mechanisms together
  catch.

A CI run loads the harness, runs terrain against the labeled corpus, and
compares the findings against the golden TPs. A drop past `union_min_recall`
fails the build.

## File layout

One YAML per rule that has (or will have) multiple mechanisms:

```
harness/recall-harnesses/
  _README.md                            # this file
  untestedExport.yaml                   # mechanisms: barrel-export + scope + def-follow
  assertionFreeImport.yaml              # mechanisms: surface-literal + def-follow
  ...
```

Phase 1 baseline has no populated harnesses; rules that already have a
single mechanism don't need one. Harnesses land alongside the rule-split PR
in Phase 2.

## Schema

```yaml
schema_version: 1
rule_id: untestedExport
union_min_recall: 0.85         # all mechanisms together must catch ≥85% of golden TPs

mechanisms:
  - id: barrel-export
    description: A7 barrel resolver follows re-exports to find the canonical export site
    min_recall: 0.4            # of TPs hinted "barrel-export", catch ≥40%
  - id: scope-classifier
    description: A3 scope classifier confirms the export is in a public scope
    min_recall: 0.3

golden_tps:
  - repo: github.com/example/app
    file: src/lib/index.ts
    line: 12
    mechanism_hint: barrel-export
    note: re-export from internal/util — only catches if barrel resolver works
  - repo: github.com/other/lib
    file: src/api.ts
    line: 5
    mechanism_hint: scope-classifier
```

## Adding TPs

Adding a golden TP requires its own PR with reviewer sign-off. Never grow
the golden set to "fix" a recall failure — fix the mechanism instead.

## Running locally

```bash
go test ./internal/recallharness/...
```
