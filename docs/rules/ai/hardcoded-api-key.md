# TER-AI-103 — Hard-Coded API Key in AI Configuration

**Type:** `aiHardcodedAPIKey`
**Domain:** AI
**Default severity:** Critical
**Severity clauses:** [`sev-critical-001`](../../severity-rubric.md)
**Status:** stable (0.2)

## What it detects

The detector scans configuration files (`*.yaml`, `*.yml`, `*.json`,
`*.env`, `*.toml`, `*.ini`, `*.cfg`) referenced by the snapshot for
strings that match a known provider's API-key prefix and shape:

| Provider | Prefix shape |
|---|---|
| OpenAI | `sk-`, `sk-proj-`, `sk-live-`, `sk-test-` followed by 20+ alphanumerics |
| Anthropic | `sk-ant-` followed by 20+ alphanumerics |
| Google | `AIza` + 35 chars |
| AWS | `AKIA` + 16 uppercase alphanumerics |
| GitHub | `ghp_`, `gho_`, `ghu_`, `ghs_`, `ghr_` + 36+ alphanumerics |
| Hugging Face | `hf_` + 30+ alphanumerics |
| Slack | `xoxb-`, `xoxa-`, `xoxp-`, `xoxs-` + token body |
| Stripe | `sk_live_`, `sk_test_`, `rk_live_`, `rk_test_` + 20+ alphanumerics |

Matches that contain placeholder substrings (`fake`, `placeholder`,
`example`, `dummy`, `xxxxx`, `00000`, `your-key-here`, `redacted`) or
that fail a basic entropy check (one character dominating the string)
are dropped to avoid flagging documentation snippets and example
configs.

## Why it's Critical

Per `sev-critical-001` ("Secret leak with production reach"): a
committed API key grants whoever reads the repo (current and future)
production access to the underlying service. Even after rotation, the
old key is forever in git history, so the only safe response is
"rotate immediately, then back-fill the cleanup".

## What you should do

1. Rotate the leaked key on the provider's console.
2. Move the secret to an environment variable or a secrets store the
   runner already understands (`OPENAI_API_KEY`, `ANTHROPIC_API_KEY`,
   etc.).
3. Reference the env var from the eval config:

   ```yaml
   provider:
     name: openai
     api_key: ${OPENAI_API_KEY}
   ```

4. Add a placeholder version of the file (`*.example.yaml`) with a
   clearly-fake key so contributors see the structure without copying a
   real one.

## Why it might be a false positive

- The string is a documented placeholder. The detector skips obvious
  markers; if you've found a less obvious placeholder pattern, file an
  issue with the example so the marker list grows.
- The provider's keys actually look this way intentionally and you've
  rotated already. Add an `expectedAbsent: aiHardcodedAPIKey` entry in
  the calibration fixture so the false-positive rate gets measured.

## Known limitations (0.2)

- Detector only inspects files already in the snapshot's TestFiles or
  Scenarios. Files outside the analysis surface are not scanned.
- Regexes target the most common providers; less common ones (Azure
  OpenAI, Cohere, Replicate, etc.) will be added as the calibration
  corpus grows.
- Long YAML lines beyond 1 MiB are silently truncated — pathological
  test data should not be embedded inline.
