package reporting

import (
	"bytes"
	"strings"
	"testing"

	"github.com/pmclSF/terrain/internal/analyze"
)

func TestRenderAnalyzeHTML_Basic(t *testing.T) {
	t.Parallel()
	report := &analyze.Report{
		SchemaVersion: "1",
		Repository:    analyze.RepositoryInfo{Name: "test-repo"},
		Headline:      "Your test suite is in good shape.",
		TestsDetected: analyze.TestSummary{
			TestFileCount: 42,
			TestCaseCount: 180,
			CodeUnitCount: 95,
			Frameworks: []analyze.FrameworkCount{
				{Name: "jest", FileCount: 10},
			},
		},
		SignalSummary: analyze.SignalBreakdown{
			Total:  12,
			High:   3,
			Medium: 7,
			Low:    2,
		},
		KeyFindings: []analyze.KeyFinding{
			{Title: "3 untested exports", Severity: "high", Category: "coverage_debt"},
			{Title: "Low skip burden", Severity: "low"},
		},
		TotalFindingCount: 5,
		RiskPosture: []analyze.RiskDimension{
			{Dimension: "reliability", Band: "strong"},
			{Dimension: "change", Band: "moderate"},
		},
		CoverageConfidence: analyze.CoverageSummary{
			HighCount: 50, MediumCount: 30, LowCount: 15, TotalFiles: 95,
		},
		DataCompleteness: []analyze.DataSource{
			{Name: "source", Available: true},
			{Name: "coverage", Available: false},
		},
	}

	var buf bytes.Buffer
	err := RenderAnalyzeHTML(&buf, report)
	if err != nil {
		t.Fatalf("RenderAnalyzeHTML failed: %v", err)
	}

	html := buf.String()

	// Structure checks.
	if !strings.Contains(html, "<!DOCTYPE html>") {
		t.Error("missing DOCTYPE")
	}
	if !strings.Contains(html, "</html>") {
		t.Error("missing closing html tag")
	}

	// Content checks.
	if !strings.Contains(html, "test-repo") {
		t.Error("missing repo name")
	}
	if !strings.Contains(html, "Your test suite is in good shape.") {
		t.Error("missing headline")
	}
	if !strings.Contains(html, "42") {
		t.Error("missing test file count")
	}
	if !strings.Contains(html, "3 untested exports") {
		t.Error("missing key finding")
	}
	if !strings.Contains(html, "reliability") {
		t.Error("missing risk dimension")
	}

	// Self-containment: no external URLs.
	if strings.Contains(html, "https://cdn") || strings.Contains(html, "https://fonts") {
		t.Error("HTML contains external CDN references — should be self-contained")
	}
}

func TestRenderAnalyzeHTML_Empty(t *testing.T) {
	t.Parallel()
	report := &analyze.Report{
		SchemaVersion: "1",
		Repository:    analyze.RepositoryInfo{Name: "empty-repo"},
	}

	var buf bytes.Buffer
	err := RenderAnalyzeHTML(&buf, report)
	if err != nil {
		t.Fatalf("RenderAnalyzeHTML failed on empty report: %v", err)
	}

	html := buf.String()
	if !strings.Contains(html, "<!DOCTYPE html>") {
		t.Error("missing DOCTYPE on empty report")
	}
	if !strings.Contains(html, "empty-repo") {
		t.Error("missing repo name on empty report")
	}
}
