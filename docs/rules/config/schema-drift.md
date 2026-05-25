# terrain/config/schema-drift — Config Schema Drift Risk

> Auto-generated stub. Edit anything below the marker; the generator preserves it.

**Type:** `configSchemaDrift`  
**Domain:** quality  
**Default severity:** medium  
**Status:** experimental

## Summary

An infra-config file (GitHub Actions workflow, docker-compose, Helm values, or k8s manifest) uses forward-compat hazards: mutable action refs, `:latest` or untagged image tags, deprecated apiVersions.

## Remediation

Pin action refs to a SHA or tagged release. Replace `:latest` and untagged images with explicit versions. Upgrade deprecated apiVersions to their current replacement.

## Promotion plan

Promotes to stable once the calibration corpus confirms regression-PR lift ≥ 1.5x on config-only PRs.

## Evidence sources

- `structural-pattern`

## Confidence range

Confidence interval: 0.50–0.80.

<!-- docs-gen: end stub. Hand-authored content below this line is preserved across regenerations. -->
