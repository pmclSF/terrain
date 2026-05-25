# terrain/engine/detector-missing-input — Detector Missing Input

> Auto-generated stub. Edit anything below the marker; the generator preserves it.

**Type:** `detectorMissingInput`  
**Domain:** quality  
**Default severity:** low  
**Status:** stable

## Summary

A registered detector requires inputs (runtime artifacts, baseline snapshot, or eval-framework results) that the current snapshot doesn't carry. The detector was skipped; the rest of the pipeline ran normally.

## Remediation

The marker explanation lists the specific flag(s) to pass to `terrain analyze` to provide the missing inputs. If you don't need this detector's signals, leave the inputs absent — the marker is informational.

## Evidence sources

- `static`

## Confidence range

Confidence interval: 1.00–1.00.

<!-- docs-gen: end stub. Hand-authored content below this line is preserved across regenerations. -->
