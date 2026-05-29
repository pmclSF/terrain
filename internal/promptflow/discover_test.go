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

func TestDiscover_TemplatePathPropagatesToTemplate(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "p.md"), "Hello {{missing_var}}")
	got, err := Discover(dir)
	if err != nil {
		t.Fatalf("Discover error: %v", err)
	}
	if len(got.Templates) != 1 {
		t.Fatalf("got %d templates, want 1", len(got.Templates))
	}
	if got.Templates[0].Tpl.Path != "p.md" {
		t.Errorf("Template.Path = %q, want %q", got.Templates[0].Tpl.Path, "p.md")
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

func TestDiscover_NonSchemaJSONWithPropertiesKeyIgnored(t *testing.T) {
	// A real-world config that uses "properties" as an organizing
	// key but isn't a JSON Schema.
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "stub.json"),
		`{"properties": {"foo": "bar", "name": "demo"}}`)
	got, err := Discover(dir)
	if err != nil {
		t.Fatalf("Discover error: %v", err)
	}
	if len(got.Schemas) != 0 {
		t.Errorf("expected 0 schemas (no $schema or type:object), got %d: %+v", len(got.Schemas), got.Schemas)
	}
}

func TestDiscover_DollarSchemaURIDetectsSchema(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"http json-schema.org draft 7",
			`{"$schema": "http://json-schema.org/draft-07/schema#",
			  "properties": {"x": {"type": "string"}}}`},
		{"https json-schema.org draft 2020-12",
			`{"$schema": "https://json-schema.org/draft/2020-12/schema",
			  "properties": {"x": {"type": "string"}}}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			mustWrite(t, filepath.Join(dir, "s.json"), c.body)
			got, err := Discover(dir)
			if err != nil {
				t.Fatalf("Discover error: %v", err)
			}
			if len(got.Schemas) != 1 {
				t.Errorf("expected 1 schema, got %d: %+v", len(got.Schemas), got.Schemas)
			}
		})
	}
}

func TestDiscover_TypeObjectWithPropertiesDetectsSchema(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "s.json"),
		`{"type": "object", "properties": {"x": {"type": "string"}}}`)
	got, err := Discover(dir)
	if err != nil {
		t.Fatalf("Discover error: %v", err)
	}
	if len(got.Schemas) != 1 {
		t.Errorf("expected 1 schema, got %d: %+v", len(got.Schemas), got.Schemas)
	}
}

func TestDiscover_SkipsNoiseDirectories(t *testing.T) {
	dir := t.TempDir()
	// Drop a real schema + template at the root.
	mustWrite(t, filepath.Join(dir, "schemas", "user.json"),
		`{"type": "object", "properties": {"name": {"type": "string"}}}`)
	mustWrite(t, filepath.Join(dir, "welcome.md"), "{{name}}")
	// Drop noise content inside each skipped directory.
	for _, sub := range []string{"node_modules", "vendor", ".git", "dist", "build", ".terrain"} {
		mustWrite(t, filepath.Join(dir, sub, "noise.md"), "{{trash}}")
		mustWrite(t, filepath.Join(dir, sub, "noise.json"),
			`{"type": "object", "properties": {"bad": {"type": "string"}}}`)
	}
	got, err := Discover(dir)
	if err != nil {
		t.Fatalf("Discover error: %v", err)
	}
	if len(got.Templates) != 1 {
		t.Errorf("expected 1 template (noise dirs ignored), got %d: %+v", len(got.Templates), got.Templates)
	}
	if len(got.Schemas) != 1 {
		t.Errorf("expected 1 schema (noise dirs ignored), got %d: %+v", len(got.Schemas), got.Schemas)
	}
}

func TestDiscover_SkipsMalformedJSON(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "broken.json"), `{not valid json}`)
	mustWrite(t, filepath.Join(dir, "valid.json"),
		`{"type": "object", "properties": {"x": {"type": "string"}}}`)

	got, err := Discover(dir)
	if err != nil {
		t.Fatalf("Discover error: %v", err)
	}
	if len(got.Schemas) != 1 {
		t.Errorf("got %d schemas, want 1 (broken.json should be skipped): %+v", len(got.Schemas), got.Schemas)
	}
}

func TestDiscover_SymlinkSkipped(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "real.md"), "Hello {{name}}")
	link := filepath.Join(root, "linked.md")
	if err := os.Symlink(filepath.Join(root, "real.md"), link); err != nil {
		t.Skipf("symlink unsupported on this platform: %v", err)
	}
	got, err := Discover(root)
	if err != nil {
		t.Fatalf("Discover error: %v", err)
	}
	if len(got.Templates) != 1 {
		t.Errorf("expected exactly 1 template (only the real file, not the symlink), got %d: %+v",
			len(got.Templates), got.Templates)
	}
	for _, tf := range got.Templates {
		if tf.Path == "linked.md" {
			t.Errorf("symlink should have been skipped, found: %s", tf.Path)
		}
	}
}

func TestDiscover_FilesAboveMaxBytesSkipped(t *testing.T) {
	root := t.TempDir()
	tiny := filepath.Join(root, "tiny.md")
	mustWrite(t, tiny, "Hello {{x}}")
	huge := filepath.Join(root, "huge.md")
	if err := os.WriteFile(huge, make([]byte, MaxFileBytes+1024), 0o644); err != nil {
		t.Fatalf("write huge: %v", err)
	}
	got, err := Discover(root)
	if err != nil {
		t.Fatalf("Discover error: %v", err)
	}
	if len(got.Templates) != 1 {
		t.Errorf("expected exactly 1 template (huge.md skipped), got %d: %+v",
			len(got.Templates), got.Templates)
	}
	if got.Templates[0].Path != "tiny.md" {
		t.Errorf("kept the wrong file: %s", got.Templates[0].Path)
	}
}

func TestDiscover_BinaryExtensionsNotRead(t *testing.T) {
	root := t.TempDir()
	// A multi-MiB binary file with no extension we recognize.
	if err := os.WriteFile(filepath.Join(root, "data.bin"),
		make([]byte, 5*1024*1024), 0o644); err != nil {
		t.Fatalf("write bin: %v", err)
	}
	// And a real template alongside.
	mustWrite(t, filepath.Join(root, "p.md"), "{{x}}")
	got, err := Discover(root)
	if err != nil {
		t.Fatalf("Discover error: %v", err)
	}
	if len(got.Templates) != 1 || got.Templates[0].Path != "p.md" {
		t.Errorf("expected the markdown template only, got %+v", got.Templates)
	}
	if len(got.Schemas) != 0 {
		t.Errorf("expected zero schemas, got %+v", got.Schemas)
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
