package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fixableRepo seeds a repo whose prompt has a one-edit typo of a real schema
// field, so the drift detector fires and the correct-side producer emits a
// validated fix.
func fixableRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "models.py"),
		"from pydantic import BaseModel\n\nclass UserProfile(BaseModel):\n    user_id: str\n")
	mustWrite(t, filepath.Join(root, "prompt.py"),
		"import openai\nfrom models import UserProfile\n\ndef build(user: UserProfile) -> str:\n    return f\"\"\"Hi {user.user_idx}.\"\"\"\n")
	return root
}

func TestRunFix_DryRunWritesNothing(t *testing.T) {
	root := fixableRepo(t)
	orig, _ := os.ReadFile(filepath.Join(root, "prompt.py"))

	out := captureStdout(t, func() {
		if err := runFix(root, false); err != nil {
			t.Fatalf("runFix dry-run: %v", err)
		}
	})

	if !strings.Contains(out, "VALIDATED FIX") {
		t.Errorf("dry-run should list a ready fix, got: %s", out)
	}
	if !strings.Contains(out, "nothing written") {
		t.Errorf("dry-run should say nothing was written, got: %s", out)
	}
	after, _ := os.ReadFile(filepath.Join(root, "prompt.py"))
	if string(orig) != string(after) {
		t.Error("dry-run must not modify the file")
	}
}

func TestRunFix_ApplyWritesTheFix(t *testing.T) {
	root := fixableRepo(t)

	out := captureStdout(t, func() {
		if err := runFix(root, true); err != nil {
			t.Fatalf("runFix --apply: %v", err)
		}
	})

	if !strings.Contains(out, "applied 1 fix") {
		t.Errorf("apply should report one fix applied, got: %s", out)
	}
	after, _ := os.ReadFile(filepath.Join(root, "prompt.py"))
	if strings.Contains(string(after), "user_idx") {
		t.Error("apply should have rewritten user_idx")
	}
	if !strings.Contains(string(after), "user_id}") {
		t.Errorf("apply should have written the corrected field, got:\n%s", after)
	}
}

func TestRunFix_NothingToFix(t *testing.T) {
	// A consistent repo (no drift) has no validated fixes.
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "models.py"),
		"from pydantic import BaseModel\n\nclass UserProfile(BaseModel):\n    user_id: str\n")
	mustWrite(t, filepath.Join(root, "prompt.py"),
		"import openai\nfrom models import UserProfile\n\ndef build(user: UserProfile) -> str:\n    return f\"\"\"Hi {user.user_id}.\"\"\"\n")

	out := captureStdout(t, func() {
		if err := runFix(root, false); err != nil {
			t.Fatalf("runFix: %v", err)
		}
	})
	if !strings.Contains(out, "nothing to fix") {
		t.Errorf("expected 'nothing to fix' on a clean repo, got: %s", out)
	}
}

// TestRunFix_ApplyIsIdempotent confirms a second apply is a no-op (the fix
// cleared the finding; nothing left to write).
func TestRunFix_ApplyIsIdempotent(t *testing.T) {
	root := fixableRepo(t)
	if err := runFix(root, true); err != nil {
		t.Fatalf("first apply: %v", err)
	}
	out := captureStdout(t, func() {
		if err := runFix(root, true); err != nil {
			t.Fatalf("second apply: %v", err)
		}
	})
	if !strings.Contains(out, "nothing to fix") {
		t.Errorf("second apply should be a no-op, got: %s", out)
	}
}
