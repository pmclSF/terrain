package sarif

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/pmclSF/terrain/internal/analyze"
)

func TestFromAnalyzeReport_Structure(t *testing.T) {
	r := &analyze.Report{
		KeyFindings: []analyze.KeyFinding{
			{Title: "Weak coverage in src/api/", Severity: "high", Category: "coverage_debt", Metric: "3 areas"},
			{Title: "12 duplicate clusters", Severity: "medium", Category: "optimization", Metric: "340 tests"},
		},
		WeakCoverageAreas: []analyze.WeakArea{
			{Path: "src/api/handlers.go", TestCount: 0, Band: "none"},
			{Path: "src/api/routes.go", TestCount: 1, Band: "low"},
		},
	}

	log := FromAnalyzeReport(r, "3.1.0")

	if log.Version != "2.1.0" {
		t.Errorf("expected SARIF version 2.1.0, got %s", log.Version)
	}
	if log.Schema != sarifSchema {
		t.Errorf("expected SARIF schema URI, got %s", log.Schema)
	}
	if len(log.Runs) != 1 {
		t.Fatalf("expected 1 run, got %d", len(log.Runs))
	}

	run := log.Runs[0]
	if run.Tool.Driver.Name != "terrain" {
		t.Errorf("expected tool name 'terrain', got %s", run.Tool.Driver.Name)
	}
	if run.Tool.Driver.Version != "3.1.0" {
		t.Errorf("expected version 3.1.0, got %s", run.Tool.Driver.Version)
	}
}

func TestFromAnalyzeReport_ResultCount(t *testing.T) {
	r := &analyze.Report{
		KeyFindings: []analyze.KeyFinding{
			{Title: "Finding 1", Severity: "high", Category: "coverage_debt"},
			{Title: "Finding 2", Severity: "medium", Category: "optimization"},
			{Title: "Finding 3", Severity: "low", Category: "architecture_debt"},
		},
	}

	log := FromAnalyzeReport(r, "1.0.0")
	if len(log.Runs[0].Results) != 3 {
		t.Errorf("expected 3 results, got %d", len(log.Runs[0].Results))
	}
}

func TestFromAnalyzeReport_SeverityMapping(t *testing.T) {
	tests := []struct {
		severity string
		expected string
	}{
		{"critical", "error"},
		{"high", "error"},
		{"medium", "warning"},
		{"low", "note"},
	}

	for _, tt := range tests {
		r := &analyze.Report{
			KeyFindings: []analyze.KeyFinding{
				{Title: "test", Severity: tt.severity, Category: "coverage_debt"},
			},
		}
		log := FromAnalyzeReport(r, "1.0.0")
		got := log.Runs[0].Results[0].Level
		if got != tt.expected {
			t.Errorf("severity %q: expected level %q, got %q", tt.severity, tt.expected, got)
		}
	}
}

func TestFromAnalyzeReport_CoverageLocations(t *testing.T) {
	r := &analyze.Report{
		KeyFindings: []analyze.KeyFinding{
			{Title: "Weak coverage", Severity: "high", Category: "coverage_debt"},
		},
		WeakCoverageAreas: []analyze.WeakArea{
			{Path: "src/api/handlers.go"},
			{Path: "src/api/routes.go"},
		},
	}

	log := FromAnalyzeReport(r, "1.0.0")
	result := log.Runs[0].Results[0]
	if len(result.Locations) != 2 {
		t.Fatalf("expected 2 locations, got %d", len(result.Locations))
	}
	if result.Locations[0].PhysicalLocation.ArtifactLocation.URI != "src/api/handlers.go" {
		t.Errorf("expected handlers.go URI, got %s", result.Locations[0].PhysicalLocation.ArtifactLocation.URI)
	}
}

func TestFromAnalyzeReport_RuleDedup(t *testing.T) {
	r := &analyze.Report{
		KeyFindings: []analyze.KeyFinding{
			{Title: "Finding 1", Severity: "high", Category: "coverage_debt"},
			{Title: "Finding 2", Severity: "medium", Category: "coverage_debt"},
		},
	}

	log := FromAnalyzeReport(r, "1.0.0")
	rules := log.Runs[0].Tool.Driver.Rules
	if len(rules) != 1 {
		t.Errorf("expected 1 deduplicated rule, got %d", len(rules))
	}
}

func TestFromAnalyzeReport_EmptyReport(t *testing.T) {
	r := &analyze.Report{}
	log := FromAnalyzeReport(r, "1.0.0")
	if len(log.Runs[0].Results) != 0 {
		t.Errorf("expected 0 results for empty report, got %d", len(log.Runs[0].Results))
	}
}

func TestFromAnalyzeReport_ValidJSON(t *testing.T) {
	r := &analyze.Report{
		KeyFindings: []analyze.KeyFinding{
			{Title: "Test finding", Severity: "high", Category: "coverage_debt", Metric: "5 files"},
		},
	}
	log := FromAnalyzeReport(r, "1.0.0")
	data, err := json.MarshalIndent(log, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal SARIF: %v", err)
	}
	s := string(data)
	if !strings.Contains(s, `"$schema"`) {
		t.Error("JSON missing $schema field")
	}
	if !strings.Contains(s, `"2.1.0"`) {
		t.Error("JSON missing version 2.1.0")
	}
}

func TestFromAnalyzeReport_FindingMessage(t *testing.T) {
	r := &analyze.Report{
		KeyFindings: []analyze.KeyFinding{
			{Title: "Weak coverage", Severity: "high", Category: "coverage_debt", Metric: "3 areas"},
		},
	}
	log := FromAnalyzeReport(r, "1.0.0")
	msg := log.Runs[0].Results[0].Message.Text
	if msg != "Weak coverage (3 areas)" {
		t.Errorf("expected 'Weak coverage (3 areas)', got %q", msg)
	}
}
