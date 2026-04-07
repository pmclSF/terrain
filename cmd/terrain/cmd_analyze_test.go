package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseRuntimePaths_Empty(t *testing.T) {
	t.Parallel()
	got := parseRuntimePaths("")
	if len(got) != 0 {
		t.Errorf("expected empty, got %v", got)
	}
}

func TestParseRuntimePaths_Single(t *testing.T) {
	t.Parallel()
	got := parseRuntimePaths("results.json")
	if len(got) != 1 || got[0] != "results.json" {
		t.Errorf("expected [results.json], got %v", got)
	}
}

func TestParseRuntimePaths_Multiple(t *testing.T) {
	t.Parallel()
	got := parseRuntimePaths("a.json, b.xml , c.json")
	if len(got) != 3 {
		t.Fatalf("expected 3 paths, got %d", len(got))
	}
	if got[0] != "a.json" || got[1] != "b.xml" || got[2] != "c.json" {
		t.Errorf("expected trimmed paths, got %v", got)
	}
}

func TestParseRuntimePaths_TrailingComma(t *testing.T) {
	t.Parallel()
	got := parseRuntimePaths("a.json,")
	if len(got) != 1 || got[0] != "a.json" {
		t.Errorf("expected [a.json], got %v", got)
	}
}

func TestValidateCommandInputs_ValidRoot(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	err := validateCommandInputs(root, "", nil, nil)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestValidateCommandInputs_InvalidRoot(t *testing.T) {
	t.Parallel()
	err := validateCommandInputs("/nonexistent", "", nil, nil)
	if err == nil {
		t.Fatal("expected error for nonexistent root")
	}
}

func TestValidateCommandInputs_RootIsFile(t *testing.T) {
	t.Parallel()
	f, _ := os.CreateTemp("", "terrain-test-*")
	f.Close()
	defer os.Remove(f.Name())
	err := validateCommandInputs(f.Name(), "", nil, nil)
	if err == nil {
		t.Fatal("expected error when root is a file, not directory")
	}
}

func TestValidateCommandInputs_ValidCoverage(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	covPath := filepath.Join(root, "lcov.info")
	if err := os.WriteFile(covPath, []byte("SF:a.ts\nend_of_record\n"), 0o644); err != nil {
		t.Fatalf("write coverage file: %v", err)
	}
	err := validateCommandInputs(root, covPath, nil, nil)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestValidateCommandInputs_InvalidCoverage(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	err := validateCommandInputs(root, "/nonexistent/lcov.info", nil, nil)
	if err == nil {
		t.Fatal("expected error for nonexistent coverage path")
	}
}

func TestValidateCommandInputs_InvalidRuntime(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	err := validateCommandInputs(root, "", []string{"/nonexistent/results.json"}, nil)
	if err == nil {
		t.Fatal("expected error for nonexistent runtime path")
	}
}

func TestValidateCommandInputs_InvalidGauntlet(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	err := validateCommandInputs(root, "", nil, []string{"/nonexistent/gauntlet.json"})
	if err == nil {
		t.Fatal("expected error for nonexistent gauntlet path")
	}
}

func TestValidateCommandInputs_ValidGauntlet(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	gauntletPath := filepath.Join(root, "gauntlet.json")
	if err := os.WriteFile(gauntletPath, []byte(`{"version":"1","provider":"terrain"}`), 0o644); err != nil {
		t.Fatalf("write gauntlet file: %v", err)
	}
	err := validateCommandInputs(root, "", nil, []string{gauntletPath})
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestPolicyStatusMessage(t *testing.T) {
	t.Parallel()
	if got := policyStatusMessage(true); got != "Policy checks passed." {
		t.Errorf("pass message = %q", got)
	}
	if got := policyStatusMessage(false); got != "Policy violations detected." {
		t.Errorf("fail message = %q", got)
	}
}
