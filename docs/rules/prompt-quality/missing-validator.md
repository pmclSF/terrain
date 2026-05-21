# terrain/prompt-quality/missing-validator — Missing Prompt Validator

> Auto-generated stub. Edit anything below the marker; the generator preserves it.

**Type:** `missingPromptValidator`  
**Domain:** ai  
**Default severity:** medium  
**Status:** experimental

## Promotion plan

Fires when a prompt template has no output-validator schema attached.

## Evidence sources

- `structural-pattern`

## Confidence range

Detector confidence is bracketed at [0.70, 0.85] (heuristic today; calibrated against a labeled corpus over time).

<!-- docs-gen: end stub. Hand-authored content below this line is preserved across regenerations. -->

## 1. Summary

A prompt template that expects structured output has no validator (instructor, guardrails, pydantic schema).

## 2. Severity & status

- **Tier:** preview
- **Default severity:** off (opt-in)
- **Status:** preview — pending validation

## 3. What this catches

- A prompt asking for JSON output without a `response_model=` constraint
- A function-calling setup with no schema validation on the tool args
- An eval that accepts any string as a valid response

## 5. Detection mechanism

AST scan for LLM call sites paired with prompt content that requests structured output (`Return JSON`, `Output the following format`, etc.). Fires when no `instructor.patch`, `guardrails.from_string`, or `response_model=PydanticModel` accompanies the call.

## 6. Worked example

```
warning[terrain/prompt-quality/missing-validator]: structured-output prompt has no validator
  --> api/extract.py:18
   = docs: https://github.com/pmclSF/terrain/blob/main/docs/rules/prompt-quality/missing-validator
```

## 9. Reproducibility

```bash
terrain test --selector prompt-quality/missing-validator
```
