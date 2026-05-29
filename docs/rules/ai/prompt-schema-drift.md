# terrain/ai/prompt-schema-drift — Prompt Template References Changed Schema Field

> Auto-generated stub. Edit anything below the marker; the generator preserves it.

**Type:** `aiPromptSchemaDrift`  
**Domain:** ai  
**Default severity:** high  
**Status:** stable

## Summary

A prompt template references a schema field that this PR removed or whose declared type changed. The template will render with a missing value (or wrong type) once merged.

## Remediation

Update the template to use the new schema field, restore the old field, or remove the variable reference.

## Promotion plan

Observability at 0.2.0. Promote to gate once an adopter corpus measurement (n>=100 PRs across >=5 repos) shows precision >=80% with no class of false positive larger than 10% of fires.

## Evidence sources

- `static`

## Confidence range

Confidence interval: 0.85–0.95.

<!-- docs-gen: end stub. Hand-authored content below this line is preserved across regenerations. -->

## What it detects

Terrain reads prompt templates (`.md` / `.markdown` files using
mustache-style `{{var}}` placeholders) and JSON Schema documents
(files with a `$schema` URI or `"type": "object"` alongside
`properties`). When a PR changes a schema field, terrain
correlates the change with template variables of the same name and
emits a finding for each affected template.

## Render before / after

The PR-comment payload includes a synthesized "before" rendering of
the template against the pre-PR schema, and an "after" rendering
against the post-PR schema. Variables whose schema property was
removed render as `MISSING(<name>)` in the after block so the
missing-ness is visible in the diff.

## Tuning

The detector ships at observability tier. To opt into gate
behavior on a repo where the precision is high enough, set:

```yaml
# .terrain/policy.yaml
detectors:
  aiPromptSchemaDrift:
    tier: gate
```
