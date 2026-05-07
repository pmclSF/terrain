package reporting

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/pmclSF/terrain/internal/comparison"
	"github.com/pmclSF/terrain/internal/heatmap"
	"github.com/pmclSF/terrain/internal/impact"
	"github.com/pmclSF/terrain/internal/measurement"
	"github.com/pmclSF/terrain/internal/metrics"
	"github.com/pmclSF/terrain/internal/migration"
	"github.com/pmclSF/terrain/internal/models"
	"github.com/pmclSF/terrain/internal/scoring"
	"github.com/pmclSF/terrain/internal/testdata"
)

func TestRenderSummaryReport_HealthyBalanced(t *testing.T) {
	t.Parallel()
	snap := testdata.HealthyBalancedSnapshot()
	snap.Risk = scoring.ComputeRisk(snap)
	measReg, mErr := measurement.DefaultRegistry()
	if mErr != nil {
		t.Fatal(mErr)
	}
	snap.Measurements = measReg.ComputeSnapshot(snap).ToModel()

	h := heatmap.Build(snap)
	var buf bytes.Buffer
	RenderSummaryReport(&buf, snap, h)
	output := buf.String()

	checks := []string{"Terrain Summary", "Key Numbers", "Test files:", "Frameworks:", "Next steps:"}
	for _, c := range checks {
		if !strings.Contains(output, c) {
			t.Errorf("summary report missing %q", c)
		}
	}
}

func TestRenderMetricsReport_Minimal(t *testing.T) {
	t.Parallel()
	snap := testdata.MinimalSnapshot()
	ms := metrics.Derive(snap)

	var buf bytes.Buffer
	RenderMetricsReport(&buf, ms)
	output := buf.String()

	if !strings.Contains(output, "Terrain Metrics") {
		t.Error("metrics report missing header")
	}
	if !strings.Contains(output, "Test files:") {
		t.Error("metrics report missing test file count")
	}
}

func TestRenderPostureReport_Healthy(t *testing.T) {
	t.Parallel()
	snap := testdata.HealthyBalancedSnapshot()
	measReg, mErr := measurement.DefaultRegistry()
	if mErr != nil {
		t.Fatal(mErr)
	}
	snap.Measurements = measReg.ComputeSnapshot(snap).ToModel()

	var buf bytes.Buffer
	RenderPostureReport(&buf, snap)
	output := buf.String()

	if !strings.Contains(output, "Terrain Posture") {
		t.Error("posture report missing header")
	}
	if !strings.Contains(output, "Next steps:") {
		t.Error("posture report missing next steps")
	}
	// Should contain overall posture line.
	if !strings.Contains(output, "Overall:") {
		t.Error("posture report missing overall posture")
	}
	// Should use human-readable dimension names.
	if strings.Contains(output, "COVERAGE_DEPTH") {
		t.Error("posture report still using raw identifier COVERAGE_DEPTH instead of COVERAGE DEPTH")
	}
}

func TestRenderPostureReport_WeakScenario(t *testing.T) {
	t.Parallel()
	snap := &models.TestSuiteSnapshot{
		Measurements: &models.MeasurementSnapshot{
			Posture: []models.DimensionPostureResult{
				{
					Dimension:           "health",
					Band:                "weak",
					Explanation:         "health posture is weak. Driven by: health.flaky_share.",
					DrivingMeasurements: []string{"health.flaky_share"},
					Measurements: []models.MeasurementResult{
						{ID: "health.flaky_share", Value: 0.35, Units: "ratio", Band: "critical", Evidence: "strong", Explanation: "35% flaky"},
					},
				},
			},
		},
	}

	var buf bytes.Buffer
	RenderPostureReport(&buf, snap)
	output := buf.String()

	// Should have visual marker for weak.
	if !strings.Contains(output, "[~]") {
		t.Error("posture report missing [~] marker for weak band")
	}
	if !strings.Contains(output, "WEAK") {
		t.Error("posture report missing WEAK band display")
	}
	if !strings.Contains(output, "Driving measurements:") {
		t.Error("posture report missing driving measurements section")
	}
}

func TestRenderPostureReport_CriticalScenario(t *testing.T) {
	t.Parallel()
	snap := &models.TestSuiteSnapshot{
		Measurements: &models.MeasurementSnapshot{
			Posture: []models.DimensionPostureResult{
				{
					Dimension:   "operational_risk",
					Band:        "critical",
					Explanation: "operational_risk posture is critical. Immediate attention needed.",
				},
			},
		},
	}

	var buf bytes.Buffer
	RenderPostureReport(&buf, snap)
	output := buf.String()

	if !strings.Contains(output, "[!!]") {
		t.Error("posture report missing [!!] marker for critical band")
	}
	if !strings.Contains(output, "CRITICAL") {
		t.Error("posture report missing CRITICAL band display")
	}
}

func TestRenderPostureReport_ElevatedScenario(t *testing.T) {
	t.Parallel()
	snap := &models.TestSuiteSnapshot{
		Measurements: &models.MeasurementSnapshot{
			Posture: []models.DimensionPostureResult{
				{
					Dimension:   "structural_risk",
					Band:        "elevated",
					Explanation: "Structural Risk posture is elevated.",
				},
			},
		},
	}

	var buf bytes.Buffer
	RenderPostureReport(&buf, snap)
	output := buf.String()

	if !strings.Contains(output, "[!]") {
		t.Error("posture report missing [!] marker for elevated band")
	}
	if !strings.Contains(output, "ELEVATED") {
		t.Error("posture report missing ELEVATED band display")
	}
}

func TestRenderPostureReport_UnknownScenario(t *testing.T) {
	t.Parallel()
	snap := &models.TestSuiteSnapshot{
		Measurements: &models.MeasurementSnapshot{
			Posture: []models.DimensionPostureResult{
				{Dimension: "health", Band: "unknown", Explanation: "No data."},
			},
		},
	}

	var buf bytes.Buffer
	RenderPostureReport(&buf, snap)
	output := buf.String()

	if !strings.Contains(output, "[?]") {
		t.Error("posture report missing [?] marker for unknown band")
	}
}

func TestRenderPostureReport_VerboseShowsInputs(t *testing.T) {
	t.Parallel()
	snap := &models.TestSuiteSnapshot{
		Measurements: &models.MeasurementSnapshot{
			Posture: []models.DimensionPostureResult{
				{
					Dimension:   "health",
					Band:        "moderate",
					Explanation: "Some room for improvement.",
					Measurements: []models.MeasurementResult{
						{
							ID: "health.flaky_share", Value: 0.1, Units: "ratio",
							Band: "moderate", Evidence: "strong",
							Explanation: "1 of 10 test file(s) flagged as flaky.",
							Inputs:      []string{"flakyTest", "unstableSuite"},
						},
					},
				},
			},
		},
	}

	// Non-verbose should NOT show inputs.
	var buf bytes.Buffer
	RenderPostureReport(&buf, snap)
	if strings.Contains(buf.String(), "Inputs:") {
		t.Error("non-verbose posture report should not show Inputs")
	}

	// Verbose SHOULD show inputs.
	buf.Reset()
	RenderPostureReport(&buf, snap, ReportOptions{Verbose: true})
	output := buf.String()
	if !strings.Contains(output, "Inputs: flakyTest, unstableSuite") {
		t.Error("verbose posture report should show Inputs")
	}
	if !strings.Contains(output, "Question:") {
		t.Error("verbose posture report should show dimension question")
	}
}

func TestRenderPostureReport_VerboseHintHidden(t *testing.T) {
	t.Parallel()
	snap := &models.TestSuiteSnapshot{
		Measurements: &models.MeasurementSnapshot{
			Posture: []models.DimensionPostureResult{
				{Dimension: "health", Band: "strong", Explanation: "All good."},
			},
		},
	}

	// Non-verbose should suggest --verbose.
	var buf bytes.Buffer
	RenderPostureReport(&buf, snap)
	if !strings.Contains(buf.String(), "--verbose") {
		t.Error("non-verbose report should suggest --verbose in next steps")
	}

	// Verbose should NOT suggest --verbose.
	buf.Reset()
	RenderPostureReport(&buf, snap, ReportOptions{Verbose: true})
	if strings.Contains(buf.String(), "--verbose") {
		t.Error("verbose report should not suggest --verbose")
	}
}

func TestRenderPostureReport_NoData(t *testing.T) {
	t.Parallel()
	snap := &models.TestSuiteSnapshot{}

	var buf bytes.Buffer
	RenderPostureReport(&buf, snap)
	output := buf.String()

	if !strings.Contains(output, "No measurement data") {
		t.Error("posture report should show no-data message")
	}
}

func TestComputeOverallPosture(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		dimensions []models.DimensionPostureResult
		wantBand   string
	}{
		{
			"all strong",
			[]models.DimensionPostureResult{
				{Dimension: "health", Band: "strong"},
				{Dimension: "coverage_depth", Band: "strong"},
			},
			"strong",
		},
		{
			"worst wins",
			[]models.DimensionPostureResult{
				{Dimension: "health", Band: "strong"},
				{Dimension: "coverage_depth", Band: "critical"},
			},
			"critical",
		},
		{
			"all unknown",
			[]models.DimensionPostureResult{
				{Dimension: "health", Band: "unknown"},
			},
			"unknown",
		},
		{
			"unknown ignored",
			[]models.DimensionPostureResult{
				{Dimension: "health", Band: "unknown"},
				{Dimension: "coverage_depth", Band: "moderate"},
			},
			"moderate",
		},
		{
			"elevated",
			[]models.DimensionPostureResult{
				{Dimension: "health", Band: "strong"},
				{Dimension: "structural_risk", Band: "elevated"},
			},
			"elevated",
		},
		{
			"weak names dimension",
			[]models.DimensionPostureResult{
				{Dimension: "health", Band: "strong"},
				{Dimension: "coverage_diversity", Band: "weak"},
			},
			"weak",
		},
		{
			"empty",
			nil,
			"unknown",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := computeOverallPosture(tt.dimensions)
			if string(result.band) != tt.wantBand {
				t.Errorf("computeOverallPosture = %q, want %q", result.band, tt.wantBand)
			}
		})
	}
}

func TestComputeOverallPosture_ExplanationContent(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		dimensions  []models.DimensionPostureResult
		wantContain string
	}{
		{
			"strong mentions all dimensions",
			[]models.DimensionPostureResult{
				{Dimension: "health", Band: "strong"},
				{Dimension: "coverage_depth", Band: "strong"},
			},
			"All 2 dimension(s) are strong",
		},
		{
			"moderate names worst dimension",
			[]models.DimensionPostureResult{
				{Dimension: "health", Band: "strong"},
				{Dimension: "coverage_depth", Band: "moderate"},
			},
			"Coverage depth",
		},
		{
			"weak names driving dimension",
			[]models.DimensionPostureResult{
				{Dimension: "health", Band: "strong"},
				{Dimension: "structural_risk", Band: "weak"},
			},
			"Structural risk",
		},
		{
			"critical names driving dimension",
			[]models.DimensionPostureResult{
				{Dimension: "operational_risk", Band: "critical"},
			},
			"Operational risk",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := computeOverallPosture(tt.dimensions)
			if !strings.Contains(result.explanation, tt.wantContain) {
				t.Errorf("explanation %q should contain %q", result.explanation, tt.wantContain)
			}
		})
	}
}

func TestRenderComparisonReport(t *testing.T) {
	t.Parallel()
	from := testdata.MinimalSnapshot()
	to := testdata.HealthyBalancedSnapshot()
	comp := comparison.Compare(from, to)

	var buf bytes.Buffer
	RenderComparisonReport(&buf, comp)
	output := buf.String()

	if !strings.Contains(output, "Terrain Snapshot Comparison") {
		t.Error("comparison report missing header")
	}
	if !strings.Contains(output, "Methodology Compatibility") {
		t.Error("comparison report missing methodology section")
	}
	if !strings.Contains(output, "Status: COMPATIBLE") {
		t.Error("comparison report missing compatible status")
	}
}

func TestRenderComparisonReport_MethodologyIncompatibilityVisibleWithoutDeltas(t *testing.T) {
	t.Parallel()
	from := &models.TestSuiteSnapshot{
		GeneratedAt: time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
		SnapshotMeta: models.SnapshotMeta{
			SchemaVersion:          models.SnapshotSchemaVersion,
			MethodologyFingerprint: "aaa",
		},
	}
	to := &models.TestSuiteSnapshot{
		GeneratedAt: time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC),
		SnapshotMeta: models.SnapshotMeta{
			SchemaVersion:          models.SnapshotSchemaVersion,
			MethodologyFingerprint: "bbb",
		},
	}
	comp := comparison.Compare(from, to)

	var buf bytes.Buffer
	RenderComparisonReport(&buf, comp)
	output := buf.String()

	if !strings.Contains(output, "Methodology Compatibility") {
		t.Error("comparison report missing methodology section")
	}
	if !strings.Contains(output, "Status: INCOMPATIBLE") {
		t.Error("comparison report missing incompatible status")
	}
	if !strings.Contains(output, "Recommended Next Steps") {
		t.Error("comparison report missing recommended next steps for incompatible methodology")
	}
	if !strings.Contains(output, "No meaningful changes detected.") {
		t.Error("comparison report should still show no-delta summary")
	}
}

func TestRenderImpactReport_WithGaps(t *testing.T) {
	t.Parallel()
	result := &impact.ImpactResult{
		Scope: impact.ChangeScope{
			ChangedFiles: []impact.ChangedFile{
				{Path: "src/api.js", ChangeKind: impact.ChangeModified},
			},
		},
		ImpactedUnits: []impact.ImpactedCodeUnit{
			{UnitID: "api:ApiClient", Name: "ApiClient", Path: "src/api.js", Exported: true, ChangeKind: impact.ChangeModified, ProtectionStatus: impact.ProtectionNone, Owner: "team-api"},
		},
		ProtectionGaps: []impact.ProtectionGap{
			{GapType: "untested_export", CodeUnitID: "api:ApiClient", Path: "src/api.js", Explanation: "Exported function ApiClient has no coverage.", Severity: "high", SuggestedAction: "Add unit tests."},
		},
		SelectedTests: []impact.ImpactedTest{
			{Path: "test/api.test.js", ImpactConfidence: impact.ConfidenceInferred, Relevance: "in same directory"},
		},
		ImpactedOwners: []string{"team-api"},
		Posture:        impact.ChangeRiskPosture{Band: "high_risk", Explanation: "Significant risk."},
		Summary:        "1 file(s) changed, 1 code unit(s) impacted.",
		Limitations:    []string{"No coverage lineage available."},
	}

	var buf bytes.Buffer
	RenderImpactReport(&buf, result)
	output := buf.String()

	checks := []string{
		"Terrain Impact Analysis",
		"Change-Risk Posture: HIGH_RISK",
		"Coverage confidence:",
		"PR risk:",
		"Protection Gaps",
		"Recommended Tests",
		"Impacted Owners: team-api",
		"Limitations",
		"Next steps:",
	}
	for _, c := range checks {
		if !strings.Contains(output, c) {
			t.Errorf("impact report missing %q", c)
		}
	}
}

func TestRenderImpactDrilldown_Units(t *testing.T) {
	t.Parallel()
	result := &impact.ImpactResult{
		ImpactedUnits: []impact.ImpactedCodeUnit{
			{Name: "Foo", Path: "src/foo.js", ChangeKind: impact.ChangeModified, Exported: true, ProtectionStatus: impact.ProtectionStrong, ImpactConfidence: impact.ConfidenceExact, CoveringTests: []string{"test/foo.test.js"}},
		},
	}

	var buf bytes.Buffer
	RenderImpactUnits(&buf, result)
	output := buf.String()

	if !strings.Contains(output, "Impacted Code Units (1)") {
		t.Error("units view missing header")
	}
	if !strings.Contains(output, "[exported]") {
		t.Error("units view missing exported tag")
	}
}

func TestRenderImpactDrilldown_Gaps(t *testing.T) {
	t.Parallel()
	result := &impact.ImpactResult{
		ProtectionGaps: []impact.ProtectionGap{
			{GapType: "no_coverage", Severity: "medium", Explanation: "No tests.", Path: "src/bar.js", SuggestedAction: "Add tests."},
			{GapType: "untested_export", Severity: "high", Explanation: "Exported.", Path: "src/foo.js"},
		},
	}

	var buf bytes.Buffer
	RenderImpactGaps(&buf, result)
	output := buf.String()

	if !strings.Contains(output, "Protection Gaps (2)") {
		t.Error("gaps view missing header")
	}
	if !strings.Contains(output, "HIGH severity (1)") {
		t.Error("gaps view missing high severity section")
	}
	if !strings.Contains(output, "MEDIUM severity (1)") {
		t.Error("gaps view missing medium severity section")
	}
}

func TestRenderImpactDrilldown_Tests(t *testing.T) {
	t.Parallel()
	result := &impact.ImpactResult{
		ImpactedTests: []impact.ImpactedTest{
			{Path: "test/a.test.js", ImpactConfidence: impact.ConfidenceExact, Relevance: "covers unit", CoversUnits: []string{"Foo"}},
			{Path: "test/b.test.js", ImpactConfidence: impact.ConfidenceInferred, Relevance: "in same dir"},
		},
		SelectedTests: []impact.ImpactedTest{
			{Path: "test/a.test.js", ImpactConfidence: impact.ConfidenceExact, Relevance: "covers unit"},
		},
	}

	var buf bytes.Buffer
	RenderImpactTests(&buf, result)
	output := buf.String()

	if !strings.Contains(output, "Impacted Tests (2 total, 1 selected)") {
		t.Error("tests view missing header")
	}
	if !strings.Contains(output, "Recommended (run these first)") {
		t.Error("tests view missing recommended section")
	}
	if !strings.Contains(output, "Additional relevant tests") {
		t.Error("tests view missing additional section")
	}
}

func TestRenderImpactDrilldown_Owners(t *testing.T) {
	t.Parallel()
	result := &impact.ImpactResult{
		ImpactedUnits: []impact.ImpactedCodeUnit{
			{Name: "Foo", Owner: "team-a", ProtectionStatus: impact.ProtectionStrong, ChangeKind: impact.ChangeModified},
			{Name: "Bar", Owner: "team-a", ProtectionStatus: impact.ProtectionNone, ChangeKind: impact.ChangeModified},
			{Name: "Baz", Owner: "team-b", ProtectionStatus: impact.ProtectionWeak, ChangeKind: impact.ChangeAdded},
		},
		ImpactedOwners: []string{"team-a", "team-b"},
	}

	var buf bytes.Buffer
	RenderImpactOwners(&buf, result)
	output := buf.String()

	if !strings.Contains(output, "Impacted Owners (2)") {
		t.Error("owners view missing header")
	}
	if !strings.Contains(output, "team-a (2 units)") {
		t.Error("owners view missing team-a")
	}
	if !strings.Contains(output, "team-b (1 unit)") {
		t.Error("owners view missing team-b")
	}
}

func TestRenderImpactDrilldown_EmptyResults(t *testing.T) {
	t.Parallel()
	result := &impact.ImpactResult{}

	var buf bytes.Buffer

	RenderImpactUnits(&buf, result)
	if !strings.Contains(buf.String(), "No impacted code units") {
		t.Error("empty units view should show message")
	}

	buf.Reset()
	RenderImpactGaps(&buf, result)
	if !strings.Contains(buf.String(), "No protection gaps") {
		t.Error("empty gaps view should show message")
	}

	buf.Reset()
	RenderImpactTests(&buf, result)
	if !strings.Contains(buf.String(), "No impacted tests") {
		t.Error("empty tests view should show message")
	}

	buf.Reset()
	RenderImpactOwners(&buf, result)
	if !strings.Contains(buf.String(), "No ownership data") {
		t.Error("empty owners view should show message")
	}
}

func TestRenderMigrationReport(t *testing.T) {
	t.Parallel()
	snap := testdata.MigrationRiskSnapshot()
	readiness := migration.ComputeReadiness(snap)

	var buf bytes.Buffer
	RenderMigrationReport(&buf, readiness)
	output := buf.String()

	if !strings.Contains(output, "Migration") {
		t.Error("migration report missing header")
	}
}
