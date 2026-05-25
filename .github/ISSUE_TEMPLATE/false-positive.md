---
name: False positive
about: A detector fired but the underlying code is fine
title: '[fp] '
labels: false-positive
assignees: ''
---

Detector false positives matter — the more concrete the reproduction,
the easier it is to add a regression test so the false positive
doesn't come back.

## Detector

Type the exact signal type as it appears in `terrain analyze --json`
(e.g. `aiToolWithoutSandbox`, `weakAssertion`).

## Code that triggered the finding

The minimal source / config snippet that caused the detector to fire,
with surrounding context preserved. **Please redact internal repo
names, customer identifiers, and any secrets/tokens before pasting** —
this issue is public.

```yaml
# or .py / .ts / .go etc.
paste here
```

## Why this isn't actually a problem

In one or two sentences, why this code is fine despite matching the
detector's pattern. Example: "`delete_cache` is a request-scoped LRU
clear, not a destructive data operation."

## Detector output

The full signal as it appears in `--json` output:

```json
paste here
```

## Suggested fix shape

If you have a sense for what would close this — a noun whitelist
expansion, a confidence downgrade, a path-shape exclusion — name it.
The maintainers will translate the suggestion into a concrete
detector change.

## Regression test opt-in

If you can share the snippet under an open-source license, would you
be willing to have it added as a regression test so this false
positive is locked out? Yes/no.
