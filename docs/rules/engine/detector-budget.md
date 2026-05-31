# terrain/engine/detector-budget — Detector Budget Exceeded

> Auto-generated stub. Edit anything below the marker; the generator preserves it.

**Type:** `detectorBudgetExceeded`  
**Domain:** quality  
**Default severity:** critical  
**Lifecycle status:** stable  
**Gating tier:** observability

## Summary

A registered detector exceeded its wall-clock budget and was abandoned by the pipeline. The rest of the pipeline continued without that detector's signals.

## Remediation

If the detector is legitimately slow on your repo, raise DetectorMeta.Budget for it. If it should be fast, the runaway suggests a quadratic-or-worse code path or a hung I/O — re-run with --log-level=debug.

## Evidence sources

- `static`

## Confidence range

Confidence interval: 1.00–1.00.

<!-- docs-gen: end stub. Hand-authored content below this line is preserved across regenerations. -->
