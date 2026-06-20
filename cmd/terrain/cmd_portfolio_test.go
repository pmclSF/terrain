package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pmclSF/terrain/internal/models"
)

func TestRunPortfolioWithManifest_JSONAggregatesSnapshotRepos(t *testing.T) {
	dir := t.TempDir()
	snapshotDir := filepath.Join(dir, "snapshots")
	if err := os.MkdirAll(snapshotDir, 0o755); err != nil {
		t.Fatalf("mkdir snapshots: %v", err)
	}

	writeSnapshot := func(name string, snap models.TestSuiteSnapshot) string {
		t.Helper()
		path := filepath.Join(snapshotDir, name+".json")
		data, err := json.Marshal(snap)
		if err != nil {
			t.Fatalf("marshal snapshot: %v", err)
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatalf("write snapshot: %v", err)
		}
		return path
	}

	webPath := writeSnapshot("web", models.TestSuiteSnapshot{
		TestFiles: []models.TestFile{
			{Path: "tests/home.test.js", Framework: "jest", TestCount: 2},
		},
	})
	apiPath := writeSnapshot("api", models.TestSuiteSnapshot{
		TestFiles: []models.TestFile{
			{Path: "test/auth.spec.js", Framework: "mocha", TestCount: 4},
		},
	})

	manifestPath := filepath.Join(dir, "repos.yaml")
	manifest := "version: 1\n" +
		"description: Rooftop portfolio\n" +
		"repos:\n" +
		"  - name: web-app\n" +
		"    snapshotPath: " + filepath.ToSlash(webPath) + "\n" +
		"    owner: web-team\n" +
		"    frameworksOfRecord: [jest]\n" +
		"    tags: [tier-1]\n" +
		"  - name: api-service\n" +
		"    snapshotPath: " + filepath.ToSlash(apiPath) + "\n" +
		"    owner: backend-team\n" +
		"    frameworksOfRecord: [jest]\n"
	if err := os.WriteFile(manifestPath, []byte(manifest), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	out, err := captureRun(func() error {
		return runPortfolioWithManifest("", manifestPath, true, false)
	})
	if err != nil {
		t.Fatalf("runPortfolioWithManifest: %v", err)
	}

	var got models.PortfolioSnapshot
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("decode output: %v\n%s", err, out)
	}
	if got.Scope != "multi_repo" {
		t.Fatalf("scope = %q, want multi_repo", got.Scope)
	}
	if got.Aggregates.TotalRepos != 2 {
		t.Errorf("totalRepos = %d, want 2", got.Aggregates.TotalRepos)
	}
	if got.Aggregates.FrameworkDriftCount != 1 {
		t.Errorf("frameworkDriftCount = %d, want 1", got.Aggregates.FrameworkDriftCount)
	}
	if len(got.Repositories) != 2 || got.Repositories[1].Status != "drift" {
		t.Fatalf("repositories = %+v, want api-service drift", got.Repositories)
	}
	if len(got.Assets) != 2 || got.Assets[1].Path != "web-app/tests/home.test.js" {
		t.Fatalf("assets = %+v, want repo-prefixed paths", got.Assets)
	}
}

func TestRunPortfolioWithManifest_MissingSnapshotReportsRepoName(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "repos.yaml")
	manifest := "version: 1\n" +
		"repos:\n" +
		"  - name: api-service\n" +
		"    snapshotPath: missing.json\n"
	if err := os.WriteFile(manifestPath, []byte(manifest), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	_, err := captureRun(func() error {
		return runPortfolioWithManifest("", manifestPath, true, false)
	})
	if err == nil {
		t.Fatal("runPortfolioWithManifest succeeded with a missing snapshot")
	}
	if !strings.Contains(err.Error(), `repo "api-service": load snapshot`) {
		t.Fatalf("error = %q, want repo-qualified snapshot failure", err)
	}
}

func TestRunPortfolioWithManifest_TextShowsFrameworkDrift(t *testing.T) {
	dir := t.TempDir()
	snapshotDir := filepath.Join(dir, "snapshots")
	if err := os.MkdirAll(snapshotDir, 0o755); err != nil {
		t.Fatalf("mkdir snapshots: %v", err)
	}

	writeSnapshot := func(name string, snap models.TestSuiteSnapshot) string {
		t.Helper()
		path := filepath.Join(snapshotDir, name+".json")
		data, err := json.Marshal(snap)
		if err != nil {
			t.Fatalf("marshal snapshot: %v", err)
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatalf("write snapshot: %v", err)
		}
		return path
	}

	apiPath := writeSnapshot("api", models.TestSuiteSnapshot{
		TestFiles: []models.TestFile{
			{Path: "test/auth.spec.js", Framework: "mocha", TestCount: 4},
		},
	})

	manifestPath := filepath.Join(dir, "repos.yaml")
	manifest := "version: 1\n" +
		"description: Rooftop portfolio\n" +
		"repos:\n" +
		"  - name: api-service\n" +
		"    snapshotPath: " + filepath.ToSlash(apiPath) + "\n" +
		"    owner: backend-team\n" +
		"    frameworksOfRecord: [jest]\n"
	if err := os.WriteFile(manifestPath, []byte(manifest), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	out, err := captureRun(func() error {
		return runPortfolioWithManifest("", manifestPath, false, false)
	})
	if err != nil {
		t.Fatalf("runPortfolioWithManifest: %v", err)
	}
	text := string(out)
	for _, want := range []string{
		"Framework drift",
		"api-service drifts from frameworksOfRecord (jest):",
		"[DRIFT] api-service",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("output missing %q:\n%s", want, text)
		}
	}
}
