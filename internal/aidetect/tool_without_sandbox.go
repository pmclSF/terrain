package aidetect

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/pmclSF/terrain/internal/mechanisms"
	"github.com/pmclSF/terrain/internal/models"
	"github.com/pmclSF/terrain/internal/saferead"
	"github.com/pmclSF/terrain/internal/signals"
	"github.com/pmclSF/terrain/internal/surfacelit"
	"gopkg.in/yaml.v3"
)

// ToolWithoutSandboxDetector flags agent tool definitions that can
// perform an irreversible operation (delete / drop / exec / shell)
// without an approval gate, sandbox, or dry-run flag.
//
// Detection scope:
//   - YAML / JSON agent and MCP-tool configs (path contains "agent",
//     "tool", "mcp", or files explicitly named tools.{yaml,json})
//   - The detector finds entries with destructive verb patterns in
//     the tool name or description, then checks for the presence of
//     approval / sandbox / dry-run hints elsewhere in the same tool
//     entry.
type ToolWithoutSandboxDetector struct {
	Root string
}

// destructiveVerbs are verb patterns whose presence in a tool name or
// description marks the tool as potentially irreversible. The list is
// intentionally generous — a false positive ("delete_cache" is fine)
// is cheaper than a false negative ("delete_user" without sandbox).
// destructiveVerbs trailing class: `(?:_|\b)` rather than `\b` alone.
// Go's `\b` treats `_` as a word character, so `\bdelete\b` would not
// match `delete_user`. Allowing `_` as a boundary catches the common
// `verb_object` snake-case form that almost every real-world tool
// definition uses.
// Two dominant false-positive failure modes shape the tightened regex:
//
//	(a) Bare "execute" / "exec" matches `execute_event_loop_cycle`
//	    (agent framework main loop) and `execute_tool $X` where the
//	    wrapped tool $X is benign (calculate, get_weather). The
//	    framework boilerplate verb doesn't make the tool destructive.
//	(b) Bare "transfer" matches `transfer_to_spanish_agent` (agent
//	    handoff between assistants), not financial transfer.
//
// Tightened regex:
//   - exec / execute now require explicit destructive context:
//     exec_shell, run_shell, run_command, spawn_process, etc.
//   - eval matches as before (Python's eval() is genuinely dangerous)
//   - transfer requires payment/money context (was matching agent
//     handoff verbs)
var destructiveVerbs = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\b(delete|destroy|drop_(?:table|database|index)|truncate|purge)(?:_|\b)`),
	// exec/execute/run/spawn require an explicit destructive object —
	// command, shell, code, script, sql, query, subprocess, arbitrary —
	// to avoid matching agent-framework boilerplate like
	// `execute_event_loop_cycle` or `execute_tool $benign_tool`.
	regexp.MustCompile(`(?i)\b(exec|execute|run|spawn)_(?:command|shell|code|script|sql|query|subprocess|process|arbitrary|raw|untrusted)(?:_|\b)`),
	// eval is destructive in any object form (eval(user_input), eval_code, etc.)
	regexp.MustCompile(`(?i)\beval(?:_|\b)`),
	regexp.MustCompile(`(?i)\b(write|overwrite|replace|patch)_(?:file|disk|prod)(?:_|\b)`),
	regexp.MustCompile(`(?i)\b(send_email|send_payment|charge_card|charge|refund_payment|transfer_funds|transfer_money|wire_transfer)(?:_|\b)`),
	// "remove" only when paired with destructive nouns (file/account/database/etc.)
	regexp.MustCompile(`(?i)\b(remove_(?:file|account|user|database|repo|repository|cluster|namespace|pod))(?:_|\b)`),
}

// approvalMarkers are substrings/keys that, when present in the tool
// definition, indicate the tool is gated. Presence of any of these
// suppresses the finding for that tool entry.
var approvalMarkers = []string{
	"approval", "approve", "confirm", "human-in-the-loop", "human_in_the_loop",
	"sandbox", "sandboxed", "dry_run", "dry-run", "preview",
	"requires_human", "interactive", "needs_approval",
}

// toolConfigMarkers identify config files we'll inspect for tool defs.
var toolConfigMarkers = []string{
	"agent", "tool", "mcp", "tools.yaml", "tools.yml", "tools.json",
}

// Detect emits SignalAIToolWithoutSandbox for each tool entry whose
// name or description matches a destructive-verb pattern but whose
// definition has no approval-marker substring.
func (d *ToolWithoutSandboxDetector) Detect(snap *models.TestSuiteSnapshot) []models.Signal {
	if d == nil || snap == nil {
		return nil
	}
	paths := d.gatherToolConfigs(snap)

	var out []models.Signal
	for _, relPath := range paths {
		// Skip test fixtures and mock-recording files. Recorded API
		// responses can contain destructive verbs in log-entry
		// display names (e.g. vcrpy cassettes of cloud-provider
		// Activity Log responses) — those aren't tool definitions
		// and shouldn't fire.
		if isTestFixturePath(relPath) {
			continue
		}
		// Test-integration schema fixtures are a known false-positive
		// source: these paths are explicitly fixtures for integration
		// tests against agent frameworks, not real tool definitions.
		lp := strings.ToLower(filepath.ToSlash(relPath))
		if strings.Contains(lp, "test_integrations/") ||
			strings.Contains(lp, "/schemas/") && strings.Contains(lp, "tests/") {
			continue
		}
		abs := filepath.Join(d.Root, relPath)
		findings := analyseToolConfig(abs)
		for _, f := range findings {
			// Mechanism gate: surface_literal_presence_gate.
			if dec := surfacelit.Gate(mechanisms.Default(), f.ToolName, abs, "aiToolWithoutSandbox"); !dec.Keep {
				continue
			}
			out = append(out, models.Signal{
				Type:            signals.SignalAIToolWithoutSandbox,
				Category:        models.CategoryAI,
				Severity:        models.SeverityHigh,
				Confidence:      0.50,
				Location:        models.SignalLocation{File: relPath, Symbol: f.ToolName},
				Explanation:     f.Explanation,
				SuggestedAction: "Wrap the tool in an approval gate, or restrict its capability surface to a sandbox / dry-run mode.",

				SeverityClauses: []string{"sev-high-005"},
				Actionability:   models.ActionabilityImmediate,
				LifecycleStages: []models.LifecycleStage{models.StageDesign},
				AIRelevance:     models.AIRelevanceHigh,
				RuleID:          "terrain/ai/tool-without-sandbox",
				RuleURI:         "docs/rules/ai/tool-without-sandbox.md",
				DetectorVersion: "0.2.0",
				ConfidenceDetail: &models.ConfidenceDetail{
					Value:        0.50,
					IntervalLow:  0.50,
					IntervalHigh: 0.50,
					Quality:      "estimate",
					Sources:      []models.EvidenceSource{models.SourceStructuralPattern},
				},
				EvidenceSource:   models.SourceStructuralPattern,
				EvidenceStrength: models.EvidenceModerate,
				Metadata: map[string]any{
					"tool": f.ToolName,
				},
			})
		}
	}
	return out
}

func (d *ToolWithoutSandboxDetector) gatherToolConfigs(snap *models.TestSuiteSnapshot) []string {
	fromSnap := snapshotPaths(snap)
	fromWalk := walkRepoForConfigs(d.Root, scanOpts{
		extensions: evalConfigExts,
		markers:    toolConfigMarkers,
	})
	merged := uniquePaths(fromSnap, fromWalk)

	var out []string
	for _, p := range merged {
		ext := strings.ToLower(filepath.Ext(p))
		if !evalConfigExts[ext] {
			continue
		}
		lower := strings.ToLower(p)
		matched := false
		for _, m := range toolConfigMarkers {
			if strings.Contains(lower, m) {
				matched = true
				break
			}
		}
		if !matched {
			continue
		}
		out = append(out, p)
	}
	return out
}

// toolFinding describes one ungated destructive tool.
type toolFinding struct {
	ToolName    string
	Explanation string
}

// analyseToolConfig parses a YAML/JSON config and returns a finding per
// destructive-named tool entry that lacks an approval marker.
func analyseToolConfig(path string) []toolFinding {
	raw, err := saferead.ReadFile(path)
	if err != nil {
		return nil
	}
	var node yaml.Node
	if err := yaml.Unmarshal(raw, &node); err != nil {
		return nil
	}

	tools := extractToolEntries(&node)
	var out []toolFinding
	for _, t := range tools {
		// classifyDestructive suppresses the well-known benign forms —
		// `delete_cache`, `purge_logs`, `remove_session`, etc. — where
		// the verb's blast radius is bounded by the object noun.
		// Always-high verbs (`exec`, `transfer`, `send_payment`) stay
		// flagged regardless of object.
		if !classifyDestructive(t.name + " " + t.description) {
			continue
		}
		if hasApprovalMarkerOnEntry(t) {
			continue
		}
		out = append(out, toolFinding{
			ToolName:    t.name,
			Explanation: "Tool `" + t.name + "` matches a destructive-verb pattern but has no visible approval gate, sandbox, or dry-run marker.",
		})
	}
	return out
}

// toolEntry is a single tool definition flattened from the parsed YAML.
type toolEntry struct {
	name        string
	description string
	raw         string                // serialised tree fragment (legacy substring scanning fallback)
	fields      map[string]*yaml.Node // structural keys; preferred for marker checks
}

// extractToolEntries walks the YAML tree looking for entries that look
// like tool definitions: a mapping with a `name` field and either
// `description`, `parameters`, `function`, or similar tool-shape keys.
// Returns one entry per match; works on the common `tools: [...]` and
// `tool: {...}` shapes.
func extractToolEntries(n *yaml.Node) []toolEntry {
	var out []toolEntry
	walkYAMLNodes(n, func(n *yaml.Node) {
		if n.Kind != yaml.MappingNode {
			return
		}
		fields := mappingFields(n)
		nameNode, hasName := fields["name"]
		if !hasName {
			return
		}
		// Heuristic: tool entries tend to have description or
		// parameters/function/inputSchema. If none, skip — it's
		// probably some other named entity (model name, etc).
		isToolish := false
		for _, k := range []string{"description", "parameters", "function", "input_schema", "inputSchema", "type"} {
			if _, ok := fields[k]; ok {
				isToolish = true
				break
			}
		}
		if !isToolish {
			return
		}

		entry := toolEntry{name: nameNode.Value, fields: fields}
		if desc, ok := fields["description"]; ok {
			entry.description = desc.Value
		}
		// Serialise the mapping for marker scanning (legacy fallback).
		buf, err := yaml.Marshal(n)
		if err == nil {
			entry.raw = string(buf)
		}
		out = append(out, entry)
	})
	return out
}

// mappingFields returns a key→value map from a Mapping yaml.Node.
// Convenience for nodes with known top-level keys.
func mappingFields(n *yaml.Node) map[string]*yaml.Node {
	out := map[string]*yaml.Node{}
	if n.Kind != yaml.MappingNode {
		return out
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		out[n.Content[i].Value] = n.Content[i+1]
	}
	return out
}

// walkYAMLNodes visits every node in the parsed tree. The visitor sees
// each node once; recursion handles document/sequence/mapping shapes.
func walkYAMLNodes(n *yaml.Node, visit func(*yaml.Node)) {
	if n == nil {
		return
	}
	visit(n)
	for _, c := range n.Content {
		walkYAMLNodes(c, visit)
	}
}

// benignDestructiveObjects identifies object nouns where a "delete" /
// "purge" / "remove" verb is almost certainly safe — caches, log
// buffers, temp files, sessions, cookies. These are the noisiest
// false positives in the wild (e.g. `delete_cache`, `purge_logs`).
// We don't suppress destructive verbs categorically — `exec`, `eval`,
// `transfer`, `send_payment` stay flagged regardless of object — but
// for the verb tier that depends on context (delete/destroy/remove/
// drop/truncate/purge), an explicit benign-object match downgrades
// the finding to a warning-tier no-op.
var benignDestructiveObjects = regexp.MustCompile(
	`(?i)\b(?:delete|destroy|remove|drop|truncate|purge)_(?:cache|caches|log|logs|tmp|temp|tempfile|tmpfile|session|sessions|cookie|cookies|buffer|history|local_state)\b`,
)

// contextDependentVerbs lists the destructive-verb families whose
// danger depends on the object noun: deleting a `cache` is fine,
// deleting a `database` is not. The benign-object downgrade
// (benignDestructiveObjects) only applies to these. Every other
// destructive verb (exec, eval, payment movement, file rewrite) is
// always-high regardless of object.
//
// Trailing boundary is `(?:_|\b)` rather than `\b` alone — Go's `\b`
// treats `_` as a word character, so `\bdelete\b` does NOT match
// `delete_cache`. Allowing `_` lets the `verb_object` form match.
var contextDependentVerbs = regexp.MustCompile(
	`(?i)\b(?:delete|destroy|remove|drop|truncate|purge)(?:_|\b)`,
)

func looksDestructive(s string) bool {
	for _, rx := range destructiveVerbs {
		if rx.MatchString(s) {
			return true
		}
	}
	return false
}

// classifyDestructive returns true if the matched destructive verb
// should fire a finding (i.e. it's not the benign-object form). Only
// the context-dependent verb family (delete / destroy / remove / drop
// / truncate / purge) gets the benign-object downgrade; every other
// destructive verb stays high regardless of object.
func classifyDestructive(s string) bool {
	if !looksDestructive(s) {
		return false
	}
	if !contextDependentVerbs.MatchString(s) {
		return true
	}
	if benignDestructiveObjects.MatchString(s) {
		return false
	}
	return true
}

// hasApprovalMarker (legacy) — kept for any external callers, but
// the per-entry path is what the detector uses. Substring match
// against the marshalled tree was bypassable: typing "preview" or
// "sandbox" anywhere in a tool's description disabled detection
// (adversarial bypass).
func hasApprovalMarker(raw string) bool {
	low := strings.ToLower(raw)
	for _, m := range approvalMarkers {
		if strings.Contains(low, m) {
			return true
		}
	}
	return false
}

// hasApprovalMarkerOnEntry checks the structural keys of a single tool
// entry rather than the marshalled-tree substring, closing the
// adversarial-bypass loophole. A marker counts when:
//   - The tool entry has a top-level key whose lowercased name
//     contains an approval marker substring
//     (e.g. `sandbox`, `requires_approval`, `dry_run`), AND
//   - The value is truthy (`true`, non-empty string, non-empty map).
//
// The "key contains marker" rule (vs strict equality) preserves
// backwards compat with shapes like `requires_approval: true` and
// `dry_run_mode: enabled` while still rejecting the substring-bypass
// where a description happens to contain the word "preview".
func hasApprovalMarkerOnEntry(t toolEntry) bool {
	if t.fields == nil {
		// Legacy fallback for callers that didn't populate fields:
		// retain substring behavior rather than emit a false positive.
		return hasApprovalMarker(t.raw)
	}
	// Skip these scalar text fields — they're free-form prose, not
	// structural opt-ins. A description containing "preview" or
	// "sandbox" no longer disables the finding.
	textFields := map[string]bool{
		"description": true,
		"summary":     true,
		"name":        true,
		"label":       true,
		"comment":     true,
		"docstring":   true,
	}
	for keyName, node := range t.fields {
		lowKey := strings.ToLower(keyName)
		if textFields[lowKey] || node == nil {
			continue
		}
		matched := false
		for _, marker := range approvalMarkers {
			if strings.Contains(lowKey, marker) {
				matched = true
				break
			}
		}
		if !matched {
			continue
		}
		// Truthy: scalar with non-empty / non-false value, or any
		// non-empty mapping/sequence.
		if node.Kind == yaml.ScalarNode {
			v := strings.ToLower(strings.TrimSpace(node.Value))
			if v != "" && v != "false" && v != "no" && v != "0" && v != "null" {
				return true
			}
			continue
		}
		if (node.Kind == yaml.MappingNode || node.Kind == yaml.SequenceNode) && len(node.Content) > 0 {
			return true
		}
	}
	return false
}
