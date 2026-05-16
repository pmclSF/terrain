# Integration template — canonical structure

> *This file is the canonical template for integration documentation. Every named integration in the catalog has a page at `docs/integrations/<tool>.md` filled from this template. See `docs/PRODUCT.md` §15 templating discipline.*

Integration docs are template-fills, not independent essays. The structure below is canonical; sections are populated per tool.

---

```markdown
# Terrain ↔ <Tool Name>

## What this integration does

One paragraph. What does Terrain consume from <Tool>? What does Terrain add on top? Why does an adopter using <Tool> benefit from wiring it in?

## Prerequisites

- <Tool> version supported: <range>
- Terrain version: 0.2.0+
- Other dependencies: <list>

## Install

How to add <Tool> to a project that doesn't have it:

\`\`\`bash
<install command>
\`\`\`

Skip this section for tools assumed to be present (e.g., the integration is "if you already have this, here's how to wire it").

## `terrain.yaml` wiring

What the adopter adds to their `terrain.yaml`:

\`\`\`yaml
# Minimal wiring
<tool-section>:
  <key>: <value>
\`\`\`

Optionally, advanced configuration:

\`\`\`yaml
<tool-section>:
  <key>: <value>
  <advanced-key>: <value>
\`\`\`

## What Terrain consumes

Concretely, what artifacts / files / API outputs Terrain reads from <Tool>:

- `<artifact 1>` — what Terrain does with it
- `<artifact 2>` — what Terrain does with it

If <Tool> is a runtime system Terrain queries, document the endpoint / authentication / rate-limit considerations.

## What Terrain adds

What rules / capabilities become available once <Tool> is integrated:

- `<rule-id-1>` — what it catches that <Tool> alone doesn't
- `<rule-id-2>` — same

Optionally: which graph edges / surface types / diagnostic information becomes richer because of <Tool>.

## End-to-end workflow

A concrete walk-through. Start state → wire integration → action → result.

1. **Start state:** [describe what the adopter has before integrating]
2. **Wire:** [the `terrain.yaml` change and any other setup]
3. **Action:** [what the adopter does — typically run `terrain test` on a representative PR]
4. **Result:** [what the adopter sees — the rule fires, the diagnostic includes <Tool>-specific information]

## Troubleshooting

Common adopter issues with this integration. Each is one paragraph: symptom + cause + fix.

- **Symptom A:** <description>. **Cause:** <reason>. **Fix:** <action>.
- **Symptom B:** <description>. **Cause:** <reason>. **Fix:** <action>.

## Version compatibility

| <Tool> version | Terrain support |
|---|---|
| <version> | Supported / Best-effort / Not supported |
| <version> | ... |

If <Tool> is itself an evolving spec (e.g., MCP, OpenAPI), document Terrain's version-tracking policy here.

## Licensing notes

If <Tool> has unusual licensing (e.g., proprietary, dual-license, copyleft), call it out. Adopters need to know whether wiring <Tool> in carries license obligations.

For most permissive-licensed tools (MIT / Apache 2.0 / BSD), this section is just "Apache 2.0; compatible with Terrain (Apache 2.0)."

## See also

- Related rule pages
- The plan section that covers this integration: `docs/PRODUCT.md` §<N>
- Upstream <Tool> documentation
```

---

## How to author a new integration page

1. Copy this template into `docs/integrations/<tool>.md`.
2. Fill the sections. Brevity over breadth; the goal is "the adopter can wire this up in 30 minutes."
3. Validate the workflow by actually running it. The end-to-end-workflow section pastes the actual diagnostic output.
4. PR with both the integration code (Terrain-side) and the integration doc; one without the other is incomplete.

## Tone

Operational. Same as the rule docs — matter-of-fact, no marketing. Adopters are wiring a tool they already use to a gate they're considering adopting; they need clarity, not pitch.
