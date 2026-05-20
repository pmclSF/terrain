# `terrain/agent-quality/tool-no-budget` *(preview)*

<!-- docs-gen: end stub. Hand-authored content below this line is preserved across regenerations. -->

## 1. Summary

An agent has tool-calling enabled but no rate limit, call ceiling, or per-tool cost cap. Adversarial inputs can trigger unbounded tool calls.

## 2. Severity & status

- **Tier:** preview
- **Default severity:** off (opt-in)
- **Status:** preview — pending validation

## 3. What this catches

- Agent with `tools=[SearchTool(), DatabaseTool()]` and no quota configuration
- An MCP-tool-enabled agent without `max_tool_calls` set
- A function-calling LLM loop with no break condition on tool-call count

## 5. Detection mechanism

AST walk: find agent constructors paired with tool registrations. Fires when no budget kwarg (`max_tool_calls`, `tool_budget`, `rate_limit`, `cost_ceiling`) is configured at either the agent or the framework level.

## 6. Worked example

```
warning[terrain/agent-quality/tool-no-budget]: tool-calling agent has no budget configured
  --> agents/researcher.py:28
   = docs: https://github.com/pmclSF/terrain/blob/main/docs/rules/agent-quality/tool-no-budget
```

## 9. Reproducibility

```bash
terrain test --selector agent-quality/tool-no-budget
```
