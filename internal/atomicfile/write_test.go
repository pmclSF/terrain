package atomicfile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteFileReplacesFileAtomically(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "artifact.json")
	if err := os.WriteFile(path, []byte("old"), 0o644); err != nil {
		t.Fatalf("seed file: %v", err)
	}

	if err := WriteFile(path, []byte("new\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(data) != "new\n" {
		t.Fatalf("file content = %q, want new", string(data))
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".artifact.json.") && strings.HasSuffix(entry.Name(), ".tmp") {
			t.Fatalf("temporary file was not cleaned up: %s", entry.Name())
		}
	}
}

func TestWriteFileRequiresExistingParentDirectory(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	path := filepath.Join(root, "missing", "artifact.json")
	if err := WriteFile(path, []byte("new"), 0o644); err == nil {
		t.Fatal("expected error for missing parent directory")
	}
}
