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

// TestEvaluateAIRunDecision_GovernanceBlock locks the precedence
// rule: an AI policy violation (governance signal with rule=block_*)
// triggers BLOCK even when no AI severity is critical. Audit-named
// gap (ai_execution_gating.E1): more decision-logic test coverage.
func TestEvaluateAIRunDecision_GovernanceBlock(t *testing.T) {
	t.Parallel()
	snap := &models.TestSuiteSnapshot{
		Signals: []models.Signal{
			// No critical AI signal — only a governance block.
			{Category: models.CategoryAI, Severity: models.SeverityMedium},
			{
				Category: models.CategoryGovernance,
				Severity: models.SeverityHigh,
				Metadata: map[string]any{"rule": "block_on_safety_failure"},
			},
		},
	}
	result := &engine.PipelineResult{Snapshot: snap}

	d := evaluateAIRunDecision(snap, result)
	if d.Action != actionBlock {
		t.Errorf("governance block_on_* should trigger BLOCK; got action=%q", d.Action)
	}
	if d.Blocked != 1 {
		t.Errorf("blocked count = %d, want 1", d.Blocked)
	}
	if !contains(d.Reason, "policy violation") {
		t.Errorf("reason = %q, want it to mention 'policy violation'", d.Reason)
	}
}

// TestEvaluateAIRunDecision_GovernanceWarn_NotBlock locks the
// distinction between block_on_* (BLOCK) and warn_on_* (no escalation).
// Adopters who set warn_on_cost_regression shouldn't have CI fail.
func TestEvaluateAIRunDecision_GovernanceWarn_NotBlock(t *testing.T) {
	t.Parallel()
	snap := &models.TestSuiteSnapshot{
		Signals: []models.Signal{
			{
				Category: models.CategoryGovernance,
				Severity: models.SeverityMedium,
				Metadata: map[string]any{"rule": "warn_on_cost_regression"},
			},
		},
	}
	result := &engine.PipelineResult{Snapshot: snap}

	d := evaluateAIRunDecision(snap, result)
	if d.Action == actionBlock {
		t.Errorf("warn_on_* governance signal should NOT trigger BLOCK; got action=%q", d.Action)
	}
	if d.Blocked != 0 {
		t.Errorf("blocked count = %d, want 0", d.Blocked)
	}
}

// TestEvaluateAIRunDecision_BlockingSignalTypes locks the special-
// case rule string "blocking_signal_types" — explicit per-signal
// allowlist. Treated like block_on_* for the gate decision.
func TestEvaluateAIRunDecision_BlockingSignalTypes(t *testing.T) {
	t.Parallel()
	snap := &models.TestSuiteSnapshot{
		Signals: []models.Signal{
			{
				Category: models.CategoryGovernance,
				Severity: models.SeverityHigh,
				Metadata: map[string]any{"rule": "blocking_signal_types"},
			},
		},
	}
	result := &engine.PipelineResult{Snapshot: snap}

	d := evaluateAIRunDecision(snap, result)
	if d.Action != actionBlock {
		t.Errorf("blocking_signal_types governance signal should trigger BLOCK; got action=%q", d.Action)
	}
}

// TestEvaluateAIRunDecision_CriticalAndPolicyTogether verifies the
// reason string lists both contributors when they fire together.
// Adopters need to see both numbers: "3 critical signal(s),
// 2 policy violation(s)".
func TestEvaluateAIRunDecision_CriticalAndPolicyTogether(t *testing.T) {
	t.Parallel()
	snap := &models.TestSuiteSnapshot{
		Signals: []models.Signal{
			{Category: models.CategoryAI, Severity: models.SeverityCritical},
			{Category: models.CategoryAI, Severity: models.SeverityCritical},
			{Category: models.CategoryAI, Severity: models.SeverityCritical},
			{
				Category: models.CategoryGovernance,
				Severity: models.SeverityHigh,
				Metadata: map[string]any{"rule": "block_on_safety_failure"},
			},
			{
				Category: models.CategoryGovernance,
				Severity: models.SeverityHigh,
				Metadata: map[string]any{"rule": "block_on_accuracy_regression"},
			},
		},
	}
	result := &engine.PipelineResult{Snapshot: snap}

	d := evaluateAIRunDecision(snap, result)
	if d.Action != actionBlock {
		t.Errorf("action = %q, want BLOCK", d.Action)
	}
	if !contains(d.Reason, "3 critical") {
		t.Errorf("reason should name critical count; got %q", d.Reason)
	}
	if !contains(d.Reason, "2 policy") {
		t.Errorf("reason should name policy-violation count; got %q", d.Reason)
	}
}

// TestEvaluateAIRunDecision_GovernanceMetadataMissing covers the
// edge case where a governance signal has no metadata — the
// decision logic should ignore it (not panic, not block).
func TestEvaluateAIRunDecision_GovernanceMetadataMissing(t *testing.T) {
	t.Parallel()
	snap := &models.TestSuiteSnapshot{
		Signals: []models.Signal{
			// No Metadata map at all.
			{Category: models.CategoryGovernance, Severity: models.SeverityHigh},
		},
	}
	result := &engine.PipelineResult{Snapshot: snap}

	// Should not panic.
	d := evaluateAIRunDecision(snap, result)
	if d.Blocked != 0 {
		t.Errorf("governance signal with no metadata should not contribute to Blocked; got %d", d.Blocked)
	}
}

// TestEvaluateAIRunDecision_GovernanceMetadataNonStringRule covers
// the edge case where Metadata["rule"] is set but isn't a string —
// the type assertion should fail safely without panic.
func TestEvaluateAIRunDecision_GovernanceMetadataNonStringRule(t *testing.T) {
	t.Parallel()
	snap := &models.TestSuiteSnapshot{
		Signals: []models.Signal{
			{
				Category: models.CategoryGovernance,
				Severity: models.SeverityHigh,
				Metadata: map[string]any{"rule": 42},
			},
		},
	}
	result := &engine.PipelineResult{Snapshot: snap}

	d := evaluateAIRunDecision(snap, result)
	if d.Blocked != 0 {
		t.Errorf("non-string rule metadata should not contribute to Blocked; got %d", d.Blocked)
	}
}

// TestEvaluateAIRunDecision_OnlyHighSeverity_DoesNotBlock locks the
// boundary: high-severity AI signal warns but does not block. The
// `--fail-on high` gate is the user-facing way to lift high to
// blocking; the AI run decision logic itself stays at warn for
// high-severity.
func TestEvaluateAIRunDecision_OnlyHighSeverity_DoesNotBlock(t *testing.T) {
	t.Parallel()
	snap := &models.TestSuiteSnapshot{
		Signals: []models.Signal{
			{Category: models.CategoryAI, Severity: models.SeverityHigh},
			{Category: models.CategoryAI, Severity: models.SeverityHigh},
		},
	}
	result := &engine.PipelineResult{Snapshot: snap}

	d := evaluateAIRunDecision(snap, result)
	if d.Action != actionWarn {
		t.Errorf("two high-severity AI signals: action = %q, want WARN", d.Action)
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
