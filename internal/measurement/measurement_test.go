package measurement

import (
	"fmt"
	"strings"
	"testing"

	"github.com/pmclSF/terrain/internal/models"
	"github.com/pmclSF/terrain/internal/signals"
)

// --- helper builders ---

func makeSnap(testFiles int, sigs ...models.Signal) *models.TestSuiteSnapshot {
	snap := &models.TestSuiteSnapshot{}
	for i := 0; i < testFiles; i++ {
		snap.TestFiles = append(snap.TestFiles, models.TestFile{
			Path:      "test/file_" + string(rune('a'+i)) + ".test.js",
			Framework: "jest",
			TestCount: 1,
		})
	}
	snap.Signals = sigs
	return snap
}

func sig(t models.SignalType) models.Signal {
	return models.Signal{
		Type:     t,
		Location: models.SignalLocation{File: "test/file_a.test.js"},
	}
}

func sigInFile(t models.SignalType, file string) models.Signal {
	return models.Signal{
		Type:     t,
		Location: models.SignalLocation{File: file},
	}
}

// --- helpers tests ---

func TestCountSignals(t *testing.T) {
	t.Parallel()
	snap := makeSnap(3,
		sig(signals.SignalFlakyTest),
		sig(signals.SignalFlakyTest),
		sig(signals.SignalSlowTest),
	)

	if got := countSignals(snap, signals.SignalFlakyTest); got != 2 {
		t.Errorf("countSignals(flakyTest) = %d, want 2", got)
	}
	if got := countSignals(snap, signals.SignalFlakyTest, signals.SignalSlowTest); got != 3 {
		t.Errorf("countSignals(flakyTest, slowTest) = %d, want 3", got)
	}
	if got := countSignals(snap, signals.SignalDeadTest); got != 0 {
		t.Errorf("countSignals(deadTest) = %d, want 0", got)
	}
}

func TestRatioToBand(t *testing.T) {
	t.Parallel()
	tests := []struct {
		ratio float64
		want  string
	}{
		{0.0, "strong"},
		{0.05, "strong"},
		{0.06, "moderate"},
		{0.15, "moderate"},
		{0.16, "weak"},
		{0.30, "weak"},
		{0.31, "critical"},
	}
	for _, tt := range tests {
		got := ratioToBand(tt.ratio, 0.05, 0.15, 0.30)
		if got != tt.want {
			t.Errorf("ratioToBand(%v) = %q, want %q", tt.ratio, got, tt.want)
		}
	}
}

func TestRuntimeEvidence(t *testing.T) {
	t.Parallel()
	// No runtime data.
	snap := makeSnap(2)
	if got := runtimeEvidence(snap); got != EvidenceWeak {
		t.Errorf("runtimeEvidence (no data) = %q, want %q", got, EvidenceWeak)
	}

	// With runtime data.
	snap.TestFiles[0].RuntimeStats = &models.RuntimeStats{AvgRuntimeMs: 100}
	if got := runtimeEvidence(snap); got != EvidenceStrong {
		t.Errorf("runtimeEvidence (with data) = %q, want %q", got, EvidenceStrong)
	}
}

func TestEvidenceLimitations(t *testing.T) {
	t.Parallel()
	if lim := evidenceLimitations(EvidenceStrong); lim != nil {
		t.Errorf("evidenceLimitations(strong) = %v, want nil", lim)
	}
	if lim := evidenceLimitations(EvidenceWeak); len(lim) == 0 {
		t.Error("evidenceLimitations(weak) should return limitations")
	}
	if lim := evidenceLimitations(EvidenceNone); len(lim) == 0 {
		t.Error("evidenceLimitations(none) should return limitations")
	}
}

// --- registry tests ---

func mustRegister(t *testing.T, r *Registry, def Definition) {
	t.Helper()
	if err := r.Register(def); err != nil {
		t.Fatal(err)
	}
}

func TestRegistry_RegisterAndLen(t *testing.T) {
	t.Parallel()
	r := NewRegistry()
	mustRegister(t, r, Definition{ID: "test.one", Dimension: DimensionHealth})
	mustRegister(t, r, Definition{ID: "test.two", Dimension: DimensionHealth})

	if r.Len() != 2 {
		t.Errorf("Len() = %d, want 2", r.Len())
	}
}

func TestRegistry_DuplicateReturnsError(t *testing.T) {
	t.Parallel()
	r := NewRegistry()
	if err := r.Register(Definition{ID: "test.dup", Dimension: DimensionHealth}); err != nil {
		t.Fatalf("first Register failed: %v", err)
	}
	if err := r.Register(Definition{ID: "test.dup", Dimension: DimensionHealth}); err == nil {
		t.Error("expected error on duplicate ID, got nil")
	}
}

func TestRegistry_ByDimension(t *testing.T) {
	t.Parallel()
	r := NewRegistry()
	mustRegister(t, r, Definition{ID: "h.one", Dimension: DimensionHealth})
	mustRegister(t, r, Definition{ID: "s.one", Dimension: DimensionStructuralRisk})
	mustRegister(t, r, Definition{ID: "h.two", Dimension: DimensionHealth})

	health := r.ByDimension(DimensionHealth)
	if len(health) != 2 {
		t.Errorf("ByDimension(health) = %d, want 2", len(health))
	}
	structural := r.ByDimension(DimensionStructuralRisk)
	if len(structural) != 1 {
		t.Errorf("ByDimension(structural) = %d, want 1", len(structural))
	}
}

func TestRegistry_Run(t *testing.T) {
	t.Parallel()
	r := NewRegistry()
	mustRegister(t, r, Definition{
		ID:        "test.constant",
		Dimension: DimensionHealth,
		Compute: func(_ *models.TestSuiteSnapshot) Result {
			return Result{ID: "test.constant", Value: 0.42, Band: "moderate"}
		},
	})

	results := r.Run(makeSnap(1))
	if len(results) != 1 {
		t.Fatalf("Run() returned %d results, want 1", len(results))
	}
	if results[0].Value != 0.42 {
		t.Errorf("result value = %v, want 0.42", results[0].Value)
	}
}

// --- default registry tests ---

func TestDefaultRegistry_AllDimensionsCovered(t *testing.T) {
	t.Parallel()
	r, err := DefaultRegistry()
	if err != nil {
		t.Fatal(err)
	}

	dims := []Dimension{
		DimensionHealth,
		DimensionCoverageDepth,
		DimensionCoverageDiversity,
		DimensionStructuralRisk,
		DimensionOperationalRisk,
	}

	for _, dim := range dims {
		defs := r.ByDimension(dim)
		if len(defs) == 0 {
			t.Errorf("no measurements registered for dimension %q", dim)
		}
	}
}

func TestDefaultRegistry_NoDuplicateIDs(t *testing.T) {
	t.Parallel()
	// DefaultRegistry returns an error on duplicate IDs; if this runs without error, no duplicates.
	r, err := DefaultRegistry()
	if err != nil {
		t.Fatalf("DefaultRegistry returned error (likely duplicate ID): %v", err)
	}
	if r.Len() == 0 {
		t.Error("default registry has no measurements")
	}
}

// --- health measurement tests ---

func TestHealth_FlakyShare_NoFiles(t *testing.T) {
	t.Parallel()
	snap := makeSnap(0)
	r := computeFlakyShare(snap)
	if r.Value != 0 || r.Band != "unknown" || r.Evidence != EvidenceNone {
		t.Errorf("unexpected result for empty snap: %+v", r)
	}
}

func TestHealth_FlakyShare_WithSignals(t *testing.T) {
	t.Parallel()
	snap := makeSnap(10,
		sigInFile(signals.SignalFlakyTest, "test/file_a.test.js"),
		sigInFile(signals.SignalFlakyTest, "test/file_b.test.js"),
		sigInFile(signals.SignalUnstableSuite, "test/file_c.test.js"),
	)
	r := computeFlakyShare(snap)
	// 3 unique files out of 10
	if r.Value != 0.3 {
		t.Errorf("flaky_share value = %v, want 0.3", r.Value)
	}
	if r.Band != "weak" {
		t.Errorf("flaky_share band = %q, want 'weak'", r.Band)
	}
}

func TestHealth_SkipDensity_StaticEvidence(t *testing.T) {
	t.Parallel()
	snap := makeSnap(20)
	snap.TestFiles[0].SkipCount = 1
	r := computeSkipDensity(snap)
	if r.Value != 0.05 {
		t.Errorf("skip_density value = %v, want 0.05", r.Value)
	}
	if r.Band != "strong" {
		t.Errorf("skip_density band = %q, want 'strong'", r.Band)
	}
	if r.Evidence != EvidencePartial {
		t.Errorf("skip_density evidence = %q, want 'partial'", r.Evidence)
	}
}

func TestHealth_SkipDensity_RuntimeEvidence(t *testing.T) {
	t.Parallel()
	snap := makeSnap(20,
		sigInFile(signals.SignalSkippedTest, "test/file_a.test.js"),
	)
	snap.TestFiles[0].RuntimeStats = &models.RuntimeStats{AvgRuntimeMs: 100}

	r := computeSkipDensity(snap)
	if r.Value != 0.05 {
		t.Errorf("skip_density value = %v, want 0.05", r.Value)
	}
	if r.Evidence != EvidenceStrong {
		t.Errorf("skip_density evidence = %q, want 'strong'", r.Evidence)
	}
}

func TestHealth_DeadTestShare(t *testing.T) {
	t.Parallel()
	snap := makeSnap(10,
		sigInFile(signals.SignalDeadTest, "test/file_a.test.js"),
		sigInFile(signals.SignalDeadTest, "test/file_b.test.js"),
	)
	r := computeDeadTestShare(snap)
	// 2 unique files out of 10
	if r.Value != 0.2 {
		t.Errorf("dead_test_share value = %v, want 0.2", r.Value)
	}
	if r.Band != "weak" {
		t.Errorf("dead_test_share band = %q, want 'weak'", r.Band)
	}
}

func TestHealth_DeadTestShare_UnknownWithoutRuntime(t *testing.T) {
	t.Parallel()
	// No runtime data and no dead test signals → band should be "unknown".
	// Dead test detection requires runtime results (tests observed only
	// in skipped state), so cannot be assessed statically.
	snap := makeSnap(10)
	r := computeDeadTestShare(snap)
	if r.Band != "unknown" {
		t.Errorf("dead_test_share band without runtime data = %q, want 'unknown'", r.Band)
	}
	if r.Evidence != EvidenceWeak {
		t.Errorf("dead_test_share evidence = %q, want 'weak'", r.Evidence)
	}
}

func TestHealth_DeadTestShare_StrongWithRuntime(t *testing.T) {
	t.Parallel()
	// With runtime data and no dead test signals → band should be "strong".
	snap := makeSnap(10)
	snap.TestFiles[0].RuntimeStats = &models.RuntimeStats{AvgRuntimeMs: 100}
	r := computeDeadTestShare(snap)
	if r.Band != "strong" {
		t.Errorf("dead_test_share band with runtime data = %q, want 'strong'", r.Band)
	}
	if r.Evidence != EvidenceStrong {
		t.Errorf("dead_test_share evidence = %q, want 'strong'", r.Evidence)
	}
}

func TestHealth_SlowTestShare_RuntimeEvidence(t *testing.T) {
	t.Parallel()
	snap := makeSnap(4,
		sig(signals.SignalSlowTest),
	)
	r := computeSlowTestShare(snap)
	if r.Evidence != EvidenceWeak {
		t.Errorf("slow_test_share evidence without runtime = %q, want 'weak'", r.Evidence)
	}

	snap.TestFiles[0].RuntimeStats = &models.RuntimeStats{AvgRuntimeMs: 500}
	r = computeSlowTestShare(snap)
	if r.Evidence != EvidenceStrong {
		t.Errorf("slow_test_share evidence with runtime = %q, want 'strong'", r.Evidence)
	}
}

func TestHealth_FlakyShare_UnknownWithoutRuntime(t *testing.T) {
	t.Parallel()
	// No runtime data and no flaky signals → band should be "unknown", not "strong".
	snap := makeSnap(10)
	r := computeFlakyShare(snap)
	if r.Band != "unknown" {
		t.Errorf("flaky_share band without runtime data = %q, want 'unknown'", r.Band)
	}
	if r.Evidence != EvidenceWeak {
		t.Errorf("flaky_share evidence = %q, want 'weak'", r.Evidence)
	}
}

func TestHealth_FlakyShare_StrongWithRuntime(t *testing.T) {
	t.Parallel()
	// With runtime data and no flaky signals → band should be "strong".
	snap := makeSnap(10)
	snap.TestFiles[0].RuntimeStats = &models.RuntimeStats{AvgRuntimeMs: 100}
	r := computeFlakyShare(snap)
	if r.Band != "strong" {
		t.Errorf("flaky_share band with runtime data = %q, want 'strong'", r.Band)
	}
	if r.Evidence != EvidenceStrong {
		t.Errorf("flaky_share evidence = %q, want 'strong'", r.Evidence)
	}
}

func TestHealth_SlowTestShare_UnknownWithoutRuntime(t *testing.T) {
	t.Parallel()
	// No runtime data and no slow signals → band should be "unknown".
	snap := makeSnap(10)
	r := computeSlowTestShare(snap)
	if r.Band != "unknown" {
		t.Errorf("slow_test_share band without runtime data = %q, want 'unknown'", r.Band)
	}
}

func TestHealth_SlowTestShare_StrongWithRuntime(t *testing.T) {
	t.Parallel()
	// With runtime data and no slow signals → band should be "strong".
	snap := makeSnap(10)
	snap.TestFiles[0].RuntimeStats = &models.RuntimeStats{AvgRuntimeMs: 100}
	r := computeSlowTestShare(snap)
	if r.Band != "strong" {
		t.Errorf("slow_test_share band with runtime data = %q, want 'strong'", r.Band)
	}
}

func TestResolvePostureBand_SkipsUnknown(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		bands []string
		want  PostureBand
	}{
		{"all unknown", []string{"unknown", "unknown"}, PostureUnknown},
		{"unknown with strong", []string{"unknown", "strong"}, PostureStrong},
		{"unknown with weak", []string{"unknown", "weak", "strong"}, PostureWeak},
		{"unknown ignored in count", []string{"unknown", "unknown", "weak"}, PostureWeak},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := resolvePostureBand(tt.bands)
			if got != tt.want {
				t.Errorf("resolvePostureBand(%v) = %q, want %q", tt.bands, got, tt.want)
			}
		})
	}
}

// --- coverage depth tests ---

func TestCoverageDepth_UncoveredExports(t *testing.T) {
	t.Parallel()
	snap := makeSnap(5)
	snap.CodeUnits = []models.CodeUnit{
		{Name: "Foo", Exported: true},
		{Name: "Bar", Exported: true},
		{Name: "baz", Exported: false},
	}
	snap.Signals = []models.Signal{
		sig(signals.SignalUntestedExport),
	}

	r := computeUncoveredExports(snap)
	// 1 untested out of 2 exported = 0.5
	if r.Value != 0.5 {
		t.Errorf("uncovered_exports value = %v, want 0.5", r.Value)
	}
}

func TestCoverageDepth_UncoveredExports_NoExports(t *testing.T) {
	t.Parallel()
	snap := makeSnap(5)
	r := computeUncoveredExports(snap)
	if r.Value != 0 || r.Band != "unknown" || r.Evidence != EvidenceNone {
		t.Errorf("uncovered_exports with no exports: want unknown/none, got: %+v", r)
	}
}

func TestCoverageDepth_WeakAssertionShare(t *testing.T) {
	t.Parallel()
	snap := makeSnap(4,
		sigInFile(signals.SignalWeakAssertion, "test/file_a.test.js"),
		sigInFile(signals.SignalWeakAssertion, "test/file_b.test.js"),
	)
	r := computeWeakAssertionShare(snap)
	// 2 unique files out of 4
	if r.Value != 0.5 {
		t.Errorf("weak_assertion_share value = %v, want 0.5", r.Value)
	}
}

func TestCoverageDepth_CoverageBreachShare_NoCovData(t *testing.T) {
	t.Parallel()
	snap := makeSnap(5)
	r := computeCoverageBreachShare(snap)
	if r.Evidence != EvidenceWeak {
		t.Errorf("coverage_breach_share without coverage data evidence = %q, want 'weak'", r.Evidence)
	}
}

func TestCoverageDepth_CoverageBreachShare_WithBreaches(t *testing.T) {
	t.Parallel()
	snap := makeSnap(10,
		sigInFile(signals.SignalCoverageThresholdBreak, "test/file_a.test.js"),
		sigInFile(signals.SignalCoverageThresholdBreak, "test/file_b.test.js"),
	)
	r := computeCoverageBreachShare(snap)
	// 2 unique files out of 10
	if r.Value != 0.2 {
		t.Errorf("coverage_breach_share value = %v, want 0.2", r.Value)
	}
	if r.Evidence != EvidenceStrong {
		t.Errorf("coverage_breach_share evidence = %q, want 'strong'", r.Evidence)
	}
}

// --- coverage diversity tests ---

func TestCoverageDiversity_MockHeavyShare(t *testing.T) {
	t.Parallel()
	snap := makeSnap(5,
		sigInFile(signals.SignalMockHeavyTest, "test/file_a.test.js"),
		sigInFile(signals.SignalMockHeavyTest, "test/file_b.test.js"),
	)
	r := computeMockHeavyShare(snap)
	// 2 unique files out of 5
	if r.Value != 0.4 {
		t.Errorf("mock_heavy_share value = %v, want 0.4", r.Value)
	}
}

func TestCoverageDiversity_FrameworkFragmentation(t *testing.T) {
	t.Parallel()
	snap := makeSnap(10)
	snap.Frameworks = []models.Framework{
		{Name: "jest", FileCount: 6},
		{Name: "mocha", FileCount: 3},
		{Name: "vitest", FileCount: 1},
	}

	r := computeFrameworkFragmentation(snap)
	if r.Value != 0.3 {
		t.Errorf("framework_fragmentation value = %v, want 0.3", r.Value)
	}
	if r.Band != "moderate" {
		t.Errorf("framework_fragmentation band = %q, want 'moderate'", r.Band)
	}
}

func TestCoverageDiversity_FrameworkFragmentation_NoFrameworks(t *testing.T) {
	t.Parallel()
	snap := makeSnap(5)
	r := computeFrameworkFragmentation(snap)
	if r.Value != 0 || r.Evidence != EvidenceNone || r.Band != "unknown" {
		t.Errorf("unexpected result for no frameworks: %+v", r)
	}
}

func TestCoverageDiversity_FrameworkFragmentation_ScalesWithSuiteSize(t *testing.T) {
	t.Parallel()
	// Fragmentation thresholds must scale: absolute counts that are alarming
	// in small suites are normal in large polyglot codebases.
	tests := []struct {
		name     string
		fwCount  int
		files    int
		wantBand string
	}{
		// Small suites: count matters
		{"1fw/1file", 1, 1, "strong"},
		{"1fw/3files", 1, 3, "strong"},
		{"2fw/2files", 2, 2, "strong"},
		{"2fw/5files", 2, 5, "strong"},
		{"3fw/10files", 3, 10, "moderate"},
		{"5fw/10files", 5, 10, "weak"},
		{"8fw/10files", 8, 10, "critical"},
		// Large suites: ratio dampens severity
		{"8fw/700files", 8, 700, "moderate"},   // 1.1% ratio — normal polyglot
		{"5fw/200files", 5, 200, "moderate"},   // 2.5% ratio — reasonable
		{"10fw/100files", 10, 100, "critical"}, // 10% ratio — genuinely fragmented
		{"5fw/50files", 5, 50, "weak"},         // 10% ratio — concerning
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			snap := makeSnap(tt.files)
			for i := 0; i < tt.fwCount; i++ {
				snap.Frameworks = append(snap.Frameworks, models.Framework{
					Name: fmt.Sprintf("fw%d", i), FileCount: 1,
				})
			}
			r := computeFrameworkFragmentation(snap)
			if r.Band != tt.wantBand {
				t.Errorf("framework_fragmentation(%s) band = %q, want %q", tt.name, r.Band, tt.wantBand)
			}
		})
	}
}

func TestCoverageDiversity_E2EConcentration(t *testing.T) {
	t.Parallel()
	snap := &models.TestSuiteSnapshot{
		TestFiles: []models.TestFile{
			{Path: "a.test.js", Framework: "cypress"},
			{Path: "b.test.js", Framework: "cypress"},
			{Path: "c.test.js", Framework: "jest"},
			{Path: "d.test.js", Framework: "jest"},
		},
		Frameworks: []models.Framework{
			{Name: "cypress", Type: models.FrameworkTypeE2E, FileCount: 2},
			{Name: "jest", Type: models.FrameworkTypeUnit, FileCount: 2},
		},
	}

	r := computeE2EConcentration(snap)
	if r.Value != 0.5 {
		t.Errorf("e2e_concentration value = %v, want 0.5", r.Value)
	}
	if r.Band != "strong" {
		t.Errorf("e2e_concentration band = %q, want 'strong'", r.Band)
	}
}

func TestCoverageDiversity_E2EConcentration_HighRatio(t *testing.T) {
	t.Parallel()
	snap := &models.TestSuiteSnapshot{
		TestFiles: []models.TestFile{
			{Path: "a.test.js", Framework: "playwright"},
			{Path: "b.test.js", Framework: "playwright"},
			{Path: "c.test.js", Framework: "playwright"},
			{Path: "d.test.js", Framework: "playwright"},
			{Path: "e.test.js", Framework: "jest"},
		},
		Frameworks: []models.Framework{
			{Name: "playwright", Type: models.FrameworkTypeE2E, FileCount: 4},
			{Name: "jest", Type: models.FrameworkTypeUnit, FileCount: 1},
		},
	}

	r := computeE2EConcentration(snap)
	if r.Value != 0.8 {
		t.Errorf("e2e_concentration value = %v, want 0.8", r.Value)
	}
	if r.Band != "moderate" {
		t.Errorf("e2e_concentration band = %q, want 'moderate'", r.Band)
	}
}

// --- structural risk tests ---

func TestStructuralRisk_MigrationBlockerDensity(t *testing.T) {
	t.Parallel()
	snap := makeSnap(10,
		sigInFile(signals.SignalMigrationBlocker, "test/file_a.test.js"),
		sigInFile(signals.SignalCustomMatcherRisk, "test/file_b.test.js"),
	)
	r := computeMigrationBlockerDensity(snap)
	// 2 unique files with blocker signals across 10 files
	if r.Value != 0.2 {
		t.Errorf("migration_blocker_density value = %v, want 0.2", r.Value)
	}
}

func TestStructuralRisk_MigrationBlockerDensity_ExcludesOtherSignals(t *testing.T) {
	t.Parallel()
	// deprecatedTestPattern and dynamicTestGeneration should NOT be counted here
	// because they have their own dedicated measurements.
	snap := makeSnap(10,
		sig(signals.SignalMigrationBlocker),
		sig(signals.SignalDeprecatedTestPattern),
		sig(signals.SignalDynamicTestGeneration),
	)
	r := computeMigrationBlockerDensity(snap)
	// Only 1 signal (migrationBlocker) counts; deprecated/dynamic are excluded
	if r.Value != 0.1 {
		t.Errorf("migration_blocker_density value = %v, want 0.1 (should exclude deprecated/dynamic)", r.Value)
	}
}

func TestStructuralRisk_DeprecatedPatternShare(t *testing.T) {
	t.Parallel()
	snap := makeSnap(20,
		sigInFile(signals.SignalDeprecatedTestPattern, "test/file_a.test.js"),
		sigInFile(signals.SignalDeprecatedTestPattern, "test/file_b.test.js"),
		sigInFile(signals.SignalDeprecatedTestPattern, "test/file_c.test.js"),
	)
	r := computeDeprecatedPatternShare(snap)
	// 3 unique files out of 20
	if r.Value != 0.15 {
		t.Errorf("deprecated_pattern_share value = %v, want 0.15", r.Value)
	}
	if r.Band != "moderate" {
		t.Errorf("deprecated_pattern_share band = %q, want 'moderate'", r.Band)
	}
}

func TestStructuralRisk_DynamicGenerationShare(t *testing.T) {
	t.Parallel()
	snap := makeSnap(10,
		sig(signals.SignalDynamicTestGeneration),
	)
	r := computeDynamicGenerationShare(snap)
	if r.Value != 0.1 {
		t.Errorf("dynamic_generation_share value = %v, want 0.1", r.Value)
	}
	if r.Band != "moderate" {
		t.Errorf("dynamic_generation_share band = %q, want 'moderate'", r.Band)
	}
}

// --- operational risk tests ---

func TestOperationalRisk_PolicyViolationDensity(t *testing.T) {
	t.Parallel()
	snap := makeSnap(10,
		sigInFile(signals.SignalPolicyViolation, "test/file_a.test.js"),
		sigInFile(signals.SignalPolicyViolation, "test/file_b.test.js"),
	)
	r := computePolicyViolationDensity(snap)
	// 2 unique files with violations across 10 files
	if r.Value != 0.2 {
		t.Errorf("policy_violation_density value = %v, want 0.2", r.Value)
	}
}

func TestOperationalRiskShareFunctions(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name         string
		compute      func(*models.TestSuiteSnapshot) Result
		snapshot     *models.TestSuiteSnapshot
		wantValue    float64
		wantEvidence EvidenceStrength
	}{
		{
			name: "legacy framework share",
			compute: func(snap *models.TestSuiteSnapshot) Result {
				return computeLegacyFrameworkShare(snap)
			},
			snapshot:     makeSnap(10, sig(signals.SignalLegacyFrameworkUsage)),
			wantValue:    0.1,
			wantEvidence: EvidenceStrong,
		},
		{
			name: "runtime budget breach share without runtime evidence",
			compute: func(snap *models.TestSuiteSnapshot) Result {
				return computeRuntimeBudgetBreachShare(snap)
			},
			snapshot:     makeSnap(5, sig(signals.SignalRuntimeBudgetExceeded)),
			wantValue:    0.2,
			wantEvidence: EvidenceWeak,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := tt.compute(tt.snapshot)
			if r.Value != tt.wantValue {
				t.Errorf("value = %v, want %v", r.Value, tt.wantValue)
			}
			if r.Evidence != tt.wantEvidence {
				t.Errorf("evidence = %q, want %q", r.Evidence, tt.wantEvidence)
			}
		})
	}
}

// --- posture computation tests ---

func TestComputeSnapshot_AllDimensionsPresent(t *testing.T) {
	t.Parallel()
	r, err := DefaultRegistry()
	if err != nil {
		t.Fatal(err)
	}
	snap := makeSnap(10,
		sig(signals.SignalFlakyTest),
		sig(signals.SignalWeakAssertion),
		sig(signals.SignalMigrationBlocker),
		sig(signals.SignalPolicyViolation),
	)

	ms := r.ComputeSnapshot(snap)
	if len(ms.Posture) != 5 {
		t.Errorf("expected 5 posture dimensions, got %d", len(ms.Posture))
	}
	if len(ms.Measurements) != r.Len() {
		t.Errorf("expected %d measurements, got %d", r.Len(), len(ms.Measurements))
	}
}

func TestPosture_StrongWhenClean(t *testing.T) {
	t.Parallel()
	r, err := DefaultRegistry()
	if err != nil {
		t.Fatal(err)
	}
	snap := makeSnap(10)

	ms := r.ComputeSnapshot(snap)
	for _, p := range ms.Posture {
		if p.Dimension == DimensionHealth {
			// No runtime data, but skip_density provides partial evidence
			// (static skip detection), which is enough for strong posture.
			// Runtime-dependent measurements (flaky, dead, slow) return
			// "unknown" and are filtered out.
			if p.Band != PostureStrong {
				t.Errorf("health posture = %q, want strong", p.Band)
			}
		}
	}
}

func TestPosture_WeakWhenManyIssues(t *testing.T) {
	t.Parallel()
	sigs := make([]models.Signal, 0)
	for i := 0; i < 8; i++ {
		sigs = append(sigs, sigInFile(signals.SignalFlakyTest, fmt.Sprintf("test/file_%d.test.js", i)))
	}
	snap := makeSnap(10, sigs...)

	r, err := DefaultRegistry()
	if err != nil {
		t.Fatal(err)
	}
	ms := r.ComputeSnapshot(snap)

	for _, p := range ms.Posture {
		if p.Dimension == DimensionHealth {
			if p.Band == PostureStrong {
				t.Errorf("health posture should not be strong with 80%% flaky: %q", p.Band)
			}
		}
	}
}

func TestResolvePostureBand(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		bands []string
		want  PostureBand
	}{
		{"empty", nil, PostureUnknown},
		{"all strong", []string{"strong", "strong"}, PostureStrong},
		{"one weak", []string{"strong", "weak"}, PostureWeak},
		{"majority weak escalates", []string{"weak", "weak", "strong"}, PostureWeak},
		{"critical dominates", []string{"strong", "critical"}, PostureCritical},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := resolvePostureBand(tt.bands)
			if got != tt.want {
				t.Errorf("resolvePostureBand(%v) = %q, want %q", tt.bands, got, tt.want)
			}
		})
	}
}

// --- integration test ---

func TestFullPipeline_EndToEnd(t *testing.T) {
	t.Parallel()
	snap := &models.TestSuiteSnapshot{
		TestFiles: []models.TestFile{
			{Path: "src/auth/auth.test.js", Framework: "jest", RuntimeStats: &models.RuntimeStats{AvgRuntimeMs: 150}},
			{Path: "src/api/api.test.js", Framework: "jest"},
			{Path: "e2e/login.spec.js", Framework: "cypress"},
			{Path: "e2e/checkout.spec.js", Framework: "cypress"},
		},
		Frameworks: []models.Framework{
			{Name: "jest", Type: models.FrameworkTypeUnit, FileCount: 2},
			{Name: "cypress", Type: models.FrameworkTypeE2E, FileCount: 2},
		},
		CodeUnits: []models.CodeUnit{
			{Name: "AuthService", Exported: true},
			{Name: "ApiClient", Exported: true},
		},
		Signals: []models.Signal{
			sigInFile(signals.SignalWeakAssertion, "src/api/api.test.js"),
			sigInFile(signals.SignalMockHeavyTest, "src/auth/auth.test.js"),
			sigInFile(signals.SignalSlowTest, "e2e/login.spec.js"),
			sigInFile(signals.SignalUntestedExport, "src/api/api.test.js"),
		},
	}

	r, err := DefaultRegistry()
	if err != nil {
		t.Fatal(err)
	}
	ms := r.ComputeSnapshot(snap)

	// Every measurement should have a non-empty ID.
	for _, m := range ms.Measurements {
		if m.ID == "" {
			t.Error("measurement has empty ID")
		}
		if m.Explanation == "" {
			t.Error("measurement has empty explanation")
		}
	}

	// Every posture dimension should be present.
	if len(ms.Posture) != 5 {
		t.Errorf("expected 5 posture dimensions, got %d", len(ms.Posture))
	}

	for _, p := range ms.Posture {
		if p.Band == "" {
			t.Errorf("posture dimension %q has empty band", p.Dimension)
		}
		if p.Explanation == "" {
			t.Errorf("posture dimension %q has empty explanation", p.Dimension)
		}
	}
}

// --- countFileSignals tests ---

func TestCountFileSignals_DeduplicatesSameFile(t *testing.T) {
	t.Parallel()
	snap := makeSnap(5,
		sigInFile(signals.SignalFlakyTest, "test/file_a.test.js"),
		sigInFile(signals.SignalFlakyTest, "test/file_a.test.js"), // duplicate
		sigInFile(signals.SignalFlakyTest, "test/file_b.test.js"),
	)

	// countSignals counts all occurrences (3)
	if got := countSignals(snap, signals.SignalFlakyTest); got != 3 {
		t.Errorf("countSignals = %d, want 3", got)
	}
	// countFileSignals counts unique files (2)
	if got := countFileSignals(snap, signals.SignalFlakyTest); got != 2 {
		t.Errorf("countFileSignals = %d, want 2", got)
	}
}

func TestCountFileSignals_MultipleTypes(t *testing.T) {
	t.Parallel()
	snap := makeSnap(5,
		sigInFile(signals.SignalFlakyTest, "test/file_a.test.js"),
		sigInFile(signals.SignalUnstableSuite, "test/file_a.test.js"), // same file, diff type
		sigInFile(signals.SignalFlakyTest, "test/file_b.test.js"),
	)
	// file_a has both types, file_b has one → 2 unique files
	if got := countFileSignals(snap, signals.SignalFlakyTest, signals.SignalUnstableSuite); got != 2 {
		t.Errorf("countFileSignals = %d, want 2", got)
	}
}

func TestCountFileSignals_IgnoresEmptyFile(t *testing.T) {
	t.Parallel()
	snap := makeSnap(5,
		models.Signal{Type: signals.SignalFlakyTest, Location: models.SignalLocation{File: ""}},
		sigInFile(signals.SignalFlakyTest, "test/file_a.test.js"),
	)
	if got := countFileSignals(snap, signals.SignalFlakyTest); got != 1 {
		t.Errorf("countFileSignals = %d, want 1 (should ignore empty file)", got)
	}
}

// --- buildPostureExplanation tests ---

func TestBuildPostureExplanation(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		dim     Dimension
		band    PostureBand
		drivers []string
		want    string
	}{
		// Positive-polarity dimension (Health) — bands read directly.
		{"strong", DimensionHealth, PostureStrong, nil, "Health posture is strong across 3 measurements."},
		{"moderate", DimensionHealth, PostureModerate, nil, "Health posture is moderate. Some measurements indicate room for improvement."},
		{"weak with drivers", DimensionHealth, PostureWeak, []string{"health.flaky_share"}, "Health posture is weak. Driven by: health.flaky_share."},
		{"weak no drivers", DimensionHealth, PostureWeak, nil, "Health posture is weak across 3 measurements."},
		{"elevated", DimensionHealth, PostureElevated, []string{"health.flaky_share", "health.skip_density"}, "Health posture is elevated. Significant issues detected in health.flaky_share, health.skip_density."},
		{"critical", DimensionHealth, PostureCritical, nil, "Health posture is critical. Immediate attention needed."},
		{"unknown", DimensionHealth, PostureUnknown, nil, "Health posture could not be determined."},
		// Coverage-depth: positive polarity, sentence-case display name.
		{"display name coverage_depth", DimensionCoverageDepth, PostureWeak, []string{"coverage_depth.uncovered_exports"}, "Coverage depth posture is weak. Driven by: coverage_depth.uncovered_exports."},
		// Risk-polarity dimension — band translates so "is strong" → "is low".
		// Pre-fix this returned "Structural Risk posture is strong across 3
		// measurement(s)." which natural-English-reads as high risk.
		{"display name structural_risk", DimensionStructuralRisk, PostureStrong, nil, "Structural risk is low across 3 measurements."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := buildPostureExplanation(tt.dim, tt.band, tt.drivers, 3)
			if got != tt.want {
				t.Errorf("buildPostureExplanation(%s, %s) =\n  %q\nwant:\n  %q", tt.dim, tt.band, got, tt.want)
			}
		})
	}
}

// --- joinMax tests ---

func TestJoinMax(t *testing.T) {
	t.Parallel()
	tests := []struct {
		items []string
		max   int
		want  string
	}{
		{nil, 3, ""},
		{[]string{"a"}, 3, "a"},
		{[]string{"a", "b", "c"}, 3, "a, b, c"},
		{[]string{"a", "b", "c", "d"}, 3, "a, b, c (+1 more)"},
		{[]string{"a", "b", "c", "d", "e"}, 2, "a, b (+3 more)"},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%d/%d", len(tt.items), tt.max), func(t *testing.T) {
			t.Parallel()
			got := joinMax(tt.items, tt.max)
			if got != tt.want {
				t.Errorf("joinMax(%v, %d) = %q, want %q", tt.items, tt.max, got, tt.want)
			}
		})
	}
}

// --- DimensionDisplayName tests ---

func TestDimensionDisplayName(t *testing.T) {
	t.Parallel()
	tests := []struct {
		dim  Dimension
		want string
	}{
		{DimensionHealth, "Health"},
		{DimensionCoverageDepth, "Coverage depth"},
		{DimensionCoverageDiversity, "Coverage diversity"},
		{DimensionStructuralRisk, "Structural risk"},
		{DimensionOperationalRisk, "Operational risk"},
		{Dimension("custom"), "custom"},
	}
	for _, tt := range tests {
		t.Run(string(tt.dim), func(t *testing.T) {
			t.Parallel()
			got := DimensionDisplayName(tt.dim)
			if got != tt.want {
				t.Errorf("DimensionDisplayName(%q) = %q, want %q", tt.dim, got, tt.want)
			}
		})
	}
}

// --- DimensionQuestion tests ---

func TestDimensionQuestion(t *testing.T) {
	t.Parallel()
	// Every known dimension should have a non-empty question.
	dims := []Dimension{
		DimensionHealth, DimensionCoverageDepth, DimensionCoverageDiversity,
		DimensionStructuralRisk, DimensionOperationalRisk,
	}
	for _, dim := range dims {
		q := DimensionQuestion(dim)
		if q == "" {
			t.Errorf("DimensionQuestion(%q) is empty", dim)
		}
	}
	// Unknown dimension should return empty.
	if q := DimensionQuestion(Dimension("unknown")); q != "" {
		t.Errorf("DimensionQuestion(unknown) = %q, want empty", q)
	}
}

// --- elevated escalation tests ---

func TestResolvePostureBand_ElevatedEscalation(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		bands []string
		want  PostureBand
	}{
		{
			"all 3 weak escalates to elevated",
			[]string{"weak", "weak", "weak"},
			PostureElevated,
		},
		{
			"2 weak does not escalate (need 3+)",
			[]string{"weak", "weak"},
			PostureWeak,
		},
		{
			"3 weak + 1 strong stays weak (not all weak)",
			[]string{"weak", "weak", "weak", "strong"},
			PostureWeak,
		},
		{
			"already critical stays critical",
			[]string{"critical", "weak", "weak"},
			PostureCritical,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := resolvePostureBand(tt.bands)
			if got != tt.want {
				t.Errorf("resolvePostureBand(%v) = %q, want %q", tt.bands, got, tt.want)
			}
		})
	}
}

// --- zero-file measurement tests ---

func TestZeroFiles_AllShareMeasurementsReturnUnknown(t *testing.T) {
	t.Parallel()
	snap := makeSnap(0)

	shareMeasurements := []struct {
		name    string
		compute func(*models.TestSuiteSnapshot) Result
	}{
		{"flaky_share", computeFlakyShare},
		{"skip_density", computeSkipDensity},
		{"dead_test_share", computeDeadTestShare},
		{"slow_test_share", computeSlowTestShare},
		{"weak_assertion_share", computeWeakAssertionShare},
		{"coverage_breach_share", computeCoverageBreachShare},
		{"mock_heavy_share", computeMockHeavyShare},
		{"e2e_concentration", computeE2EConcentration},
		{"migration_blocker_density", computeMigrationBlockerDensity},
		{"deprecated_pattern_share", computeDeprecatedPatternShare},
		{"dynamic_generation_share", computeDynamicGenerationShare},
		{"policy_violation_density", computePolicyViolationDensity},
		{"legacy_framework_share", computeLegacyFrameworkShare},
		{"runtime_budget_breach_share", computeRuntimeBudgetBreachShare},
	}

	for _, m := range shareMeasurements {
		t.Run(m.name, func(t *testing.T) {
			t.Parallel()
			r := m.compute(snap)
			if r.Band != "unknown" {
				t.Errorf("%s with zero files: band = %q, want 'unknown'", m.name, r.Band)
			}
			if r.Evidence != EvidenceNone {
				t.Errorf("%s with zero files: evidence = %q, want 'none'", m.name, r.Evidence)
			}
		})
	}
}

// --- measurement ID stability test ---

func TestMeasurementIDs_Stable(t *testing.T) {
	t.Parallel()
	r, err := DefaultRegistry()
	if err != nil {
		t.Fatal(err)
	}

	expectedIDs := []string{
		"health.flaky_share",
		"health.skip_density",
		"health.dead_test_share",
		"health.slow_test_share",
		"coverage_depth.uncovered_exports",
		"coverage_depth.weak_assertion_share",
		"coverage_depth.coverage_breach_share",
		"coverage_diversity.mock_heavy_share",
		"coverage_diversity.framework_fragmentation",
		"coverage_diversity.e2e_concentration",
		"coverage_diversity.e2e_only_units",
		"coverage_diversity.unit_test_coverage",
		"structural_risk.migration_blocker_density",
		"structural_risk.deprecated_pattern_share",
		"structural_risk.dynamic_generation_share",
		"operational_risk.policy_violation_density",
		"operational_risk.legacy_framework_share",
		"operational_risk.runtime_budget_breach_share",
	}

	defs := r.All()
	if len(defs) != len(expectedIDs) {
		t.Errorf("expected %d measurements, got %d", len(expectedIDs), len(defs))
	}

	idSet := map[string]bool{}
	for _, d := range defs {
		idSet[d.ID] = true
	}
	for _, id := range expectedIDs {
		if !idSet[id] {
			t.Errorf("expected measurement ID %q not found in registry", id)
		}
	}
}

// --- ratioToBand additional boundary tests ---

func TestRatioToBand_DifferentThresholds(t *testing.T) {
	t.Parallel()
	// Policy violation uses 0.0/0.05/0.15 — any violations trigger moderate.
	tests := []struct {
		ratio float64
		want  string
	}{
		{0.0, "strong"},
		{0.001, "moderate"}, // just above zero → moderate immediately
		{0.05, "moderate"},
		{0.051, "weak"},
		{0.15, "weak"},
		{0.151, "critical"},
	}
	for _, tt := range tests {
		got := ratioToBand(tt.ratio, 0.0, 0.05, 0.15)
		if got != tt.want {
			t.Errorf("ratioToBand(%v, 0/0.05/0.15) = %q, want %q", tt.ratio, got, tt.want)
		}
	}
}

// --- posture computation with mixed evidence ---

func TestPosture_ModerateWhenNoStrongEvidence(t *testing.T) {
	t.Parallel()
	// All measurements return "strong" band but only weak evidence.
	// Dimension posture should be capped at moderate.
	results := []Result{
		{ID: "test.a", Dimension: DimensionHealth, Band: "strong", Evidence: EvidenceWeak},
		{ID: "test.b", Dimension: DimensionHealth, Band: "strong", Evidence: EvidenceWeak},
	}
	dp := computeDimensionPosture(DimensionHealth, results)
	if dp.Band != PostureModerate {
		t.Errorf("expected moderate (evidence cap), got %q", dp.Band)
	}
}

func TestPosture_StrongWithPartialEvidence(t *testing.T) {
	t.Parallel()
	// At least one partial-evidence measurement → strong is allowed.
	results := []Result{
		{ID: "test.a", Dimension: DimensionHealth, Band: "strong", Evidence: EvidencePartial},
		{ID: "test.b", Dimension: DimensionHealth, Band: "strong", Evidence: EvidenceWeak},
	}
	dp := computeDimensionPosture(DimensionHealth, results)
	if dp.Band != PostureStrong {
		t.Errorf("expected strong (partial evidence sufficient), got %q", dp.Band)
	}
}

func TestPosture_DriversListedForWeakMeasurements(t *testing.T) {
	t.Parallel()
	results := []Result{
		{ID: "test.ok", Dimension: DimensionHealth, Band: "strong", Evidence: EvidenceStrong},
		{ID: "test.bad", Dimension: DimensionHealth, Band: "weak", Evidence: EvidenceStrong},
		{ID: "test.worse", Dimension: DimensionHealth, Band: "critical", Evidence: EvidenceStrong},
	}
	dp := computeDimensionPosture(DimensionHealth, results)
	if len(dp.DrivingMeasurements) != 2 {
		t.Errorf("expected 2 driving measurements, got %d: %v", len(dp.DrivingMeasurements), dp.DrivingMeasurements)
	}
}

func TestPosture_ExplanationUsesDisplayName(t *testing.T) {
	t.Parallel()
	results := []Result{
		{ID: "cd.x", Dimension: DimensionCoverageDepth, Band: "strong", Evidence: EvidenceStrong},
	}
	dp := computeDimensionPosture(DimensionCoverageDepth, results)
	if !strings.Contains(dp.Explanation, "Coverage depth") {
		t.Errorf("explanation should use display name, got: %q", dp.Explanation)
	}
	if strings.Contains(dp.Explanation, "coverage_depth") {
		t.Errorf("explanation should not use raw identifier, got: %q", dp.Explanation)
	}
}

// --- coverage-summary measurement tests ---

func TestCoverageDiversity_E2EOnlyUnits_WithData(t *testing.T) {
	t.Parallel()
	snap := makeSnap(5)
	snap.CoverageSummary = &models.CoverageSummary{
		TotalCodeUnits:   100,
		CoveredOnlyByE2E: 20,
	}

	r := computeE2EOnlyUnits(snap)
	if r.Value != 0.2 {
		t.Errorf("e2e_only_units value = %v, want 0.2", r.Value)
	}
	if r.Band != "weak" {
		t.Errorf("e2e_only_units band = %q, want 'weak'", r.Band)
	}
	if r.Evidence != EvidenceStrong {
		t.Errorf("e2e_only_units evidence = %q, want 'strong'", r.Evidence)
	}
	if r.Explanation == "" {
		t.Error("e2e_only_units explanation should not be empty")
	}
}

func TestCoverageDiversity_E2EOnlyUnits_LowRatio(t *testing.T) {
	t.Parallel()
	snap := makeSnap(5)
	snap.CoverageSummary = &models.CoverageSummary{
		TotalCodeUnits:   100,
		CoveredOnlyByE2E: 3,
	}

	r := computeE2EOnlyUnits(snap)
	if r.Band != "strong" {
		t.Errorf("e2e_only_units band = %q, want 'strong' (3%% < 5%% threshold)", r.Band)
	}
}

func TestCoverageDiversity_UnitTestCoverage_HighCoverage(t *testing.T) {
	t.Parallel()
	snap := makeSnap(5)
	snap.CoverageSummary = &models.CoverageSummary{
		TotalCodeUnits:     100,
		CoveredByUnitTests: 80,
	}

	r := computeUnitTestCoverage(snap)
	if r.Value != 0.8 {
		t.Errorf("unit_test_coverage value = %v, want 0.8", r.Value)
	}
	// 80% >= 70% → strong (inverted band logic: higher is better)
	if r.Band != "strong" {
		t.Errorf("unit_test_coverage band = %q, want 'strong'", r.Band)
	}
	if r.Evidence != EvidenceStrong {
		t.Errorf("unit_test_coverage evidence = %q, want 'strong'", r.Evidence)
	}
}

func TestCoverageDiversity_UnitTestCoverage_ModerateCoverage(t *testing.T) {
	t.Parallel()
	snap := makeSnap(5)
	snap.CoverageSummary = &models.CoverageSummary{
		TotalCodeUnits:     100,
		CoveredByUnitTests: 60,
	}

	r := computeUnitTestCoverage(snap)
	// 60% is >= 50% but < 70% → moderate
	if r.Band != "moderate" {
		t.Errorf("unit_test_coverage band = %q, want 'moderate'", r.Band)
	}
}

func TestCoverageDiversity_UnitTestCoverage_LowCoverage(t *testing.T) {
	t.Parallel()
	snap := makeSnap(5)
	snap.CoverageSummary = &models.CoverageSummary{
		TotalCodeUnits:     100,
		CoveredByUnitTests: 30,
	}

	r := computeUnitTestCoverage(snap)
	// 30% < 50% → weak
	if r.Band != "weak" {
		t.Errorf("unit_test_coverage band = %q, want 'weak'", r.Band)
	}
}

// --- E2E concentration critical band ---

func TestCoverageDiversity_E2EConcentration_Critical(t *testing.T) {
	t.Parallel()
	// > 95% E2E → critical
	snap := &models.TestSuiteSnapshot{
		TestFiles: []models.TestFile{
			{Path: "a.spec.js", Framework: "playwright"},
			{Path: "b.spec.js", Framework: "playwright"},
			{Path: "c.spec.js", Framework: "playwright"},
			{Path: "d.spec.js", Framework: "playwright"},
			{Path: "e.spec.js", Framework: "playwright"},
			{Path: "f.spec.js", Framework: "playwright"},
			{Path: "g.spec.js", Framework: "playwright"},
			{Path: "h.spec.js", Framework: "playwright"},
			{Path: "i.spec.js", Framework: "playwright"},
			{Path: "j.spec.js", Framework: "playwright"},
			{Path: "k.spec.js", Framework: "playwright"},
			{Path: "l.spec.js", Framework: "playwright"},
			{Path: "m.spec.js", Framework: "playwright"},
			{Path: "n.spec.js", Framework: "playwright"},
			{Path: "o.spec.js", Framework: "playwright"},
			{Path: "p.spec.js", Framework: "playwright"},
			{Path: "q.spec.js", Framework: "playwright"},
			{Path: "r.spec.js", Framework: "playwright"},
			{Path: "s.spec.js", Framework: "playwright"},
			{Path: "t.test.js", Framework: "jest"},
		},
		Frameworks: []models.Framework{
			{Name: "playwright", Type: models.FrameworkTypeE2E, FileCount: 19},
			{Name: "jest", Type: models.FrameworkTypeUnit, FileCount: 1},
		},
	}

	r := computeE2EConcentration(snap)
	// 19/20 = 0.95 → not > 0.95, so "weak" (> 0.80)
	if r.Value != 0.95 {
		t.Errorf("e2e_concentration value = %v, want 0.95", r.Value)
	}
	if r.Band != "weak" {
		t.Errorf("e2e_concentration band at 95%% = %q, want 'weak'", r.Band)
	}

	// Add one more E2E file to push over 95%: 20/21 = 0.952
	snap.TestFiles = append(snap.TestFiles, models.TestFile{Path: "u.spec.js", Framework: "playwright"})
	snap.Frameworks[0].FileCount = 20
	r = computeE2EConcentration(snap)
	if r.Band != "critical" {
		t.Errorf("e2e_concentration band at 95.2%% = %q, want 'critical'", r.Band)
	}
}

// --- coverage breach with existing coverage data but zero breaches ---

func TestCoverageDepth_CoverageBreachShare_WithCoverageDataNoBreaches(t *testing.T) {
	t.Parallel()
	snap := makeSnap(5,
		// CoverageBlindSpot signal indicates coverage data exists
		sigInFile(signals.SignalCoverageBlindSpot, "src/module.js"),
	)
	r := computeCoverageBreachShare(snap)
	// Coverage data present but no breaches → strong with strong evidence
	if r.Band != "strong" {
		t.Errorf("coverage_breach_share band = %q, want 'strong'", r.Band)
	}
	if r.Evidence != EvidenceStrong {
		t.Errorf("coverage_breach_share evidence = %q, want 'strong' (coverage data available)", r.Evidence)
	}
	if len(r.Limitations) != 0 {
		t.Errorf("coverage_breach_share should have no limitations when coverage data present, got: %v", r.Limitations)
	}
}

// --- ToModel conversion ---

func TestToModel_PreservesAllFields(t *testing.T) {
	t.Parallel()
	snap := &Snapshot{
		Posture: []DimensionPosture{
			{
				Dimension:           DimensionHealth,
				Band:                PostureWeak,
				Explanation:         "test explanation",
				DrivingMeasurements: []string{"health.flaky_share"},
				Measurements: []Result{
					{
						ID: "health.flaky_share", Dimension: DimensionHealth,
						Value: 0.3, Units: UnitsRatio, Band: "weak",
						Evidence: EvidenceStrong, Explanation: "30% flaky",
						Inputs: []string{"flakyTest"}, Limitations: []string{"some limitation"},
					},
				},
			},
		},
		Measurements: []Result{
			{
				ID: "health.flaky_share", Dimension: DimensionHealth,
				Value: 0.3, Units: UnitsRatio, Band: "weak",
				Evidence: EvidenceStrong, Explanation: "30% flaky",
				Inputs: []string{"flakyTest"}, Limitations: []string{"some limitation"},
			},
		},
	}

	model := snap.ToModel()

	// Verify posture dimension preserved.
	if len(model.Posture) != 1 {
		t.Fatalf("posture count = %d, want 1", len(model.Posture))
	}
	p := model.Posture[0]
	if p.Dimension != "health" {
		t.Errorf("dimension = %q, want 'health'", p.Dimension)
	}
	if p.Band != "weak" {
		t.Errorf("band = %q, want 'weak'", p.Band)
	}
	if p.Explanation != "test explanation" {
		t.Errorf("explanation = %q, want 'test explanation'", p.Explanation)
	}
	if len(p.DrivingMeasurements) != 1 || p.DrivingMeasurements[0] != "health.flaky_share" {
		t.Errorf("driving measurements = %v, want [health.flaky_share]", p.DrivingMeasurements)
	}

	// Verify nested measurement preserved.
	if len(p.Measurements) != 1 {
		t.Fatalf("nested measurement count = %d, want 1", len(p.Measurements))
	}
	m := p.Measurements[0]
	if m.ID != "health.flaky_share" || m.Value != 0.3 || m.Units != "ratio" || m.Band != "weak" {
		t.Errorf("nested measurement fields wrong: %+v", m)
	}
	if m.Evidence != "strong" || m.Explanation != "30% flaky" {
		t.Errorf("nested measurement metadata wrong: evidence=%q explanation=%q", m.Evidence, m.Explanation)
	}
	if len(m.Inputs) != 1 || m.Inputs[0] != "flakyTest" {
		t.Errorf("inputs = %v, want [flakyTest]", m.Inputs)
	}
	if len(m.Limitations) != 1 || m.Limitations[0] != "some limitation" {
		t.Errorf("limitations = %v, want [some limitation]", m.Limitations)
	}

	// Verify flat measurement list preserved.
	if len(model.Measurements) != 1 {
		t.Fatalf("flat measurement count = %d, want 1", len(model.Measurements))
	}
	if model.Measurements[0].ID != "health.flaky_share" {
		t.Errorf("flat measurement ID = %q, want 'health.flaky_share'", model.Measurements[0].ID)
	}
}

// --- RunDimension ---

func TestRegistry_ByDimensionAndCompute(t *testing.T) {
	t.Parallel()
	r := NewRegistry()
	mustRegister(t, r, Definition{
		ID: "h.one", Dimension: DimensionHealth,
		Compute: func(_ *models.TestSuiteSnapshot) Result {
			return Result{ID: "h.one", Value: 0.1}
		},
	})
	mustRegister(t, r, Definition{
		ID: "s.one", Dimension: DimensionStructuralRisk,
		Compute: func(_ *models.TestSuiteSnapshot) Result {
			return Result{ID: "s.one", Value: 0.2}
		},
	})
	mustRegister(t, r, Definition{
		ID: "h.two", Dimension: DimensionHealth,
		Compute: func(_ *models.TestSuiteSnapshot) Result {
			return Result{ID: "h.two", Value: 0.3}
		},
	})

	// Run only health-dimension measurements via ByDimension + manual compute.
	defs := r.ByDimension(DimensionHealth)
	if len(defs) != 2 {
		t.Fatalf("ByDimension(health) = %d defs, want 2", len(defs))
	}
	snap := makeSnap(1)
	results := make([]Result, len(defs))
	for i, d := range defs {
		results[i] = d.Compute(snap)
	}
	if results[0].ID != "h.one" || results[1].ID != "h.two" {
		t.Errorf("ByDimension(health) IDs = [%q, %q], want [h.one, h.two]", results[0].ID, results[1].ID)
	}

	// Dimension with no measurements returns nil.
	empty := r.ByDimension(DimensionOperationalRisk)
	if len(empty) != 0 {
		t.Errorf("ByDimension(operational_risk) = %d defs, want 0", len(empty))
	}
}

// --- ComputeSnapshot empty dimension ---

func TestComputeSnapshot_EmptyDimension(t *testing.T) {
	t.Parallel()
	// Register measurements for only one dimension.
	r := NewRegistry()
	mustRegister(t, r, Definition{
		ID: "h.one", Dimension: DimensionHealth,
		Compute: func(_ *models.TestSuiteSnapshot) Result {
			return Result{ID: "h.one", Dimension: DimensionHealth, Band: "strong", Evidence: EvidenceStrong}
		},
	})

	snap := r.ComputeSnapshot(makeSnap(1))
	if len(snap.Posture) != 5 {
		t.Fatalf("expected 5 dimensions, got %d", len(snap.Posture))
	}

	// Health should have a real posture.
	if snap.Posture[0].Band != PostureStrong {
		t.Errorf("health band = %q, want 'strong'", snap.Posture[0].Band)
	}

	// All other dimensions should be unknown (no measurements registered).
	for _, p := range snap.Posture[1:] {
		if p.Band != PostureUnknown {
			t.Errorf("dimension %q band = %q, want 'unknown' (no measurements)", p.Dimension, p.Band)
		}
		if p.Explanation == "" {
			t.Errorf("dimension %q should have explanation for unknown", p.Dimension)
		}
	}
}
