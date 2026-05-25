# terrain/structural/phantom-eval — Phantom Eval Scenario

> Auto-generated stub. Edit anything below the marker; the generator preserves it.

**Type:** `phantomEvalScenario`  
**Domain:** ai  
**Default severity:** high  
**Status:** stable

## Summary

Eval scenarios claim to validate AI surfaces but have no import-graph path to those surfaces — typically caused by a prompt/surface rename that wasn't propagated to the eval YAML.

## Remediation

Verify the test file actually imports and exercises the target code, or correct the surface mapping.

## Promotion plan

Stable. Ships at observability tier because a silent eval-coverage gap is informational, not gate-blocking. Severity is High because the failure mode (eval reports passing while running zero tests) silently degrades trust in CI signal.

## Evidence sources

- `graph-traversal`

## Confidence range

Confidence interval: 0.60–0.85.

<!-- docs-gen: end stub. Hand-authored content below this line is preserved across regenerations. -->
