package evaladapter

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRegistry_All(t *testing.T) {
	t.Parallel()
	adapters := All()
	if len(adapters) == 0 {
		t.Fatal("expected at least one adapter registered")
	}
	// Promptfoo must be present at 0.2.0.
	found := false
	for _, a := range adapters {
		if a.Name() == FrameworkPromptfoo {
			found = true
		}
	}
	if !found {
		t.Error("promptfoo adapter missing from registry")
	}
}

func TestRegistry_For(t *testing.T) {
	t.Parallel()
	if a := For(FrameworkPromptfoo); a == nil {
		t.Error("For(promptfoo) returned nil")
	}
	if a := For(Framework("nonsense")); a != nil {
		t.Errorf("For(nonsense) should return nil, got %v", a)
	}
}

func TestRegistry_AutoIngest_PromptfooDispatch(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "results.json")
	_ = os.WriteFile(path, []byte(`{
  "evalId": "x",
  "results": {
    "version": 3,
    "timestamp": "2026-05-01T00:00:00Z",
    "results": [{"id": "a", "success": true, "score": 1.0, "testCase": {"description": "a"}}],
    "stats": {"successes": 1, "failures": 0}
  }
}`), 0o644)

	run, err := AutoIngest(path)
	if err != nil {
		t.Fatalf("AutoIngest: %v", err)
	}
	if run.Framework != FrameworkPromptfoo {
		t.Errorf("framework = %q, want promptfoo", run.Framework)
	}
	if len(run.Cases) != 1 {
		t.Errorf("cases = %d, want 1", len(run.Cases))
	}
}

func TestRegistry_AutoIngest_NoMatch(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "stranger.json")
	_ = os.WriteFile(path, []byte(`{"hello": "world"}`), 0o644)
	if _, err := AutoIngest(path); err == nil {
		t.Error("expected error when no adapter matches")
	}
}
