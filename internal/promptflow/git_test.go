package promptflow

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// gitInit sets up a minimal repo in dir with one commit on `main`.
func gitInit(t *testing.T, dir string) {
	t.Helper()
	for _, args := range [][]string{
		{"init", "-q", "-b", "main"},
		{"config", "user.email", "t@example.com"},
		{"config", "user.name", "Test"},
		{"config", "commit.gpgsign", "false"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
}

func gitCommit(t *testing.T, dir, msg string) {
	t.Helper()
	cmd := exec.Command("git", "add", "-A")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git add: %v\n%s", err, out)
	}
	cmd = exec.Command("git", "commit", "-q", "--allow-empty", "-m", msg)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v\n%s", err, out)
	}
}

func TestDiscoverFromGit_PullsBaseSchema(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := t.TempDir()
	gitInit(t, dir)
	// Commit a schema on main with field "user_id".
	mustWrite(t, filepath.Join(dir, "schemas", "user.json"),
		`{"type": "object", "properties": {"user_id": {"type": "string"}}}`)
	gitCommit(t, dir, "init schema with user_id")

	// Now modify the schema (rename user_id → userId).
	if err := os.WriteFile(filepath.Join(dir, "schemas", "user.json"),
		[]byte(`{"type": "object", "properties": {"userId": {"type": "string"}}}`), 0o644); err != nil {
		t.Fatalf("write modified schema: %v", err)
	}

	after, before, err := DiscoverFromGit(context.Background(), dir, "main")
	if err != nil {
		t.Fatalf("DiscoverFromGit error: %v", err)
	}
	if len(after.Schemas) != 1 {
		t.Fatalf("expected 1 after-schema, got %d", len(after.Schemas))
	}
	body, ok := before["schemas/user.json"]
	if !ok {
		t.Fatalf("expected before map to contain schemas/user.json, got keys %v", keys(before))
	}
	if !contains(body, "user_id") {
		t.Errorf("before body should contain user_id, got %q", string(body))
	}
}

func TestDiscoverFromGit_InvalidBaseRefReturnsError(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := t.TempDir()
	gitInit(t, dir)
	gitCommit(t, dir, "init")
	_, _, err := DiscoverFromGit(context.Background(), dir, "definitely-not-a-real-ref-1234")
	if err == nil {
		t.Fatalf("expected error for invalid base-ref, got nil")
	}
	if !contains([]byte(err.Error()), "definitely-not-a-real-ref-1234") {
		t.Errorf("error should mention the bad ref by name: %v", err)
	}
}

func TestDiscoverFromGit_EmptyBaseRefReturnsError(t *testing.T) {
	dir := t.TempDir()
	_, _, err := DiscoverFromGit(context.Background(), dir, "")
	if err == nil {
		t.Errorf("expected error for empty base-ref, got nil")
	}
}

func TestDiscoverFromGit_RejectsDashPrefixedBaseRef(t *testing.T) {
	dir := t.TempDir()
	_, _, err := DiscoverFromGit(context.Background(), dir, "--upload-pack=evil")
	if err == nil {
		t.Fatalf("expected error for dash-prefixed base-ref, got nil")
	}
	if !contains([]byte(err.Error()), "--upload-pack=evil") {
		t.Errorf("error should name the bad ref: %v", err)
	}
}

func TestDiscoverFromGit_CancelledContextReturnsImmediately(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := t.TempDir()
	gitInit(t, dir)
	gitCommit(t, dir, "init")

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel up front
	_, _, err := DiscoverFromGit(ctx, dir, "main")
	if err == nil {
		t.Errorf("expected error from cancelled context, got nil")
	}
}

func TestDiscoverFromGit_NewSchemaAbsentFromBefore(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := t.TempDir()
	gitInit(t, dir)
	gitCommit(t, dir, "empty initial commit")

	// Add a new schema that didn't exist in main.
	mustWrite(t, filepath.Join(dir, "schemas", "new.json"),
		`{"type": "object", "properties": {"x": {"type": "string"}}}`)

	_, before, err := DiscoverFromGit(context.Background(), dir, "main")
	if err != nil {
		t.Fatalf("DiscoverFromGit error: %v", err)
	}
	if _, ok := before["schemas/new.json"]; ok {
		t.Errorf("brand-new schema should be absent from before map, got keys %v", keys(before))
	}
}

func keys(m map[string][]byte) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func contains(body []byte, sub string) bool {
	for i := 0; i+len(sub) <= len(body); i++ {
		match := true
		for j := 0; j < len(sub); j++ {
			if body[i+j] != sub[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
