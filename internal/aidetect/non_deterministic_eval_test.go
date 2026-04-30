package aidetect

import (
	"testing"

	"github.com/pmclSF/terrain/internal/models"
	"github.com/pmclSF/terrain/internal/signals"
)

func TestNonDeterministicEval_FlagsTemperatureNonZero(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	rel := writeFile(t, root, "evals/agent.yaml", `
provider:
  name: openai
  model: gpt-4-0613
  temperature: 0.7
`)
	snap := &models.TestSuiteSnapshot{
		TestFiles: []models.TestFile{{Path: rel}},
	}
	got := (&NonDeterministicEvalDetector{Root: root}).Detect(snap)
	if len(got) != 1 {
		t.Fatalf("got %d signals, want 1", len(got))
	}
	if got[0].Type != signals.SignalAINonDeterministicEval {
		t.Errorf("type = %q", got[0].Type)
	}
	if got[0].Severity != models.SeverityMedium {
		t.Errorf("severity = %q", got[0].Severity)
	}
}

func TestNonDeterministicEval_FlagsMissingTemperature(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	rel := writeFile(t, root, "promptfoo/eval.yaml", `
providers:
  - id: anthropic
    config:
      model: claude-3-opus-20240229
`)
	snap := &models.TestSuiteSnapshot{
		TestFiles: []models.TestFile{{Path: rel}},
	}
	got := (&NonDeterministicEvalDetector{Root: root}).Detect(snap)
	if len(got) != 1 {
		t.Fatalf("got %d signals, want 1 (missing temperature)", len(got))
	}
}

func TestNonDeterministicEval_PassesTemperatureZero(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	rel := writeFile(t, root, "evals/agent.yaml", `
provider:
  name: openai
  model: gpt-4-0613
  temperature: 0
  seed: 42
`)
	snap := &models.TestSuiteSnapshot{
		TestFiles: []models.TestFile{{Path: rel}},
	}
	got := (&NonDeterministicEvalDetector{Root: root}).Detect(snap)
	if len(got) != 0 {
		t.Errorf("temperature=0 should not fire, got %d signals", len(got))
	}
}

func TestNonDeterministicEval_IgnoresUnrelatedYAML(t *testing.T) {
	t.Parallel()

	// CI workflow YAML — has nothing to do with evals; detector should
	// not fire even though it might lack `temperature`.
	root := t.TempDir()
	rel := writeFile(t, root, ".github/workflows/ci.yml", `
name: CI
on: [push]
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v6
`)
	snap := &models.TestSuiteSnapshot{
		TestFiles: []models.TestFile{{Path: rel}},
	}
	got := (&NonDeterministicEvalDetector{Root: root}).Detect(snap)
	if len(got) != 0 {
		t.Errorf("non-eval YAML should not fire, got %d signals", len(got))
	}
}

func TestNonDeterministicEval_IgnoresNonAILYAML(t *testing.T) {
	t.Parallel()

	// File doesn't look like an eval/agent/prompt config — out of scope.
	root := t.TempDir()
	rel := writeFile(t, root, "config/database.yaml", `
host: localhost
port: 5432
`)
	snap := &models.TestSuiteSnapshot{
		TestFiles: []models.TestFile{{Path: rel}},
	}
	got := (&NonDeterministicEvalDetector{Root: root}).Detect(snap)
	if len(got) != 0 {
		t.Errorf("non-eval-shaped path should not fire, got %d signals", len(got))
	}
}
