package mcp

import (
	"encoding/json"
	"fmt"
)

// Tool is one MCP tool registered by the server.
type Tool struct {
	Name        string
	Description string
	InputSchema map[string]any
	Handler     func(*Artifacts, json.RawMessage) (any, error)
}

// toolRegistry is the canonical tool inventory exposed by the server.
var toolRegistry = []Tool{
	{
		Name:        "list_findings",
		Description: "List all findings from the most recent Terrain analyze run. Returns an array of {id, rule_id, severity, primary_loc, short_message}.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"severity": map[string]any{
					"type":        "string",
					"description": "Optional: filter to one severity (error, warning, notice).",
				},
				"rule_id": map[string]any{
					"type":        "string",
					"description": "Optional: filter to one rule ID.",
				},
			},
		},
		Handler: handleListFindings,
	},
	{
		Name:        "get_finding",
		Description: "Retrieve one finding by ID. ID format: '<rule_id>:<primary_loc.path>:<line>'. Returns the full Finding shape from schemas/finding.v1.json.",
		InputSchema: map[string]any{
			"type":     "object",
			"required": []string{"id"},
			"properties": map[string]any{
				"id": map[string]any{"type": "string"},
			},
		},
		Handler: handleGetFinding,
	},
	{
		Name:        "get_cause_path",
		Description: "Return the cause-path chain for a finding — the ordered list of graph nodes from primary_loc back to cause_loc.",
		InputSchema: map[string]any{
			"type":     "object",
			"required": []string{"id"},
			"properties": map[string]any{
				"id": map[string]any{"type": "string"},
			},
		},
		Handler: handleGetCausePath,
	},
	{
		Name:        "read_surface",
		Description: "Read an AI/ML surface description by name (from terrain.yaml surfaces section or auto-derived).",
		InputSchema: map[string]any{
			"type":     "object",
			"required": []string{"name"},
			"properties": map[string]any{
				"name": map[string]any{"type": "string"},
			},
		},
		Handler: handleReadSurface,
	},
	{
		Name:        "read_eval",
		Description: "Read an eval definition by ID. Returns {id, name, path, framework, covered_surface_ids}.",
		InputSchema: map[string]any{
			"type":     "object",
			"required": []string{"id"},
			"properties": map[string]any{
				"id": map[string]any{"type": "string"},
			},
		},
		Handler: handleReadEval,
	},
	{
		Name:        "read_baseline",
		Description: "Read a baseline run summary by name. Default name: 'latest'.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{
					"type":        "string",
					"description": "Baseline name. Default: 'latest'.",
				},
			},
		},
		Handler: handleReadBaseline,
	},
	{
		Name:        "suggest_action",
		Description: "Return the suggested remediation actions for a finding.",
		InputSchema: map[string]any{
			"type":     "object",
			"required": []string{"id"},
			"properties": map[string]any{
				"id": map[string]any{"type": "string"},
			},
		},
		Handler: handleSuggestAction,
	},
	{
		Name:        "reproduction_command",
		Description: "Return the exact CLI command to reproduce a finding locally.",
		InputSchema: map[string]any{
			"type":     "object",
			"required": []string{"id"},
			"properties": map[string]any{
				"id": map[string]any{"type": "string"},
			},
		},
		Handler: handleReproductionCommand,
	},
}

func toolByName(name string) (*Tool, bool) {
	for i := range toolRegistry {
		if toolRegistry[i].Name == name {
			return &toolRegistry[i], true
		}
	}
	return nil, false
}

// --- tool handlers ---

func handleListFindings(a *Artifacts, args json.RawMessage) (any, error) {
	if a == nil || a.FindingsArtifact == nil {
		return map[string]any{"findings": []any{}}, nil
	}
	var params struct {
		Severity string `json:"severity"`
		RuleID   string `json:"rule_id"`
	}
	_ = unmarshalArgs(args, &params)

	var out []map[string]any
	for _, f := range a.FindingsArtifact.Findings {
		if params.Severity != "" && string(f.Severity) != params.Severity {
			continue
		}
		if params.RuleID != "" && f.RuleID != params.RuleID {
			continue
		}
		out = append(out, map[string]any{
			"id":            findingID(f),
			"rule_id":       f.RuleID,
			"severity":      f.Severity,
			"primary_loc":   f.PrimaryLoc,
			"short_message": f.ShortMessage,
		})
	}
	return map[string]any{"findings": out}, nil
}

func handleGetFinding(a *Artifacts, args json.RawMessage) (any, error) {
	var params struct {
		ID string `json:"id"`
	}
	if err := unmarshalArgs(args, &params); err != nil {
		return nil, err
	}
	if params.ID == "" {
		return nil, fmt.Errorf("id is required")
	}
	if a == nil || a.FindingsArtifact == nil {
		return nil, fmt.Errorf("no findings artifact loaded")
	}
	f, ok := findingByID(a.FindingsArtifact, params.ID)
	if !ok {
		return nil, fmt.Errorf("no finding with id %q", params.ID)
	}
	return f, nil
}

func handleGetCausePath(a *Artifacts, args json.RawMessage) (any, error) {
	var params struct {
		ID string `json:"id"`
	}
	if err := unmarshalArgs(args, &params); err != nil {
		return nil, err
	}
	if a == nil || a.FindingsArtifact == nil {
		return nil, fmt.Errorf("no findings artifact loaded")
	}
	f, ok := findingByID(a.FindingsArtifact, params.ID)
	if !ok {
		return nil, fmt.Errorf("no finding with id %q", params.ID)
	}
	return map[string]any{
		"cause_loc":  f.CauseLoc,
		"cause_path": f.CausePath,
	}, nil
}

func handleReadSurface(a *Artifacts, args json.RawMessage) (any, error) {
	var params struct {
		Name string `json:"name"`
	}
	if err := unmarshalArgs(args, &params); err != nil {
		return nil, err
	}
	if a == nil {
		return nil, fmt.Errorf("no artifacts loaded")
	}
	s, ok := a.Surfaces[params.Name]
	if !ok {
		return nil, fmt.Errorf("no surface named %q", params.Name)
	}
	return s, nil
}

func handleReadEval(a *Artifacts, args json.RawMessage) (any, error) {
	var params struct {
		ID string `json:"id"`
	}
	if err := unmarshalArgs(args, &params); err != nil {
		return nil, err
	}
	if a == nil {
		return nil, fmt.Errorf("no artifacts loaded")
	}
	e, ok := a.Evals[params.ID]
	if !ok {
		return nil, fmt.Errorf("no eval with id %q", params.ID)
	}
	return e, nil
}

func handleReadBaseline(a *Artifacts, args json.RawMessage) (any, error) {
	var params struct {
		Name string `json:"name"`
	}
	_ = unmarshalArgs(args, &params)
	if params.Name == "" {
		params.Name = "latest"
	}
	if a == nil {
		return nil, fmt.Errorf("no artifacts loaded")
	}
	raw, ok := a.Baselines[params.Name]
	if !ok {
		return nil, fmt.Errorf("no baseline named %q", params.Name)
	}
	var generic any
	if err := json.Unmarshal(raw, &generic); err != nil {
		return nil, fmt.Errorf("baseline parse: %w", err)
	}
	return generic, nil
}

func handleSuggestAction(a *Artifacts, args json.RawMessage) (any, error) {
	var params struct {
		ID string `json:"id"`
	}
	if err := unmarshalArgs(args, &params); err != nil {
		return nil, err
	}
	if a == nil || a.FindingsArtifact == nil {
		return nil, fmt.Errorf("no findings artifact loaded")
	}
	f, ok := findingByID(a.FindingsArtifact, params.ID)
	if !ok {
		return nil, fmt.Errorf("no finding with id %q", params.ID)
	}
	return map[string]any{"suggestions": f.Suggestions}, nil
}

func handleReproductionCommand(a *Artifacts, args json.RawMessage) (any, error) {
	var params struct {
		ID string `json:"id"`
	}
	if err := unmarshalArgs(args, &params); err != nil {
		return nil, err
	}
	if a == nil || a.FindingsArtifact == nil {
		return nil, fmt.Errorf("no findings artifact loaded")
	}
	f, ok := findingByID(a.FindingsArtifact, params.ID)
	if !ok {
		return nil, fmt.Errorf("no finding with id %q", params.ID)
	}
	return map[string]any{"command": f.Reproduction}, nil
}
