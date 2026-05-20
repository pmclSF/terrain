package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRunDiscover_EmptyRepo checks the no-args discovery report stays friendly
// on an empty directory and routes to the right next-step suggestion.
func TestRunDiscover_EmptyRepo(t *testing.T) {
	tmp := t.TempDir()

	out := captureStdout(t, func() {
		if err := runDiscover(tmp); err != nil {
			t.Fatalf("runDiscover: %v", err)
		}
	})

	if !strings.Contains(out, "Terrain — discovery report") {
		t.Errorf("expected header in output, got: %s", out)
	}
	if !strings.Contains(out, "Nothing AI-specific detected") {
		t.Errorf("expected empty-repo guidance, got: %s", out)
	}
	if !strings.Contains(out, "terrain analyze") {
		t.Errorf("expected next-step pointer at `terrain analyze`, got: %s", out)
	}
}

// TestRunDiscover_AIRepo verifies the report surfaces AI surfaces when present
// and routes to the AI-first next-step suggestions.
func TestRunDiscover_AIRepo(t *testing.T) {
	tmp := t.TempDir()

	// Seed a minimal AI-shaped layout: a prompt file + an eval config.
	mustWrite(t, filepath.Join(tmp, "prompts", "answer.txt"),
		"You are a helpful assistant. {{user_input}}")
	mustWrite(t, filepath.Join(tmp, "evals", "promptfoo.yaml"),
		"prompts:\n  - prompts/answer.txt\nproviders:\n  - openai:gpt-4\n")
	mustWrite(t, filepath.Join(tmp, "main.py"),
		"import openai\nopenai.ChatCompletion.create(model='gpt-4')\n")

	out := captureStdout(t, func() {
		if err := runDiscover(tmp); err != nil {
			t.Fatalf("runDiscover: %v", err)
		}
	})

	if !strings.Contains(out, "terrain ai findings") {
		t.Errorf("expected ai-findings next-step pointer when AI surfaces present, got: %s", out)
	}
}

// TestRunDiscover_TestRepo verifies the report doesn't suggest `ai findings`
// when there are tests but no AI surfaces.
func TestRunDiscover_TestRepo(t *testing.T) {
	tmp := t.TempDir()

	mustWrite(t, filepath.Join(tmp, "package.json"),
		`{"name": "x", "scripts": {"test": "jest"}}`)
	mustWrite(t, filepath.Join(tmp, "src", "thing.test.js"),
		"test('thing', () => { expect(1).toBe(1) })")

	out := captureStdout(t, func() {
		if err := runDiscover(tmp); err != nil {
			t.Fatalf("runDiscover: %v", err)
		}
	})

	if !strings.Contains(out, "terrain insights") {
		t.Errorf("expected non-AI next-step routing to `terrain insights`, got: %s", out)
	}
	if strings.Contains(out, "terrain ai findings") {
		t.Errorf("did not expect AI next-step on non-AI repo, got: %s", out)
	}
}

// TestIsFixturePath spot-checks the heuristic that prevents the report from
// surfacing borrowed test-fixture content as if it were the adopter's code.
func TestIsFixturePath(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		{"benchmarks/cypress-playwright/cal.com/foo.ts", true},
		{"tests/fixtures/ai-eval-suite/foo.py", true},
		{"vendor/some-pkg/schemas/types.ts", true},
		{"src/app/main.py", false},
		{"schemas/finding.v1.json", false},
		{"docs/schema/analysis.schema.json", false},
	}
	for _, tc := range cases {
		if got := isFixturePath(tc.path); got != tc.want {
			t.Errorf("isFixturePath(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	defer func() { os.Stdout = old }()

	done := make(chan struct{})
	var buf bytes.Buffer
	go func() {
		_, _ = io.Copy(&buf, r)
		close(done)
	}()

	fn()
	_ = w.Close()
	<-done
	return buf.String()
}

func mustWrite(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
