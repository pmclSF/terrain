package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunEstimate_JSON(t *testing.T) {
	root := t.TempDir()
	testDir := filepath.Join(root, "tests")
	if err := os.MkdirAll(testDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(testDir, "test_example.py"), []byte("import pytest\n\ndef test_example():\n    assert True\n"), 0o644); err != nil {
		t.Fatalf("write input: %v", err)
	}

	out, err := captureRun(func() error {
		return runEstimate(root, "pytest", "unittest", true)
	})
	if err != nil {
		t.Fatalf("runEstimate returned error: %v", err)
	}

	var payload struct {
		From    string `json:"from"`
		To      string `json:"to"`
		Summary struct {
			TotalFiles int `json:"totalFiles"`
		} `json:"summary"`
	}
	if err := json.Unmarshal(out, &payload); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, out)
	}
	if payload.From != "pytest" || payload.To != "unittest" {
		t.Fatalf("direction = %s -> %s, want pytest -> unittest", payload.From, payload.To)
	}
	if payload.Summary.TotalFiles != 1 {
		t.Fatalf("total files = %d, want 1", payload.Summary.TotalFiles)
	}
}

func TestRunMigrateThenStatusAndChecklist(t *testing.T) {
	root := t.TempDir()
	testDir := filepath.Join(root, "tests")
	outputDir := filepath.Join(root, "converted")
	if err := os.MkdirAll(testDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	input := `import pytest

def test_example():
    assert True
`
	if err := os.WriteFile(filepath.Join(testDir, "test_example.py"), []byte(input), 0o644); err != nil {
		t.Fatalf("write input: %v", err)
	}

	if err := runCaptured(func() error {
		return runMigrate(root, migrateCommandOptions{
			From:        "pytest",
			To:          "unittest",
			Output:      outputDir,
			Concurrency: 2,
		})
	}); err != nil {
		t.Fatalf("runMigrate returned error: %v", err)
	}

	statusOut, err := captureRun(func() error {
		return runStatus(root, true)
	})
	if err != nil {
		t.Fatalf("runStatus returned error: %v", err)
	}
	var statusPayload struct {
		Exists bool `json:"exists"`
		Status struct {
			Converted int `json:"converted"`
		} `json:"status"`
	}
	if err := json.Unmarshal(statusOut, &statusPayload); err != nil {
		t.Fatalf("invalid status JSON: %v\noutput: %s", err, statusOut)
	}
	if !statusPayload.Exists || statusPayload.Status.Converted != 1 {
		t.Fatalf("unexpected status payload: %+v", statusPayload)
	}

	checklistOut, err := captureRun(func() error {
		return runChecklist(root, false)
	})
	if err != nil {
		t.Fatalf("runChecklist returned error: %v", err)
	}
	if !strings.Contains(string(checklistOut), "# Migration Checklist") {
		t.Fatalf("expected checklist output, got:\n%s", checklistOut)
	}
}

func TestRunDoctor_JSON(t *testing.T) {
	root := t.TempDir()
	testDir := filepath.Join(root, "tests")
	if err := os.MkdirAll(testDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(testDir, "auth.test.js"), []byte("describe('auth', () => { expect(true).toBe(true) })\n"), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	out, err := captureRun(func() error {
		_, err := runDoctor(root, true, false)
		return err
	})
	if err != nil {
		t.Fatalf("runDoctor returned error: %v", err)
	}
	var payload struct {
		Checks []struct {
			ID string `json:"id"`
		} `json:"checks"`
	}
	if err := json.Unmarshal(out, &payload); err != nil {
		t.Fatalf("invalid doctor JSON: %v\noutput: %s", err, out)
	}
	if len(payload.Checks) == 0 {
		t.Fatalf("expected doctor checks, got none")
	}
}

func TestRunMigrate_StrictValidateMarksInvalidOutputsFailed(t *testing.T) {
	root := t.TempDir()
	testDir := filepath.Join(root, "tests")
	outputDir := filepath.Join(root, "converted")
	if err := os.MkdirAll(testDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(testDir, "broken.test.js"), []byte("describe('broken', () => {\n"), 0o644); err != nil {
		t.Fatalf("write input: %v", err)
	}

	if err := runCaptured(func() error {
		return runMigrate(root, migrateCommandOptions{
			From:           "jest",
			To:             "vitest",
			Output:         outputDir,
			Concurrency:    2,
			StrictValidate: true,
		})
	}); err != nil {
		t.Fatalf("runMigrate returned error: %v", err)
	}

	statusOut, err := captureRun(func() error {
		return runStatus(root, true)
	})
	if err != nil {
		t.Fatalf("runStatus returned error: %v", err)
	}
	var statusPayload struct {
		Exists bool `json:"exists"`
		Status struct {
			Converted int `json:"converted"`
			Failed    int `json:"failed"`
		} `json:"status"`
	}
	if err := json.Unmarshal(statusOut, &statusPayload); err != nil {
		t.Fatalf("invalid status JSON: %v\noutput: %s", err, statusOut)
	}
	if !statusPayload.Exists || statusPayload.Status.Converted != 0 || statusPayload.Status.Failed != 1 {
		t.Fatalf("unexpected status payload: %+v", statusPayload)
	}
	if _, statErr := os.Stat(filepath.Join(outputDir, "tests", "broken.test.js")); !os.IsNotExist(statErr) {
		t.Fatalf("expected invalid migrated output to be removed, got err=%v", statErr)
	}
}

func TestRunReset_ClearsState(t *testing.T) {
	root := t.TempDir()
	testDir := filepath.Join(root, "tests")
	outputDir := filepath.Join(root, "converted")
	if err := os.MkdirAll(testDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(testDir, "test_example.py"), []byte("import pytest\n\ndef test_example():\n    assert True\n"), 0o644); err != nil {
		t.Fatalf("write input: %v", err)
	}
	if err := runCaptured(func() error {
		return runMigrate(root, migrateCommandOptions{
			From:        "pytest",
			To:          "unittest",
			Output:      outputDir,
			Concurrency: 2,
		})
	}); err != nil {
		t.Fatalf("runMigrate returned error: %v", err)
	}

	if err := runCaptured(func() error {
		return runReset(root, true, false)
	}); err != nil {
		t.Fatalf("runReset returned error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".terrain", "migration", "state.json")); !os.IsNotExist(err) {
		t.Fatalf("expected migration state to be removed, got err=%v", err)
	}
}
