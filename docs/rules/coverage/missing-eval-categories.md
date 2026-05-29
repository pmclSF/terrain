# terrain/coverage/missing-eval-categories — Missing Eval Categories

> Auto-generated stub. Edit anything below the marker; the generator preserves it.

**Type:** `missingEvalCategories`  
**Domain:** ai  
**Default severity:** low  
**Lifecycle status:** experimental  
**Gating tier:** observability

## Promotion plan

Fires when an eval suite has happy-path coverage but no adversarial or edge-case categories.

## Evidence sources

- `structural-pattern`

## Confidence range

Confidence interval: 0.65–0.80.

<!-- docs-gen: end stub. Hand-authored content below this line is preserved across regenerations. -->

## 1. Summary

An eval suite has cases tagged `happy_path` but no `adversarial` / `edge_case` / `safety` categories. The suite's reported coverage may be misleading.

## 2. Severity & status

- **Tier:** preview
- **Default severity:** off (opt-in)
- **Status:** preview — pending validation

## 3. What this catches

- An eval folder where every scenario is `category: happy_path`
- A safety eval with no adversarial probes
- A QA eval with no edge-case category at all

## 5. Detection mechanism

Inventory Eval.Category values; fire when configured target categories aren't represented (`adversarial`, `edge_case`, `safety` by default).

## 6. Worked example

```
info[terrain/coverage/missing-eval-categories]: no adversarial or safety eval categories
  --> evals/
   = docs: https://github.com/pmclSF/terrain/blob/main/docs/rules/coverage/missing-eval-categories
```

## 9. Reproducibility

```bash
terrain test --selector coverage/missing-eval-categories
```
