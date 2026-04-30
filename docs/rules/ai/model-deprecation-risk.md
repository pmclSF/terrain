# TER-AI-106 — Model Pinned to Deprecated or Floating Tag

> Auto-generated stub. Edit anything below the marker; the generator preserves it.

**Type:** `aiModelDeprecationRisk`  
**Domain:** ai  
**Default severity:** medium  
**Status:** stable

## Summary

Code references a model name that resolves to a deprecated version or a floating tag (e.g. `gpt-4`, `gpt-3.5-turbo`).

## Remediation

Pin to a dated model variant or upgrade to a supported tier.

## Evidence sources

- `structural-pattern`

## Confidence range

Detector confidence is bracketed at [0.80, 0.95] (heuristic in 0.2; calibration in 0.3).

<!-- docs-gen: end stub. Hand-authored content below this line is preserved across regenerations. -->

# TER-AI-106 — Model Pinned to Deprecated or Floating Tag

**Type:** `aiModelDeprecationRisk`
**Domain:** AI
**Default severity:** Medium
**Severity clauses:** [`sev-medium-005`](../../severity-rubric.md)
**Status:** stable (0.2)

## What it detects

The detector scans config files (YAML / JSON / TOML / .env / .ini /
.cfg) and source files (.py / .js / .ts / .tsx / .jsx / .go / .java /
.rb / .rs) referenced by the snapshot for references to known
deprecated or floating model tags.

| Tag | Category | Why |
|---|---|---|
| `gpt-4`, `gpt-3.5-turbo` | floating | provider's "current best" alias; resolution shifts under your feet |
| `claude-3-opus`, `claude-3-sonnet`, `claude-3-haiku` | floating | same — pin to a dated variant |
| `text-davinci-003`, `text-davinci-002`, `code-davinci-*` | deprecated | sunset by OpenAI in 2024 |
| `claude-2`, `claude-1` | deprecated | sunset by Anthropic |

Dated variants (e.g. `gpt-4-0613`, `claude-3-opus-20240229`) are
explicitly NOT matched — those are the safe form.

Comments that document a deprecation history (`# Migrated from gpt-4
to gpt-4-0613`) are filtered out, so changelog-style mentions don't
fire the detector.

## Why it's Medium

Per `sev-medium-005`. A floating tag isn't broken — it's a footgun
that silently changes behaviour over time. The remediation is fast
(pin a dated variant); leaving it is a deferred cost.

## What you should do

```python
# Bad:
client.chat.completions.create(model="gpt-4", ...)

# Good:
client.chat.completions.create(model="gpt-4-0613", ...)
```

For deprecated models, migrate to the supported lineage before the
provider's sunset date.

## Why it might be a false positive

- The detector hits a string that looks like a model tag but is
  actually unrelated (e.g. an internal product code). File the fixture
  under `tests/calibration/` with `expectedAbsent` so the marker list
  evolves.
- Comment-style documentation triggers it even though the file uses
  a dated variant elsewhere. The detector tries to filter changelog
  comments by keyword (`migrate`, `deprecat`, `sunset`, `eol`,
  `switch to`); add the missing keyword if you have a counter-example.

## Known limitations (0.2)

- Hand-curated deprecation list. Less common providers (Azure OpenAI,
  Cohere, Replicate, Mistral, etc.) are not yet covered.
- Per-line dedup only; multiple distinct floating tags on the same
  line emit one signal each.
