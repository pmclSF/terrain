package manifest

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetect_FindsAllManifests(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	// Top-level manifests
	mustWrite(t, filepath.Join(root, "package.json"), `{"dependencies":{"a":"1.0.0"}}`)
	mustWrite(t, filepath.Join(root, "pyproject.toml"), `[project]
name = "x"
dependencies = ["requests"]
`)
	mustWrite(t, filepath.Join(root, "requirements.txt"), "flask>=2.0\n")

	// Nested subproject
	sub := filepath.Join(root, "subproject")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(sub, "requirements-dev.txt"), "pytest\n")

	// Skipped: vendor dir should not be walked
	vendor := filepath.Join(root, "node_modules", "express")
	if err := os.MkdirAll(vendor, 0o755); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(vendor, "package.json"), `{"name":"express"}`)

	manifests, errs := Detect(root)
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(manifests) != 4 {
		t.Fatalf("got %d manifests, want 4: %+v", len(manifests), manifestPaths(manifests))
	}

	// The node_modules package.json should NOT appear.
	for _, m := range manifests {
		if filepath.Base(filepath.Dir(m.Path)) == "express" {
			t.Errorf("vendored manifest leaked through: %s", m.Path)
		}
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func manifestPaths(ms []*Manifest) []string {
	out := make([]string, len(ms))
	for i, m := range ms {
		out[i] = m.Path
	}
	return out
}
