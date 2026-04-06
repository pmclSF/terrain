package convert

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunTestMigration_AutoDetectPlan(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	path := filepath.Join(root, "auth.test.js")
	input := `describe('auth', () => {
  it('works', () => {
    expect(true).toBe(true);
  });
});
`
	if err := os.WriteFile(path, []byte(input), 0o644); err != nil {
		t.Fatalf("write input: %v", err)
	}

	result, err := RunTestMigration(path, TestMigrationOptions{
		To:         "vitest",
		AutoDetect: true,
		Plan:       true,
	})
	if err != nil {
		t.Fatalf("RunTestMigration returned error: %v", err)
	}
	if result.Plan == nil {
		t.Fatal("expected plan result, got nil")
	}
	if result.Direction.From != "jest" || result.Direction.To != "vitest" {
		t.Fatalf("direction = %s -> %s, want jest -> vitest", result.Direction.From, result.Direction.To)
	}
	if result.SourceDetection == nil || result.SourceDetection.Framework != "jest" {
		t.Fatalf("expected source detection for jest, got %#v", result.SourceDetection)
	}
	if result.Plan.ExecutionStatus != "executable" {
		t.Fatalf("execution status = %q, want executable", result.Plan.ExecutionStatus)
	}
}

func TestRunTestMigration_UsesShorthandAliasForExecution(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	path := filepath.Join(root, "auth.test.js")
	input := `describe('auth', () => {
  it('works', () => {
    expect(true).toBe(true);
  });
});
`
	if err := os.WriteFile(path, []byte(input), 0o644); err != nil {
		t.Fatalf("write input: %v", err)
	}

	result, err := RunTestMigration(path, TestMigrationOptions{
		Alias: "jesttovt",
	})
	if err != nil {
		t.Fatalf("RunTestMigration returned error: %v", err)
	}
	if result.Execution == nil {
		t.Fatal("expected execution result, got nil")
	}
	if result.Direction.From != "jest" || result.Direction.To != "vitest" {
		t.Fatalf("direction = %s -> %s, want jest -> vitest", result.Direction.From, result.Direction.To)
	}
	if !strings.Contains(result.Execution.StdoutContent, "import { describe, it, expect } from 'vitest';") {
		t.Fatalf("expected vitest output, got:\n%s", result.Execution.StdoutContent)
	}
}

func TestRunTestMigration_ValidateSyntaxRemovesInvalidOutput(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	inputPath := filepath.Join(root, "broken.test.js")
	outputDir := filepath.Join(root, "converted")
	input := "describe('broken', () => {\n"
	if err := os.WriteFile(inputPath, []byte(input), 0o644); err != nil {
		t.Fatalf("write input: %v", err)
	}

	_, err := RunTestMigration(inputPath, TestMigrationOptions{
		From:           "jest",
		To:             "vitest",
		Output:         outputDir,
		ValidateSyntax: true,
	})
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if !strings.Contains(err.Error(), "syntax validation failed") {
		t.Fatalf("expected syntax validation error, got: %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(outputDir, "broken.test.js")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("expected invalid converted output to be removed, got err=%v", statErr)
	}
}

func TestRunTestMigration_DirectoryUsesBatchAndConcurrencyOptions(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	outputDir := filepath.Join(root, "converted")
	if err := os.WriteFile(filepath.Join(root, "first.test.js"), []byte("describe('a', () => { expect(true).toBe(true) })\n"), 0o644); err != nil {
		t.Fatalf("write first input: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "second.test.js"), []byte("describe('b', () => { expect(true).toBe(true) })\n"), 0o644); err != nil {
		t.Fatalf("write second input: %v", err)
	}

	result, err := RunTestMigration(root, TestMigrationOptions{
		From:           "jest",
		To:             "vitest",
		Output:         outputDir,
		BatchSize:      1,
		Concurrency:    2,
		ValidateSyntax: true,
	})
	if err != nil {
		t.Fatalf("RunTestMigration returned error: %v", err)
	}
	if result.Execution == nil {
		t.Fatal("expected execution result, got nil")
	}
	if result.Execution.Mode != "directory" {
		t.Fatalf("mode = %q, want directory", result.Execution.Mode)
	}
	if len(result.Execution.Files) != 2 {
		t.Fatalf("converted files = %d, want 2", len(result.Execution.Files))
	}
	for _, path := range []string{
		filepath.Join(outputDir, "first.test.js"),
		filepath.Join(outputDir, "second.test.js"),
	} {
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatalf("read output %s: %v", path, readErr)
		}
		if !strings.Contains(string(content), "from 'vitest'") {
			t.Fatalf("expected vitest import in %s, got:\n%s", path, content)
		}
	}
}

func TestRunTestMigration_AutoDetectRejectsMixedDirectory(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "playwright.spec.ts"), []byte("import { test } from '@playwright/test';\n"), 0o644); err != nil {
		t.Fatalf("write playwright input: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "legacy.test.js"), []byte("describe('legacy', () => { expect(true).toBe(true) })\n"), 0o644); err != nil {
		t.Fatalf("write jest input: %v", err)
	}

	_, err := RunTestMigration(root, TestMigrationOptions{
		To:         "vitest",
		AutoDetect: true,
		Plan:       true,
	})
	if err == nil {
		t.Fatal("expected mixed-directory auto-detect error, got nil")
	}
	if !strings.Contains(err.Error(), "mixed source frameworks") {
		t.Fatalf("expected mixed-directory error, got %v", err)
	}
	if !strings.Contains(err.Error(), "playwright") || !strings.Contains(err.Error(), "jest") {
		t.Fatalf("expected candidate list in error, got %v", err)
	}
}
