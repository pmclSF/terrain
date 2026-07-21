// Package findings defines the Finding v1 type — the canonical load-
// bearing artifact emitted by every Terrain rule and consumed by the
// CLI / JUnit / SARIF / Step Summary surfaces.
//
// The schema is documented in schemas/finding.v1.json and locked from
// v0.2.0.
package findings

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
)

// SchemaVersion is the current Finding schema version. Stable from
// v0.2.0; changes are one-cycle deprecated.
const SchemaVersion = 1

// Severity classifies the rule outcome.
type Severity string

const (
	// SeverityError is gate-blocking and emitted as a JUnit failure.
	SeverityError Severity = "error"

	// SeverityWarning is advisory — surfaced in Step Summary and PR
	// annotations but NOT in JUnit failures.
	SeverityWarning Severity = "warning"

	// SeverityNotice is informational only.
	SeverityNotice Severity = "notice"
)

// Tier classifies the rule's stability.
type Tier string

const (
	TierStable  Tier = "stable"
	TierPreview Tier = "preview"
)

// Finding is the canonical result emitted by every Terrain rule.
type Finding struct {
	// Version is the schema version. Always 1 at this revision.
	Version int `json:"version"`

	// RuleID is the canonical rule identifier (terrain/<category>/<rule>).
	RuleID string `json:"rule_id"`

	// FindingID is the canonical per-finding identifier
	// (detector@path:anchor#hash) that suppressions, `terrain explain
	// finding`, and the MCP/webhook surfaces all reference. Optional:
	// present when the artifact was built from a located signal.
	FindingID string `json:"finding_id,omitempty"`

	// Severity classifies the outcome.
	Severity Severity `json:"severity"`

	// Tier classifies the rule's stability (stable or preview).
	Tier Tier `json:"tier,omitempty"`

	// PrimaryLoc is where the assertion / detection fired.
	PrimaryLoc Location `json:"primary_loc"`

	// CauseLoc is where the change that caused this finding lives.
	// Often equals PrimaryLoc.
	CauseLoc *Location `json:"cause_loc,omitempty"`

	// CausePath is the ordered chain of graph nodes from PrimaryLoc
	// back to CauseLoc. Empty for findings without cross-stack
	// causation.
	CausePath []Location `json:"cause_path,omitempty"`

	// ShortMessage is the single-line summary (≤280 chars). Used in
	// JUnit failure message, GH annotation title, terminal first line.
	ShortMessage string `json:"short_message"`

	// LongMessage is the multi-line context, used in Step Summary and
	// JUnit failure body.
	LongMessage string `json:"long_message,omitempty"`

	// Evidence carries concrete proof for the finding.
	Evidence *Evidence `json:"evidence,omitempty"`

	// Suggestions lists candidate fix actions.
	Suggestions []Suggestion `json:"suggestions,omitempty"`

	// DocsURL is the canonical rule page URL.
	DocsURL string `json:"docs_url"`

	// Reproduction is the exact CLI command to reproduce locally.
	Reproduction string `json:"reproduction,omitempty"`

	// Metadata is rule-specific structured data.
	Metadata map[string]any `json:"metadata,omitempty"`
}

// Location identifies a position in the repository.
type Location struct {
	Path      string `json:"path"`
	Line      int    `json:"line,omitempty"`
	Column    int    `json:"column,omitempty"`
	EndLine   int    `json:"end_line,omitempty"`
	EndColumn int    `json:"end_column,omitempty"`
	NodeKind  string `json:"node_kind,omitempty"`
	NodeID    string `json:"node_id,omitempty"`
}

// Evidence is concrete proof of the finding.
type Evidence struct {
	IOExamples  []IOExample  `json:"io_examples,omitempty"`
	MetricDelta *MetricDelta `json:"metric_delta,omitempty"`
	CodeExcerpt string       `json:"code_excerpt,omitempty"`
}

// IOExample is one input/expected/actual triple.
type IOExample struct {
	Input    string `json:"input"`
	Expected string `json:"expected"`
	Actual   string `json:"actual,omitempty"`
}

// MetricDelta captures the metric change that triggered a regression.
type MetricDelta struct {
	Name            string  `json:"name"`
	Before          float64 `json:"before"`
	After           float64 `json:"after"`
	Threshold       float64 `json:"threshold,omitempty"`
	ConfidenceAlpha float64 `json:"confidence_alpha,omitempty"`
}

// Suggestion is a candidate fix action.
type Suggestion struct {
	Text      string    `json:"text"`
	AppliesTo *Location `json:"applies_to,omitempty"`
	Command   string    `json:"command,omitempty"`

	// Fix, when present, is a mechanically-applicable remediation that the
	// closed-loop validator can apply and re-verify. Absent for judge-only
	// suggestions whose application requires human work (e.g. "write a test
	// exercising Foo"); those carry Text only.
	Fix *Fix `json:"fix,omitempty"`
}

// FixKind enumerates the mechanically-applicable remediation shapes. The
// closed-loop validator switches on Kind to apply a Fix, then re-runs to
// confirm the finding clears. New kinds are added as detector families gain
// applicable remediations.
type FixKind string

const (
	// FixNewFile writes Content to Path. Path must not already exist; the
	// applier treats a pre-existing file as a no-op (the fix is already in
	// place). Covers scaffold-style remediations (eval YAML, tracker stub).
	FixNewFile FixKind = "new_file"

	// FixEditInPlace replaces the entire contents of an existing Path with
	// Content. The applier restores the prior contents on revert. Covers
	// whole-file rewrites such as pinning a manifest's moving-target deps.
	FixEditInPlace FixKind = "edit_in_place"
)

// Fix is a structured, mechanically-applicable remediation. It carries
// enough to apply the change without an LLM, so the gate path stays
// key-free; the closed-loop validator applies it and asserts the finding
// clears with no new findings.
type Fix struct {
	// Kind selects the applier.
	Kind FixKind `json:"kind"`

	// Path is the repo-relative file the fix targets.
	Path string `json:"path"`

	// Content is the file body (FixNewFile) or replacement text.
	Content string `json:"content,omitempty"`
}

// Artifact is the top-level findings.json shape: a version field plus
// the list of findings. JUnit / SARIF / Step Summary all consume the
// same artifact.
type Artifact struct {
	Version  int       `json:"version"`
	Findings []Finding `json:"findings"`
}

// NewArtifact constructs an Artifact at the current schema version,
// sorting findings deterministically (rule_id ASC, primary_loc.path
// ASC, line ASC) so consumers see stable output across runs.
func NewArtifact(findings []Finding) *Artifact {
	out := make([]Finding, len(findings))
	copy(out, findings)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].RuleID != out[j].RuleID {
			return out[i].RuleID < out[j].RuleID
		}
		if out[i].PrimaryLoc.Path != out[j].PrimaryLoc.Path {
			return out[i].PrimaryLoc.Path < out[j].PrimaryLoc.Path
		}
		return out[i].PrimaryLoc.Line < out[j].PrimaryLoc.Line
	})
	return &Artifact{Version: SchemaVersion, Findings: out}
}

// RedactSource blanks source-code excerpts from every finding so emitted
// artifacts carry no code content (terrain.yaml redact_source). Positional
// data — paths and line numbers — is preserved so findings stay navigable.
func (a *Artifact) RedactSource() {
	for i := range a.Findings {
		if a.Findings[i].Evidence != nil {
			a.Findings[i].Evidence.CodeExcerpt = ""
		}
	}
}

// ReadArtifact decodes a findings artifact previously written by WriteJSON.
//
// Artifact contract:
//
//	A1. NewArtifact sorts findings deterministically (rule_id, path, line).
//	A2. WriteJSON emits stable JSON; ReadArtifact inverts it, including
//	    nested evidence.
//	A3. RedactSource blanks every finding's Evidence.CodeExcerpt and leaves
//	    all other fields intact.
func ReadArtifact(r io.Reader) (*Artifact, error) {
	var a Artifact
	if err := json.NewDecoder(r).Decode(&a); err != nil {
		return nil, fmt.Errorf("findings: decode artifact: %w", err)
	}
	return &a, nil
}

// WriteJSON marshals the artifact to JSON with stable indentation.
func (a *Artifact) WriteJSON(w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(a); err != nil {
		return fmt.Errorf("findings: encode artifact: %w", err)
	}
	return nil
}

// Validate returns an error when the finding violates the v1 schema's
// hard constraints (required fields, rule-ID format, severity / tier
// enum). Soft constraints (max-length on short_message etc.) are
// validated as warnings via Lint.
func (f *Finding) Validate() error {
	if f.Version != SchemaVersion {
		return fmt.Errorf("findings: version=%d, want %d", f.Version, SchemaVersion)
	}
	if f.RuleID == "" {
		return fmt.Errorf("findings: rule_id is required")
	}
	if !validRuleID(f.RuleID) {
		return fmt.Errorf("findings: rule_id %q does not match terrain/<category>/<rule-name>", f.RuleID)
	}
	switch f.Severity {
	case SeverityError, SeverityWarning, SeverityNotice:
		// ok
	default:
		return fmt.Errorf("findings: invalid severity %q", f.Severity)
	}
	if f.Tier != "" && f.Tier != TierStable && f.Tier != TierPreview {
		return fmt.Errorf("findings: invalid tier %q", f.Tier)
	}
	if f.PrimaryLoc.Path == "" {
		return fmt.Errorf("findings: primary_loc.path is required")
	}
	if f.ShortMessage == "" {
		return fmt.Errorf("findings: short_message is required")
	}
	if f.DocsURL == "" {
		return fmt.Errorf("findings: docs_url is required")
	}
	return nil
}

// validRuleID checks the terrain/<category>/<rule-name> shape:
// lowercase, hyphenated, three segments, both segments non-empty.
func validRuleID(id string) bool {
	const prefix = "terrain/"
	if len(id) < len("terrain/x/y") {
		return false
	}
	if id[:len(prefix)] != prefix {
		return false
	}
	rest := id[len(prefix):]
	slashIdx := -1
	for i := 0; i < len(rest); i++ {
		c := rest[i]
		switch {
		case c == '/':
			if slashIdx >= 0 {
				return false
			}
			slashIdx = i
		case c == '-':
			// ok
		case c >= 'a' && c <= 'z':
			// ok
		case c >= '0' && c <= '9':
			if slashIdx < 0 {
				return false // digits not allowed in category
			}
		default:
			return false
		}
	}
	// Both category and rule-name must be non-empty.
	if slashIdx <= 0 || slashIdx >= len(rest)-1 {
		return false
	}
	return true
}

// Lint returns non-fatal warnings about a finding's quality:
//   - short_message > 280 chars (violates schema soft constraint)
//   - cause_loc absent when cause_path is populated
//   - reproduction empty (every finding should be locally reproducible)
func (f *Finding) Lint() []string {
	var out []string
	if len(f.ShortMessage) > 280 {
		out = append(out, "short_message exceeds 280 chars")
	}
	if len(f.CausePath) > 0 && f.CauseLoc == nil {
		out = append(out, "cause_path is populated but cause_loc is not — set cause_loc to the chain's terminal node")
	}
	if f.Reproduction == "" {
		out = append(out, "reproduction is empty — every finding should be locally reproducible")
	}
	return out
}
