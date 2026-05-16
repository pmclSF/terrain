package preview

import (
	"strings"

	"github.com/pmclSF/terrain/internal/models"
	"github.com/pmclSF/terrain/internal/signals"
)

// DetectAgentLoopRisk fires when a Python file constructs an agent
// without an iteration bound. Implements
// terrain/agent-quality/loop-risk.
func DetectAgentLoopRisk(sourceFiles map[string][]byte) []models.Signal {
	var out []models.Signal
	for path, content := range sourceFiles {
		s := string(content)
		if !looksLikeAgentConstructor(s) {
			continue
		}
		if hasIterationBound(s) {
			continue
		}
		out = append(out, signal(
			signals.SignalAgentLoopRisk, models.SeverityMedium,
			"terrain/agent-quality/loop-risk",
			"docs/rules/agent-quality/loop-risk.md",
			models.SignalLocation{File: path},
			"Agent constructor without max_iterations / max_turns / recursion_limit.",
			"Add max_iterations=<N> (LangChain), recursion_limit=<N> (LangGraph), or an explicit counter in the loop body.",
			map[string]any{},
		))
	}
	return out
}

func looksLikeAgentConstructor(s string) bool {
	markers := []string{
		"AgentExecutor.from_agent",
		"AgentExecutor(",
		"StateGraph(",
		"ReActAgent.from_",
		"ReActAgent(",
		"create_react_agent",
		"create_openai_tools_agent",
		"initialize_agent(",
	}
	for _, m := range markers {
		if strings.Contains(s, m) {
			return true
		}
	}
	return false
}

func hasIterationBound(s string) bool {
	markers := []string{
		"max_iterations",
		"max_turns",
		"recursion_limit",
		"max_depth",
		"max_execution_time",
	}
	for _, m := range markers {
		if strings.Contains(s, m) {
			return true
		}
	}
	return false
}

// DetectToolWithoutBudget fires when an agent is paired with tools
// but no budget / rate-limit configuration appears nearby. Implements
// terrain/agent-quality/tool-no-budget.
func DetectToolWithoutBudget(sourceFiles map[string][]byte) []models.Signal {
	var out []models.Signal
	for path, content := range sourceFiles {
		s := string(content)
		if !looksLikeAgentConstructor(s) {
			continue
		}
		// Confirm tools are involved.
		hasTools := strings.Contains(s, "tools=") || strings.Contains(s, "tool_choice") ||
			strings.Contains(s, "@tool") || strings.Contains(s, "Tool(")
		if !hasTools {
			continue
		}
		if hasBudgetMarker(s) {
			continue
		}
		out = append(out, signal(
			signals.SignalToolWithoutBudget, models.SeverityMedium,
			"terrain/agent-quality/tool-no-budget",
			"docs/rules/agent-quality/tool-no-budget.md",
			models.SignalLocation{File: path},
			"Tool-calling agent without a budget / rate-limit / cost-ceiling.",
			"Configure max_tool_calls / tool_budget / rate_limit on the agent. Adversarial inputs can otherwise trigger unbounded tool calls.",
			map[string]any{},
		))
	}
	return out
}

func hasBudgetMarker(s string) bool {
	markers := []string{
		"max_tool_calls",
		"tool_budget",
		"rate_limit",
		"cost_ceiling",
		"max_concurrent_calls",
		"call_quota",
	}
	for _, m := range markers {
		if strings.Contains(s, m) {
			return true
		}
	}
	return false
}
