package changescope

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/pmclSF/terrain/internal/impact"
	"github.com/pmclSF/terrain/internal/models"
)

func testSnapshot() *models.TestSuiteSnapshot {
	return &models.TestSuiteSnapshot{
		Repository: models.RepositoryMetadata{Name: "test-repo"},
		Frameworks: []models.Framework{
			{Name: "jest", Type: models.FrameworkTypeUnit, FileCount: 4, TestCount: 20},
		},
		TestFiles: []models.TestFile{
			{Path: "src/__tests__/auth.test.js", Framework: "jest", TestCount: 5, AssertionCount: 8, LinkedCodeUnits: []string{"AuthService"}},
			{Path: "src/__tests__/user.test.js", Framework: "jest", TestCount: 4, AssertionCount: 6, LinkedCodeUnits: []string{"UserService"}},
			{Path: "src/__tests__/payment.test.js", Framework: "jest", TestCount: 5, AssertionCount: 7, LinkedCodeUnits: []string{"PaymentProcessor"}},
			{Path: "src/__tests__/config.test.js", Framework: "jest", TestCount: 3, AssertionCount: 4},
		},
		CodeUnits: []models.CodeUnit{
			{Name: "AuthService", Path: "src/auth.js", Kind: models.CodeUnitKindClass, Exported: true},
			{Name: "UserService", Path: "src/user.js", Kind: models.CodeUnitKindClass, Exported: true},
			{Name: "PaymentProcessor", Path: "src/payment.js", Kind: models.CodeUnitKindClass, Exported: true},
			{Name: "helperFn", Path: "src/utils.js", Kind: models.CodeUnitKindFunction, Exported: false},
		},
		Ownership: map[string][]string{
			"src/auth.js":    {"team-platform"},
			"src/payment.js": {"team-payments"},
		},
		DataSources: []models.DataSource{
			{Name: "coverage", Status: models.DataSourceAvailable},
		},
	}
}

func TestAnalyzePR_NarrowChange(t *testing.T) {
	t.Parallel()
	snap := testSnapshot()
	scope := impact.ChangeScopeFromPaths([]string{"src/auth.js"}, impact.ChangeModified)

	pr := AnalyzePR(scope, snap)

	if pr.ChangedFileCount != 1 {
		t.Errorf("changed file count = %d, want 1", pr.ChangedFileCount)
	}
	if pr.ChangedSourceCount != 1 {
		t.Errorf("changed source count = %d, want 1", pr.ChangedSourceCount)
	}
	if pr.ChangedTestCount != 0 {
		t.Errorf("changed test count = %d, want 0", pr.ChangedTestCount)
	}
	if pr.PostureBand == "" {
		t.Error("expected non-empty posture band")
	}
	if pr.Summary == "" {
		t.Error("expected non-empty summary")
	}
	if pr.ImpactedUnitCount == 0 {
		t.Error("expected impacted units for auth.js change")
	}
}

func TestAnalyzePR_BroadChange(t *testing.T) {
	t.Parallel()
	snap := testSnapshot()
	scope := impact.ChangeScopeFromPaths(
		[]string{"src/auth.js", "src/payment.js", "src/utils.js"},
		impact.ChangeModified,
	)

	pr := AnalyzePR(scope, snap)

	if pr.ChangedSourceCount != 3 {
		t.Errorf("changed source count = %d, want 3", pr.ChangedSourceCount)
	}
	if len(pr.AffectedOwners) == 0 {
		t.Error("expected affected owners")
	}
}

func TestAnalyzePR_WithTestChanges(t *testing.T) {
	t.Parallel()
	snap := testSnapshot()
	scope := impact.ChangeScopeFromPaths(
		[]string{"src/auth.js", "src/__tests__/auth.test.js"},
		impact.ChangeModified,
	)

	pr := AnalyzePR(scope, snap)

	if pr.ChangedTestCount != 1 {
		t.Errorf("changed test count = %d, want 1", pr.ChangedTestCount)
	}
	if pr.ChangedSourceCount != 1 {
		t.Errorf("changed source count = %d, want 1", pr.ChangedSourceCount)
	}
}

func TestAnalyzePR_UntestedExport(t *testing.T) {
	t.Parallel()
	snap := testSnapshot()
	// Add a new exported function with no test coverage.
	snap.CodeUnits = append(snap.CodeUnits, models.CodeUnit{
		Name: "NewFeature", Path: "src/feature.js",
		Kind: models.CodeUnitKindFunction, Exported: true,
	})
	scope := impact.ChangeScopeFromPaths([]string{"src/feature.js"}, impact.ChangeAdded)

	pr := AnalyzePR(scope, snap)

	// Untested exports are surfaced as protection_gap findings (not duplicated).
	hasProtectionGap := false
	for _, f := range pr.NewFindings {
		if f.Type == "protection_gap" && f.Severity == "high" && strings.Contains(f.Path, "feature.js") {
			hasProtectionGap = true
		}
	}
	if !hasProtectionGap {
		t.Error("expected high-severity protection_gap finding for untested export")
	}
	// Verify no duplicate — untested_export_in_change should not exist.
	for _, f := range pr.NewFindings {
		if f.Type == "untested_export_in_change" {
			t.Error("unexpected duplicate untested_export_in_change finding — should only appear as protection_gap")
		}
	}
}

func TestAnalyzeChangedPaths(t *testing.T) {
	t.Parallel()
	snap := testSnapshot()
	pr := AnalyzeChangedPaths([]string{"src/auth.js"}, impact.ChangeModified, snap)

	if pr == nil {
		t.Fatal("expected non-nil PRAnalysis")
	}
	if pr.ChangedFileCount != 1 {
		t.Errorf("changed file count = %d, want 1", pr.ChangedFileCount)
	}
}

func TestRenderPRSummaryMarkdown(t *testing.T) {
	t.Parallel()
	snap := testSnapshot()
	scope := impact.ChangeScopeFromPaths([]string{"src/auth.js"}, impact.ChangeModified)
	pr := AnalyzePR(scope, snap)

	var buf bytes.Buffer
	RenderPRSummaryMarkdown(&buf, pr)
	output := buf.String()

	if !strings.Contains(output, "Terrain") {
		t.Error("expected Terrain in markdown header")
	}
	if !strings.Contains(output, "Changed files") {
		t.Error("expected stats table")
	}
	if !strings.Contains(output, "merge") && !strings.Contains(output, "Merge") {
		t.Error("expected merge recommendation")
	}
}

func TestRenderPRCommentConcise(t *testing.T) {
	t.Parallel()
	snap := testSnapshot()
	scope := impact.ChangeScopeFromPaths([]string{"src/auth.js"}, impact.ChangeModified)
	pr := AnalyzePR(scope, snap)

	var buf bytes.Buffer
	RenderPRCommentConcise(&buf, pr)
	output := buf.String()

	if !strings.Contains(output, "Terrain") {
		t.Error("expected Terrain in concise comment")
	}
}

func TestRenderCIAnnotation(t *testing.T) {
	t.Parallel()
	snap := testSnapshot()
	// Add a new untested export to generate findings.
	snap.CodeUnits = append(snap.CodeUnits, models.CodeUnit{
		Name: "UntestedFn", Path: "src/untested.js",
		Kind: models.CodeUnitKindFunction, Exported: true,
	})
	scope := impact.ChangeScopeFromPaths([]string{"src/untested.js"}, impact.ChangeAdded)
	pr := AnalyzePR(scope, snap)

	var buf bytes.Buffer
	RenderCIAnnotation(&buf, pr)
	output := buf.String()

	if len(pr.NewFindings) > 0 && !strings.Contains(output, "::") {
		t.Error("expected CI annotation format")
	}
}

func TestRenderChangeScopedReport(t *testing.T) {
	t.Parallel()
	snap := testSnapshot()
	scope := impact.ChangeScopeFromPaths([]string{"src/auth.js", "src/payment.js"}, impact.ChangeModified)
	pr := AnalyzePR(scope, snap)

	var buf bytes.Buffer
	RenderChangeScopedReport(&buf, pr)
	output := buf.String()

	if !strings.Contains(output, "Change-Scoped Analysis") {
		t.Error("expected report header")
	}
	if !strings.Contains(output, "Posture:") {
		t.Error("expected posture line")
	}
}

func TestPostureBadge(t *testing.T) {
	t.Parallel()
	tests := []struct {
		band string
		want string
	}{
		{"well_protected", "[PASS]"},
		{"partially_protected", "[WARN]"},
		{"weakly_protected", "[RISK]"},
		{"high_risk", "[FAIL]"},
		{"evidence_limited", "[INFO]"},
		{"unknown", "[????]"},
	}
	for _, tt := range tests {
		got := postureBadge(tt.band)
		if got != tt.want {
			t.Errorf("postureBadge(%q) = %q, want %q", tt.band, got, tt.want)
		}
	}
}

func TestExtractUnitNames(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		unitIDs []string
		want    []string
	}{
		{"path:Name format", []string{"src/auth.js:AuthService", "src/db.js:DBPool"}, []string{"AuthService", "DBPool"}},
		{"deduplicates names", []string{"a.js:Foo", "b.js:Foo"}, []string{"Foo"}},
		{"no colon fallback", []string{"bare_name"}, []string{"bare_name"}},
		{"empty input", nil, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractUnitNames(tt.unitIDs)
			if len(got) != len(tt.want) {
				t.Fatalf("extractUnitNames() = %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("extractUnitNames()[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestFormatTestReasons_ShowsUnitNames(t *testing.T) {
	t.Parallel()
	selections := []TestSelection{
		{
			Path:       "test/auth.test.js",
			Confidence: "exact",
			CoversUnits: []string{
				"src/auth.js:AuthService",
				"src/auth.js:SessionManager",
			},
			Reasons: []string{"exact coverage of impacted unit", "exact coverage of impacted unit"},
		},
	}

	reasons := formatTestReasons(selections)
	why := reasons["test/auth.test.js"]

	if !strings.Contains(why, "AuthService") {
		t.Errorf("expected unit name AuthService in reason, got: %s", why)
	}
	if !strings.Contains(why, "SessionManager") {
		t.Errorf("expected unit name SessionManager in reason, got: %s", why)
	}
	if strings.Contains(why, "exact coverage of impacted unit") {
		t.Errorf("expected unit names instead of generic reason, got: %s", why)
	}
}

func TestFormatTestReasons_TruncatesLongLists(t *testing.T) {
	t.Parallel()
	units := make([]string, 6)
	for i := range units {
		units[i] = fmt.Sprintf("src/mod%d.js:Unit%d", i, i)
	}
	selections := []TestSelection{
		{
			Path:        "test/all.test.js",
			Confidence:  "exact",
			CoversUnits: units,
		},
	}

	reasons := formatTestReasons(selections)
	why := reasons["test/all.test.js"]

	if !strings.Contains(why, "more") {
		t.Errorf("expected truncation with 'more', got: %s", why)
	}
}

func TestFormatTestReasons_FallsBackToReasons(t *testing.T) {
	t.Parallel()
	selections := []TestSelection{
		{
			Path:       "test/nearby.test.js",
			Confidence: "inferred",
			Reasons:    []string{"inferred structural relationship"},
		},
	}

	reasons := formatTestReasons(selections)
	why := reasons["test/nearby.test.js"]

	if why != "inferred structural relationship" {
		t.Errorf("expected fallback to reason string, got: %s", why)
	}
}

func TestFormatTestReasons_UniqueVsShared(t *testing.T) {
	t.Parallel()
	selections := []TestSelection{
		{
			Path:       "test/auth.test.js",
			Confidence: "exact",
			CoversUnits: []string{
				"src/auth.js:AuthService",  // unique to this test
				"src/shared.js:SharedUtil", // shared with test B
			},
		},
		{
			Path:       "test/user.test.js",
			Confidence: "exact",
			CoversUnits: []string{
				"src/user.js:UserService",  // unique to this test
				"src/shared.js:SharedUtil", // shared with test A
			},
		},
	}

	reasons := formatTestReasons(selections)

	authWhy := reasons["test/auth.test.js"]
	userWhy := reasons["test/user.test.js"]

	// Each test should lead with its unique unit.
	if !strings.Contains(authWhy, "AuthService") {
		t.Errorf("auth test should mention unique unit AuthService, got: %s", authWhy)
	}
	if !strings.Contains(userWhy, "UserService") {
		t.Errorf("user test should mention unique unit UserService, got: %s", userWhy)
	}
	// Output should be clean and focused on the unique units.
	if authWhy == "" {
		t.Error("auth test should have a reason")
	}
}

func TestFormatTestReasons_AllShared(t *testing.T) {
	t.Parallel()
	selections := []TestSelection{
		{
			Path:        "test/a.test.js",
			Confidence:  "exact",
			CoversUnits: []string{"src/x.js:Foo", "src/y.js:Bar"},
		},
		{
			Path:        "test/b.test.js",
			Confidence:  "exact",
			CoversUnits: []string{"src/x.js:Foo", "src/y.js:Bar"},
		},
	}

	reasons := formatTestReasons(selections)
	why := reasons["test/a.test.js"]

	// When all units are shared, should still list unit names.
	if !strings.Contains(why, "Foo") {
		t.Errorf("expected unit name Foo, got: %s", why)
	}
}

func TestAnalyzePR_SchemaVersion(t *testing.T) {
	t.Parallel()
	snap := &models.TestSuiteSnapshot{}
	scope := &impact.ChangeScope{}
	pr := AnalyzePR(scope, snap)
	if pr.SchemaVersion != PRAnalysisSchemaVersion {
		t.Errorf("schemaVersion = %q, want %q", pr.SchemaVersion, PRAnalysisSchemaVersion)
	}
}

func TestAnalyzePRFromChangeSet_Basic(t *testing.T) {
	t.Parallel()
	snap := testSnapshot()
	cs := &models.ChangeSet{
		Source: "explicit",
		ChangedFiles: []models.ChangedFile{
			{Path: "src/auth/login.js", ChangeKind: models.ChangeModified},
		},
	}

	pr := AnalyzePRFromChangeSet(cs, snap)
	if pr.SchemaVersion != PRAnalysisSchemaVersion {
		t.Errorf("schemaVersion = %q, want %q", pr.SchemaVersion, PRAnalysisSchemaVersion)
	}
	if pr.ChangedFileCount != 1 {
		t.Errorf("changedFileCount = %d, want 1", pr.ChangedFileCount)
	}
	if pr.ChangedSourceCount != 1 {
		t.Errorf("changedSourceCount = %d, want 1", pr.ChangedSourceCount)
	}
	if pr.ChangedTestCount != 0 {
		t.Errorf("changedTestCount = %d, want 0", pr.ChangedTestCount)
	}
	if pr.Summary == "" {
		t.Error("expected non-empty summary")
	}
	if pr.PostureBand == "" {
		t.Error("expected non-empty posture band")
	}
	if pr.PostureDelta == nil {
		t.Error("expected non-nil posture delta")
	}
}

func TestAnalyzePRFromChangeSet_WithTestChanges(t *testing.T) {
	t.Parallel()
	snap := testSnapshot()
	cs := &models.ChangeSet{
		Source: "explicit",
		ChangedFiles: []models.ChangedFile{
			{Path: "src/__tests__/auth.test.js", ChangeKind: models.ChangeModified, IsTestFile: true},
			{Path: "src/auth/login.js", ChangeKind: models.ChangeModified},
		},
	}

	pr := AnalyzePRFromChangeSet(cs, snap)
	if pr.ChangedTestCount != 1 {
		t.Errorf("changedTestCount = %d, want 1", pr.ChangedTestCount)
	}
	if pr.ChangedSourceCount != 1 {
		t.Errorf("changedSourceCount = %d, want 1", pr.ChangedSourceCount)
	}
	if pr.ChangedFileCount != 2 {
		t.Errorf("changedFileCount = %d, want 2", pr.ChangedFileCount)
	}
}

func TestAnalyzePRFromChangeSet_Empty(t *testing.T) {
	t.Parallel()
	snap := &models.TestSuiteSnapshot{}
	cs := &models.ChangeSet{Source: "explicit"}

	pr := AnalyzePRFromChangeSet(cs, snap)
	if pr.ChangedFileCount != 0 {
		t.Errorf("changedFileCount = %d, want 0", pr.ChangedFileCount)
	}
	if pr.Summary == "" {
		t.Error("expected non-empty summary even for empty change set")
	}
}

func TestBuildAIValidationSummary_NoAI(t *testing.T) {
	t.Parallel()
	result := &impact.ImpactResult{}
	snap := &models.TestSuiteSnapshot{}
	ai := buildAIValidationSummary(result, snap)
	if ai != nil {
		t.Error("expected nil AI summary when no scenarios or AI signals")
	}
}

func TestBuildAIValidationSummary_WithScenarios(t *testing.T) {
	t.Parallel()
	result := &impact.ImpactResult{
		ImpactedScenarios: []impact.ImpactedScenario{
			{Name: "safety-check", Capability: "safety", Relevance: "prompt changed"},
			{Name: "accuracy-test", Capability: "accuracy", Relevance: "model updated"},
		},
	}
	snap := &models.TestSuiteSnapshot{
		Scenarios: []models.Scenario{
			{ScenarioID: "sc:1", Name: "safety-check"},
			{ScenarioID: "sc:2", Name: "accuracy-test"},
			{ScenarioID: "sc:3", Name: "latency-test"},
		},
	}

	ai := buildAIValidationSummary(result, snap)
	if ai == nil {
		t.Fatal("expected non-nil AI summary")
	}
	if ai.TotalScenarios != 3 {
		t.Errorf("totalScenarios = %d, want 3", ai.TotalScenarios)
	}
	if ai.SelectedScenarios != 2 {
		t.Errorf("selectedScenarios = %d, want 2", ai.SelectedScenarios)
	}
	if len(ai.ImpactedCapabilities) != 2 {
		t.Errorf("impactedCapabilities = %d, want 2", len(ai.ImpactedCapabilities))
	}
	if len(ai.Scenarios) != 2 {
		t.Errorf("scenarios = %d, want 2", len(ai.Scenarios))
	}
}

// TestBuildAIValidationSummary_WithSignals locks in the contract that
// the AI validation summary in `terrain pr` output is impact-scoped:
// only AI signals whose Location.File appears in the PR's changed-
// files set are reported as Blocking / Warning. Pre-fix the loop
// included every AI signal in the snapshot, so a doc-only PR
// surfaced every calibration-corpus fixture as a "blocking" finding.
//
// Fixture: three AI signals + one Quality signal. Two of the AI
// signals are on a changed file ("src/prompt.ts"); one is on an
// unchanged file ("internal/aidetect/foo.go"). Expectations:
//   - Critical AI signal on changed file → BlockingSignals (1 entry)
//   - Medium AI signal on changed file → WarningSignals (1 entry)
//   - High AI signal on UNCHANGED file → dropped (impact filter)
//   - Quality signal → dropped (category filter)
func TestBuildAIValidationSummary_WithSignals(t *testing.T) {
	t.Parallel()
	result := &impact.ImpactResult{
		Scope: impact.ChangeScope{
			ChangedFiles: []impact.ChangedFile{
				{Path: "src/prompt.ts"},
			},
		},
		ImpactedScenarios: []impact.ImpactedScenario{
			{Name: "test", Capability: "search"},
		},
	}
	snap := &models.TestSuiteSnapshot{
		Scenarios: []models.Scenario{
			{ScenarioID: "sc:1", Name: "test"},
		},
		Signals: []models.Signal{
			{Type: "safetyFailure", Category: models.CategoryAI, Severity: models.SeverityCritical,
				Explanation: "safety eval failed", Location: models.SignalLocation{File: "src/prompt.ts"}},
			{Type: "costRegression", Category: models.CategoryAI, Severity: models.SeverityMedium,
				Explanation: "cost increased 20%", Location: models.SignalLocation{File: "src/prompt.ts"}},
			{Type: "aiModelDeprecationRisk", Category: models.CategoryAI, Severity: models.SeverityHigh,
				Explanation: "pre-existing on a file the PR didn't touch",
				Location: models.SignalLocation{File: "internal/aidetect/foo.go"}},
			{Type: "weakAssertion", Category: models.CategoryQuality, Severity: models.SeverityMedium,
				Explanation: "not AI", Location: models.SignalLocation{File: "src/prompt.ts"}},
		},
	}

	ai := buildAIValidationSummary(result, snap)
	if ai == nil {
		t.Fatal("expected non-nil AI summary")
	}
	if len(ai.BlockingSignals) != 1 {
		t.Errorf("blocking signals = %d, want 1 (critical on changed file)", len(ai.BlockingSignals))
	}
	if len(ai.WarningSignals) != 1 {
		t.Errorf("warning signals = %d, want 1 (medium on changed file)", len(ai.WarningSignals))
	}
}

// TestBuildAIValidationSummary_DropsSignalsOnUnchangedFiles is the
// regression test for the noisy-AI-gate bug: a doc-only PR shouldn't
// surface calibration-corpus fixture signals as merge blockers.
func TestBuildAIValidationSummary_DropsSignalsOnUnchangedFiles(t *testing.T) {
	t.Parallel()
	result := &impact.ImpactResult{
		Scope: impact.ChangeScope{
			// Doc-only PR — no source files changed.
			ChangedFiles: []impact.ChangedFile{
				{Path: "docs/feature-status.md"},
				{Path: "CHANGELOG.md"},
			},
		},
	}
	snap := &models.TestSuiteSnapshot{
		Signals: []models.Signal{
			// Calibration-fixture-shaped signal: pre-existing in repo,
			// not introduced by this PR.
			{Type: "aiModelDeprecationRisk", Category: models.CategoryAI, Severity: models.SeverityHigh,
				Explanation: "OpenAI text-davinci-003 reached EOL",
				Location: models.SignalLocation{File: "tests/calibration/floating-model-tag/config.yaml"}},
			{Type: "aiToolWithoutSandbox", Category: models.CategoryAI, Severity: models.SeverityHigh,
				Explanation: "Tool delete_user matches a destructive-verb pattern",
				Location: models.SignalLocation{File: "tests/calibration/agent-without-safety-eval/agent.yaml"}},
			// Whole-repo emergent signal with no Location.File — also
			// shouldn't appear in PR-scoped output.
			{Type: "uncoveredAISurface", Category: models.CategoryAI, Severity: models.SeverityHigh,
				Explanation: "emergent — no Location.File",
				Location: models.SignalLocation{}},
		},
	}

	ai := buildAIValidationSummary(result, snap)
	if ai != nil && len(ai.BlockingSignals) > 0 {
		t.Errorf("doc-only PR should produce zero blocking signals; got %d: %+v",
			len(ai.BlockingSignals), ai.BlockingSignals)
	}
}

func TestBuildAIValidationSummary_UncoveredContexts(t *testing.T) {
	t.Parallel()
	result := &impact.ImpactResult{
		Scope: impact.ChangeScope{
			ChangedFiles: []impact.ChangedFile{
				{Path: "src/prompt.ts"},
			},
		},
		ImpactedScenarios: []impact.ImpactedScenario{
			{Name: "test"},
		},
	}
	snap := &models.TestSuiteSnapshot{
		Scenarios: []models.Scenario{
			{ScenarioID: "sc:1", Name: "test"},
		},
		CodeSurfaces: []models.CodeSurface{
			{SurfaceID: "surface:src/prompt.ts:sysPrompt", Name: "sysPrompt", Path: "src/prompt.ts", Kind: models.SurfaceContext},
		},
	}

	ai := buildAIValidationSummary(result, snap)
	if ai == nil {
		t.Fatal("expected non-nil AI summary")
	}
	if len(ai.UncoveredContexts) != 1 {
		t.Errorf("uncoveredContexts = %d, want 1", len(ai.UncoveredContexts))
	}
}

func TestBuildChangeScopedFindings_ExistingSignals(t *testing.T) {
	t.Parallel()
	result := &impact.ImpactResult{
		Scope: impact.ChangeScope{
			ChangedFiles: []impact.ChangedFile{
				{Path: "src/auth.ts"},
			},
		},
	}
	snap := &models.TestSuiteSnapshot{
		Signals: []models.Signal{
			{Type: "weakAssertion", Category: models.CategoryQuality, Severity: models.SeverityMedium,
				Location: models.SignalLocation{File: "src/auth.ts"}, Explanation: "too few assertions"},
			{Type: "flakyTest", Category: models.CategoryHealth, Severity: models.SeverityHigh,
				Location: models.SignalLocation{File: "src/other.ts"}, Explanation: "not on changed file"},
		},
	}

	findings := buildChangeScopedFindings(result, snap)
	// Should include the signal on the changed file but not the one on other.ts.
	existingSignals := 0
	for _, f := range findings {
		if f.Type == "existing_signal" {
			existingSignals++
			if f.Path != "src/auth.ts" {
				t.Errorf("expected signal on src/auth.ts, got %q", f.Path)
			}
			if f.Scope != "direct" {
				t.Errorf("expected scope=direct, got %q", f.Scope)
			}
		}
	}
	if existingSignals != 1 {
		t.Errorf("expected 1 existing signal finding, got %d", existingSignals)
	}
}

func TestBuildChangeScopedFindings_SkipsDuplicateUntestedExport(t *testing.T) {
	t.Parallel()
	result := &impact.ImpactResult{
		Scope: impact.ChangeScope{
			ChangedFiles: []impact.ChangedFile{
				{Path: "src/auth.ts"},
			},
		},
		ProtectionGaps: []impact.ProtectionGap{
			{Path: "src/auth.ts", Severity: "high", Explanation: "no test coverage"},
		},
	}
	snap := &models.TestSuiteSnapshot{
		Signals: []models.Signal{
			// This untestedExport signal should be skipped because a gap already exists for the same path.
			{Type: "untestedExport", Category: models.CategoryQuality, Severity: models.SeverityHigh,
				Location: models.SignalLocation{File: "src/auth.ts"}, Explanation: "login not tested"},
		},
	}

	findings := buildChangeScopedFindings(result, snap)
	for _, f := range findings {
		if f.Type == "existing_signal" && strings.Contains(f.Explanation, "untestedExport") {
			t.Error("untestedExport signal should be deduplicated when a gap exists for the same path")
		}
	}
}

func TestBuildPostureDelta_WellProtected(t *testing.T) {
	t.Parallel()
	result := &impact.ImpactResult{
		Posture: impact.ChangeRiskPosture{Band: "well_protected"},
	}
	delta := buildPostureDelta(result)
	if delta.OverallDirection != "unchanged" {
		t.Errorf("direction = %q, want unchanged", delta.OverallDirection)
	}
	if delta.Explanation == "" {
		t.Error("expected non-empty explanation")
	}
}

func TestBuildPostureDelta_HighRisk(t *testing.T) {
	t.Parallel()
	result := &impact.ImpactResult{
		Posture:        impact.ChangeRiskPosture{Band: "high_risk"},
		ProtectionGaps: []impact.ProtectionGap{{Path: "a"}, {Path: "b"}},
	}
	delta := buildPostureDelta(result)
	if delta.OverallDirection != "worsened" {
		t.Errorf("direction = %q, want worsened", delta.OverallDirection)
	}
	if delta.NewGapCount != 2 {
		t.Errorf("newGapCount = %d, want 2", delta.NewGapCount)
	}
}

func TestPopulateTestSelections_FallbackToSelectedTests(t *testing.T) {
	t.Parallel()
	pr := &PRAnalysis{}
	result := &impact.ImpactResult{
		// No ProtectiveSet — should fall back to SelectedTests.
		SelectedTests: []impact.ImpactedTest{
			{Path: "tests/auth.test.ts", ImpactConfidence: impact.ConfidenceExact, Relevance: "direct coverage", CoversUnits: []string{"AuthService"}},
			{Path: "tests/util.test.ts", ImpactConfidence: impact.ConfidenceWeak, Relevance: "proximity"},
		},
	}

	populateTestSelections(pr, result)
	if len(pr.RecommendedTests) != 2 {
		t.Fatalf("recommendedTests = %d, want 2", len(pr.RecommendedTests))
	}
	if len(pr.TestSelections) != 2 {
		t.Fatalf("testSelections = %d, want 2", len(pr.TestSelections))
	}
	if pr.TestSelections[0].Confidence != "exact" {
		t.Errorf("first test confidence = %q, want exact", pr.TestSelections[0].Confidence)
	}
	if len(pr.TestSelections[0].CoversUnits) != 1 {
		t.Errorf("first test coversUnits = %d, want 1", len(pr.TestSelections[0].CoversUnits))
	}
}
