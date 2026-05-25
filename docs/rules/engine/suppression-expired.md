# terrain/engine/suppression-expired — Suppression Expired

> Auto-generated stub. Edit anything below the marker; the generator preserves it.

**Type:** `suppressionExpired`  
**Domain:** governance  
**Default severity:** medium  
**Status:** stable

## Summary

A `.terrain/suppressions.yaml` entry has passed its `expires` date and is no longer in effect. The underlying findings will fire again until the entry is renewed or removed.

## Remediation

Edit `.terrain/suppressions.yaml`: extend the `expires` date if the suppression is still warranted, or remove the entry if the underlying issue is resolved.

## Evidence sources

- `policy`

## Confidence range

Confidence interval: 1.00–1.00.

<!-- docs-gen: end stub. Hand-authored content below this line is preserved across regenerations. -->
