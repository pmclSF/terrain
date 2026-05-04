package portfolio

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadRepoManifest_Canonical(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "repos.yaml")
	if err := os.WriteFile(path, []byte(`
version: 1
description: Acme engineering portfolio
repos:
  - name: web-app
    path: ../web-app
    owner: web-team
    frameworksOfRecord: [jest, playwright]
    tags: [tier-1, customer-facing]
  - name: api-service
    path: ../api-service
    owner: backend-team
    frameworksOfRecord: [pytest]
  - name: archive-tool
    snapshotPath: snapshots/archive-tool.json
    owner: data-team
`), 0o644); err != nil {
		t.Fatal(err)
	}

	m, err := LoadRepoManifest(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if m.Version != 1 {
		t.Errorf("Version = %d, want 1", m.Version)
	}
	if len(m.Repos) != 3 {
		t.Errorf("Repos count = %d, want 3", len(m.Repos))
	}
	if m.Repos[0].Name != "web-app" {
		t.Errorf("first repo name = %q, want web-app", m.Repos[0].Name)
	}
	if got := m.Repos[2].SnapshotPath; got != "snapshots/archive-tool.json" {
		t.Errorf("snapshotPath = %q", got)
	}
}

func TestLoadRepoManifest_RejectsMissingVersion(t *testing.T) {
	t.Parallel()
	_, err := ParseRepoManifest([]byte(`
repos:
  - name: x
    path: /tmp/x
`), "test")
	if err == nil || !strings.Contains(err.Error(), "version") {
		t.Errorf("expected version-required error, got: %v", err)
	}
}

func TestLoadRepoManifest_RejectsUnsupportedVersion(t *testing.T) {
	t.Parallel()
	_, err := ParseRepoManifest([]byte(`
version: 99
repos:
  - name: x
    path: /tmp/x
`), "test")
	if err == nil || !strings.Contains(err.Error(), "unsupported manifest version") {
		t.Errorf("expected unsupported-version error, got: %v", err)
	}
}

func TestLoadRepoManifest_RejectsEmptyRepos(t *testing.T) {
	t.Parallel()
	_, err := ParseRepoManifest([]byte(`
version: 1
repos: []
`), "test")
	if err == nil || !strings.Contains(err.Error(), "empty") {
		t.Errorf("expected empty-repos error, got: %v", err)
	}
}

func TestLoadRepoManifest_RejectsMissingName(t *testing.T) {
	t.Parallel()
	_, err := ParseRepoManifest([]byte(`
version: 1
repos:
  - path: /tmp/x
`), "test")
	if err == nil || !strings.Contains(err.Error(), "'name' is required") {
		t.Errorf("expected name-required error, got: %v", err)
	}
}

func TestLoadRepoManifest_RejectsMissingPathAndSnapshot(t *testing.T) {
	t.Parallel()
	_, err := ParseRepoManifest([]byte(`
version: 1
repos:
  - name: orphan
    owner: nobody
`), "test")
	if err == nil || !strings.Contains(err.Error(), "must set 'path' or 'snapshotPath'") {
		t.Errorf("expected path-or-snapshot error, got: %v", err)
	}
}

func TestLoadRepoManifest_RejectsDuplicateName(t *testing.T) {
	t.Parallel()
	_, err := ParseRepoManifest([]byte(`
version: 1
repos:
  - name: app
    path: /tmp/a
  - name: app
    path: /tmp/b
`), "test")
	if err == nil || !strings.Contains(err.Error(), "duplicate name") {
		t.Errorf("expected duplicate-name error, got: %v", err)
	}
}

func TestResolveRepoPath_Relative(t *testing.T) {
	t.Parallel()
	// Build an absolute manifestDir using filepath.Join so the test
	// passes on Windows (\) and POSIX (/) hosts. ResolveRepoPath
	// returns paths in the host separator format via filepath.Clean
	// / filepath.Join internally.
	manifestDir := filepath.Join(string(filepath.Separator)+"work", ".terrain")
	got := ResolveRepoPath(manifestDir, RepoEntry{Path: "../web-app"})
	want := filepath.Join(string(filepath.Separator)+"work", "web-app")
	if got != want {
		t.Errorf("ResolveRepoPath = %q, want %q", got, want)
	}
}

func TestResolveRepoPath_Absolute(t *testing.T) {
	t.Parallel()
	abs := filepath.Join(string(filepath.Separator)+"elsewhere", "repo")
	got := ResolveRepoPath(filepath.Join(string(filepath.Separator)+"work", ".terrain"),
		RepoEntry{Path: abs})
	if got != abs {
		t.Errorf("ResolveRepoPath = %q, want absolute path %q preserved", got, abs)
	}
}

func TestResolveRepoPath_PrefersPathOverSnapshot(t *testing.T) {
	t.Parallel()
	got := ResolveRepoPath(filepath.Join(string(filepath.Separator)+"work", ".terrain"),
		RepoEntry{
			Path:         "../code",
			SnapshotPath: "snap.json",
		})
	// Compare via filepath.Base since the host separator varies.
	if filepath.Base(got) != "code" {
		t.Errorf("ResolveRepoPath = %q, want path preferred (basename `code`)", got)
	}
}

func TestResolveRepoPath_FallsBackToSnapshot(t *testing.T) {
	t.Parallel()
	got := ResolveRepoPath("/work/.terrain", RepoEntry{
		SnapshotPath: "snap.json",
	})
	if !strings.HasSuffix(got, "snap.json") {
		t.Errorf("ResolveRepoPath = %q, want snapshot fallback", got)
	}
}

func TestResolveSnapshotPath_Empty(t *testing.T) {
	t.Parallel()
	got := ResolveSnapshotPath("/work/.terrain", RepoEntry{Path: "../code"})
	if got != "" {
		t.Errorf("ResolveSnapshotPath without snapshotPath = %q, want empty", got)
	}
}
