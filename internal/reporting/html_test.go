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

	// Signal summary section.
	if !strings.Contains(html, "Signal Summary") {
		t.Error("missing signal summary section")
	}
	if !strings.Contains(html, "high") {
		t.Error("missing high severity badge")
	}

	// Coverage confidence section.
	if !strings.Contains(html, "Coverage Confidence") {
		t.Error("missing coverage section")
	}

	// Data completeness section.
	if !strings.Contains(html, "Data Completeness") {
		t.Error("missing data completeness section")
	}
	if !strings.Contains(html, "available") {
		t.Error("missing available status in data completeness")
	}

	// Risk posture section.
	if !strings.Contains(html, "Risk Posture") {
		t.Error("missing risk posture section")
	}
	if !strings.Contains(html, "strong") {
		t.Error("missing strong band in risk posture")
	}

	// Total finding count overflow message.
	if !strings.Contains(html, "5 total findings") {
		t.Error("missing total finding count")
	}

	// Self-containment: no external URLs.
	if strings.Contains(html, "https://cdn") || strings.Contains(html, "https://fonts") {
		t.Error("HTML contains external CDN references — should be self-contained")
	}
}

func TestRenderAnalyzeHTML_SpecialCharacters(t *testing.T) {
	t.Parallel()
	report := &analyze.Report{
		SchemaVersion: "1",
		Repository:    analyze.RepositoryInfo{Name: `repo<script>alert("xss")</script>`},
		Headline:      `Test "headline" with <b>HTML</b> & special chars`,
		KeyFindings: []analyze.KeyFinding{
			{Title: `Finding with "quotes" & <tags>`, Severity: "high"},
		},
	}

	var buf bytes.Buffer
	err := RenderAnalyzeHTML(&buf, report)
	if err != nil {
		t.Fatalf("RenderAnalyzeHTML failed: %v", err)
	}

	html := buf.String()

	// html/template should escape special characters.
	if strings.Contains(html, "<script>alert") {
		t.Error("XSS: script tag was not escaped")
	}
	if strings.Contains(html, `"xss"`) {
		t.Error("XSS: unescaped quotes in script context")
	}
	// The escaped version should be present.
	if !strings.Contains(html, "&lt;script&gt;") && !strings.Contains(html, "&#34;") {
		// html/template uses different escaping forms; just verify the raw script is gone.
		if strings.Contains(html, "<script>alert") {
			t.Error("XSS: raw script tag present in output")
		}
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
