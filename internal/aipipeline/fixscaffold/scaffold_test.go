package fixscaffold

import (
	"strings"
	"testing"
)

func TestRegistryGenerate_MissingEval_YAML(t *testing.T) {
	t.Parallel()
	r := NewRegistry()
	s := r.Generate("ai.surface.missing_eval", "app/prompts/qa.txt", "python")
	if s == nil {
		t.Fatalf("expected scaffold; got nil")
	}
	if s.Language != "yaml" {
		t.Errorf("expected yaml; got %q", s.Language)
	}
	if !strings.HasPrefix(s.TargetPath, "evals/promptfoo/") {
		t.Errorf("expected evals/promptfoo path prefix; got %q", s.TargetPath)
	}
	for _, want := range []string{"description:", "prompts:", "providers:", "tests:", "app/prompts/qa.txt"} {
		if !strings.Contains(s.Body, want) {
			t.Errorf("body missing %q; full body:\n%s", want, s.Body)
		}
	}
}

func TestRegistryGenerate_DeepEvalPython(t *testing.T) {
	t.Parallel()
	r := NewRegistry()
	s := r.Generate("ai.surface.missing_eval.deepeval", "app/agent/runner.py", "python")
	if s == nil {
		t.Fatalf("expected scaffold; got nil")
	}
	if s.Language != "python" {
		t.Errorf("expected python; got %q", s.Language)
	}
	for _, want := range []string{"deepeval", "LLMTestCase", "FaithfulnessMetric"} {
		if !strings.Contains(s.Body, want) {
			t.Errorf("body missing %q", want)
		}
	}
}

func TestRegistryGenerate_TrackerPython(t *testing.T) {
	t.Parallel()
	r := NewRegistry()
	s := r.Generate("ai.train.missing_tracker", "scripts/train.py", "python")
	if s == nil {
		t.Fatalf("expected scaffold; got nil")
	}
	if !strings.Contains(s.Body, "mlflow") {
		t.Errorf("expected mlflow snippet for python tracker scaffold")
	}
}

func TestRegistryGenerate_TrackerNode(t *testing.T) {
	t.Parallel()
	r := NewRegistry()
	s := r.Generate("ai.train.missing_tracker", "src/train.ts", "typescript")
	if s == nil {
		t.Fatalf("expected scaffold; got nil")
	}
	if !strings.Contains(s.Body, "@wandb/sdk") && !strings.Contains(s.Body, "wandb") {
		t.Errorf("expected wandb snippet for node tracker scaffold; got\n%s", s.Body)
	}
}

func TestRegistryGenerate_TrackerGeneric(t *testing.T) {
	t.Parallel()
	r := NewRegistry()
	s := r.Generate("ai.train.missing_tracker", "src/train.rb", "ruby")
	if s == nil {
		t.Fatalf("expected scaffold even for unsupported language; got nil")
	}
	if !strings.Contains(s.Body, "Terrain detected") {
		t.Errorf("expected generic guidance preamble")
	}
}

func TestRegistryGenerate_UnknownRule(t *testing.T) {
	t.Parallel()
	r := NewRegistry()
	s := r.Generate("not.a.real.rule", "x.py", "python")
	if s != nil {
		t.Errorf("expected nil for unknown rule; got %+v", s)
	}
}

func TestRegistryAdapter_SatisfiesPipelineInterface(t *testing.T) {
	t.Parallel()
	a := NewRegistryAdapter(NewRegistry())
	body, target, desc := a.GenerateScaffold("ai.surface.missing_eval", "x.py", "python")
	if body == "" || target == "" || desc == "" {
		t.Errorf("adapter should return non-empty scaffold for known rule")
	}
}

func TestRegistryAdapter_NilSafety(t *testing.T) {
	t.Parallel()
	var a *RegistryAdapter
	body, target, desc := a.GenerateScaffold("ai.surface.missing_eval", "x.py", "python")
	if body != "" || target != "" || desc != "" {
		t.Errorf("nil adapter should return empty strings safely")
	}
}
