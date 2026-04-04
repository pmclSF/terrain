package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunConvertConfig_AutoDetectsAndPrintsToStdout(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	path := filepath.Join(root, "jest.config.js")
	input := `module.exports = { testEnvironment: 'node', testTimeout: 30000 };`
	if err := os.WriteFile(path, []byte(input), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	out, err := captureRun(func() error {
		return runConvertConfig(path, convertConfigCommandOptions{
			To: "vitest",
		})
	})
	if err != nil {
		t.Fatalf("runConvertConfig returned error: %v", err)
	}

	text := string(out)
	if !strings.Contains(text, "import { defineConfig } from 'vitest/config';") {
		t.Fatalf("expected vitest config in stdout, got:\n%s", text)
	}
	if !strings.Contains(text, "environment: 'node'") {
		t.Fatalf("expected environment mapping, got:\n%s", text)
	}
}

func TestRunConvertConfig_WritesOutputFile(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	sourcePath := filepath.Join(root, "cypress.config.js")
	outputPath := filepath.Join(root, "converted", "playwright.config.ts")
	input := `module.exports = { baseUrl: 'http://localhost:3000', retries: 2 };`
	if err := os.WriteFile(sourcePath, []byte(input), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	if err := runCaptured(func() error {
		return runConvertConfig(sourcePath, convertConfigCommandOptions{
			From:   "cypress",
			To:     "playwright",
			Output: outputPath,
		})
	}); err != nil {
		t.Fatalf("runConvertConfig returned error: %v", err)
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

func TestRunConvertConfig_DryRunDoesNotWrite(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	sourcePath := filepath.Join(root, "wdio.conf.js")
	outputPath := filepath.Join(root, "converted", "playwright.config.ts")
	input := `exports.config = { baseUrl: 'http://localhost:3000' };`
	if err := os.WriteFile(sourcePath, []byte(input), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	out, err := captureRun(func() error {
		return runConvertConfig(sourcePath, convertConfigCommandOptions{
			From:   "webdriverio",
			To:     "playwright",
			Output: outputPath,
			DryRun: true,
		})
	})
	if err != nil {
		t.Fatalf("runConvertConfig returned error: %v", err)
	}

	if _, statErr := os.Stat(outputPath); !os.IsNotExist(statErr) {
		t.Fatalf("expected no output file during dry run, stat err: %v", statErr)
	}

	text := string(out)
	if !strings.Contains(text, "Dry run") {
		t.Fatalf("expected dry-run summary, got:\n%s", text)
	}
	if !strings.Contains(text, "Detected framework: webdriverio") {
		t.Fatalf("expected source framework in dry-run output, got:\n%s", text)
	}
}
