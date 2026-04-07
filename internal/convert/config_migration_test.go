package convert

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunConfigMigration_AutoDetectsAndPrintsToStdout(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	path := filepath.Join(root, "jest.config.js")
	input := `module.exports = { testEnvironment: 'node', testTimeout: 30000 };`
	if err := os.WriteFile(path, []byte(input), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	result, err := RunConfigMigration(path, ConfigMigrationOptions{
		To: "vitest",
	})
	if err != nil {
		t.Fatalf("RunConfigMigration returned error: %v", err)
	}
	if result.Output != "" {
		t.Fatalf("expected stdout mode, got output %q", result.Output)
	}
	if result.From != "jest" || result.To != "vitest" {
		t.Fatalf("direction = %s -> %s, want jest -> vitest", result.From, result.To)
	}
	if result.ValidationMode != string(ValidationModeStrict) {
		t.Fatalf("validation mode = %q, want %q", result.ValidationMode, ValidationModeStrict)
	}
	if !result.Validated {
		t.Fatal("expected strict config migration to validate successfully")
	}
	if !result.AutoDetected {
		t.Fatal("expected autoDetected = true")
	}
	if !strings.Contains(result.ConvertedContent, "import { defineConfig } from 'vitest/config';") {
		t.Fatalf("expected vitest config output, got:\n%s", result.ConvertedContent)
	}
}

func TestRunConfigMigration_WritesValidatedOutput(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	sourcePath := filepath.Join(root, "cypress.config.js")
	outputPath := filepath.Join(root, "converted", "playwright.config.ts")
	input := `module.exports = { baseUrl: 'http://localhost:3000', retries: 2 };`
	if err := os.WriteFile(sourcePath, []byte(input), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	result, err := RunConfigMigration(sourcePath, ConfigMigrationOptions{
		From:           "cypress",
		To:             "playwright",
		Output:         outputPath,
		ValidateSyntax: true,
	})
	if err != nil {
		t.Fatalf("RunConfigMigration returned error: %v", err)
	}
	if result.Output != outputPath {
		t.Fatalf("output path = %q, want %q", result.Output, outputPath)
	}

	output, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	text := string(output)
	if !strings.Contains(text, "@playwright/test") {
		t.Fatalf("expected playwright import, got:\n%s", text)
	}
	if !strings.Contains(text, "projects: [") {
		t.Fatalf("expected default projects, got:\n%s", text)
	}
}

func TestRunConfigMigration_BestEffortKeepsInvalidOutputAndReportsWarning(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	sourcePath := filepath.Join(root, "broken.config.js")
	outputPath := filepath.Join(root, "converted", "vitest.config.ts")
	input := `module.exports = { testEnvironment: 'node };`
	if err := os.WriteFile(sourcePath, []byte(input), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	result, err := RunConfigMigration(sourcePath, ConfigMigrationOptions{
		From:           "jest",
		To:             "vitest",
		Output:         outputPath,
		ValidationMode: string(ValidationModeBestEffort),
	})
	if err != nil {
		t.Fatalf("RunConfigMigration returned error: %v", err)
	}
	if result.Validated {
		t.Fatal("expected best-effort config migration to report failed validation")
	}
	if len(result.Warnings) == 0 || !strings.Contains(strings.Join(result.Warnings, "\n"), "best-effort mode kept output despite validation failure") {
		t.Fatalf("expected best-effort warning, got %v", result.Warnings)
	}
	if _, statErr := os.Stat(outputPath); statErr != nil {
		t.Fatalf("expected best-effort config output to remain on disk, got %v", statErr)
	}
}
