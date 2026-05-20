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

Promoted to Stable 2026-05-12 via Track 3 hand-validation against documented promptfoo rename incidents. Tier: Observability — silent eval-coverage gap, not gate-relevant. Severity raised from Medium → High because the failure mode (eval reports passing while running zero tests) is severe.

## Evidence sources

- `graph-traversal`

## Confidence range

Detector confidence is bracketed at [0.60, 0.85] (heuristic today; calibrated against a labeled corpus over time).

<!-- docs-gen: end stub. Hand-authored content below this line is preserved across regenerations. -->
