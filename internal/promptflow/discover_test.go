package promptflow

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestDiscover_FindsMarkdownTemplate(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "prompts", "welcome.md"),
		"Hello, {{user_id}}!")

	got, err := Discover(dir)
	if err != nil {
		t.Fatalf("Discover error: %v", err)
	}
	if len(got.Templates) != 1 {
		t.Fatalf("got %d templates, want 1: %+v", len(got.Templates), got.Templates)
	}
	tf := got.Templates[0]
	if filepath.ToSlash(tf.Path) != "prompts/welcome.md" {
		t.Errorf("Path = %q, want %q", filepath.ToSlash(tf.Path), "prompts/welcome.md")
	}
	wantVars := []string{"user_id"}
	if !slices.Equal(tf.Tpl.Vars(), wantVars) {
		t.Errorf("Vars = %v, want %v", tf.Tpl.Vars(), wantVars)
	}
}

func TestDiscover_FindsJSONSchema(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "schemas", "user.json"),
		`{"type": "object", "properties": {"user_id": {"type": "string"}}}`)

	got, err := Discover(dir)
	if err != nil {
		t.Fatalf("Discover error: %v", err)
	}
	if len(got.Schemas) != 1 {
		t.Fatalf("got %d schemas, want 1: %+v", len(got.Schemas), got.Schemas)
	}
	if filepath.ToSlash(got.Schemas[0].Path) != "schemas/user.json" {
		t.Errorf("Path = %q, want %q", filepath.ToSlash(got.Schemas[0].Path), "schemas/user.json")
	}
}

func TestDiscover_IgnoresNonMatchingFiles(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "README.txt"), "hello {{world}}")
	mustWrite(t, filepath.Join(dir, "package.json"), `{"name": "x"}`)
	mustWrite(t, filepath.Join(dir, "main.go"), `package main`)

	got, err := Discover(dir)
	if err != nil {
		t.Fatalf("Discover error: %v", err)
	}
	if len(got.Templates) != 0 {
		t.Errorf("got %d templates, want 0: %+v", len(got.Templates), got.Templates)
	}
	if len(got.Schemas) != 0 {
		t.Errorf("got %d schemas, want 0: %+v", len(got.Schemas), got.Schemas)
	}
}

func TestDiscover_WalksSubdirectories(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "a", "b", "c", "deep.md"), "{{x}}")
	mustWrite(t, filepath.Join(dir, "top.markdown"), "{{y}}")

	got, err := Discover(dir)
	if err != nil {
		t.Fatalf("Discover error: %v", err)
	}
	if len(got.Templates) != 2 {
		t.Fatalf("got %d templates, want 2: %+v", len(got.Templates), got.Templates)
	}
}

func TestDiscover_MissingDirectoryReturnsError(t *testing.T) {
	_, err := Discover("/this/path/does/not/exist/at/all")
	if err == nil {
		t.Errorf("expected error for missing directory, got nil")
	}
}

func TestDiscover_SkipsMalformedJSON(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "broken.json"), `{not valid json}`)
	mustWrite(t, filepath.Join(dir, "valid.json"),
		`{"properties": {"x": {"type": "string"}}}`)

	got, err := Discover(dir)
	if err != nil {
		t.Fatalf("Discover error: %v", err)
	}
	if len(got.Schemas) != 1 {
		t.Errorf("got %d schemas, want 1 (broken.json should be skipped): %+v", len(got.Schemas), got.Schemas)
	}
}

func mustWrite(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
