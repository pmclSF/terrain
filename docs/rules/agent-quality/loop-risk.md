# `terrain/agent-quality/loop-risk` *(preview)*

<!-- docs-gen: end stub. Hand-authored content below this line is preserved across regenerations. -->

## 1. Summary

An agent flow has no `max_iterations` / `max_turns` / equivalent bound, leaving it free to loop indefinitely on adversarial input.

## 2. Severity & status

- **Tier:** preview
- **Default severity:** off (opt-in)
- **Status:** preview — pending validation

## 3. What this catches

- `AgentExecutor.from_agent_and_tools(...)` without `max_iterations=`
- LangGraph state machines with no terminal-state check
- Custom while-loop agents with no iteration counter

## 5. Detection mechanism

AST walk for agent-constructor calls (LangChain AgentExecutor, LangGraph StateGraph, LlamaIndex ReActAgent, etc.). Fires when the constructor lacks any of `max_iterations`, `max_turns`, `recursion_limit`, `max_depth`, or a manual counter in the loop body.

## 6. Worked example

```
warning[terrain/agent-quality/loop-risk]: agent has no max_iterations bound
  --> agents/router.py:14
   = docs: https://github.com/pmclSF/terrain/blob/main/docs/rules/agent-quality/loop-risk
```

## 9. Reproducibility

```bash
terrain test --selector agent-quality/loop-risk
```
