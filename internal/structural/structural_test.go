package structural

import (
	"fmt"
	"testing"

	"github.com/pmclSF/terrain/internal/depgraph"
	"github.com/pmclSF/terrain/internal/models"
	"github.com/pmclSF/terrain/internal/signals"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// buildGraph creates a fresh graph with the given nodes and edges, then seals it.
func buildGraph(nodes []*depgraph.Node, edges []*depgraph.Edge) *depgraph.Graph {
	g := depgraph.NewGraph()
	for _, n := range nodes {
		g.AddNode(n)
	}
	for _, e := range edges {
		g.AddEdge(e)
	}
	g.Seal()
	return g
}

func signalOfType(sigs []models.Signal, st models.SignalType) *models.Signal {
	for i := range sigs {
		if sigs[i].Type == st {
			return &sigs[i]
		}
	}
	return nil
}

func signalsOfType(sigs []models.Signal, st models.SignalType) []models.Signal {
	var out []models.Signal
	for _, s := range sigs {
		if s.Type == st {
			out = append(out, s)
		}
	}
	return out
}

// ===========================================================================
// UncoveredAISurfaceDetector
// ===========================================================================

func TestUncoveredAISurface_DetectReturnsNil(t *testing.T) {
	t.Parallel()
	d := &UncoveredAISurfaceDetector{}
	if got := d.Detect(&models.TestSuiteSnapshot{}); got != nil {
		t.Fatalf("Detect() should return nil, got %v", got)
	}
}

func TestUncoveredAISurface_UncoveredPromptFlagged(t *testing.T) {
	t.Parallel()
	g := buildGraph(
		[]*depgraph.Node{
			{ID: "prompt:sys", Name: "system-prompt", Type: depgraph.NodePrompt, Path: "src/prompts/sys.ts"},
		},
		nil,
	)

	d := &UncoveredAISurfaceDetector{}
	sigs := d.DetectWithGraph(&models.TestSuiteSnapshot{}, g)
	if len(sigs) != 1 {
		t.Fatalf("expected 1 signal, got %d", len(sigs))
	}
	s := sigs[0]
	if s.Type != signals.SignalUncoveredAISurface {
		t.Errorf("type = %q, want %q", s.Type, signals.SignalUncoveredAISurface)
	}
	if s.Severity != models.SeverityCritical {
		t.Errorf("severity = %q, want Critical (prompt)", s.Severity)
	}
	if s.Location.File != "src/prompts/sys.ts" {
		t.Errorf("location file = %q", s.Location.File)
	}
}

func TestUncoveredAISurface_CoveredPromptNotFlagged(t *testing.T) {
	t.Parallel()
	g := buildGraph(
		[]*depgraph.Node{
			{ID: "prompt:sys", Name: "system-prompt", Type: depgraph.NodePrompt, Path: "src/prompts/sys.ts"},
			{ID: "test:eval:accuracy", Name: "accuracy", Type: depgraph.NodeTestFile, Path: "tests/eval/accuracy.test.ts"},
		},
		[]*depgraph.Edge{
			{From: "test:eval:accuracy", To: "prompt:sys", Type: depgraph.EdgeCoversCodeSurface, Confidence: 0.9, EvidenceType: depgraph.EvidenceStaticAnalysis},
		},
	)

	d := &UncoveredAISurfaceDetector{}
	sigs := d.DetectWithGraph(&models.TestSuiteSnapshot{}, g)
	if len(sigs) != 0 {
		t.Fatalf("expected 0 signals for covered prompt, got %d", len(sigs))
	}
}

func TestUncoveredAISurface_ManualCoverageCountsAsCovered(t *testing.T) {
	t.Parallel()
	g := buildGraph(
		[]*depgraph.Node{
			{ID: "dataset:eval", Name: "eval-data", Type: depgraph.NodeDataset, Path: "data/eval.json"},
			{ID: "manual:qa", Name: "qa-check", Type: depgraph.NodeTestFile, Path: "qa/checklist.md"},
		},
		[]*depgraph.Edge{
			{From: "manual:qa", To: "dataset:eval", Type: depgraph.EdgeManualCovers, Confidence: 0.7, EvidenceType: depgraph.EvidenceManual},
		},
	)

	d := &UncoveredAISurfaceDetector{}
	sigs := d.DetectWithGraph(&models.TestSuiteSnapshot{}, g)
	if len(sigs) != 0 {
		t.Fatalf("expected 0 signals when manual coverage exists, got %d", len(sigs))
	}
}

func TestUncoveredAISurface_SeverityVariesByType(t *testing.T) {
	t.Parallel()
	g := buildGraph(
		[]*depgraph.Node{
			{ID: "prompt:p", Name: "p", Type: depgraph.NodePrompt, Path: "a.ts"},
			{ID: "model:m", Name: "m", Type: depgraph.NodeModel, Path: "b.ts"},
			{ID: "dataset:d", Name: "d", Type: depgraph.NodeDataset, Path: "c.ts"},
			{ID: "eval_metric:e", Name: "e", Type: depgraph.NodeEvalMetric, Path: "d.ts"},
		},
		nil,
	)

	d := &UncoveredAISurfaceDetector{}
	sigs := d.DetectWithGraph(&models.TestSuiteSnapshot{}, g)
	if len(sigs) != 4 {
		t.Fatalf("expected 4 signals, got %d", len(sigs))
	}

	expected := map[string]models.SignalSeverity{
		"a.ts": models.SeverityCritical, // prompt
		"b.ts": models.SeverityHigh,     // model
		"c.ts": models.SeverityMedium,   // dataset
		"d.ts": models.SeverityLow,      // eval metric
	}
	for _, s := range sigs {
		want, ok := expected[s.Location.File]
		if !ok {
			t.Errorf("unexpected signal for file %q", s.Location.File)
			continue
		}
		if s.Severity != want {
			t.Errorf("file %q: severity = %q, want %q", s.Location.File, s.Severity, want)
		}
	}
}

func TestUncoveredAISurface_EmptyGraph(t *testing.T) {
	t.Parallel()
	g := buildGraph(nil, nil)
	d := &UncoveredAISurfaceDetector{}
	sigs := d.DetectWithGraph(&models.TestSuiteSnapshot{}, g)
	if len(sigs) != 0 {
		t.Fatalf("expected 0 signals for empty graph, got %d", len(sigs))
	}
}

func TestUncoveredAISurface_ValidationForSurfaceSuffices(t *testing.T) {
	t.Parallel()
	g := buildGraph(
		[]*depgraph.Node{
			{ID: "prompt:sys", Name: "sys", Type: depgraph.NodePrompt, Path: "prompt.ts"},
			{ID: "scenario:accuracy", Name: "accuracy", Type: depgraph.NodeTestFile, Path: "eval/accuracy.py"},
		},
		[]*depgraph.Edge{
			{From: "scenario:accuracy", To: "prompt:sys", Type: depgraph.EdgeCoversCodeSurface, Confidence: 0.9, EvidenceType: depgraph.EvidenceStaticAnalysis},
		},
	)

	d := &UncoveredAISurfaceDetector{}
	sigs := d.DetectWithGraph(&models.TestSuiteSnapshot{}, g)
	if len(sigs) != 0 {
		t.Fatalf("expected 0 signals when validation exists, got %d", len(sigs))
	}
}

// ===========================================================================
// PhantomEvalScenarioDetector
// ===========================================================================

func TestPhantomEvalScenario_DetectReturnsNil(t *testing.T) {
	t.Parallel()
	d := &PhantomEvalScenarioDetector{}
	if got := d.Detect(&models.TestSuiteSnapshot{}); got != nil {
		t.Fatalf("Detect() should return nil, got %v", got)
	}
}

func TestPhantomEvalScenario_ReachableSurfaceNotFlagged(t *testing.T) {
	t.Parallel()
	g := buildGraph(
		[]*depgraph.Node{
			{ID: "scenario:accuracy", Name: "accuracy", Type: depgraph.NodeTestFile, Path: "tests/eval/accuracy.test.ts"},
			{ID: "file:tests/eval/accuracy.test.ts", Name: "accuracy.test.ts", Type: depgraph.NodeTestFile, Path: "tests/eval/accuracy.test.ts"},
			{ID: "file:src/prompts/sys.ts", Name: "sys.ts", Type: depgraph.NodeSourceFile, Path: "src/prompts/sys.ts"},
			{ID: "surface:sys", Name: "sys-prompt", Type: depgraph.NodePrompt, Path: "src/prompts/sys.ts"},
		},
		[]*depgraph.Edge{
			{From: "file:tests/eval/accuracy.test.ts", To: "file:src/prompts/sys.ts", Type: depgraph.EdgeImportsModule, Confidence: 1.0, EvidenceType: depgraph.EvidenceStaticAnalysis},
		},
	)

	snap := &models.TestSuiteSnapshot{
		Scenarios: []models.Scenario{
			{
				ScenarioID:        "accuracy",
				Name:              "accuracy",
				Path:              "tests/eval/accuracy.test.ts",
				CoveredSurfaceIDs: []string{"surface:sys"},
			},
		},
	}

	d := &PhantomEvalScenarioDetector{}
	sigs := d.DetectWithGraph(snap, g)
	if len(sigs) != 0 {
		t.Fatalf("expected 0 signals when surface is reachable, got %d", len(sigs))
	}
}

func TestPhantomEvalScenario_UnreachableSurfaceFlagged(t *testing.T) {
	t.Parallel()
	g := buildGraph(
		[]*depgraph.Node{
			{ID: "scenario:accuracy", Name: "accuracy", Type: depgraph.NodeTestFile, Path: "tests/eval/accuracy.test.ts"},
			{ID: "file:tests/eval/accuracy.test.ts", Name: "accuracy.test.ts", Type: depgraph.NodeTestFile, Path: "tests/eval/accuracy.test.ts"},
			{ID: "file:src/prompts/sys.ts", Name: "sys.ts", Type: depgraph.NodeSourceFile, Path: "src/prompts/sys.ts"},
			{ID: "surface:sys", Name: "sys-prompt", Type: depgraph.NodePrompt, Path: "src/prompts/sys.ts"},
			// No import edge from test to source — surface is unreachable
		},
		nil,
	)

	snap := &models.TestSuiteSnapshot{
		Scenarios: []models.Scenario{
			{
				ScenarioID:        "accuracy",
				Name:              "accuracy",
				Path:              "tests/eval/accuracy.test.ts",
				CoveredSurfaceIDs: []string{"surface:sys"},
			},
		},
	}

	d := &PhantomEvalScenarioDetector{}
	sigs := d.DetectWithGraph(snap, g)
	if len(sigs) != 1 {
		t.Fatalf("expected 1 signal for unreachable surface, got %d", len(sigs))
	}
	if sigs[0].Type != signals.SignalPhantomEvalScenario {
		t.Errorf("type = %q, want %q", sigs[0].Type, signals.SignalPhantomEvalScenario)
	}
	unreachable, ok := sigs[0].Metadata["unreachableSurfaces"].([]string)
	if !ok || len(unreachable) != 1 {
		t.Fatalf("expected 1 unreachable surface in metadata, got %v", sigs[0].Metadata["unreachableSurfaces"])
	}
	if unreachable[0] != "surface:sys" {
		t.Errorf("unreachable surface = %q, want %q", unreachable[0], "surface:sys")
	}
}

func TestPhantomEvalScenario_ExecutableSeverityIsHigh(t *testing.T) {
	t.Parallel()
	g := buildGraph(
		[]*depgraph.Node{
			{ID: "scenario:safety", Name: "safety", Type: depgraph.NodeTestFile, Path: "tests/eval/safety.test.ts"},
			{ID: "file:tests/eval/safety.test.ts", Name: "safety.test.ts", Type: depgraph.NodeTestFile, Path: "tests/eval/safety.test.ts"},
			{ID: "surface:guard", Name: "guard", Type: depgraph.NodePrompt, Path: "src/guard.ts"},
		},
		nil,
	)

	snap := &models.TestSuiteSnapshot{
		Scenarios: []models.Scenario{
			{
				ScenarioID:        "safety",
				Name:              "safety",
				Path:              "tests/eval/safety.test.ts",
				CoveredSurfaceIDs: []string{"surface:guard"},
				Executable:        true,
			},
		},
	}

	d := &PhantomEvalScenarioDetector{}
	sigs := d.DetectWithGraph(snap, g)
	if len(sigs) != 1 {
		t.Fatalf("expected 1 signal, got %d", len(sigs))
	}
	if sigs[0].Severity != models.SeverityHigh {
		t.Errorf("severity = %q, want High for executable scenario", sigs[0].Severity)
	}
}

func TestPhantomEvalScenario_NoCoveredSurfacesSkipped(t *testing.T) {
	t.Parallel()
	g := buildGraph(
		[]*depgraph.Node{
			{ID: "scenario:basic", Name: "basic", Type: depgraph.NodeTestFile, Path: "tests/basic.test.ts"},
		},
		nil,
	)

	snap := &models.TestSuiteSnapshot{
		Scenarios: []models.Scenario{
			{ScenarioID: "basic", Name: "basic", Path: "tests/basic.test.ts"},
		},
	}

	d := &PhantomEvalScenarioDetector{}
	sigs := d.DetectWithGraph(snap, g)
	if len(sigs) != 0 {
		t.Fatalf("expected 0 signals for scenario with no claimed surfaces, got %d", len(sigs))
	}
}

// ===========================================================================
// UntestedPromptFlowDetector
// ===========================================================================

func TestUntestedPromptFlow_DetectReturnsNil(t *testing.T) {
	t.Parallel()
	d := &UntestedPromptFlowDetector{}
	if got := d.Detect(&models.TestSuiteSnapshot{}); got != nil {
		t.Fatalf("Detect() should return nil, got %v", got)
	}
}

func TestUntestedPromptFlow_TestedFlowNotFlagged(t *testing.T) {
	t.Parallel()
	g := buildGraph(
		[]*depgraph.Node{
			{ID: "prompt:sys", Name: "system-prompt", Type: depgraph.NodePrompt, Path: "src/prompts/sys.ts"},
			{ID: "file:src/prompts/sys.ts", Name: "sys.ts", Type: depgraph.NodeSourceFile, Path: "src/prompts/sys.ts"},
			{ID: "file:src/chain.ts", Name: "chain.ts", Type: depgraph.NodeSourceFile, Path: "src/chain.ts"},
			{ID: "file:tests/chain.test.ts", Name: "chain.test.ts", Type: depgraph.NodeTestFile, Path: "tests/chain.test.ts"},
		},
		[]*depgraph.Edge{
			{From: "prompt:sys", To: "file:src/prompts/sys.ts", Type: depgraph.EdgeAIDefinedInFile, Confidence: 1.0, EvidenceType: depgraph.EvidenceStaticAnalysis},
			{From: "file:src/chain.ts", To: "file:src/prompts/sys.ts", Type: depgraph.EdgeSourceImportsSource, Confidence: 1.0, EvidenceType: depgraph.EvidenceStaticAnalysis},
			{From: "file:tests/chain.test.ts", To: "file:src/chain.ts", Type: depgraph.EdgeImportsModule, Confidence: 1.0, EvidenceType: depgraph.EvidenceStaticAnalysis},
		},
	)

	d := &UntestedPromptFlowDetector{}
	sigs := d.DetectWithGraph(&models.TestSuiteSnapshot{}, g)
	if len(sigs) != 0 {
		t.Fatalf("expected 0 signals when flow has test coverage, got %d", len(sigs))
	}
}

func TestUntestedPromptFlow_UntestedFlowFlagged(t *testing.T) {
	t.Parallel()
	g := buildGraph(
		[]*depgraph.Node{
			{ID: "prompt:sys", Name: "system-prompt", Type: depgraph.NodePrompt, Path: "src/prompts/sys.ts"},
			{ID: "file:src/prompts/sys.ts", Name: "sys.ts", Type: depgraph.NodeSourceFile, Path: "src/prompts/sys.ts"},
			{ID: "file:src/chain.ts", Name: "chain.ts", Type: depgraph.NodeSourceFile, Path: "src/chain.ts"},
			// No test file — entire flow is untested
		},
		[]*depgraph.Edge{
			{From: "prompt:sys", To: "file:src/prompts/sys.ts", Type: depgraph.EdgeAIDefinedInFile, Confidence: 1.0, EvidenceType: depgraph.EvidenceStaticAnalysis},
			{From: "file:src/chain.ts", To: "file:src/prompts/sys.ts", Type: depgraph.EdgeSourceImportsSource, Confidence: 1.0, EvidenceType: depgraph.EvidenceStaticAnalysis},
		},
	)

	d := &UntestedPromptFlowDetector{}
	sigs := d.DetectWithGraph(&models.TestSuiteSnapshot{}, g)
	if len(sigs) != 1 {
		t.Fatalf("expected 1 signal for untested flow, got %d", len(sigs))
	}
	s := sigs[0]
	if s.Type != signals.SignalUntestedPromptFlow {
		t.Errorf("type = %q, want %q", s.Type, signals.SignalUntestedPromptFlow)
	}
	if s.Severity != models.SeverityHigh {
		t.Errorf("severity = %q, want High", s.Severity)
	}
	flowLen, _ := s.Metadata["flowLength"].(int)
	if flowLen < 2 {
		t.Errorf("flowLength = %d, want >= 2", flowLen)
	}
}

func TestUntestedPromptFlow_LongChainIsCritical(t *testing.T) {
	t.Parallel()
	g := buildGraph(
		[]*depgraph.Node{
			{ID: "prompt:p", Name: "p", Type: depgraph.NodePrompt, Path: "src/prompt.ts"},
			{ID: "file:src/prompt.ts", Name: "prompt.ts", Type: depgraph.NodeSourceFile, Path: "src/prompt.ts"},
			{ID: "file:src/a.ts", Name: "a.ts", Type: depgraph.NodeSourceFile, Path: "src/a.ts"},
			{ID: "file:src/b.ts", Name: "b.ts", Type: depgraph.NodeSourceFile, Path: "src/b.ts"},
			{ID: "file:src/c.ts", Name: "c.ts", Type: depgraph.NodeSourceFile, Path: "src/c.ts"},
		},
		[]*depgraph.Edge{
			{From: "prompt:p", To: "file:src/prompt.ts", Type: depgraph.EdgeAIDefinedInFile, Confidence: 1.0, EvidenceType: depgraph.EvidenceStaticAnalysis},
			{From: "file:src/a.ts", To: "file:src/prompt.ts", Type: depgraph.EdgeSourceImportsSource, Confidence: 1.0, EvidenceType: depgraph.EvidenceStaticAnalysis},
			{From: "file:src/b.ts", To: "file:src/a.ts", Type: depgraph.EdgeSourceImportsSource, Confidence: 1.0, EvidenceType: depgraph.EvidenceStaticAnalysis},
			{From: "file:src/c.ts", To: "file:src/b.ts", Type: depgraph.EdgeSourceImportsSource, Confidence: 1.0, EvidenceType: depgraph.EvidenceStaticAnalysis},
		},
	)

	d := &UntestedPromptFlowDetector{}
	sigs := d.DetectWithGraph(&models.TestSuiteSnapshot{}, g)
	if len(sigs) != 1 {
		t.Fatalf("expected 1 signal, got %d", len(sigs))
	}
	if sigs[0].Severity != models.SeverityCritical {
		t.Errorf("severity = %q, want Critical for long chain (>=3 hops)", sigs[0].Severity)
	}
	if sigs[0].Confidence != 0.75 {
		t.Errorf("confidence = %f, want 0.75 for long chain", sigs[0].Confidence)
	}
}

func TestUntestedPromptFlow_NoDefinedFileSkipped(t *testing.T) {
	t.Parallel()
	g := buildGraph(
		[]*depgraph.Node{
			{ID: "prompt:orphan", Name: "orphan", Type: depgraph.NodePrompt, Path: "orphan.ts"},
		},
		nil, // No EdgeAIDefinedInFile
	)

	d := &UntestedPromptFlowDetector{}
	sigs := d.DetectWithGraph(&models.TestSuiteSnapshot{}, g)
	if len(sigs) != 0 {
		t.Fatalf("expected 0 signals when prompt has no defined file, got %d", len(sigs))
	}
}

// ===========================================================================
// CapabilityValidationGapDetector
// ===========================================================================

func TestCapabilityValidationGap_DetectReturnsNil(t *testing.T) {
	t.Parallel()
	d := &CapabilityValidationGapDetector{}
	if got := d.Detect(&models.TestSuiteSnapshot{}); got != nil {
		t.Fatalf("Detect() should return nil, got %v", got)
	}
}

func TestCapabilityValidationGap_NoScenarios(t *testing.T) {
	t.Parallel()
	g := buildGraph(
		[]*depgraph.Node{
			{ID: "capability:rag", Name: "retrieval_augmented_generation", Type: depgraph.NodeCapability},
		},
		nil,
	)

	d := &CapabilityValidationGapDetector{}
	sigs := d.DetectWithGraph(&models.TestSuiteSnapshot{}, g)
	if len(sigs) != 1 {
		t.Fatalf("expected 1 signal for capability with no scenarios, got %d", len(sigs))
	}
	if sigs[0].Severity != models.SeverityHigh {
		t.Errorf("severity = %q, want High for zero scenarios", sigs[0].Severity)
	}
	if sigs[0].Confidence != 0.80 {
		t.Errorf("confidence = %f, want 0.80", sigs[0].Confidence)
	}
}

func TestCapabilityValidationGap_NonExecutableScenarios(t *testing.T) {
	t.Parallel()
	g := buildGraph(
		[]*depgraph.Node{
			{ID: "capability:rag", Name: "rag", Type: depgraph.NodeCapability},
			{ID: "scenario:rag-check", Name: "rag-check", Type: depgraph.NodeTestFile, Metadata: map[string]string{"executable": "false"}},
		},
		[]*depgraph.Edge{
			{From: "scenario:rag-check", To: "capability:rag", Type: depgraph.EdgeScenarioValidatesCapability, Confidence: 0.9, EvidenceType: depgraph.EvidenceStaticAnalysis},
		},
	)

	d := &CapabilityValidationGapDetector{}
	sigs := d.DetectWithGraph(&models.TestSuiteSnapshot{}, g)
	if len(sigs) != 1 {
		t.Fatalf("expected 1 signal for non-executable scenario, got %d", len(sigs))
	}
	if sigs[0].Severity != models.SeverityMedium {
		t.Errorf("severity = %q, want Medium for non-executable scenarios", sigs[0].Severity)
	}
	if sigs[0].Confidence != 0.65 {
		t.Errorf("confidence = %f, want 0.65", sigs[0].Confidence)
	}
}

func TestCapabilityValidationGap_ExecutableScenarioNotFlagged(t *testing.T) {
	t.Parallel()
	g := buildGraph(
		[]*depgraph.Node{
			{ID: "capability:tool_use", Name: "tool_use", Type: depgraph.NodeCapability},
			{ID: "scenario:tool-test", Name: "tool-test", Type: depgraph.NodeTestFile, Metadata: map[string]string{"executable": "true"}},
		},
		[]*depgraph.Edge{
			{From: "scenario:tool-test", To: "capability:tool_use", Type: depgraph.EdgeScenarioValidatesCapability, Confidence: 0.9, EvidenceType: depgraph.EvidenceStaticAnalysis},
		},
	)

	d := &CapabilityValidationGapDetector{}
	sigs := d.DetectWithGraph(&models.TestSuiteSnapshot{}, g)
	if len(sigs) != 0 {
		t.Fatalf("expected 0 signals when executable scenario exists, got %d", len(sigs))
	}
}

func TestCapabilityValidationGap_EmptyGraph(t *testing.T) {
	t.Parallel()
	g := buildGraph(nil, nil)
	d := &CapabilityValidationGapDetector{}
	sigs := d.DetectWithGraph(&models.TestSuiteSnapshot{}, g)
	if len(sigs) != 0 {
		t.Fatalf("expected 0 signals for empty graph, got %d", len(sigs))
	}
}

// ===========================================================================
// AssertionFreeImportDetector
// ===========================================================================

func TestAssertionFreeImport_DetectReturnsNil(t *testing.T) {
	t.Parallel()
	d := &AssertionFreeImportDetector{}
	if got := d.Detect(&models.TestSuiteSnapshot{}); got != nil {
		t.Fatalf("Detect() should return nil, got %v", got)
	}
}

func TestAssertionFreeImport_ZeroAssertionsFlagged(t *testing.T) {
	t.Parallel()
	g := buildGraph(
		[]*depgraph.Node{
			{ID: "test_file:tests/auth.test.ts", Name: "auth.test.ts", Type: depgraph.NodeTestFile, Path: "tests/auth.test.ts"},
			{ID: "file:src/auth.ts", Name: "auth.ts", Type: depgraph.NodeSourceFile, Path: "src/auth.ts"},
		},
		[]*depgraph.Edge{
			{From: "test_file:tests/auth.test.ts", To: "file:src/auth.ts", Type: depgraph.EdgeImportsModule, Confidence: 1.0, EvidenceType: depgraph.EvidenceStaticAnalysis},
		},
	)

	snap := &models.TestSuiteSnapshot{
		TestFiles: []models.TestFile{
			{Path: "tests/auth.test.ts", TestCount: 5, AssertionCount: 0},
		},
	}

	d := &AssertionFreeImportDetector{}
	sigs := d.DetectWithGraph(snap, g)
	if len(sigs) != 1 {
		t.Fatalf("expected 1 signal, got %d", len(sigs))
	}
	if sigs[0].Type != signals.SignalAssertionFreeImport {
		t.Errorf("type = %q, want %q", sigs[0].Type, signals.SignalAssertionFreeImport)
	}
	if sigs[0].Severity != models.SeverityMedium {
		t.Errorf("severity = %q, want Medium for <3 imports", sigs[0].Severity)
	}
}

func TestAssertionFreeImport_ManyImportsIsHighSeverity(t *testing.T) {
	t.Parallel()
	g := buildGraph(
		[]*depgraph.Node{
			{ID: "test_file:tests/big.test.ts", Name: "big.test.ts", Type: depgraph.NodeTestFile, Path: "tests/big.test.ts"},
			{ID: "file:src/a.ts", Name: "a.ts", Type: depgraph.NodeSourceFile, Path: "src/a.ts"},
			{ID: "file:src/b.ts", Name: "b.ts", Type: depgraph.NodeSourceFile, Path: "src/b.ts"},
			{ID: "file:src/c.ts", Name: "c.ts", Type: depgraph.NodeSourceFile, Path: "src/c.ts"},
		},
		[]*depgraph.Edge{
			{From: "test_file:tests/big.test.ts", To: "file:src/a.ts", Type: depgraph.EdgeImportsModule, Confidence: 1.0, EvidenceType: depgraph.EvidenceStaticAnalysis},
			{From: "test_file:tests/big.test.ts", To: "file:src/b.ts", Type: depgraph.EdgeImportsModule, Confidence: 1.0, EvidenceType: depgraph.EvidenceStaticAnalysis},
			{From: "test_file:tests/big.test.ts", To: "file:src/c.ts", Type: depgraph.EdgeImportsModule, Confidence: 1.0, EvidenceType: depgraph.EvidenceStaticAnalysis},
		},
	)

	snap := &models.TestSuiteSnapshot{
		TestFiles: []models.TestFile{
			{Path: "tests/big.test.ts", TestCount: 10, AssertionCount: 0},
		},
	}

	d := &AssertionFreeImportDetector{}
	sigs := d.DetectWithGraph(snap, g)
	if len(sigs) != 1 {
		t.Fatalf("expected 1 signal, got %d", len(sigs))
	}
	if sigs[0].Severity != models.SeverityHigh {
		t.Errorf("severity = %q, want High for >=3 imports", sigs[0].Severity)
	}
}

func TestAssertionFreeImport_WithAssertionsNotFlagged(t *testing.T) {
	t.Parallel()
	g := buildGraph(
		[]*depgraph.Node{
			{ID: "test_file:tests/auth.test.ts", Name: "auth.test.ts", Type: depgraph.NodeTestFile, Path: "tests/auth.test.ts"},
			{ID: "file:src/auth.ts", Name: "auth.ts", Type: depgraph.NodeSourceFile, Path: "src/auth.ts"},
		},
		[]*depgraph.Edge{
			{From: "test_file:tests/auth.test.ts", To: "file:src/auth.ts", Type: depgraph.EdgeImportsModule, Confidence: 1.0, EvidenceType: depgraph.EvidenceStaticAnalysis},
		},
	)

	snap := &models.TestSuiteSnapshot{
		TestFiles: []models.TestFile{
			{Path: "tests/auth.test.ts", TestCount: 5, AssertionCount: 10},
		},
	}

	d := &AssertionFreeImportDetector{}
	sigs := d.DetectWithGraph(snap, g)
	if len(sigs) != 0 {
		t.Fatalf("expected 0 signals when assertions present, got %d", len(sigs))
	}
}

func TestAssertionFreeImport_NoImportsNotFlagged(t *testing.T) {
	t.Parallel()
	g := buildGraph(
		[]*depgraph.Node{
			{ID: "test_file:tests/lonely.test.ts", Name: "lonely.test.ts", Type: depgraph.NodeTestFile, Path: "tests/lonely.test.ts"},
		},
		nil,
	)

	snap := &models.TestSuiteSnapshot{
		TestFiles: []models.TestFile{
			{Path: "tests/lonely.test.ts", TestCount: 3, AssertionCount: 0},
		},
	}

	d := &AssertionFreeImportDetector{}
	sigs := d.DetectWithGraph(snap, g)
	if len(sigs) != 0 {
		t.Fatalf("expected 0 signals when no source imports exist, got %d", len(sigs))
	}
}

func TestAssertionFreeImport_ZeroTestsNotFlagged(t *testing.T) {
	t.Parallel()
	g := buildGraph(
		[]*depgraph.Node{
			{ID: "test_file:tests/empty.test.ts", Name: "empty.test.ts", Type: depgraph.NodeTestFile, Path: "tests/empty.test.ts"},
			{ID: "file:src/auth.ts", Name: "auth.ts", Type: depgraph.NodeSourceFile, Path: "src/auth.ts"},
		},
		[]*depgraph.Edge{
			{From: "test_file:tests/empty.test.ts", To: "file:src/auth.ts", Type: depgraph.EdgeImportsModule, Confidence: 1.0, EvidenceType: depgraph.EvidenceStaticAnalysis},
		},
	)

	snap := &models.TestSuiteSnapshot{
		TestFiles: []models.TestFile{
			{Path: "tests/empty.test.ts", TestCount: 0, AssertionCount: 0},
		},
	}

	d := &AssertionFreeImportDetector{}
	sigs := d.DetectWithGraph(snap, g)
	if len(sigs) != 0 {
		t.Fatalf("expected 0 signals when test count is 0, got %d", len(sigs))
	}
}

// ===========================================================================
// BlastRadiusHotspotDetector
// ===========================================================================

func TestBlastRadiusHotspot_DetectReturnsNil(t *testing.T) {
	t.Parallel()
	d := &BlastRadiusHotspotDetector{}
	if got := d.Detect(&models.TestSuiteSnapshot{}); got != nil {
		t.Fatalf("Detect() should return nil, got %v", got)
	}
}

func TestBlastRadiusHotspot_HighTestCountFlagged(t *testing.T) {
	t.Parallel()
	// Build a source file with 25 test files, each containing a test case.
	// AnalyzeCoverage needs EdgeTestDefinedInFile from test nodes → test file nodes.
	nodes := []*depgraph.Node{
		{ID: "file:src/core.ts", Name: "core.ts", Type: depgraph.NodeSourceFile, Path: "src/core.ts"},
	}
	var edges []*depgraph.Edge
	for i := 0; i < 25; i++ {
		fileID := fmt.Sprintf("file:tests/t%d.test.ts", i)
		testCaseID := fmt.Sprintf("test:t%d:1:case", i)
		nodes = append(nodes,
			&depgraph.Node{ID: fileID, Name: fmt.Sprintf("t%d.test.ts", i), Type: depgraph.NodeTestFile, Path: fmt.Sprintf("tests/t%d.test.ts", i)},
			&depgraph.Node{ID: testCaseID, Name: fmt.Sprintf("case_%d", i), Type: depgraph.NodeTest},
		)
		edges = append(edges,
			&depgraph.Edge{From: fileID, To: "file:src/core.ts", Type: depgraph.EdgeImportsModule, Confidence: 1.0, EvidenceType: depgraph.EvidenceStaticAnalysis},
			&depgraph.Edge{From: testCaseID, To: fileID, Type: depgraph.EdgeTestDefinedInFile, Confidence: 1.0, EvidenceType: depgraph.EvidenceStaticAnalysis},
		)
	}

	g := buildGraph(nodes, edges)

	d := &BlastRadiusHotspotDetector{}
	sigs := d.DetectWithGraph(&models.TestSuiteSnapshot{}, g)
	if len(sigs) == 0 {
		t.Fatal("expected at least 1 signal for file with 25 test dependents")
	}
	if sigs[0].Type != signals.SignalBlastRadiusHotspot {
		t.Errorf("type = %q, want %q", sigs[0].Type, signals.SignalBlastRadiusHotspot)
	}
	if sigs[0].Severity != models.SeverityMedium && sigs[0].Severity != models.SeverityHigh {
		t.Errorf("severity = %q, want Medium or High for 25 tests", sigs[0].Severity)
	}
}

func TestBlastRadiusHotspot_LowTestCountNotFlagged(t *testing.T) {
	t.Parallel()
	nodes := []*depgraph.Node{
		{ID: "file:src/util.ts", Name: "util.ts", Type: depgraph.NodeSourceFile, Path: "src/util.ts"},
	}
	var edges []*depgraph.Edge
	for i := 0; i < 3; i++ {
		fileID := fmt.Sprintf("file:tests/t%d.test.ts", i)
		testCaseID := fmt.Sprintf("test:t%d:1:case", i)
		nodes = append(nodes,
			&depgraph.Node{ID: fileID, Name: fmt.Sprintf("t%d", i), Type: depgraph.NodeTestFile, Path: fmt.Sprintf("tests/t%d.test.ts", i)},
			&depgraph.Node{ID: testCaseID, Name: fmt.Sprintf("case_%d", i), Type: depgraph.NodeTest},
		)
		edges = append(edges,
			&depgraph.Edge{From: fileID, To: "file:src/util.ts", Type: depgraph.EdgeImportsModule, Confidence: 1.0, EvidenceType: depgraph.EvidenceStaticAnalysis},
			&depgraph.Edge{From: testCaseID, To: fileID, Type: depgraph.EdgeTestDefinedInFile, Confidence: 1.0, EvidenceType: depgraph.EvidenceStaticAnalysis},
		)
	}

	g := buildGraph(nodes, edges)

	d := &BlastRadiusHotspotDetector{}
	sigs := d.DetectWithGraph(&models.TestSuiteSnapshot{}, g)
	if len(sigs) != 0 {
		t.Fatalf("expected 0 signals for file with only 3 test dependents, got %d", len(sigs))
	}
}

func TestBlastRadiusHotspot_EmptyGraph(t *testing.T) {
	t.Parallel()
	g := buildGraph(nil, nil)
	d := &BlastRadiusHotspotDetector{}
	sigs := d.DetectWithGraph(&models.TestSuiteSnapshot{}, g)
	if len(sigs) != 0 {
		t.Fatalf("expected 0 signals for empty graph, got %d", len(sigs))
	}
}

// ===========================================================================
// FixtureFragilityHotspotDetector
// ===========================================================================

func TestFixtureFragility_DetectReturnsNil(t *testing.T) {
	t.Parallel()
	d := &FixtureFragilityHotspotDetector{}
	if got := d.Detect(&models.TestSuiteSnapshot{}); got != nil {
		t.Fatalf("Detect() should return nil, got %v", got)
	}
}

func TestFixtureFragility_HighDependencyFlagged(t *testing.T) {
	t.Parallel()
	nodes := []*depgraph.Node{
		{ID: "fixture:db-seed", Name: "db-seed", Type: depgraph.NodeFixture, Path: "fixtures/db-seed.ts"},
	}
	var edges []*depgraph.Edge
	for i := 0; i < 10; i++ {
		testID := fmt.Sprintf("test:t%d", i)
		path := fmt.Sprintf("tests/t%d.test.ts", i)
		nodes = append(nodes, &depgraph.Node{ID: testID, Name: fmt.Sprintf("t%d", i), Type: depgraph.NodeTestFile, Path: path})
		edges = append(edges, &depgraph.Edge{From: testID, To: "fixture:db-seed", Type: depgraph.EdgeTestUsesFixture, Confidence: 1.0, EvidenceType: depgraph.EvidenceStaticAnalysis})
	}

	g := buildGraph(nodes, edges)

	d := &FixtureFragilityHotspotDetector{}
	sigs := d.DetectWithGraph(&models.TestSuiteSnapshot{}, g)
	if len(sigs) != 1 {
		t.Fatalf("expected 1 signal for fixture with 10 dependents, got %d", len(sigs))
	}
	if sigs[0].Type != signals.SignalFixtureFragilityHotspot {
		t.Errorf("type = %q, want %q", sigs[0].Type, signals.SignalFixtureFragilityHotspot)
	}
	// 10 tests from 10 unique files → len(testFiles) > 5 → High severity
	if sigs[0].Severity != models.SeverityHigh {
		t.Errorf("severity = %q, want High for 10 tests across 10 files", sigs[0].Severity)
	}
}

func TestFixtureFragility_ManyFilesIsHigh(t *testing.T) {
	t.Parallel()
	nodes := []*depgraph.Node{
		{ID: "fixture:shared", Name: "shared", Type: depgraph.NodeFixture, Path: "fixtures/shared.ts"},
	}
	var edges []*depgraph.Edge
	for i := 0; i < 25; i++ {
		testID := fmt.Sprintf("test:t%d", i)
		path := fmt.Sprintf("tests/suite%d/t%d.test.ts", i, i) // unique directories = unique files
		nodes = append(nodes, &depgraph.Node{ID: testID, Name: fmt.Sprintf("t%d", i), Type: depgraph.NodeTestFile, Path: path})
		edges = append(edges, &depgraph.Edge{From: testID, To: "fixture:shared", Type: depgraph.EdgeTestUsesFixture, Confidence: 1.0, EvidenceType: depgraph.EvidenceStaticAnalysis})
	}

	g := buildGraph(nodes, edges)

	d := &FixtureFragilityHotspotDetector{}
	sigs := d.DetectWithGraph(&models.TestSuiteSnapshot{}, g)
	if len(sigs) != 1 {
		t.Fatalf("expected 1 signal, got %d", len(sigs))
	}
	if sigs[0].Severity != models.SeverityHigh {
		t.Errorf("severity = %q, want High for >20 tests across >5 files", sigs[0].Severity)
	}
}

func TestFixtureFragility_LowDependencyNotFlagged(t *testing.T) {
	t.Parallel()
	nodes := []*depgraph.Node{
		{ID: "fixture:small", Name: "small", Type: depgraph.NodeFixture, Path: "fixtures/small.ts"},
	}
	var edges []*depgraph.Edge
	for i := 0; i < 3; i++ {
		testID := fmt.Sprintf("test:t%d", i)
		nodes = append(nodes, &depgraph.Node{ID: testID, Name: fmt.Sprintf("t%d", i), Type: depgraph.NodeTestFile, Path: "tests/t.test.ts"})
		edges = append(edges, &depgraph.Edge{From: testID, To: "fixture:small", Type: depgraph.EdgeTestUsesFixture, Confidence: 1.0, EvidenceType: depgraph.EvidenceStaticAnalysis})
	}

	g := buildGraph(nodes, edges)

	d := &FixtureFragilityHotspotDetector{}
	sigs := d.DetectWithGraph(&models.TestSuiteSnapshot{}, g)
	if len(sigs) != 0 {
		t.Fatalf("expected 0 signals for fixture with <5 dependents, got %d", len(sigs))
	}
}

func TestFixtureFragility_EmptyGraph(t *testing.T) {
	t.Parallel()
	g := buildGraph(nil, nil)
	d := &FixtureFragilityHotspotDetector{}
	sigs := d.DetectWithGraph(&models.TestSuiteSnapshot{}, g)
	if len(sigs) != 0 {
		t.Fatalf("expected 0 signals for empty graph, got %d", len(sigs))
	}
}

// ===========================================================================
// Edge case tests: metadata validation, nil-safety, signal field completeness
// ===========================================================================

func TestUncoveredAISurface_EmptyNameUsesID(t *testing.T) {
	t.Parallel()
	g := buildGraph(
		[]*depgraph.Node{
			{ID: "prompt:anon", Name: "", Type: depgraph.NodePrompt, Path: "src/p.ts"},
		},
		nil,
	)

	d := &UncoveredAISurfaceDetector{}
	sigs := d.DetectWithGraph(&models.TestSuiteSnapshot{}, g)
	if len(sigs) != 1 {
		t.Fatalf("expected 1 signal, got %d", len(sigs))
	}
	if sigs[0].Location.Symbol != "prompt:anon" {
		t.Errorf("expected symbol to fall back to ID, got %q", sigs[0].Location.Symbol)
	}
}

func TestUncoveredAISurface_MetadataContainsRequiredKeys(t *testing.T) {
	t.Parallel()
	g := buildGraph(
		[]*depgraph.Node{
			{ID: "prompt:sys", Name: "sys", Type: depgraph.NodePrompt, Path: "p.ts"},
		},
		nil,
	)

	d := &UncoveredAISurfaceDetector{}
	sigs := d.DetectWithGraph(&models.TestSuiteSnapshot{}, g)
	if len(sigs) != 1 {
		t.Fatalf("expected 1 signal, got %d", len(sigs))
	}
	m := sigs[0].Metadata
	if _, ok := m["surfaceKind"]; !ok {
		t.Error("missing surfaceKind in metadata")
	}
	if _, ok := m["surfaceName"]; !ok {
		t.Error("missing surfaceName in metadata")
	}
	if _, ok := m["surfaceID"]; !ok {
		t.Error("missing surfaceID in metadata")
	}
}

func TestPhantomEvalScenario_MetadataContainsRequiredKeys(t *testing.T) {
	t.Parallel()
	g := buildGraph(
		[]*depgraph.Node{
			{ID: "scenario:s1", Name: "s1", Type: depgraph.NodeTestFile, Path: "eval/s1.test.ts"},
			{ID: "file:eval/s1.test.ts", Name: "s1.test.ts", Type: depgraph.NodeTestFile, Path: "eval/s1.test.ts"},
			{ID: "surface:x", Name: "x", Type: depgraph.NodePrompt, Path: "src/x.ts"},
		},
		nil,
	)

	snap := &models.TestSuiteSnapshot{
		Scenarios: []models.Scenario{
			{ScenarioID: "s1", Name: "s1", Path: "eval/s1.test.ts", CoveredSurfaceIDs: []string{"surface:x"}},
		},
	}

	d := &PhantomEvalScenarioDetector{}
	sigs := d.DetectWithGraph(snap, g)
	if len(sigs) != 1 {
		t.Fatalf("expected 1 signal, got %d", len(sigs))
	}
	m := sigs[0].Metadata
	if _, ok := m["scenarioName"]; !ok {
		t.Error("missing scenarioName in metadata")
	}
	if _, ok := m["claimedSurfaces"]; !ok {
		t.Error("missing claimedSurfaces in metadata")
	}
	if _, ok := m["unreachableSurfaces"]; !ok {
		t.Error("missing unreachableSurfaces in metadata")
	}
}

func TestUntestedPromptFlow_MetadataContainsFlowChain(t *testing.T) {
	t.Parallel()
	g := buildGraph(
		[]*depgraph.Node{
			{ID: "prompt:p", Name: "p", Type: depgraph.NodePrompt, Path: "src/p.ts"},
			{ID: "file:src/p.ts", Name: "p.ts", Type: depgraph.NodeSourceFile, Path: "src/p.ts"},
			{ID: "file:src/consumer.ts", Name: "consumer.ts", Type: depgraph.NodeSourceFile, Path: "src/consumer.ts"},
		},
		[]*depgraph.Edge{
			{From: "prompt:p", To: "file:src/p.ts", Type: depgraph.EdgeAIDefinedInFile, Confidence: 1.0, EvidenceType: depgraph.EvidenceStaticAnalysis},
			{From: "file:src/consumer.ts", To: "file:src/p.ts", Type: depgraph.EdgeSourceImportsSource, Confidence: 1.0, EvidenceType: depgraph.EvidenceStaticAnalysis},
		},
	)

	d := &UntestedPromptFlowDetector{}
	sigs := d.DetectWithGraph(&models.TestSuiteSnapshot{}, g)
	if len(sigs) != 1 {
		t.Fatalf("expected 1 signal, got %d", len(sigs))
	}
	m := sigs[0].Metadata
	if _, ok := m["promptName"]; !ok {
		t.Error("missing promptName in metadata")
	}
	if _, ok := m["flowLength"]; !ok {
		t.Error("missing flowLength in metadata")
	}
	chain, ok := m["flowChain"].([]string)
	if !ok {
		t.Error("missing or wrong type for flowChain in metadata")
	}
	if len(chain) < 2 {
		t.Errorf("expected flowChain length >= 2, got %d", len(chain))
	}
}

func TestCapabilityValidationGap_MissingMetadataKeyNotPanic(t *testing.T) {
	t.Parallel()
	// Test that a capability node with nil Metadata doesn't cause a panic.
	g := buildGraph(
		[]*depgraph.Node{
			{ID: "capability:rag", Name: "rag", Type: depgraph.NodeCapability},
			{ID: "scenario:s", Name: "s", Type: depgraph.NodeTestFile, Metadata: nil},
		},
		[]*depgraph.Edge{
			{From: "scenario:s", To: "capability:rag", Type: depgraph.EdgeScenarioValidatesCapability, Confidence: 0.9, EvidenceType: depgraph.EvidenceStaticAnalysis},
		},
	)

	d := &CapabilityValidationGapDetector{}
	// Should not panic — nil Metadata means executable key is missing, treated as not executable.
	sigs := d.DetectWithGraph(&models.TestSuiteSnapshot{}, g)
	if len(sigs) != 1 {
		t.Fatalf("expected 1 signal (non-executable scenario), got %d", len(sigs))
	}
	if sigs[0].Severity != models.SeverityMedium {
		t.Errorf("severity = %q, want Medium (scenario exists but not executable)", sigs[0].Severity)
	}
}

func TestCapabilityValidationGap_MetadataContainsRequiredKeys(t *testing.T) {
	t.Parallel()
	g := buildGraph(
		[]*depgraph.Node{
			{ID: "capability:tool", Name: "tool_use", Type: depgraph.NodeCapability},
		},
		nil,
	)

	d := &CapabilityValidationGapDetector{}
	sigs := d.DetectWithGraph(&models.TestSuiteSnapshot{}, g)
	if len(sigs) != 1 {
		t.Fatalf("expected 1 signal, got %d", len(sigs))
	}
	m := sigs[0].Metadata
	if _, ok := m["capabilityName"]; !ok {
		t.Error("missing capabilityName in metadata")
	}
	if _, ok := m["scenarioCount"]; !ok {
		t.Error("missing scenarioCount in metadata")
	}
	if _, ok := m["executableScenarios"]; !ok {
		t.Error("missing executableScenarios in metadata")
	}
}

func TestAssertionFreeImport_MetadataContainsImportDetails(t *testing.T) {
	t.Parallel()
	g := buildGraph(
		[]*depgraph.Node{
			{ID: "test_file:tests/a.test.ts", Name: "a.test.ts", Type: depgraph.NodeTestFile, Path: "tests/a.test.ts"},
			{ID: "file:src/a.ts", Name: "a.ts", Type: depgraph.NodeSourceFile, Path: "src/a.ts"},
		},
		[]*depgraph.Edge{
			{From: "test_file:tests/a.test.ts", To: "file:src/a.ts", Type: depgraph.EdgeImportsModule, Confidence: 1.0, EvidenceType: depgraph.EvidenceStaticAnalysis},
		},
	)
	snap := &models.TestSuiteSnapshot{
		TestFiles: []models.TestFile{
			{Path: "tests/a.test.ts", TestCount: 3, AssertionCount: 0},
		},
	}

	d := &AssertionFreeImportDetector{}
	sigs := d.DetectWithGraph(snap, g)
	if len(sigs) != 1 {
		t.Fatalf("expected 1 signal, got %d", len(sigs))
	}
	m := sigs[0].Metadata
	if _, ok := m["importedSources"]; !ok {
		t.Error("missing importedSources in metadata")
	}
	if _, ok := m["importCount"]; !ok {
		t.Error("missing importCount in metadata")
	}
	if _, ok := m["testCount"]; !ok {
		t.Error("missing testCount in metadata")
	}
}

func TestAllDetectors_SignalFieldsArePopulated(t *testing.T) {
	t.Parallel()
	// Build a graph that triggers one signal from each AI detector.
	g := buildGraph(
		[]*depgraph.Node{
			{ID: "prompt:uncovered", Name: "uncovered-prompt", Type: depgraph.NodePrompt, Path: "src/prompt.ts"},
			{ID: "capability:orphan", Name: "orphan-cap", Type: depgraph.NodeCapability},
		},
		nil,
	)

	detectors := []struct {
		name string
		d    interface {
			DetectWithGraph(*models.TestSuiteSnapshot, *depgraph.Graph) []models.Signal
		}
	}{
		{"UncoveredAISurface", &UncoveredAISurfaceDetector{}},
		{"CapabilityValidationGap", &CapabilityValidationGapDetector{}},
	}

	snap := &models.TestSuiteSnapshot{}
	for _, tc := range detectors {
		sigs := tc.d.DetectWithGraph(snap, g)
		for i, s := range sigs {
			if s.Type == "" {
				t.Errorf("%s signal[%d]: empty Type", tc.name, i)
			}
			if s.Category == "" {
				t.Errorf("%s signal[%d]: empty Category", tc.name, i)
			}
			if s.Severity == "" {
				t.Errorf("%s signal[%d]: empty Severity", tc.name, i)
			}
			if s.Confidence == 0 {
				t.Errorf("%s signal[%d]: zero Confidence", tc.name, i)
			}
			if s.Explanation == "" {
				t.Errorf("%s signal[%d]: empty Explanation", tc.name, i)
			}
			if s.SuggestedAction == "" {
				t.Errorf("%s signal[%d]: empty SuggestedAction", tc.name, i)
			}
			if s.EvidenceSource == "" {
				t.Errorf("%s signal[%d]: empty EvidenceSource", tc.name, i)
			}
		}
	}
}
