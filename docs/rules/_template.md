# Rule template — canonical structure

<!-- docs-gen: end stub. Hand-authored content below this line is preserved across regenerations. -->


> *This file is the canonical template for rule documentation. Every rule in the catalog has a page at `docs/rules/<category>/<rule-id>.md` filled from this template.*

The lifecycle status and default severity are set by the generated header above the marker (for example, experimental or stable) and should match the manifest. Fill the sections below with user-facing content only.

---

## Rule page template

```markdown
# `terrain/<category>/<rule-name>`

## 1. Summary

One-line description of what this rule catches. Plain English, no jargon.

## 2. Status

A short, neutral, user-facing status line, e.g.:

- Experimental — off by default; enable in `terrain.yaml`.
- Stable — on by default; configurable in `terrain.yaml`.

## 3. What this catches

3–5 concrete examples in plain language. One sentence each:

- Example 1 — concrete scenario the rule fires on
- Example 2 — concrete scenario the rule fires on
- Example 3 — concrete scenario the rule fires on
- (Optional) Example 4 — boundary case
- (Optional) Example 5 — interaction with another rule

## 4. Why this matters

The problem class this rule exists to prevent, described in operational terms. Reads like engineering documentation, not marketing.

## 5. Detection mechanism

How the rule identifies its target, described in user terms:

- **Approach:** AST scan / graph traversal / threshold / hybrid
- **Languages supported:** which languages the rule applies to (if applicable)
- **Inputs consumed:** file types, configs, graph nodes
- **Outputs:** a finding per match, with location and evidence
- **Edge cases handled:** how the rule treats ambiguous matches

## 6. Worked example

Show a failing diagnostic rendered in the terminal:

\`\`\`
error[terrain/<category>/<rule-name>]: <short message>
  --> <file>:<line>
   = help: <suggested action>
   = docs: https://github.com/pmclSF/terrain/blob/main/docs/rules/<category>/<rule-name>.md
\`\`\`

Then show the before/after diff that would make the finding go away:

**Before:**
\`\`\`<lang>
<offending code>
\`\`\`

**After:**
\`\`\`<lang>
<fixed code>
\`\`\`

## 7. Configuration

`terrain.yaml` snippets for the most common adopter needs.

**Disable the rule on specific paths:**
\`\`\`yaml
ignore:
  rules:
    <category>/<rule-name>:
      - "vendor/**"
      - "third_party/**"
\`\`\`

**Downgrade to warning:**
\`\`\`yaml
rules:
  <category>/<rule-name>: warning
\`\`\`

**Tune thresholds (rule-specific; only if the rule has tunable parameters):**
\`\`\`yaml
rules:
  <category>/<rule-name>:
    severity: error
    threshold: <value>
\`\`\`

## 8. False positives

Known patterns where the rule trips falsely, and how to handle them.

- **Pattern A:** [describe]. **How to handle:** [path-ignore or severity downgrade].
- **Pattern B:** [describe]. [same shape]

If you encounter a sustained false-positive pattern outside the documented ones, file a GitHub issue with a reproducer.

## 9. Reproducibility

How to reproduce the finding locally:

\`\`\`bash
git clone <repo>
cd <repo>
terrain test --selector <category>/<rule-name>
\`\`\`

Or, from a CI run:

\`\`\`bash
terrain explain <category>/<rule-name> --from-run <run-id>
\`\`\`

The local diagnostic output matches the CI surface for this rule.

## 10. Related rules

- `terrain/<category>/<sibling-1>` — adjacent concern; difference vs. this rule
- `terrain/<category>/<sibling-2>` — same shape, different target
- `terrain/<other-category>/<related>` — different category but interacts
```

---

## How to author a new rule page

1. Copy this template into `docs/rules/<category>/<rule-id>.md`.
2. Replace placeholders. Resist the urge to add sections; the structure is canonical.
3. Generate the worked-example diagnostic by running the rule on a known-failing fixture and pasting the output verbatim. Don't compose example output by hand — it drifts.

## Tone

Matter-of-fact engineering documentation. Reads like `cargo`, `mypy`, or `eslint` rule pages, not marketing copy. The reader is either a developer hitting the rule for the first time or a decision-maker evaluating Terrain. Both want the same thing: clarity about what the rule does, why it exists, and what it costs them to keep on.
