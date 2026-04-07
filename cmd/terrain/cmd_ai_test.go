package main

import (
	"testing"

	"github.com/pmclSF/terrain/internal/aidetect"
	"github.com/pmclSF/terrain/internal/engine"
	"github.com/pmclSF/terrain/internal/models"
)

// ---------------------------------------------------------------------------
// evaluateAIRunDecision
// ---------------------------------------------------------------------------

func TestEvaluateAIRunDecision_NoSignals_Pass(t *testing.T) {
	t.Parallel()
	snap := &models.TestSuiteSnapshot{}
	result := &engine.PipelineResult{Snapshot: snap}

	d := evaluateAIRunDecision(snap, result)
	if d.Action != actionPass {
		t.Errorf("action = %q, want %q", d.Action, actionPass)
	}
	if d.Signals != 0 {
		t.Errorf("signals = %d, want 0", d.Signals)
	}
}

func TestEvaluateAIRunDecision_CriticalSignal_Block(t *testing.T) {
	t.Parallel()
	snap := &models.TestSuiteSnapshot{
		Signals: []models.Signal{
			{Category: models.CategoryAI, Severity: models.SeverityCritical},
		},
	}
	result := &engine.PipelineResult{Snapshot: snap}

	d := evaluateAIRunDecision(snap, result)
	if d.Action != actionBlock {
		t.Errorf("action = %q, want %q", d.Action, actionBlock)
	}
	if d.Signals != 1 {
		t.Errorf("signals = %d, want 1", d.Signals)
	}
}

func TestEvaluateAIRunDecision_HighSignal_Warn(t *testing.T) {
	t.Parallel()
	snap := &models.TestSuiteSnapshot{
		Signals: []models.Signal{
			{Category: models.CategoryAI, Severity: models.SeverityHigh},
		},
	}
	result := &engine.PipelineResult{Snapshot: snap}

	d := evaluateAIRunDecision(snap, result)
	if d.Action != actionWarn {
		t.Errorf("action = %q, want %q", d.Action, actionWarn)
	}
}

func TestEvaluateAIRunDecision_MediumSignal_Warn(t *testing.T) {
	t.Parallel()
	snap := &models.TestSuiteSnapshot{
		Signals: []models.Signal{
			{Category: models.CategoryAI, Severity: models.SeverityMedium},
		},
	}
	result := &engine.PipelineResult{Snapshot: snap}

	d := evaluateAIRunDecision(snap, result)
	if d.Action != actionWarn {
		t.Errorf("action = %q, want %q", d.Action, actionWarn)
	}
}

func TestEvaluateAIRunDecision_LowSignal_Pass(t *testing.T) {
	t.Parallel()
	snap := &models.TestSuiteSnapshot{
		Signals: []models.Signal{
			{Category: models.CategoryAI, Severity: models.SeverityLow},
		},
	}
	result := &engine.PipelineResult{Snapshot: snap}

	d := evaluateAIRunDecision(snap, result)
	if d.Action != actionPass {
		t.Errorf("action = %q, want %q (low severity should pass)", d.Action, actionPass)
	}
}

func TestEvaluateAIRunDecision_NonAISignals_Ignored(t *testing.T) {
	t.Parallel()
	snap := &models.TestSuiteSnapshot{
		Signals: []models.Signal{
			{Category: models.CategoryStructure, Severity: models.SeverityCritical},
			{Category: models.CategoryHealth, Severity: models.SeverityHigh},
		},
	}
	result := &engine.PipelineResult{Snapshot: snap}

	d := evaluateAIRunDecision(snap, result)
	if d.Action != actionPass {
		t.Errorf("action = %q, want %q (non-AI signals should be ignored)", d.Action, actionPass)
	}
	if d.Signals != 0 {
		t.Errorf("signals = %d, want 0 (non-AI signals should not be counted)", d.Signals)
	}
}

func TestEvaluateAIRunDecision_GovernanceBlockingRule_Block(t *testing.T) {
	t.Parallel()
	snap := &models.TestSuiteSnapshot{
		Signals: []models.Signal{
			{Category: models.CategoryGovernance, Metadata: map[string]any{"rule": "block_on_regression"}},
		},
	}
	result := &engine.PipelineResult{Snapshot: snap}

	d := evaluateAIRunDecision(snap, result)
	if d.Action != actionBlock {
		t.Errorf("action = %q, want %q", d.Action, actionBlock)
	}
	if d.Blocked != 1 {
		t.Errorf("blocked = %d, want 1", d.Blocked)
	}
}

func TestEvaluateAIRunDecision_MixedSignals_WorstWins(t *testing.T) {
	t.Parallel()
	snap := &models.TestSuiteSnapshot{
		Signals: []models.Signal{
			{Category: models.CategoryAI, Severity: models.SeverityLow},
			{Category: models.CategoryAI, Severity: models.SeverityMedium},
			{Category: models.CategoryAI, Severity: models.SeverityCritical},
		},
	}
	result := &engine.PipelineResult{Snapshot: snap}

	d := evaluateAIRunDecision(snap, result)
	if d.Action != actionBlock {
		t.Errorf("action = %q, want %q (critical should dominate)", d.Action, actionBlock)
	}
	if d.Signals != 3 {
		t.Errorf("signals = %d, want 3", d.Signals)
	}
}

// ---------------------------------------------------------------------------
// buildEvalCommand
// ---------------------------------------------------------------------------

func TestBuildEvalCommand_Promptfoo(t *testing.T) {
	t.Parallel()
	det := &aidetect.DetectResult{
		Frameworks: []aidetect.Framework{
			{Name: "promptfoo", ConfigFile: "promptfooconfig.yaml", Confidence: 0.95},
		},
	}
	snap := &models.TestSuiteSnapshot{}

	cmd := buildEvalCommand("promptfoo", det, nil, snap)
	if len(cmd) < 3 {
		t.Fatalf("expected at least 3 args, got %d: %v", len(cmd), cmd)
	}
	if cmd[0] != "npx" || cmd[1] != "promptfoo" || cmd[2] != "eval" {
		t.Errorf("cmd = %v, want [npx promptfoo eval ...]", cmd)
	}
	// Should include config flag.
	if len(cmd) < 5 || cmd[3] != "-c" || cmd[4] != "promptfooconfig.yaml" {
		t.Errorf("expected -c flag with config, got %v", cmd)
	}
}

func TestBuildEvalCommand_Deepeval(t *testing.T) {
	t.Parallel()
	det := &aidetect.DetectResult{}
	snap := &models.TestSuiteSnapshot{}

	cmd := buildEvalCommand("deepeval", det, nil, snap)
	if len(cmd) != 3 || cmd[0] != "deepeval" {
		t.Errorf("cmd = %v, want [deepeval test run]", cmd)
	}
}

func TestBuildEvalCommand_Ragas(t *testing.T) {
	t.Parallel()
	det := &aidetect.DetectResult{}
	snap := &models.TestSuiteSnapshot{}

	cmd := buildEvalCommand("ragas", det, nil, snap)
	if len(cmd) != 4 || cmd[0] != "python" {
		t.Errorf("cmd = %v, want [python -m ragas evaluate]", cmd)
	}
}

func TestBuildEvalCommand_Unknown_NoScenarios_Nil(t *testing.T) {
	t.Parallel()
	det := &aidetect.DetectResult{}
	snap := &models.TestSuiteSnapshot{}

	cmd := buildEvalCommand("unknown", det, nil, snap)
	if cmd != nil {
		t.Errorf("expected nil for unknown framework with no scenarios, got %v", cmd)
	}
}

func TestBuildEvalCommand_Unknown_UsesVitestDefault(t *testing.T) {
	t.Parallel()
	det := &aidetect.DetectResult{}
	snap := &models.TestSuiteSnapshot{}
	scenarios := []aiRunScenario{
		{Path: "tests/eval/accuracy.test.ts"},
	}

	cmd := buildEvalCommand("unknown", det, scenarios, snap)
	if len(cmd) == 0 {
		t.Fatal("expected command, got nil")
	}
	if cmd[0] != "npx" || cmd[1] != "vitest" {
		t.Errorf("expected vitest default, got %v", cmd[:2])
	}
}

func TestBuildEvalCommand_Unknown_DetectsPytest(t *testing.T) {
	t.Parallel()
	det := &aidetect.DetectResult{}
	snap := &models.TestSuiteSnapshot{
		TestFiles: []models.TestFile{
			{Path: "tests/test_eval.py", Framework: "pytest"},
		},
	}
	scenarios := []aiRunScenario{
		{Path: "tests/test_eval.py"},
	}

	cmd := buildEvalCommand("unknown", det, scenarios, snap)
	if len(cmd) == 0 {
		t.Fatal("expected command, got nil")
	}
	if cmd[0] != "pytest" {
		t.Errorf("expected pytest runner, got %v", cmd[0])
	}
}

func TestBuildEvalCommand_Unknown_DetectsJest(t *testing.T) {
	t.Parallel()
	det := &aidetect.DetectResult{}
	snap := &models.TestSuiteSnapshot{
		TestFiles: []models.TestFile{
			{Path: "tests/eval.test.js", Framework: "jest"},
		},
	}
	scenarios := []aiRunScenario{
		{Path: "tests/eval.test.js"},
	}

	cmd := buildEvalCommand("unknown", det, scenarios, snap)
	if len(cmd) == 0 {
		t.Fatal("expected command, got nil")
	}
	if cmd[0] != "npx" || cmd[1] != "jest" {
		t.Errorf("expected jest runner, got %v", cmd[:2])
	}
}

// ---------------------------------------------------------------------------
// isEvalPath
// ---------------------------------------------------------------------------

func TestIsEvalPath_Positive(t *testing.T) {
	t.Parallel()
	cases := []string{
		"tests/eval/accuracy.test.ts",
		"tests/evals/safety.test.py",
		"evaluations/benchmark.test.js",
		"__evals__/test_harm.py",
		"benchmarks/latency_test.go",
		"src/tests/eval/test_factual.py",
	}
	for _, path := range cases {
		if !isEvalPath(path) {
			t.Errorf("isEvalPath(%q) = false, want true", path)
		}
	}
}

func TestIsEvalPath_Negative(t *testing.T) {
	t.Parallel()
	cases := []string{
		"src/evaluation.ts",
		"tests/unit/auth.test.ts",
		"lib/benchmark.ts",
		"tests/integration/api.test.js",
	}
	for _, path := range cases {
		if isEvalPath(path) {
			t.Errorf("isEvalPath(%q) = true, want false", path)
		}
	}
}

// ---------------------------------------------------------------------------
// cliExitError
// ---------------------------------------------------------------------------

func TestCliExitError_CarriesCode(t *testing.T) {
	t.Parallel()
	err := cliExitError{code: 42, message: "blocked"}
	if exitCodeForCLIError(err) != 42 {
		t.Errorf("exitCodeForCLIError = %d, want 42", exitCodeForCLIError(err))
	}
	if err.Error() != "blocked" {
		t.Errorf("Error() = %q, want %q", err.Error(), "blocked")
	}
}
