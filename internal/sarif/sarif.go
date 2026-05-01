// Package sarif provides SARIF 2.1.0 output for Terrain findings.
// SARIF (Static Analysis Results Interchange Format) is an OASIS standard
// consumed by GitHub Code Scanning, VS Code, and other tools.
//
// No external dependencies — uses only encoding/json from stdlib.
package sarif

// Log is the top-level SARIF container.
type Log struct {
	Schema  string `json:"$schema"`
	Version string `json:"version"`
	Runs    []Run  `json:"runs"`
}

// Run represents a single invocation of an analysis tool.
type Run struct {
	Tool    Tool     `json:"tool"`
	Results []Result `json:"results"`
}

// Tool describes the analysis tool.
type Tool struct {
	Driver ToolComponent `json:"driver"`
}

// ToolComponent provides metadata about the tool.
type ToolComponent struct {
	Name           string `json:"name"`
	Version        string `json:"version"`
	InformationURI string `json:"informationUri,omitempty"`
	Rules          []Rule `json:"rules,omitempty"`
}

// Rule defines a finding category.
type Rule struct {
	ID               string     `json:"id"`
	ShortDescription Message    `json:"shortDescription"`
	DefaultConfig    RuleConfig `json:"defaultConfiguration,omitempty"`
	// HelpURI links to the rule's documentation. SARIF consumers
	// (GitHub Code Scanning, IDE integrations) render this as a
	// clickthrough so a finding pivots to its docs/rules/<rule>.md
	// page. Pre-0.2.x this field was missing entirely; rule pages were
	// dead-end strings.
	HelpURI string `json:"helpUri,omitempty"`
}

// RuleConfig specifies the default severity level.
type RuleConfig struct {
	Level string `json:"level"`
}

// Result is a single finding.
type Result struct {
	RuleID    string     `json:"ruleId"`
	Level     string     `json:"level"`
	Message   Message    `json:"message"`
	Locations []Location `json:"locations,omitempty"`
}

// Message wraps a text string.
type Message struct {
	Text string `json:"text"`
}

// Location identifies where a finding occurs.
type Location struct {
	PhysicalLocation PhysicalLocation `json:"physicalLocation"`
}

// PhysicalLocation is a file path with optional line info.
type PhysicalLocation struct {
	ArtifactLocation ArtifactLocation `json:"artifactLocation"`
	Region           *Region          `json:"region,omitempty"`
}

// ArtifactLocation is a relative file URI.
type ArtifactLocation struct {
	URI string `json:"uri"`
}

// Region identifies a line within a file.
type Region struct {
	StartLine int `json:"startLine,omitempty"`
}
