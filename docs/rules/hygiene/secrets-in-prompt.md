# terrain/hygiene/secrets-in-prompt — Secrets in Prompt

> Auto-generated stub. Edit anything below the marker; the generator preserves it.

**Type:** `secretsInPrompt`  
**Domain:** ai  
**Default severity:** critical  
**Status:** stable

## Summary

A prompt-classified file contains embedded credentials (OpenAI / Anthropic / GitHub / Slack / AWS keys, JWT, bearer tokens). Anyone with read access to the prompt has access to the credential.

## Remediation

Rotate the leaked credential immediately, then move it to an environment variable or secret manager.

## Promotion plan

Stable — Go-native regex detector ships first; richer secret-vocabulary integration is a possible follow-up.

## Evidence sources

- `structural-pattern`

## Confidence range

Detector confidence is bracketed at [0.95, 0.99] (heuristic today; calibrated against a labeled corpus over time).

<!-- docs-gen: end stub. Hand-authored content below this line is preserved across regenerations. -->

## 1. Summary

A prompt-classified file contains an embedded credential. Anyone with read access to the prompt has access to the credential; rotation is the only mitigation.

## 2. Severity & status

- **Tier:** stable
- **Default severity:** critical
- **Stable since:** v0.2.0

## 3. What this catches

- A prompt template that hardcodes an OpenAI API key (`sk-...`) as an example
- A system prompt that includes a GitHub token for downstream tool calls
- A retrieval prompt that embeds AWS credentials for an S3 lookup
- A bearer token / JWT pasted into a prompt as a debugging artifact

## 4. Why this matters

Prompts ship to the model and may be persisted in logs, traces, telemetry, or memory. A credential inside a prompt has a much larger blast radius than one in a config file — it's seen by the model provider, any third-party logger, and anyone with repo access. Critical severity is appropriate because rotation is the only safe response, regardless of how the credential ended up there.

## 5. Detection mechanism

- **Approach:** scan files classified as SurfacePrompt for high-signal credential shapes.
- **Patterns at 0.2.0** (Go-native regex defaults):
  - OpenAI API key: `sk-[A-Za-z0-9]{20+}`
  - Anthropic API key: `sk-ant-[A-Za-z0-9_-]{30+}`
  - GitHub token: `gh[psour]_[A-Za-z0-9]{36+}`
  - Slack token: `xox[bapr]-[A-Za-z0-9-]{10+}`
  - AWS access key: `AKIA[0-9A-Z]{16}`
  - JWT: `eyJ...\.eyJ...\..*`
  - Bearer token in authorization-like context
- **Suppression:** none at the detector level. Adopters who deliberately include example-shaped values (e.g., `AKIAIOSFODNN7EXAMPLE`) must ignore the path in terrain.yaml.
- **0.2.0 deferred:** gitleaks library integration for the broader vocabulary of detectable secrets (database URLs, generic high-entropy strings with context).

## 6. Worked example

```
critical[terrain/hygiene/secrets-in-prompt]: prompt file contains credential (openai-api-key, aws-access-key)
  --> prompts/admin.txt
   = help: Rotate the leaked credential immediately, then move it to an environment variable or secret manager.
   = docs: https://github.com/pmclSF/terrain/blob/main/docs/rules/hygiene/secrets-in-prompt
```

## 7. Configuration

```yaml
ignore:
  rules:
    hygiene/secrets-in-prompt:
      - "prompts/examples/**"  # contains intentional example shapes
```

## 9. Reproducibility

```bash
terrain test --selector hygiene/secrets-in-prompt
```

## 10. Stability commitment

Rule ID, severity, and the 0.2.0 credential vocabulary are stable. Adding new token shapes is additive.

## 11. Related rules

- `terrain/security/pii-in-eval` — sister rule for PII in eval datasets
- `terrain/hygiene/model-fixture-unpinned` — model artifacts can also leak via repo
