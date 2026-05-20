# `terrain/prompt-quality/version-skew` *(preview)*

<!-- docs-gen: end stub. Hand-authored content below this line is preserved across regenerations. -->

## 1. Summary

The same prompt template is referenced under different version names across eval scenarios, so it's unclear which version production uses.

## 2. Severity & status

- **Tier:** preview
- **Default severity:** off (opt-in)
- **Status:** preview — pending validation

## 3. What this catches

- `prompts/summarize_v1.txt` referenced by one eval, `prompts/summarize.txt` by another
- A prompt's content edited in place but old eval scenarios reference the file by a stale name
- Multiple branches of a prompt living in `prompts/` without a canonical version

## 5. Detection mechanism

Graph traversal: find SurfacePrompt entries whose content hash matches but path differs. Cross-reference Eval.CoveredSurfaceIDs to detect when adopters split coverage across versions.

## 6. Worked example

```
warning[terrain/prompt-quality/version-skew]: prompt content duplicated under multiple paths
  --> prompts/summarize.txt + prompts/summarize_v1.txt
   = docs: https://github.com/pmclSF/terrain/blob/main/docs/rules/prompt-quality/version-skew
```

## 9. Reproducibility

```bash
terrain test --selector prompt-quality/version-skew
```
