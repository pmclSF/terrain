package reporting

import (
	"bytes"
	"strings"
	"testing"

	"github.com/pmclSF/terrain/internal/analyze"
)

func TestRenderAnalyzeReportV2_KeyFindings(t *testing.T) {
	t.Parallel()
	r := &analyze.Report{
		SchemaVersion: "1.0.0",
		Repository:    analyze.RepositoryInfo{Name: "test-repo"},
		RepoProfile: analyze.ProfileSummary{
			TestVolume:         "medium",
			CIPressure:         "low",
			CoverageConfidence: "low",
		},
		KeyFindings: []analyze.KeyFinding{
			{Title: "3 critical signals detected", Severity: "critical", Category: "reliability", Metric: "3 critical"},
			{Title: "80 source files have low coverage", Severity: "high", Category: "coverage_debt", Metric: "80 files"},
			{Title: "200 duplicate tests", Severity: "medium", Category: "optimization", Metric: "200 duplicates"},
		},
		TotalFindingCount: 6,
		TestsDetected:     analyze.TestSummary{TestFileCount: 42},
		SignalSummary:     analyze.SignalBreakdown{Total: 10, Critical: 3},
	}

	var buf bytes.Buffer
	RenderAnalyzeReportV2(&buf, r)
	output := buf.String()

	// Key Findings section should appear.
	if !strings.Contains(output, "Key Findings") {
		t.Error("output should contain 'Key Findings' section")
	}

	// All 3 findings should be rendered.
	if !strings.Contains(output, "[CRITICAL]") {
		t.Error("output should contain [CRITICAL] badge")
	}
	if !strings.Contains(output, "[HIGH]") {
		t.Error("output should contain [HIGH] badge")
	}
	if !strings.Contains(output, "[MEDIUM]") {
		t.Error("output should contain [MEDIUM] badge")
	}

	// Numbered list.
	if !strings.Contains(output, "1. [CRITICAL]") {
		t.Error("findings should be numbered starting at 1")
	}

	// Remaining count.
	if !strings.Contains(output, "3 more findings available") {
		t.Error("output should show remaining finding count")
	}
	if !strings.Contains(output, "terrain insights") {
		t.Error("output should reference terrain insights command")
	}
}

func TestRenderAnalyzeReportV2_NoFindings(t *testing.T) {
	t.Parallel()
	r := &analyze.Report{
		SchemaVersion: "1.0.0",
		Repository:    analyze.RepositoryInfo{Name: "clean-repo"},
		RepoProfile: analyze.ProfileSummary{
			TestVolume: "small",
		},
		KeyFindings:       nil,
		TotalFindingCount: 0,
		TopInsight:        "No major issues detected.",
		TestsDetected:     analyze.TestSummary{TestFileCount: 5},
	}

	var buf bytes.Buffer
	RenderAnalyzeReportV2(&buf, r)
	output := buf.String()

	// Should fall back to TopInsight.
	if !strings.Contains(output, "Top Insight") {
		t.Error("output should fall back to 'Top Insight' when no key findings")
	}
	if !strings.Contains(output, "No major issues detected.") {
		t.Error("output should show the TopInsight text")
	}
}

func TestRenderAnalyzeReportV2_NextStepsShowsFindingCount(t *testing.T) {
	t.Parallel()
	r := &analyze.Report{
		SchemaVersion: "1.0.0",
		Repository:    analyze.RepositoryInfo{Name: "test-repo"},
		RepoProfile:   analyze.ProfileSummary{TestVolume: "medium"},
		KeyFindings: []analyze.KeyFinding{
			{Title: "issue 1", Severity: "high", Category: "reliability"},
		},
		TotalFindingCount: 5,
		TestsDetected:     analyze.TestSummary{TestFileCount: 10},
	}

	var buf bytes.Buffer
	RenderAnalyzeReportV2(&buf, r)
	output := buf.String()

	// Next steps should reference the total finding count.
	if !strings.Contains(output, "5 findings") {
		t.Error("next steps should mention total finding count")
	}
}

func TestRenderAnalyzeReportV2_Deterministic(t *testing.T) {
	t.Parallel()
	r := &analyze.Report{
		SchemaVersion: "1.0.0",
		Repository:    analyze.RepositoryInfo{Name: "test-repo"},
		RepoProfile:   analyze.ProfileSummary{TestVolume: "medium"},
		KeyFindings: []analyze.KeyFinding{
			{Title: "a", Severity: "high", Category: "reliability"},
			{Title: "b", Severity: "medium", Category: "optimization"},
		},
		TotalFindingCount: 2,
		TestsDetected:     analyze.TestSummary{TestFileCount: 10},
	}

	var buf1, buf2 bytes.Buffer
	RenderAnalyzeReportV2(&buf1, r)
	RenderAnalyzeReportV2(&buf2, r)

	if buf1.String() != buf2.String() {
		t.Error("rendering should be deterministic")
	}
}

func TestRenderAnalyzeReportV2_AllSections(t *testing.T) {
	t.Parallel()
	r := &analyze.Report{
		SchemaVersion: "1",
		Headline:      "Your test suite has 2 issues requiring attention.",
		Repository:    analyze.RepositoryInfo{Name: "full-repo", Languages: []string{"typescript"}},
		RepoProfile: analyze.ProfileSummary{
			TestVolume:         "large",
			CIPressure:         "high",
			CoverageConfidence: "medium",
			RedundancyLevel:    "moderate",
			FanoutBurden:       "high",
			SkipBurden:         "low",
			FlakeBurden:        "moderate",
		},
		TestsDetected: analyze.TestSummary{
			TestFileCount:    100,
			TestCaseCount:    500,
			CodeUnitCount:    200,
			CodeSurfaceCount: 50,
			ScenarioCount:    3,
			PromptCount:      2,
			DatasetCount:     1,
			Frameworks: []analyze.FrameworkCount{
				{Name: "jest", FileCount: 80, Type: "unit"},
				{Name: "playwright", FileCount: 20, Type: "e2e"},
			},
		},
		CoverageConfidence: analyze.CoverageSummary{
			TotalFiles:  100,
			HighCount:   60,
			MediumCount: 30,
			LowCount:    10,
		},
		HighFanout: analyze.FanoutSummary{
			FlaggedCount: 3,
			Threshold:    10,
			TopNodes: []analyze.FanoutNode{
				{Path: "src/db.ts", NodeType: "source_file", TransitiveFanout: 45},
			},
		},
		SkippedTestBurden: analyze.SkipSummary{
			SkippedCount: 8,
			TotalTests:   500,
			SkipRatio:    0.016,
		},
		WeakCoverageAreas: []analyze.WeakArea{
			{Path: "src/legacy/", TestCount: 0},
			{Path: "src/utils/", TestCount: 2},
		},
		CIOptimization: analyze.CIOptimizationSummary{
			DuplicateTestsRemovable: 20,
			SkippedTestsReviewable:  8,
			HighFanoutNodes:         3,
			Recommendation:          "Consider splitting high-fanout modules.",
		},
		RiskPosture: []analyze.RiskDimension{
			{Dimension: "health", Band: "STRONG"},
			{Dimension: "coverage_depth", Band: "MODERATE"},
		},
		SignalSummary: analyze.SignalBreakdown{
			Total:    25,
			Critical: 1,
			High:     10,
			Medium:   12,
			Low:      2,
		},
		KeyFindings: []analyze.KeyFinding{
			{Title: "1 critical signal detected", Severity: "critical"},
			{Title: "10 high-fanout nodes", Severity: "high"},
		},
		TotalFindingCount: 5,
		DataCompleteness: []analyze.DataSource{
			{Name: "source", Available: true},
			{Name: "coverage", Available: false},
			{Name: "runtime", Available: true},
		},
		Limitations: []string{
			"No coverage data provided.",
		},
		DiscoveredArtifacts: []analyze.DiscoveredArtifact{
			{Kind: "runtime", Path: "jest-results.json", Format: "jest-json"},
		},
		NextActions: []analyze.NextAction{
			{Title: "Add coverage", Command: "npx jest --coverage", Explanation: "Enables coverage analysis."},
		},
	}

	var buf bytes.Buffer
	RenderAnalyzeReportV2(&buf, r)
	output := buf.String()

	// Verify all major sections appear.
	sections := []string{
		"Terrain — Test Suite Analysis",
		"Your test suite has 2 issues",
		"Auto-detected runtime",
		"Key Findings",
		"What to do next:",
		"Repository Profile",
		"Validation Inventory",
		"Coverage Confidence",
		"High-Fanout Nodes",
		"Skipped Test Burden",
		"Weak Coverage Areas",
		"CI Optimization Potential",
		"Risk Posture",
		"Signals: 25 total",
		"Data Completeness",
		"Limitations",
		"Next steps:",
	}
	for _, section := range sections {
		if !strings.Contains(output, section) {
			t.Errorf("missing section: %q", section)
		}
	}

	// Verify specific content.
	if !strings.Contains(output, "jest") {
		t.Error("expected jest framework in output")
	}
	if !strings.Contains(output, "playwright") {
		t.Error("expected playwright framework in output")
	}
	if !strings.Contains(output, "STRONG") {
		t.Error("expected STRONG in risk posture")
	}
	if !strings.Contains(output, "src/legacy/") {
		t.Error("expected weak coverage area path")
	}
	if !strings.Contains(output, "1 critical") {
		t.Error("expected signal breakdown in output")
	}
}

func TestRenderAnalyzeReportV2_StabilityHintWithoutRuntime(t *testing.T) {
	t.Parallel()
	r := &analyze.Report{
		SchemaVersion: "1",
		RepoProfile:   analyze.ProfileSummary{TestVolume: "small"},
		TestsDetected: analyze.TestSummary{TestFileCount: 5},
		DataCompleteness: []analyze.DataSource{
			{Name: "source", Available: true},
		},
	}

	var buf bytes.Buffer
	RenderAnalyzeReportV2(&buf, r)
	output := buf.String()

	if !strings.Contains(output, "Stability") {
		t.Error("expected Stability section when no runtime data")
	}
	if !strings.Contains(output, "No runtime data provided") {
		t.Error("expected runtime hint in stability section")
	}
}

func TestPct(t *testing.T) {
	t.Parallel()
	cases := []struct {
		n, total, want int
	}{
		{60, 100, 60},
		{1, 3, 33},
		{0, 100, 0},
		{0, 0, 0}, // division by zero guard
		{50, 50, 100},
	}
	for _, tc := range cases {
		got := pct(tc.n, tc.total)
		if got != tc.want {
			t.Errorf("pct(%d, %d) = %d, want %d", tc.n, tc.total, got, tc.want)
		}
	}
}

func TestHasDataSource(t *testing.T) {
	t.Parallel()
	sources := []analyze.DataSource{
		{Name: "coverage", Available: true},
		{Name: "runtime", Available: false},
	}
	if !hasDataSource(sources, "coverage") {
		t.Error("expected coverage available")
	}
	if hasDataSource(sources, "runtime") {
		t.Error("expected runtime unavailable")
	}
	if hasDataSource(sources, "policy") {
		t.Error("expected missing source unavailable")
	}
	if hasDataSource(nil, "coverage") {
		t.Error("expected nil sources unavailable")
	}
}
