# terrain/ai/few-shot-contamination — Few-Shot Contamination

> Auto-generated stub. Edit anything below the marker; the generator preserves it.

**Type:** `aiFewShotContamination`  
**Domain:** ai  
**Default severity:** medium  
**Lifecycle status:** experimental  
**Gating tier:** observability

## Summary

Few-shot examples in a prompt overlap verbatim with the inputs of eval scenarios that exercise that prompt, inflating reported scores.

## Remediation

Hold out the contaminated examples from the prompt's few-shot block, or rewrite the eval input so it isn't a copy of an example. Re-run the eval after de-duplication.

## Promotion plan

Substring-overlap detector ships today; promotes to stable once broader validation tunes the threshold and adds token-level n-gram + semantic-similarity passes.

## Evidence sources

- `structural-pattern`

## Confidence range

Confidence interval: 0.55–0.83.

<!-- docs-gen: end stub. Hand-authored content below this line is preserved across regenerations. -->
