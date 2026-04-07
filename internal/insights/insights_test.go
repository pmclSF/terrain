package insights

import (
	"testing"

	"github.com/pmclSF/terrain/internal/depgraph"
	"github.com/pmclSF/terrain/internal/matrix"
	"github.com/pmclSF/terrain/internal/models"
)

func TestBuild_EmptySnapshot(t *testing.T) {
	t.Parallel()
	snap := &models.TestSuiteSnapshot{}
	input := &BuildInput{
		Snapshot: snap,
		Coverage: depgraph.CoverageResult{BandCounts: map[depgraph.CoverageBand]int{}},
	}

	r := Build(input)

	if r.HealthGrade != "A" {
		t.Errorf("expected health grade A for empty snapshot, got %s", r.HealthGrade)
	}
	if len(r.Findings) != 0 {
		t.Errorf("expected 0 findings for empty snapshot, got %d", len(r.Findings))
	}
	if r.Headline == "" {
		t.Error("expected non-empty headline")
	}
}

func TestBuild_DuplicateFindings(t *testing.T) {
	t.Parallel()
	snap := &models.TestSuiteSnapshot{}
	input := &BuildInput{
		Snapshot: snap,
		Coverage: depgraph.CoverageResult{BandCounts: map[depgraph.CoverageBand]int{}},
		Duplicates: depgraph.DuplicateResult{
			DuplicateCount: 150,
			TestsAnalyzed:  500,
			Clusters: []depgraph.DuplicateCluster{
				{ID: 1, Tests: []string{"t1", "t2", "t3"}, Similarity: 0.85},
				{ID: 2, Tests: []string{"t4", "t5"}, Similarity: 0.72},
			},
		},
	}

	r := Build(input)

	found := false
	for _, f := range r.Findings {
		if f.Category == CategoryOptimization {
			found = true
			if f.Severity != SeverityHigh {
				t.Errorf("expected high severity for 150 duplicates, got %s", f.Severity)
			}
		}
	}
	if !found {
		t.Error("expected optimization finding for duplicates")
	}
}

func TestBuild_FanoutFindings(t *testing.T) {
	t.Parallel()
	snap := &models.TestSuiteSnapshot{}
	input := &BuildInput{
		Snapshot: snap,
		Coverage: depgraph.CoverageResult{BandCounts: map[depgraph.CoverageBand]int{}},
		Fanout: depgraph.FanoutResult{
			FlaggedCount: 3,
			Threshold:    10,
			NodeCount:    100,
			Entries: []depgraph.FanoutEntry{
				{NodeID: "n1", Path: "fixtures/auth.js", NodeType: "helper", TransitiveFanout: 200, Flagged: true},
				{NodeID: "n2", Path: "fixtures/db.js", NodeType: "helper", TransitiveFanout: 50, Flagged: true},
			},
		},
	}

	r := Build(input)

	found := false
	for _, f := range r.Findings {
		if f.Category == CategoryArchitectureDebt {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected architecture debt finding for high fanout")
	}
}

func TestBuild_CoverageFindings(t *testing.T) {
	t.Parallel()
	snap := &models.TestSuiteSnapshot{}
	input := &BuildInput{
		Snapshot: snap,
		Coverage: depgraph.CoverageResult{
			SourceCount: 100,
			BandCounts: map[depgraph.CoverageBand]int{
				depgraph.CoverageBandLow:    60,
				depgraph.CoverageBandMedium: 25,
				depgraph.CoverageBandHigh:   15,
			},
			Sources: []depgraph.SourceCoverage{
				{Path: "src/billing.js", TestCount: 0, Band: depgraph.CoverageBandLow},
			},
		},
	}

	r := Build(input)

	found := false
	for _, f := range r.Findings {
		if f.Category == CategoryCoverageDebt {
			found = true
			if f.Severity != SeverityHigh {
				t.Errorf("expected high severity for 60%% uncovered, got %s", f.Severity)
			}
		}
	}
	if !found {
		t.Error("expected coverage debt finding")
	}
}

func TestBuild_SkipFindings(t *testing.T) {
	t.Parallel()
	snap := &models.TestSuiteSnapshot{
		TestFiles: []models.TestFile{
			{Path: "test/a.test.js", TestCount: 100, SkipCount: 5},
		},
	}
	input := &BuildInput{
		Snapshot: snap,
		Coverage: depgraph.CoverageResult{BandCounts: map[depgraph.CoverageBand]int{}},
	}

	r := Build(input)

	found := false
	for _, f := range r.Findings {
		if f.Category == CategoryReliability && f.Metric == "5 skipped" {
			found = true
		}
	}
	if !found {
		t.Error("expected reliability finding for skipped tests")
	}
}

func TestBuild_StabilityFindings(t *testing.T) {
	t.Parallel()
	snap := &models.TestSuiteSnapshot{
		Signals: []models.Signal{
			{Type: "flakyTest", Severity: models.SeverityHigh},
			{Type: "flakyTest", Severity: models.SeverityHigh},
			{Type: "unstableSuite", Severity: models.SeverityHigh},
		},
	}
	input := &BuildInput{
		Snapshot: snap,
		Coverage: depgraph.CoverageResult{BandCounts: map[depgraph.CoverageBand]int{}},
	}

	r := Build(input)

	found := false
	for _, f := range r.Findings {
		if f.Category == CategoryReliability && f.Metric == "3 flaky signals" {
			found = true
		}
	}
	if !found {
		t.Error("expected reliability finding for flaky tests")
	}
}

func TestBuild_HealthGrade(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		signals  []models.Signal
		coverage depgraph.CoverageResult
		want     string
	}{
		{
			name:     "grade A - no findings",
			want:     "A",
			coverage: depgraph.CoverageResult{BandCounts: map[depgraph.CoverageBand]int{}},
		},
		{
			name: "grade D - critical signals",
			signals: []models.Signal{
				{Type: "flakyTest", Severity: models.SeverityCritical},
				{Type: "flakyTest", Severity: models.SeverityCritical},
			},
			coverage: depgraph.CoverageResult{BandCounts: map[depgraph.CoverageBand]int{}},
			want:     "D",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			snap := &models.TestSuiteSnapshot{Signals: tt.signals}
			input := &BuildInput{
				Snapshot: snap,
				Coverage: tt.coverage,
			}
			r := Build(input)
			if r.HealthGrade != tt.want {
				t.Errorf("expected grade %s, got %s", tt.want, r.HealthGrade)
			}
		})
	}
}

func TestBuild_RecommendationsPrioritized(t *testing.T) {
	t.Parallel()
	snap := &models.TestSuiteSnapshot{
		TestFiles: []models.TestFile{
			{Path: "test/a.test.js", TestCount: 100},
		},
		Signals: []models.Signal{
			{Type: "skippedTest", Severity: models.SeverityMedium},
		},
	}
	input := &BuildInput{
		Snapshot: snap,
		Coverage: depgraph.CoverageResult{
			SourceCount: 10,
			BandCounts: map[depgraph.CoverageBand]int{
				depgraph.CoverageBandLow: 5,
			},
			Sources: []depgraph.SourceCoverage{
				{Path: "src/a.js", Band: depgraph.CoverageBandLow},
			},
		},
		Fanout: depgraph.FanoutResult{
			FlaggedCount: 2,
			Threshold:    10,
			NodeCount:    50,
			Entries: []depgraph.FanoutEntry{
				{Path: "helpers/auth.js", TransitiveFanout: 100, Flagged: true},
			},
		},
	}

	r := Build(input)

	if len(r.Recommendations) == 0 {
		t.Fatal("expected recommendations")
	}
	for i, rec := range r.Recommendations {
		if rec.Priority != i+1 {
			t.Errorf("recommendation %d has priority %d, expected %d", i, rec.Priority, i+1)
		}
	}
}

func TestBuild_Limitations(t *testing.T) {
	t.Parallel()
	snap := &models.TestSuiteSnapshot{}
	input := &BuildInput{
		Snapshot: snap,
		Coverage: depgraph.CoverageResult{BandCounts: map[depgraph.CoverageBand]int{}},
	}

	r := Build(input)

	if len(r.Limitations) == 0 {
		t.Error("expected limitations for empty snapshot")
	}
}

func TestBuild_CategorySummary(t *testing.T) {
	t.Parallel()
	snap := &models.TestSuiteSnapshot{
		Signals: []models.Signal{
			{Type: "flakyTest", Severity: models.SeverityHigh},
		},
	}
	input := &BuildInput{
		Snapshot: snap,
		Coverage: depgraph.CoverageResult{
			SourceCount: 20,
			BandCounts:  map[depgraph.CoverageBand]int{depgraph.CoverageBandLow: 10},
			Sources: []depgraph.SourceCoverage{
				{Path: "src/a.js", Band: depgraph.CoverageBandLow},
			},
		},
	}

	r := Build(input)

	if len(r.CategorySummary) == 0 {
		t.Error("expected non-empty category summary")
	}
}

// --- AI Scenario Duplication ---

func TestBuild_ScenarioDuplication(t *testing.T) {
	t.Parallel()
	snap := &models.TestSuiteSnapshot{
		Scenarios: []models.Scenario{
			{
				ScenarioID:        "scenario:safety",
				Name:              "safety-check",
				CoveredSurfaceIDs: []string{"surface:prompts.ts:system", "surface:prompts.ts:user"},
			},
			{
				ScenarioID:        "scenario:accuracy",
				Name:              "accuracy-check",
				CoveredSurfaceIDs: []string{"surface:prompts.ts:system", "surface:prompts.ts:user"},
			},
			{
				ScenarioID:        "scenario:latency",
				Name:              "latency-check",
				CoveredSurfaceIDs: []string{"surface:api.ts:predict"},
			},
		},
	}
	input := &BuildInput{
		Snapshot: snap,
		Coverage: depgraph.CoverageResult{BandCounts: map[depgraph.CoverageBand]int{}},
	}

	r := Build(input)

	// safety and accuracy overlap 100% (2/2 shared surfaces).
	// latency has no overlap with either.
	found := false
	for _, f := range r.Findings {
		if f.Category == CategoryOptimization && f.Title != "" {
			if contains(f.Title, "scenario pair") {
				found = true
				if f.Severity != SeverityLow && f.Severity != SeverityMedium {
					t.Errorf("expected low or medium severity, got %s", f.Severity)
				}
			}
		}
	}
	if !found {
		t.Error("expected scenario duplication finding when 2 scenarios share >50% surfaces")
	}
}

func TestBuild_ScenarioDuplication_NoOverlap(t *testing.T) {
	t.Parallel()
	snap := &models.TestSuiteSnapshot{
		Scenarios: []models.Scenario{
			{ScenarioID: "scenario:a", CoveredSurfaceIDs: []string{"s1"}},
			{ScenarioID: "scenario:b", CoveredSurfaceIDs: []string{"s2"}},
		},
	}
	input := &BuildInput{
		Snapshot: snap,
		Coverage: depgraph.CoverageResult{BandCounts: map[depgraph.CoverageBand]int{}},
	}

	r := Build(input)

	for _, f := range r.Findings {
		if contains(f.Title, "scenario pair") {
			t.Error("should not report scenario duplication when no surfaces overlap")
		}
	}
}

func TestBuild_ScenarioDuplication_SingleScenario(t *testing.T) {
	t.Parallel()
	snap := &models.TestSuiteSnapshot{
		Scenarios: []models.Scenario{
			{ScenarioID: "scenario:only", CoveredSurfaceIDs: []string{"s1", "s2"}},
		},
	}
	input := &BuildInput{
		Snapshot: snap,
		Coverage: depgraph.CoverageResult{BandCounts: map[depgraph.CoverageBand]int{}},
	}

	r := Build(input)

	for _, f := range r.Findings {
		if contains(f.Title, "scenario pair") {
			t.Error("should not report scenario duplication with only 1 scenario")
		}
	}
}

func TestDeduplicateInsightFindings(t *testing.T) {
	t.Parallel()
	findings := []Finding{
		{Category: CategoryOptimization, Title: "3 duplicate clusters", Scope: "tests/unit"},
		{Category: CategoryOptimization, Title: "3 duplicate clusters", Scope: "tests/unit"}, // exact dup
		{Category: CategoryArchitectureDebt, Title: "5 high-fanout nodes", Scope: "src/shared"},
		{Category: CategoryOptimization, Title: "3 duplicate clusters", Scope: "tests/e2e"}, // different scope
	}

	deduped := deduplicateInsightFindings(findings)
	if len(deduped) != 3 {
		t.Errorf("expected 3 after dedup, got %d", len(deduped))
	}
}

func TestBuild_FindingsRankedBySeverityThenCategory(t *testing.T) {
	t.Parallel()
	snap := &models.TestSuiteSnapshot{
		Signals: []models.Signal{
			{Type: "flakyTest", Severity: models.SeverityHigh},
			{Type: "weakAssertion", Severity: models.SeverityMedium},
		},
	}
	input := &BuildInput{
		Snapshot: snap,
		Coverage: depgraph.CoverageResult{BandCounts: map[depgraph.CoverageBand]int{}},
		Fanout: depgraph.FanoutResult{
			FlaggedCount: 1,
			Threshold:    10,
			Entries:      []depgraph.FanoutEntry{{NodeID: "n1", NodeType: "file", TransitiveFanout: 20, Flagged: true}},
		},
	}

	r := Build(input)
	if len(r.Findings) < 2 {
		t.Fatalf("expected at least 2 findings, got %d", len(r.Findings))
	}

	// Verify sorted: higher severity first.
	for i := 1; i < len(r.Findings); i++ {
		si := severityOrder(r.Findings[i-1].Severity)
		sj := severityOrder(r.Findings[i].Severity)
		if si < sj {
			t.Errorf("finding %d (sev=%s) ranked after %d (sev=%s) — wrong severity order",
				i-1, r.Findings[i-1].Severity, i, r.Findings[i].Severity)
		}
	}
}

func TestBuild_DeterministicOutput(t *testing.T) {
	t.Parallel()
	makeInput := func() *BuildInput {
		return &BuildInput{
			Snapshot: &models.TestSuiteSnapshot{
				Signals: []models.Signal{
					{Type: "flakyTest", Severity: models.SeverityHigh},
					{Type: "weakAssertion", Severity: models.SeverityMedium},
				},
			},
			Coverage: depgraph.CoverageResult{BandCounts: map[depgraph.CoverageBand]int{}},
			Duplicates: depgraph.DuplicateResult{
				DuplicateCount: 10,
				Clusters:       []depgraph.DuplicateCluster{{ID: 1, Tests: []string{"t1", "t2"}, Similarity: 0.8}},
			},
		}
	}

	r1 := Build(makeInput())
	r2 := Build(makeInput())

	if len(r1.Findings) != len(r2.Findings) {
		t.Fatalf("non-deterministic finding count: %d vs %d", len(r1.Findings), len(r2.Findings))
	}
	for i := range r1.Findings {
		if r1.Findings[i].Title != r2.Findings[i].Title {
			t.Errorf("finding %d differs: %q vs %q", i, r1.Findings[i].Title, r2.Findings[i].Title)
		}
		if r1.Findings[i].Priority != r2.Findings[i].Priority {
			t.Errorf("finding %d priority differs: %d vs %d", i, r1.Findings[i].Priority, r2.Findings[i].Priority)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && searchString(s, substr)))
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// matrixFindings
// ---------------------------------------------------------------------------

func TestBuild_MatrixFindings_Gaps(t *testing.T) {
	t.Parallel()
	input := &BuildInput{
		Snapshot: &models.TestSuiteSnapshot{},
		Coverage: depgraph.CoverageResult{BandCounts: map[depgraph.CoverageBand]int{}},
		MatrixCoverage: &matrix.MatrixResult{
			ClassesAnalyzed: 1,
			Classes: []matrix.ClassCoverage{
				{ClassID: "envclass:browser", ClassName: "Browsers", Dimension: "browser", TotalMembers: 3, CoveredMembers: 1},
			},
			Gaps: []matrix.CoverageGap{
				{ClassID: "envclass:browser", ClassName: "Browsers", Dimension: "browser", MemberID: "env:firefox", MemberName: "Firefox"},
				{ClassID: "envclass:browser", ClassName: "Browsers", Dimension: "browser", MemberID: "env:webkit", MemberName: "WebKit"},
			},
		},
	}

	r := Build(input)

	found := false
	for _, f := range r.Findings {
		if f.Category == CategoryCoverageDebt && contains(f.Title, "coverage gaps") {
			found = true
			if !contains(f.Description, "Firefox") {
				t.Errorf("expected Firefox in gap description, got %q", f.Description)
			}
		}
	}
	if !found {
		t.Error("expected coverage debt finding for matrix gaps")
	}
}

func TestBuild_MatrixFindings_Concentration(t *testing.T) {
	t.Parallel()
	input := &BuildInput{
		Snapshot: &models.TestSuiteSnapshot{},
		Coverage: depgraph.CoverageResult{BandCounts: map[depgraph.CoverageBand]int{}},
		MatrixCoverage: &matrix.MatrixResult{
			ClassesAnalyzed: 1,
			Classes: []matrix.ClassCoverage{
				{ClassID: "envclass:device", ClassName: "Devices", Dimension: "device", TotalMembers: 3, CoveredMembers: 2},
			},
			Concentrations: []matrix.Concentration{
				{ClassID: "envclass:device", ClassName: "Devices", Dimension: "device",
					DominantMember: "device:iphone", DominantName: "iPhone 15", DominantShare: 0.85,
					TotalMembers: 3, CoveredMembers: 2},
			},
		},
	}

	r := Build(input)

	found := false
	for _, f := range r.Findings {
		if f.Category == CategoryCoverageDebt && contains(f.Title, "concentration") {
			found = true
			if !contains(f.Title, "iPhone 15") {
				t.Errorf("expected 'iPhone 15' in concentration title, got %q", f.Title)
			}
		}
	}
	if !found {
		t.Error("expected coverage debt finding for device concentration")
	}
}

func TestBuild_MatrixFindings_NilMatrix(t *testing.T) {
	t.Parallel()
	input := &BuildInput{
		Snapshot:       &models.TestSuiteSnapshot{},
		Coverage:       depgraph.CoverageResult{BandCounts: map[depgraph.CoverageBand]int{}},
		MatrixCoverage: nil,
	}

	r := Build(input)

	for _, f := range r.Findings {
		if contains(f.Title, "coverage gaps") || contains(f.Title, "concentration") {
			t.Errorf("should not produce matrix findings with nil MatrixCoverage, got %q", f.Title)
		}
	}
}

// ---------------------------------------------------------------------------
// signalFindings
// ---------------------------------------------------------------------------

func TestBuild_SignalFindings_CriticalSignals(t *testing.T) {
	t.Parallel()
	input := &BuildInput{
		Snapshot: &models.TestSuiteSnapshot{
			Signals: []models.Signal{
				{Type: "uncoveredAISurface", Severity: models.SeverityCritical},
				{Type: "phantomEvalScenario", Severity: models.SeverityCritical},
			},
		},
		Coverage: depgraph.CoverageResult{BandCounts: map[depgraph.CoverageBand]int{}},
	}

	r := Build(input)

	found := false
	for _, f := range r.Findings {
		if f.Severity == SeverityCritical && contains(f.Title, "critical-severity") {
			found = true
			if !contains(f.Metric, "2 critical") {
				t.Errorf("expected '2 critical' in metric, got %q", f.Metric)
			}
		}
	}
	if !found {
		t.Error("expected critical signal finding")
	}
}

func TestBuild_SignalFindings_HighSignalsAboveThreshold(t *testing.T) {
	t.Parallel()
	sigs := make([]models.Signal, 12)
	for i := range sigs {
		sigs[i] = models.Signal{Type: "weakAssertion", Severity: models.SeverityHigh}
	}
	input := &BuildInput{
		Snapshot: &models.TestSuiteSnapshot{Signals: sigs},
		Coverage: depgraph.CoverageResult{BandCounts: map[depgraph.CoverageBand]int{}},
	}

	r := Build(input)

	found := false
	for _, f := range r.Findings {
		if contains(f.Title, "high-severity signals") {
			found = true
		}
	}
	if !found {
		t.Error("expected high-severity signal finding when >10 high signals")
	}
}

func TestBuild_SignalFindings_HighSignalsBelowThreshold(t *testing.T) {
	t.Parallel()
	sigs := make([]models.Signal, 5)
	for i := range sigs {
		sigs[i] = models.Signal{Type: "weakAssertion", Severity: models.SeverityHigh}
	}
	input := &BuildInput{
		Snapshot: &models.TestSuiteSnapshot{Signals: sigs},
		Coverage: depgraph.CoverageResult{BandCounts: map[depgraph.CoverageBand]int{}},
	}

	r := Build(input)

	for _, f := range r.Findings {
		if contains(f.Title, "high-severity signals") {
			t.Error("should not produce high-signal finding when <=10 high signals")
		}
	}
}

func TestBuild_SignalFindings_E2EOnlyCoverage(t *testing.T) {
	t.Parallel()
	input := &BuildInput{
		Snapshot: &models.TestSuiteSnapshot{
			CoverageSummary: &models.CoverageSummary{CoveredOnlyByE2E: 15},
		},
		Coverage: depgraph.CoverageResult{BandCounts: map[depgraph.CoverageBand]int{}},
	}

	r := Build(input)

	found := false
	for _, f := range r.Findings {
		if contains(f.Title, "e2e tests") {
			found = true
		}
	}
	if !found {
		t.Error("expected E2E-only coverage finding")
	}
}

// ---------------------------------------------------------------------------
// manualCoverageFindings
// ---------------------------------------------------------------------------

func TestBuild_ManualCoverageFindings_StaleArtifacts(t *testing.T) {
	t.Parallel()
	input := &BuildInput{
		Snapshot: &models.TestSuiteSnapshot{
			ManualCoverage: []models.ManualCoverageArtifact{
				{Name: "security audit", LastExecuted: "", Criticality: "high"},
				{Name: "accessibility check", LastExecuted: "", Criticality: "low"},
				{Name: "performance review", LastExecuted: "2025-01-01", Criticality: "medium"},
			},
		},
		Coverage: depgraph.CoverageResult{BandCounts: map[depgraph.CoverageBand]int{}},
	}

	r := Build(input)

	found := false
	for _, f := range r.Findings {
		if contains(f.Title, "manual coverage") {
			found = true
			// 2 of 3 stale → >= total/2, and 1 is high criticality → SeverityMedium.
			if f.Severity != SeverityMedium {
				t.Errorf("expected medium severity (stale high-crit artifact), got %s", f.Severity)
			}
		}
	}
	if !found {
		t.Error("expected manual coverage finding for stale artifacts")
	}
}

func TestBuild_ManualCoverageFindings_NoStale(t *testing.T) {
	t.Parallel()
	input := &BuildInput{
		Snapshot: &models.TestSuiteSnapshot{
			ManualCoverage: []models.ManualCoverageArtifact{
				{Name: "security audit", LastExecuted: "2025-06-01", Criticality: "high"},
			},
		},
		Coverage: depgraph.CoverageResult{BandCounts: map[depgraph.CoverageBand]int{}},
	}

	r := Build(input)

	for _, f := range r.Findings {
		if contains(f.Title, "manual coverage") {
			t.Error("should not produce manual coverage finding when no artifacts are stale")
		}
	}
}

// ---------------------------------------------------------------------------
// recommendation builder: finding-specific matching
// ---------------------------------------------------------------------------

func TestBuild_Recommendations_ScenarioDupGetsScenarioRec(t *testing.T) {
	t.Parallel()
	input := &BuildInput{
		Snapshot: &models.TestSuiteSnapshot{
			Scenarios: []models.Scenario{
				{ScenarioID: "s1", CoveredSurfaceIDs: []string{"a", "b"}},
				{ScenarioID: "s2", CoveredSurfaceIDs: []string{"a", "b"}},
			},
		},
		Coverage: depgraph.CoverageResult{BandCounts: map[depgraph.CoverageBand]int{}},
		Duplicates: depgraph.DuplicateResult{
			DuplicateCount: 10,
			Clusters:       []depgraph.DuplicateCluster{{ID: 1, Tests: []string{"t1"}, Similarity: 0.8}},
		},
	}

	r := Build(input)

	// Should have BOTH a duplicate-cluster rec AND a scenario rec — not
	// two duplicate-cluster recs.
	hasDuplicateRec := false
	hasScenarioRec := false
	for _, rec := range r.Recommendations {
		if contains(rec.Action, "Consolidate") {
			hasDuplicateRec = true
		}
		if contains(rec.Action, "overlapping") || contains(rec.Action, "scenario") {
			hasScenarioRec = true
		}
	}
	if !hasDuplicateRec {
		t.Error("expected duplicate-cluster recommendation")
	}
	if !hasScenarioRec {
		t.Error("expected scenario-overlap recommendation (not another duplicate rec)")
	}
}
