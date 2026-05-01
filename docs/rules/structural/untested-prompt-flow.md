# TER-STRUCT-003 — Untested Prompt Flow

> Auto-generated stub. Edit anything below the marker; the generator preserves it.

**Type:** `untestedPromptFlow`  
**Domain:** ai  
**Default severity:** high  
**Status:** experimental

## Summary

A prompt flows through multiple source files via imports with zero test coverage at any point in the chain.

## Remediation

Add integration tests at the prompt's consumption points to catch behavioral regressions.

## Promotion plan

Detection currently misses prompt flows that go through framework abstractions (LangChain runnables, LlamaIndex query engines). 0.2 ships AST-based prompt-flow tracing; promote once recall measures >=0.8 on the AI fixture corpus.

## Evidence sources

- `graph-traversal`

## Confidence range

Detector confidence is bracketed at [0.60, 0.85] (heuristic in 0.2; calibration in 0.3).

<!-- docs-gen: end stub. Hand-authored content below this line is preserved across regenerations. -->
