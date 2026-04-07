package explain

import (
	"testing"

	"github.com/pmclSF/terrain/internal/impact"
	"github.com/pmclSF/terrain/internal/models"
)

func TestExplainTest_NilResult(t *testing.T) {
	_, err := ExplainTest("test.js", nil)
	if err == nil {
		t.Fatal("expected error for nil result")
	}
}

func TestExplainTest_NotFound(t *testing.T) {
	result := &impact.ImpactResult{}
	_, err := ExplainTest("nonexistent.test.js", result)
	if err == nil {
		t.Fatal("expected error for missing test")
	}
}

func TestExplainSelection_NilResult(t *testing.T) {
	_, err := ExplainSelection(nil)
	if err == nil {
		t.Fatal("expected error for nil result")
	}
}

func TestExplainSelection_EmptyResult(t *testing.T) {
	result := &impact.ImpactResult{}
	sel, err := ExplainSelection(result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sel.TotalSelected != 0 {
		t.Errorf("expected 0 selected, got %d", sel.TotalSelected)
	}
	if sel.Strategy != "none" {
		t.Errorf("expected strategy 'none', got %q", sel.Strategy)
	}
}

func TestClassifyConfidenceBand(t *testing.T) {
	tests := []struct {
		conf float64
		want string
	}{
		{0.95, "high"},
		{0.70, "high"},
		{0.65, "medium"},
		{0.40, "medium"},
		{0.30, "low"},
		{0.0, "low"},
	}
	for _, tt := range tests {
		got := classifyConfidenceBand(tt.conf)
		if got != tt.want {
			t.Errorf("classifyConfidenceBand(%v) = %q, want %q", tt.conf, got, tt.want)
		}
	}
}

func TestConfidenceScore(t *testing.T) {
	tests := []struct {
		conf impact.Confidence
		want float64
	}{
		{impact.ConfidenceExact, 0.95},
		{impact.ConfidenceInferred, 0.65},
		{impact.ConfidenceWeak, 0.30},
	}
	for _, tt := range tests {
		got := confidenceScore(tt.conf)
		if got != tt.want {
			t.Errorf("confidenceScore(%q) = %v, want %v", tt.conf, got, tt.want)
		}
	}
}

func TestEdgeKindLabel(t *testing.T) {
	tests := []struct {
		kind impact.EdgeKind
		want string
	}{
		{impact.EdgeExactCoverage, "exact per-test coverage"},
		{impact.EdgeBucketCoverage, "file-level coverage link"},
		{impact.EdgeStructuralLink, "import/export dependency"},
		{impact.EdgeDirectoryProximity, "directory proximity"},
		{impact.EdgeNameConvention, "naming convention match"},
	}
	for _, tt := range tests {
		got := edgeKindLabel(tt.kind)
		if got != tt.want {
			t.Errorf("edgeKindLabel(%q) = %q, want %q", tt.kind, got, tt.want)
		}
	}
}

func TestClassifyReason(t *testing.T) {
	tests := []struct {
		name string
		test impact.ImpactedTest
		want string
	}{
		{"directly changed", impact.ImpactedTest{IsDirectlyChanged: true}, "directlyChanged"},
		{"exact confidence", impact.ImpactedTest{ImpactConfidence: impact.ConfidenceExact}, "directDependency"},
		{"directory proximity", impact.ImpactedTest{Relevance: "in same directory tree as changed code"}, "directoryProximity"},
		{"default", impact.ImpactedTest{ImpactConfidence: impact.ConfidenceInferred}, "fixtureDependency"},
	}
	for _, tt := range tests {
		got := classifyReason(&tt.test)
		if got != tt.want {
			t.Errorf("classifyReason(%s) = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestFindTest_PartialMatch(t *testing.T) {
	result := &impact.ImpactResult{
		ImpactedTests: []impact.ImpactedTest{
			{Path: "test/integration/auth.test.js"},
		},
	}
	test, found := findTest("auth.test.js", result)
	if !found {
		t.Fatal("expected partial match to find test")
	}
	if test.Path != "test/integration/auth.test.js" {
		t.Errorf("got %q, want test/integration/auth.test.js", test.Path)
	}
}

func TestBuildVerdict_NoPath(t *testing.T) {
	te := &TestExplanation{
		Target:         TestTarget{Path: "test/foo.test.js"},
		ConfidenceBand: "low",
	}
	verdict := buildVerdict(te)
	if verdict == "" {
		t.Error("expected non-empty verdict")
	}
}

// --- Rich Scenario Explain Tests ---

func TestExplainScenarioRich_PromptOnly(t *testing.T) {
	t.Parallel()
	snap := &models.TestSuiteSnapshot{
		CodeSurfaces: []models.CodeSurface{
			{SurfaceID: "surface:src/prompt.ts:buildPrompt", Name: "buildPrompt", Path: "src/prompt.ts", Kind: models.SurfacePrompt},
		},
		Scenarios: []models.Scenario{
			{ScenarioID: "sc:safety", Name: "safety-check", Category: "safety", Capability: "safety",
				CoveredSurfaceIDs: []string{"surface:src/prompt.ts:buildPrompt"}},
		},
	}
	result := &impact.ImpactResult{
		ImpactedScenarios: []impact.ImpactedScenario{
			{ScenarioID: "sc:safety", Name: "safety-check", Category: "safety",
				ImpactConfidence: impact.ConfidenceExact, Capability: "safety",
				CoversSurfaces: []string{"surface:src/prompt.ts:buildPrompt"},
				Relevance:      "prompt changed (buildPrompt)"},
		},
	}

	se, err := ExplainScenarioRich("safety-check", result, snap)
	if err != nil {
		t.Fatalf("explain: %v", err)
	}
	if se.Capability != "safety" {
		t.Errorf("capability = %q", se.Capability)
	}
	if se.RelatedSurfaces == nil {
		t.Fatal("expected related surfaces")
	}
	if len(se.RelatedSurfaces.Prompts) != 1 {
		t.Errorf("expected 1 prompt, got %d", len(se.RelatedSurfaces.Prompts))
	}
	if !se.RelatedSurfaces.Prompts[0].Changed {
		t.Error("expected prompt to be marked as changed")
	}
	if se.PolicyDecision != "pass" {
		t.Errorf("policy = %q, want pass", se.PolicyDecision)
	}
}

func TestExplainScenarioRich_RAGSystem(t *testing.T) {
	t.Parallel()
	snap := &models.TestSuiteSnapshot{
		CodeSurfaces: []models.CodeSurface{
			{SurfaceID: "s:chunk", Name: "chunkConfig", Path: "src/rag/chunk.ts", Kind: models.SurfaceRetrieval},
			{SurfaceID: "s:rerank", Name: "rerankerConfig", Path: "src/rag/rerank.ts", Kind: models.SurfaceRetrieval},
			{SurfaceID: "s:prompt", Name: "searchPrompt", Path: "src/prompts.ts", Kind: models.SurfacePrompt},
		},
		Scenarios: []models.Scenario{
			{ScenarioID: "sc:search", Name: "enterprise-search", Category: "accuracy", Capability: "search",
				CoveredSurfaceIDs: []string{"s:chunk", "s:rerank", "s:prompt"}},
		},
	}
	result := &impact.ImpactResult{
		ImpactedScenarios: []impact.ImpactedScenario{
			{ScenarioID: "sc:search", Name: "enterprise-search", Category: "accuracy",
				ImpactConfidence: impact.ConfidenceExact, Capability: "search",
				CoversSurfaces: []string{"s:chunk"},
				Relevance:      "retrieval config changed (chunkConfig)"},
		},
	}

	se, err := ExplainScenarioRich("enterprise-search", result, snap)
	if err != nil {
		t.Fatalf("explain: %v", err)
	}
	if len(se.RelatedSurfaces.Retrievals) != 2 {
		t.Errorf("expected 2 retrieval surfaces, got %d", len(se.RelatedSurfaces.Retrievals))
	}
	if len(se.RelatedSurfaces.Prompts) != 1 {
		t.Errorf("expected 1 prompt surface, got %d", len(se.RelatedSurfaces.Prompts))
	}
	// Only chunk should be marked changed.
	changedCount := 0
	for _, r := range se.RelatedSurfaces.Retrievals {
		if r.Changed {
			changedCount++
		}
	}
	if changedCount != 1 {
		t.Errorf("expected 1 changed retrieval, got %d", changedCount)
	}
}

func TestExplainScenarioRich_ToolAgent(t *testing.T) {
	t.Parallel()
	snap := &models.TestSuiteSnapshot{
		CodeSurfaces: []models.CodeSurface{
			{SurfaceID: "s:tool", Name: "searchTool", Path: "src/tools.ts", Kind: models.SurfaceToolDef},
			{SurfaceID: "s:agent", Name: "agentRouter", Path: "src/agent.ts", Kind: models.SurfaceAgent},
			{SurfaceID: "s:ctx", Name: "systemPrompt", Path: "src/context.ts", Kind: models.SurfaceContext},
		},
		Scenarios: []models.Scenario{
			{ScenarioID: "sc:agent", Name: "agent-tool-use", Category: "accuracy",
				CoveredSurfaceIDs: []string{"s:tool", "s:agent", "s:ctx"}},
		},
	}
	result := &impact.ImpactResult{
		ImpactedScenarios: []impact.ImpactedScenario{
			{ScenarioID: "sc:agent", Name: "agent-tool-use", Category: "accuracy",
				ImpactConfidence: impact.ConfidenceExact,
				CoversSurfaces:   []string{"s:tool", "s:agent"},
				Relevance:        "tool schema changed; agent config changed"},
		},
	}

	se, err := ExplainScenarioRich("agent-tool-use", result, snap)
	if err != nil {
		t.Fatalf("explain: %v", err)
	}
	if len(se.RelatedSurfaces.ToolDefs) != 1 {
		t.Errorf("expected 1 tool def, got %d", len(se.RelatedSurfaces.ToolDefs))
	}
	if len(se.RelatedSurfaces.Agents) != 1 {
		t.Errorf("expected 1 agent, got %d", len(se.RelatedSurfaces.Agents))
	}
	if len(se.RelatedSurfaces.Contexts) != 1 {
		t.Errorf("expected 1 context, got %d", len(se.RelatedSurfaces.Contexts))
	}
	// Context not changed (not in CoversSurfaces).
	if se.RelatedSurfaces.Contexts[0].Changed {
		t.Error("context should not be marked as changed")
	}
}

func TestExplainScenarioRich_WithSignals(t *testing.T) {
	t.Parallel()
	snap := &models.TestSuiteSnapshot{
		CodeSurfaces: []models.CodeSurface{
			{SurfaceID: "s:p", Name: "prompt", Path: "src/p.ts", Kind: models.SurfacePrompt},
		},
		Scenarios: []models.Scenario{
			{ScenarioID: "sc:safe", Name: "safety", CoveredSurfaceIDs: []string{"s:p"}},
		},
		Signals: []models.Signal{
			{Type: "safetyFailure", Category: models.CategoryAI, Severity: models.SeverityHigh,
				Location:    models.SignalLocation{ScenarioID: "sc:safe"},
				Explanation: "Safety eval failed"},
			{Type: "policyViolation", Category: models.CategoryGovernance, Severity: models.SeverityCritical,
				Explanation: "Block on safety failure",
				Metadata:    map[string]any{"rule": "block_on_safety_failure"}},
		},
	}
	result := &impact.ImpactResult{
		ImpactedScenarios: []impact.ImpactedScenario{
			{ScenarioID: "sc:safe", Name: "safety", ImpactConfidence: impact.ConfidenceExact,
				CoversSurfaces: []string{"s:p"}, Relevance: "prompt changed"},
		},
	}

	se, err := ExplainScenarioRich("safety", result, snap)
	if err != nil {
		t.Fatalf("explain: %v", err)
	}
	if len(se.Signals) != 1 {
		t.Errorf("expected 1 signal, got %d", len(se.Signals))
	}
	if se.Signals[0].Type != "safetyFailure" {
		t.Errorf("signal type = %q", se.Signals[0].Type)
	}
	if se.PolicyDecision == "" || se.PolicyDecision == "pass" {
		t.Errorf("expected blocked policy, got %q", se.PolicyDecision)
	}
}

func TestExplainTest_DirectlyChangedTest(t *testing.T) {
	t.Parallel()
	result := &impact.ImpactResult{
		ImpactedTests: []impact.ImpactedTest{
			{
				Path:              "tests/auth.test.ts",
				Relevance:         "test file directly modified",
				ImpactConfidence:  impact.ConfidenceExact,
				IsDirectlyChanged: true,
			},
		},
	}

	te, err := ExplainTest("tests/auth.test.ts", result)
	if err != nil {
		t.Fatalf("explain: %v", err)
	}
	if te.Verdict == "" {
		t.Error("expected non-empty verdict")
	}
	// buildSyntheticChains should produce a high-confidence chain for directly changed test.
	if te.StrongestPath == nil {
		t.Error("expected non-empty StrongestPath for directly changed test")
	}
	if te.Confidence < 0.9 {
		t.Errorf("expected high confidence for directly changed test, got %f", te.Confidence)
	}
}

func TestExplainTest_CoversUnits(t *testing.T) {
	t.Parallel()
	result := &impact.ImpactResult{
		ImpactedTests: []impact.ImpactedTest{
			{
				Path:             "tests/billing.test.ts",
				Relevance:        "covers changed code unit",
				ImpactConfidence: impact.ConfidenceInferred,
				CoversUnits:      []string{"src/billing.ts:calculateTotal"},
			},
		},
	}

	te, err := ExplainTest("tests/billing.test.ts", result)
	if err != nil {
		t.Fatalf("explain: %v", err)
	}
	if te.Verdict == "" {
		t.Error("expected non-empty verdict")
	}
	if te.StrongestPath == nil {
		t.Error("expected non-empty StrongestPath for test covering units")
	}
	if te.Confidence == 0 {
		t.Error("expected non-zero confidence")
	}
}

func TestExplainTest_WeakRelevance(t *testing.T) {
	t.Parallel()
	result := &impact.ImpactResult{
		ImpactedTests: []impact.ImpactedTest{
			{
				Path:             "tests/util.test.ts",
				Relevance:        "nearby file changed",
				ImpactConfidence: impact.ConfidenceWeak,
			},
		},
	}

	te, err := ExplainTest("tests/util.test.ts", result)
	if err != nil {
		t.Fatalf("explain: %v", err)
	}
	if te.Verdict == "" {
		t.Error("expected non-empty verdict")
	}
	if te.Confidence > 0.5 {
		t.Errorf("expected low confidence for weak relevance, got %f", te.Confidence)
	}
}

func TestExplainScenario_NotFound(t *testing.T) {
	t.Parallel()
	result := &impact.ImpactResult{}
	_, err := ExplainScenario("nonexistent", result)
	if err == nil {
		t.Fatal("expected error for nonexistent scenario")
	}
}

func TestExplainScenario_NilResult(t *testing.T) {
	t.Parallel()
	_, err := ExplainScenario("anything", nil)
	if err == nil {
		t.Fatal("expected error for nil result")
	}
}

func TestExplainScenario_FoundByName(t *testing.T) {
	t.Parallel()
	result := &impact.ImpactResult{
		ImpactedScenarios: []impact.ImpactedScenario{
			{
				ScenarioID:       "sc:safety",
				Name:             "safety-check",
				Category:         "safety",
				Framework:        "deepeval",
				ImpactConfidence: impact.ConfidenceExact,
				CoversSurfaces:   []string{"surface:src/prompt.ts:buildPrompt"},
				Relevance:        "prompt changed",
				Capability:       "safety",
			},
		},
	}

	se, err := ExplainScenario("safety-check", result)
	if err != nil {
		t.Fatalf("explain: %v", err)
	}
	if se.Name != "safety-check" {
		t.Errorf("name = %q, want safety-check", se.Name)
	}
	if se.Category != "safety" {
		t.Errorf("category = %q, want safety", se.Category)
	}
	if se.Capability != "safety" {
		t.Errorf("capability = %q, want safety", se.Capability)
	}
	if len(se.ChangedSurfaces) != 1 {
		t.Errorf("changed surfaces = %d, want 1", len(se.ChangedSurfaces))
	}
	if se.Verdict == "" {
		t.Error("expected non-empty verdict")
	}
}

func TestExplainScenario_SingleSurfaceVerdict(t *testing.T) {
	t.Parallel()
	result := &impact.ImpactResult{
		ImpactedScenarios: []impact.ImpactedScenario{
			{
				ScenarioID:     "sc:1",
				Name:           "test-scenario",
				CoversSurfaces: []string{"surface:src/a.ts:fn"},
			},
		},
	}

	se, err := ExplainScenario("test-scenario", result)
	if err != nil {
		t.Fatalf("explain: %v", err)
	}
	// Single surface verdict should name the specific surface.
	if se.Verdict == "" || !containsStr(se.Verdict, "surface:src/a.ts:fn") {
		t.Errorf("expected single-surface verdict to name the surface, got %q", se.Verdict)
	}
}

func TestExplainSelection_WithTests(t *testing.T) {
	t.Parallel()
	result := &impact.ImpactResult{
		SelectedTests: []impact.ImpactedTest{
			{Path: "tests/auth.test.ts", ImpactConfidence: impact.ConfidenceExact, Relevance: "direct coverage"},
			{Path: "tests/util.test.ts", ImpactConfidence: impact.ConfidenceWeak, Relevance: "nearby"},
		},
		ImpactedTests: []impact.ImpactedTest{
			{Path: "tests/auth.test.ts", ImpactConfidence: impact.ConfidenceExact, Relevance: "direct coverage"},
			{Path: "tests/util.test.ts", ImpactConfidence: impact.ConfidenceWeak, Relevance: "nearby"},
		},
		CoverageConfidence: "medium",
		ReasonCategories: impact.ReasonCategories{
			DirectDependency: 1,
		},
	}

	sel, err := ExplainSelection(result)
	if err != nil {
		t.Fatalf("explain: %v", err)
	}
	if sel.TotalSelected != 2 {
		t.Errorf("total selected = %d, want 2", sel.TotalSelected)
	}
	if sel.Strategy != "exact" {
		t.Errorf("strategy = %q, want exact", sel.Strategy)
	}
	if sel.Summary == "" {
		t.Error("expected non-empty summary")
	}
	if sel.ReasonBreakdown["directDependency"] != 1 {
		t.Errorf("directDependency count = %d, want 1", sel.ReasonBreakdown["directDependency"])
	}
	// Should have bucketed tests by confidence.
	total := len(sel.HighConfidenceTests) + len(sel.MediumConfidenceTests) + len(sel.LowConfidenceTests)
	if total != 2 {
		t.Errorf("bucketed tests = %d, want 2", total)
	}
}

func TestFindTest_SuffixMatchInSelectedTests(t *testing.T) {
	t.Parallel()
	result := &impact.ImpactResult{
		SelectedTests: []impact.ImpactedTest{
			{Path: "tests/integration/auth.test.ts", Relevance: "selected"},
		},
	}

	found, ok := findTest("auth.test.ts", result)
	if !ok {
		t.Fatal("expected suffix match on SelectedTests")
	}
	if found.Path != "tests/integration/auth.test.ts" {
		t.Errorf("path = %q", found.Path)
	}
}

func TestBuildTestLimitations_WeakConfidence(t *testing.T) {
	t.Parallel()
	test := &impact.ImpactedTest{
		ImpactConfidence: impact.ConfidenceWeak,
	}
	result := &impact.ImpactResult{}

	lims := buildTestLimitations(test, result)
	if len(lims) == 0 {
		t.Error("expected limitations for weak confidence")
	}
	found := false
	for _, l := range lims {
		if containsStr(l, "Low confidence") {
			found = true
		}
	}
	if !found {
		t.Error("expected 'Low confidence' limitation")
	}
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
