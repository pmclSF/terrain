# terrain/ai/prompt-schema-drift — Prompt Template References Changed Schema Field

> Auto-generated stub. Edit anything below the marker; the generator preserves it.

**Type:** `aiPromptSchemaDrift`  
**Domain:** ai  
**Default severity:** high  
**Lifecycle status:** stable  
**Gating tier:** gate

## Summary

A prompt template references a schema field that this PR removed or whose declared type changed. The template will render with a missing value (or wrong type) once merged.

## Remediation

Update the template to use the new schema field, restore the old field, or remove the variable reference.

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

The detector ships at **gate** tier. Under the default trust floor it fails the
build only when its correct-side fix is validated (a prompt reference that is a
typo of a real schema field); a structural mismatch with no confident fix
surfaces as advisory. To gate on every drift finding regardless of fix
validation, run with `--no-trust-floor` (or set `trust_floor: false` in
`.terrain/terrain.yaml`).
