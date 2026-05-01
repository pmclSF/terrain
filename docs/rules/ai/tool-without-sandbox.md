# TER-AI-104 — Destructive Tool Without Sandbox

> Auto-generated stub. Edit anything below the marker; the generator preserves it.

**Type:** `aiToolWithoutSandbox`  
**Domain:** ai  
**Default severity:** high  
**Status:** stable

## Summary

An agent tool definition can perform an irreversible operation (delete, drop, exec) without an explicit approval gate, sandbox, or dry-run mode.

## Remediation

Wrap the tool in an approval gate or restrict its capability surface to a sandbox.

## Evidence sources

- `structural-pattern`

## Confidence range

Detector confidence is bracketed at [0.70, 0.90] (heuristic in 0.2; calibration in 0.3).

<!-- docs-gen: end stub. Hand-authored content below this line is preserved across regenerations. -->

# TER-AI-104 — Destructive Tool Without Sandbox

**Type:** `aiToolWithoutSandbox`
**Domain:** AI
**Default severity:** High
**Severity clauses:** [`sev-high-004`](../../severity-rubric.md)
**Status:** stable (0.2)

## What it detects

The detector parses YAML and JSON config files in the snapshot whose
path indicates an agent or MCP tool definition (`agent`, `tool`, `mcp`,
or files named `tools.{yaml,json}`) and inspects each tool entry for
two things:

1. **A destructive verb** in the tool's `name` or `description`:
   - delete / destroy / remove / drop / truncate / purge
   - exec / execute / run_shell / run_command / spawn / eval
   - write_file / overwrite_disk / replace_prod / patch_file
   - send_email / send_payment / charge / refund / transfer
2. **No approval marker** anywhere in the tool entry. Markers checked:
   - `approval`, `approve`, `confirm`
   - `human-in-the-loop` / `human_in_the_loop` / `requires_human`
   - `sandbox`, `sandboxed`, `dry_run`, `dry-run`, `preview`
   - `interactive: true`, `needs_approval`

A tool that has a destructive name AND lacks an approval marker fires
one signal at file-symbol granularity (the symbol is the tool name).

## Why it's High

Per `sev-high-004` ("Missing safety eval on agent surface" — closely
related). An agent that can take an irreversible action without a
gate is a foot-gun: a model misfire (hallucinated user request,
prompt injection, ambiguous instruction) can delete production data,
exfiltrate funds, or run arbitrary commands.

## What you should do

Wrap the tool in an approval gate or sandbox before merging.

```yaml
tools:
  - name: delete_user
    description: Delete a user account by id.
    parameters:
      type: object
      properties:
        user_id: {type: string}
    requires_approval: true       # ← gate added
```

For commands that genuinely need automation, restrict the surface:

```yaml
tools:
  - name: exec_command
    description: Run shell command in a sandboxed container.
    sandbox: true                 # ← runner enforces sandbox
    allowed_commands: [ls, cat, grep, echo]
```

## Why it might be a false positive

- The tool's name happens to contain a destructive verb but the
  underlying operation is read-only (e.g. `delete_cache_entry` that
  only removes an in-memory cache). Add an approval marker or rename
  the tool — the latter is cheaper since the verb match is conservative.
- The approval is enforced outside this file (e.g. the runner
  intercepts every tool call). Add an `approval: external` field —
  the marker scan will see it.

## Known limitations (0.2)

- Only YAML and JSON. Python decorator-style tool definitions
  (`@tool` / `@mcp_tool`) are not yet parsed.
- The destructive-verb list is hand-curated. False negatives on
  domain-specific destructive verbs (`unsubscribe_*`, `revoke_*`)
  are tracked in `tests/calibration/`.
- Doesn't follow tool dispatch chains: a "router" tool that delegates
  to a destructive sub-tool isn't flagged.
