package analyze

import (
	"testing"

	"github.com/pmclSF/terrain/internal/depgraph"
	"github.com/pmclSF/terrain/internal/models"
	"github.com/pmclSF/terrain/internal/skipstats"
)

func TestBuild_EmptySnapshot(t *testing.T) {
	report := Build(&BuildInput{Snapshot: &models.TestSuiteSnapshot{}})
	if report == nil {
		t.Fatal("expected non-nil report")
	}
	if report.TestsDetected.TestFileCount != 0 {
		t.Errorf("expected 0 test files, got %d", report.TestsDetected.TestFileCount)
	}
	if report.TopInsight == "" {
		t.Error("expected non-empty top insight even with no data")
	}
}

func TestBuildSignalSummary(t *testing.T) {
	snap := &models.TestSuiteSnapshot{
		Signals: []models.Signal{
			{Severity: models.SeverityCritical, Category: models.CategoryHealth},
			{Severity: models.SeverityHigh, Category: models.CategoryHealth},
			{Severity: models.SeverityMedium, Category: models.CategoryQuality},
			{Severity: models.SeverityLow, Category: models.CategoryQuality},
			{Severity: models.SeverityLow, Category: models.CategoryMigration},
		},
	}
	sb := buildSignalSummary(snap)
	if sb.Total != 5 {
		t.Errorf("total = %d, want 5", sb.Total)
	}
	if sb.Critical != 1 {
		t.Errorf("critical = %d, want 1", sb.Critical)
	}
	if sb.High != 1 {
		t.Errorf("high = %d, want 1", sb.High)
	}
	if sb.ByCategory["health"] != 2 {
		t.Errorf("health = %d, want 2", sb.ByCategory["health"])
	}
}

func TestBuildSkipSummary(t *testing.T) {
	snap := &models.TestSuiteSnapshot{
		TestFiles: []models.TestFile{
			{Path: "test/a.test.js", TestCount: 10, SkipCount: 3},
			{Path: "test/b.test.js", TestCount: 5},
		},
	}
	stats := skipstats.Summarize(snap)
	ss := SkipSummary{
		SkippedCount: stats.SkippedTests,
		TotalTests:   stats.TotalTests,
		SkipRatio:    stats.TestRatio,
	}
	if ss.SkippedCount != 3 {
		t.Errorf("skipped = %d, want 3", ss.SkippedCount)
	}
	if ss.TotalTests != 15 {
		t.Errorf("total = %d, want 15", ss.TotalTests)
	}
	if ss.SkipRatio != 0.2 {
		t.Errorf("ratio = %v, want 0.2", ss.SkipRatio)
	}
}

func TestBuildCIOptimization_UsesMergedSkipCounts(t *testing.T) {
	t.Parallel()

	ci := buildCIOptimization(
		&depgraph.DuplicateResult{DuplicateCount: 4},
		&depgraph.FanoutResult{FlaggedCount: 2},
		3, // skippedTests
	)

	if ci.SkippedTestsReviewable != 3 {
		t.Fatalf("SkippedTestsReviewable = %d, want 3", ci.SkippedTestsReviewable)
	}
	if ci.Recommendation == "" {
		t.Fatal("expected recommendation")
	}
}

func TestBuildLimitations(t *testing.T) {
	snap := &models.TestSuiteSnapshot{}
	lims := buildLimitations(snap, false)
	if len(lims) == 0 {
		t.Error("expected limitations for empty snapshot")
	}
}

func TestBuild_WithSignals(t *testing.T) {
	snap := &models.TestSuiteSnapshot{
		Repository: models.RepositoryMetadata{Name: "test"},
		Frameworks: []models.Framework{
			{Name: "jest", Type: models.FrameworkTypeUnit, FileCount: 2},
		},
		TestFiles: []models.TestFile{
			{Path: "test/a.test.js", Framework: "jest", TestCount: 3},
			{Path: "test/b.test.js", Framework: "jest", TestCount: 2},
		},
		Signals: []models.Signal{
			{Type: "weakAssertion", Category: models.CategoryQuality, Severity: models.SeverityMedium},
		},
	}
	report := Build(&BuildInput{Snapshot: snap})
	if report.TestsDetected.TestFileCount != 2 {
		t.Errorf("test files = %d, want 2", report.TestsDetected.TestFileCount)
	}
	if report.SignalSummary.Total != 1 {
		t.Errorf("signals = %d, want 1", report.SignalSummary.Total)
	}
	if len(report.TestsDetected.Frameworks) != 1 {
		t.Errorf("frameworks = %d, want 1", len(report.TestsDetected.Frameworks))
	}
}

func TestBuild_TopInsight(t *testing.T) {
	snap := smallRepoSnapshot()
	report := Build(&BuildInput{Snapshot: snap})
	if report.TopInsight == "" {
		t.Error("expected non-empty top insight")
	}
}

// --- Edge Case Scenario Tests ---

// TestEdgeCase_FewTests verifies analyze handles repos with very few tests.
func TestEdgeCase_FewTests(t *testing.T) {
	t.Parallel()
	snap := &models.TestSuiteSnapshot{
		TestFiles: []models.TestFile{
			{Path: "test/a.test.js", Framework: "jest", TestCount: 2},
		},
		TestCases: []models.TestCase{
			{TestID: "t1", TestName: "login", FilePath: "test/a.test.js"},
			{TestID: "t2", TestName: "logout", FilePath: "test/a.test.js"},
		},
		Frameworks: []models.Framework{
			{Name: "jest", Type: models.FrameworkTypeUnit, FileCount: 1},
		},
	}

	report := Build(&BuildInput{Snapshot: snap})

	// Should detect few tests in edge cases.
	hasFewTests := false
	for _, ec := range report.EdgeCases {
		if ec.Type == "FEW_TESTS" {
			hasFewTests = true
		}
	}
	if !hasFewTests {
		t.Error("expected FEW_TESTS edge case for repo with 2 tests")
	}

	// Validation inventory should still be populated.
	if report.TestsDetected.TestCaseCount != 2 {
		t.Errorf("expected 2 test cases, got %d", report.TestsDetected.TestCaseCount)
	}
}

// TestEdgeCase_HeavyManualValidation verifies analyze handles repos
// with significant manual coverage overlay.
func TestEdgeCase_HeavyManualValidation(t *testing.T) {
	t.Parallel()

	var manualItems []models.ManualCoverageArtifact
	for i := 0; i < 25; i++ {
		manualItems = append(manualItems, models.ManualCoverageArtifact{
			ArtifactID: "manual:" + string(rune('a'+i)),
			Name:       "manual-" + string(rune('a'+i)),
			Area:       "billing",
			Source:     "checklist",
		})
	}

	snap := &models.TestSuiteSnapshot{
		TestFiles: []models.TestFile{
			{Path: "test/a.test.js", Framework: "jest", TestCount: 5},
		},
		Frameworks:     []models.Framework{{Name: "jest", FileCount: 1}},
		ManualCoverage: manualItems,
	}

	report := Build(&BuildInput{Snapshot: snap})

	// Should show manual coverage in report.
	if report.ManualCoverage == nil {
		t.Fatal("expected manual coverage section")
	}
	if report.ManualCoverage.ArtifactCount != 25 {
		t.Errorf("expected 25 manual artifacts, got %d", report.ManualCoverage.ArtifactCount)
	}

	// Should detect LARGE_MANUAL_SUITE edge case.
	hasManual := false
	for _, ec := range report.EdgeCases {
		if ec.Type == "LARGE_MANUAL_SUITE" {
			hasManual = true
		}
	}
	if !hasManual {
		t.Error("expected LARGE_MANUAL_SUITE edge case for 25 manual items")
	}
}

// TestEdgeCase_AIHeavyValidation verifies analyze handles repos with
// scenarios, prompts, and datasets.
func TestEdgeCase_AIHeavyValidation(t *testing.T) {
	t.Parallel()
	snap := &models.TestSuiteSnapshot{
		TestFiles: []models.TestFile{
			{Path: "tests/eval/test_accuracy.py", Framework: "pytest", TestCount: 10},
		},
		TestCases: []models.TestCase{
			{TestID: "t1", TestName: "test_accuracy", FilePath: "tests/eval/test_accuracy.py"},
		},
		Frameworks: []models.Framework{
			{Name: "pytest", Type: models.FrameworkTypeUnit, FileCount: 1},
		},
		CodeSurfaces: []models.CodeSurface{
			{SurfaceID: "s1", Name: "build_prompt", Kind: models.SurfacePrompt, Path: "src/ai/prompts.py"},
			{SurfaceID: "s2", Name: "system_template", Kind: models.SurfacePrompt, Path: "src/ai/prompts.py"},
			{SurfaceID: "s3", Name: "load_dataset", Kind: models.SurfaceDataset, Path: "src/data/loader.py"},
			{SurfaceID: "s4", Name: "predict", Kind: models.SurfaceFunction, Path: "src/model.py"},
		},
		Scenarios: []models.Scenario{
			{ScenarioID: "sc1", Name: "safety-check", Category: "safety", CoveredSurfaceIDs: []string{"s1", "s2"}},
			{ScenarioID: "sc2", Name: "accuracy", Category: "accuracy", CoveredSurfaceIDs: []string{"s4"}},
		},
	}

	report := Build(&BuildInput{Snapshot: snap})

	// Validation inventory should include prompts, datasets, scenarios.
	if report.TestsDetected.PromptCount != 2 {
		t.Errorf("expected 2 prompts, got %d", report.TestsDetected.PromptCount)
	}
	if report.TestsDetected.DatasetCount != 1 {
		t.Errorf("expected 1 dataset, got %d", report.TestsDetected.DatasetCount)
	}
	if report.TestsDetected.ScenarioCount != 2 {
		t.Errorf("expected 2 scenarios, got %d", report.TestsDetected.ScenarioCount)
	}
	if report.TestsDetected.CodeSurfaceCount != 4 {
		t.Errorf("expected 4 code surfaces, got %d", report.TestsDetected.CodeSurfaceCount)
	}
}

// TestEdgeCase_FlakyTests verifies analyze handles repos with flaky test signals.
func TestEdgeCase_FlakyTests(t *testing.T) {
	t.Parallel()
	snap := &models.TestSuiteSnapshot{
		TestFiles: []models.TestFile{
			{Path: "test/a.test.js", Framework: "jest", TestCount: 20},
			{Path: "test/b.test.js", Framework: "jest", TestCount: 20},
			{Path: "test/c.test.js", Framework: "jest", TestCount: 20},
		},
		Frameworks: []models.Framework{{Name: "jest", FileCount: 3}},
		Signals: []models.Signal{
			{Type: "flakyTest", Severity: models.SeverityMedium, Location: models.SignalLocation{File: "test/a.test.js"}},
			{Type: "flakyTest", Severity: models.SeverityMedium, Location: models.SignalLocation{File: "test/b.test.js"}},
			{Type: "unstableSuite", Severity: models.SeverityHigh, Location: models.SignalLocation{File: "test/c.test.js"}},
		},
	}

	report := Build(&BuildInput{Snapshot: snap})

	// Signals should be counted.
	if report.SignalSummary.Total != 3 {
		t.Errorf("expected 3 signals, got %d", report.SignalSummary.Total)
	}
	if report.SignalSummary.High != 1 {
		t.Errorf("expected 1 high signal, got %d", report.SignalSummary.High)
	}
	if report.SignalSummary.Medium != 2 {
		t.Errorf("expected 2 medium signals, got %d", report.SignalSummary.Medium)
	}
}

// TestEdgeCase_ExternalServiceAndGenerated verifies the edge case detection
// at the depgraph level for external services and generated artifacts.
// These are tested via depgraph.DetectEdgeCases (profile_test.go) since the
// analyze pipeline derives these counts from graph stats, not from BuildInput.
// This test verifies the analyze report processes edge cases correctly
// when they are detected.
func TestEdgeCase_AnalyzeReportIncludesEdgeCases(t *testing.T) {
	t.Parallel()
	// A repo with low coverage confidence triggers LOW_GRAPH_VISIBILITY.
	snap := &models.TestSuiteSnapshot{
		TestFiles: []models.TestFile{
			{Path: "test/a.test.js", Framework: "jest", TestCount: 3},
		},
		CodeUnits: []models.CodeUnit{
			{UnitID: "src/a.js:fn", Name: "fn", Path: "src/a.js", Exported: true},
			{UnitID: "src/b.js:fn", Name: "fn", Path: "src/b.js", Exported: true},
			{UnitID: "src/c.js:fn", Name: "fn", Path: "src/c.js", Exported: true},
		},
		Frameworks: []models.Framework{{Name: "jest", FileCount: 1}},
	}

	report := Build(&BuildInput{Snapshot: snap})

	// Should have edge cases (at minimum FAST_CI_ALREADY for low test count).
	if len(report.EdgeCases) == 0 {
		t.Error("expected at least one edge case for small repo")
	}

	// Edge cases should have type and severity.
	for _, ec := range report.EdgeCases {
		if ec.Type == "" {
			t.Error("edge case missing type")
		}
		if ec.Severity == "" {
			t.Error("edge case missing severity")
		}
		if ec.Description == "" {
			t.Error("edge case missing description")
		}
	}

	// Policy should be generated from edge cases.
	if report.Policy == nil {
		t.Error("expected policy to be generated from edge cases")
	}
}

// --- Artifact Schema Tests ---

func TestBuild_SchemaVersionPresent(t *testing.T) {
	t.Parallel()
	report := Build(&BuildInput{Snapshot: &models.TestSuiteSnapshot{}})
	if report.SchemaVersion != AnalyzeReportSchemaVersion {
		t.Errorf("schemaVersion = %q, want %q", report.SchemaVersion, AnalyzeReportSchemaVersion)
	}
}

func TestBuild_SchemaVersionStable(t *testing.T) {
	t.Parallel()
	// Version should be "1" — not empty, not "0".
	if AnalyzeReportSchemaVersion != "1" {
		t.Errorf("expected schema version '1', got %q", AnalyzeReportSchemaVersion)
	}
}

func TestBuildRiskPosture_NilMeasurements(t *testing.T) {
	t.Parallel()
	snap := &models.TestSuiteSnapshot{}
	dims := buildRiskPosture(snap)
	if dims != nil {
		t.Errorf("expected nil for nil measurements, got %v", dims)
	}
}

func TestBuildRiskPosture_PopulatesDimensions(t *testing.T) {
	t.Parallel()
	snap := &models.TestSuiteSnapshot{
		Measurements: &models.MeasurementSnapshot{
			Posture: []models.DimensionPostureResult{
				{Dimension: "health", Band: "STRONG"},
				{Dimension: "coverage_depth", Band: "MODERATE"},
			},
		},
	}
	dims := buildRiskPosture(snap)
	if len(dims) != 2 {
		t.Fatalf("expected 2 dimensions, got %d", len(dims))
	}
	if dims[0].Dimension != "health" || dims[0].Band != "STRONG" {
		t.Errorf("dim[0] = %v", dims[0])
	}
	if dims[1].Dimension != "coverage_depth" || dims[1].Band != "MODERATE" {
		t.Errorf("dim[1] = %v", dims[1])
	}
}

func TestFanoutNodeLabel(t *testing.T) {
	t.Parallel()
	cases := []struct {
		nodeID   string
		nodeType string
		want     string
	}{
		{"behavior:module:src/auth.ts", "behavior_surface", "src/auth.ts"},
		{"file:src/db.ts", "source_file", "src/db.ts"},
		{"owner:ml-team", "owner", "ml-team"},
		{"singleton", "test", "test"},
		{"singleton", "", "singleton"},
	}
	for _, tc := range cases {
		got := fanoutNodeLabel(tc.nodeID, tc.nodeType)
		if got != tc.want {
			t.Errorf("fanoutNodeLabel(%q, %q) = %q, want %q", tc.nodeID, tc.nodeType, got, tc.want)
		}
	}
}

func TestDeriveTopInsight_NoDuplicatesNoFanout(t *testing.T) {
	t.Parallel()
	r := &Report{SkippedTestBurden: SkipSummary{SkippedCount: 5}}
	fanout := &depgraph.FanoutResult{}
	dupes := &depgraph.DuplicateResult{}
	cov := &depgraph.CoverageResult{BandCounts: map[depgraph.CoverageBand]int{}}

	got := deriveTopInsight(r, fanout, dupes, cov)
	if got == "" {
		t.Fatal("expected non-empty insight")
	}
	// Should fall through to skip burden.
	if !contains(got, "skipped") {
		t.Errorf("expected skip-related insight, got %q", got)
	}
}

func TestDeriveTopInsight_Default(t *testing.T) {
	t.Parallel()
	r := &Report{}
	fanout := &depgraph.FanoutResult{}
	dupes := &depgraph.DuplicateResult{}
	cov := &depgraph.CoverageResult{BandCounts: map[depgraph.CoverageBand]int{}}

	got := deriveTopInsight(r, fanout, dupes, cov)
	if got == "" {
		t.Fatal("expected non-empty default insight")
	}
	if !contains(got, "No major issues") {
		t.Errorf("expected default insight, got %q", got)
	}
}

func TestDsAvailable(t *testing.T) {
	t.Parallel()
	snap := &models.TestSuiteSnapshot{
		DataSources: []models.DataSource{
			{Name: "coverage", Status: models.DataSourceAvailable},
			{Name: "runtime", Status: models.DataSourceUnavailable},
		},
	}
	if !dsAvailable(snap, "coverage") {
		t.Error("expected coverage to be available")
	}
	if dsAvailable(snap, "runtime") {
		t.Error("expected runtime to be unavailable")
	}
	if dsAvailable(snap, "policy") {
		t.Error("expected policy (not present) to be unavailable")
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsSubstring(s, sub))
}

func containsSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
