# terrain/engine/detector-panic — Detector Panic

> Auto-generated stub. Edit anything below the marker; the generator preserves it.

**Type:** `detectorPanic`  
**Domain:** quality  
**Default severity:** critical  
**Status:** stable

## Summary

A registered detector panicked during the run; safeDetect caught the panic and emitted this marker so the rest of the pipeline could continue.

## Remediation

Re-run with --log-level=debug to capture the stack trace, then file an issue at https://github.com/pmclSF/terrain/issues with the detector ID and the input that triggered the panic.

## Evidence sources

- `static`

## Confidence range

Confidence interval: 1.00–1.00.

<!-- docs-gen: end stub. Hand-authored content below this line is preserved across regenerations. -->
