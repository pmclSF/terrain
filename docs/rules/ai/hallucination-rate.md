# terrain/ai/hallucination-rate — Eval-Flagged Hallucination Share

> Auto-generated stub. Edit anything below the marker; the generator preserves it.

**Type:** `aiHallucinationRate`  
**Domain:** ai  
**Default severity:** high  
**Lifecycle status:** stable  
**Gating tier:** gate

## Summary

The eval framework's own hallucination metadata reports a share of cases above the project-configured threshold (default 5%). Terrain reads this from the framework output (Promptfoo / DeepEval / Ragas) — Terrain does not judge hallucinations directly.

## Remediation

Investigate the underlying eval-flagged cases; tighten retrieval or grounding before merging. If you disagree with the eval framework's classification, fix the eval scenario or raise the threshold (with a documented justification).

## Evidence sources

- `runtime`

## Confidence range

Confidence interval: 0.80–0.95.

<!-- docs-gen: end stub. Hand-authored content below this line is preserved across regenerations. -->
