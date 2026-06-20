# Rule template — canonical structure

<!-- docs-gen: end stub. Hand-authored content below this line is preserved across regenerations. -->


> *This file is the canonical template for rule documentation. Every rule in the catalog has a page at `docs/rules/<category>/<rule-id>.md` filled from this template. See the per-rule docs convention for the templating discipline.*

Stable rules fill all 11 sections (~800–1500 words). Preview rules fill the **short-form subset** — sections 1, 2, 3, 5, 6, 9 only (~250 words total); the omitted sections carry a "preview — completed at graduation to stable" stub.

---

## Stable-rule template (all 11 sections)

```markdown
# `terrain/<category>/<rule-name>`

## 1. Summary

One-line description of what this rule catches. Plain English, no jargon.

## 2. Severity & status

- **Tier:** stable
- **Default severity:** error | warning
- **Stable since:** v0.2.0
- **Configurable via `terrain.yaml`:** yes — see [docs/configuration.md](../configuration.md)

## 3. What this catches

3–5 concrete examples in plain language. One sentence each:

- Example 1 — concrete scenario the rule fires on
- Example 2 — concrete scenario the rule fires on
- Example 3 — concrete scenario the rule fires on
- (Optional) Example 4 — boundary case
- (Optional) Example 5 — interaction with another rule

## 4. Why this matters

The problem class this rule exists to prevent. References to incident patterns or industry consensus where applicable. Reads like a postmortem retrospective, not marketing.

Anchor in a real failure mode. Example: "Pinning model fixtures to `latest` is the root cause of [public-incident pattern]. The model behind `gpt-4o-latest` changed three times in 2024; teams shipped silently-different production behavior each time without code changes." Cite a specific case if one is public; otherwise describe the failure pattern in operational terms.

## 5. Detection mechanism

How the rule identifies its target. Transparent so engineering teams trust it.

- **Approach:** AST scan / graph traversal / threshold / external library / hybrid
- **Languages supported:** which of Go / JS / TS / Python / Java the rule applies to (if applicable)
- **Inputs consumed:** file types, configs, graph nodes
- **Outputs:** a `Finding` per match, with primary_loc / cause_loc / cause_path / evidence
- **Edge cases handled:** how the rule treats ambiguous matches
- **Edge cases NOT handled in the current release:** what's deferred to a later release

## 6. Worked example

Show a failing diagnostic rendered in the terminal:

\`\`\`
error[terrain/<category>/<rule-name>]: <short message>
  --> <file>:<line>
   = <key>: <value>
   ...
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

## 8. False-positive characterization

Known patterns where the rule trips falsely, and how to handle them.

- **Pattern A:** [describe]. **Why it's a false positive:** [reason]. **How to handle:** [path-ignore, severity downgrade, or fix-the-detection-mechanism-in-Nth-release].
- **Pattern B:** [describe]. [same shape]
- **Measurement status:** link the measured per-rule readiness card when one exists for the release; otherwise state that no measured card has been published yet.

When a measured readiness card is published for this rule, it carries the FP-rate evidence for the release. If you encounter a sustained FP pattern outside the documented ones, file a GitHub issue with a reproducer.

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

The local diagnostic output is byte-equivalent to the CI surface for this rule (local-CI parity guarantee).

## 10. Stability commitment

This rule's ID and behavior are stable from v0.2.0. The one-cycle deprecation contract applies:

- **Renames:** alias the old ID for one minor version; deprecation message written to stderr; document in `CHANGELOG.md`.
- **Severity default changes:** treated as a breaking change to the default behavior; same deprecation cycle.
- **Threshold default changes:** breaking change; same cycle.
- **Detection-mechanism changes that alter findings for unchanged input:** breaking; same cycle.
- **Detection improvements that change which previously-undetected cases now fire:** treated as additive; documented in `CHANGELOG.md` but not deprecation-cycled.

## 11. Related rules

- `terrain/<category>/<sibling-1>` — adjacent concern; difference vs. this rule
- `terrain/<category>/<sibling-2>` — same shape, different target
- `terrain/<other-category>/<related>` — different category but interacts

End with one sentence on which rule to enable instead if this one is too aggressive for an adopter's context.
```

---

## Preview-rule template (short-form subset)

```markdown
# `terrain/<category>/<rule-name>` *(preview)*

## 1. Summary

One-line description.

## 2. Severity & status

- **Tier:** preview
- **Default severity:** off (opt-in via `terrain.yaml`)
- **Status:** preview — pending validation
- **Graduation criteria:** triage time, false-positive rate, and recall measured at target against representative repos.

## 3. What this catches

3 concrete examples.

## 5. Detection mechanism

Brief description of approach. Same shape as stable but no edge-case enumeration required at preview tier.

## 6. Worked example

Failing diagnostic + before/after fix.

## 9. Reproducibility

`terrain test --selector <category>/<rule-name>` command. Note: preview rules ship default-off; enable in `terrain.yaml` first.

---

**Preview status:** Sections 4 (Why this matters), 7 (Configuration depth), 8 (FP characterization), 10 (Stability commitment), and 11 (Related rules) are filled at graduation to stable. The detection logic and rendered diagnostic are equivalent to a stable rule; the validation measurements and broader documentation are what graduate.
```

---

## How to author a new rule page

1. Copy this template into `docs/rules/<category>/<rule-id>.md`.
2. Replace placeholders. Resist the urge to add sections; the structure is canonical.
3. Validate against an existing stable rule page (`regression/eval-regression.md`, `regression/test-failed.md`, `coverage/no-tests.md` are the three reference pages).
4. Generate the worked-example diagnostic by running the rule on a known-failing fixture and pasting the output verbatim. Don't compose example output by hand — it drifts.
5. The doc page is part of the same PR as the rule's detection code; both gate on review.

## Tone

Matter-of-fact engineering documentation. Reads like `cargo`, `mypy`, or `eslint` rule pages, not marketing copy. The reader is either a developer hitting the rule for the first time or a senior decision-maker evaluating Terrain. Both want the same thing: clarity about what the rule does, why it exists, and what it costs them to keep on.
