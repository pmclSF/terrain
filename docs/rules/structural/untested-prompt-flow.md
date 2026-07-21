# terrain/structural/untested-prompt-flow — Untested Prompt Flow

> Auto-generated stub. Edit anything below the marker; the generator preserves it.

**Type:** `untestedPromptFlow`  
**Domain:** ai  
**Default severity:** high  
**Lifecycle status:** experimental  
**Gating tier:** observability

## Summary

A prompt flows through multiple source files via imports with zero test coverage at any point in the chain.

## Remediation

Add integration tests at the prompt's consumption points to catch behavioral regressions.

## Evidence sources

- `graph-traversal`

## Confidence range

Confidence interval: 0.60–0.85.

<!-- docs-gen: end stub. Hand-authored content below this line is preserved across regenerations. -->
