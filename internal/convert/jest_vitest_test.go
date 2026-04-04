package convert

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConvertJestToVitestSource_GoldenFixtures(t *testing.T) {
	t.Parallel()

	cases := []string{
		"test/javascript/jest-to-vitest/imports/IMPORT-001",
		"test/javascript/jest-to-vitest/hooks/HOOKS-001",
		"test/javascript/jest-to-vitest/mocking/MOCK-001",
		"test/javascript/jest-to-vitest/imports/IMPORT-007",
	}

	for _, fixture := range cases {
		t.Run(filepath.Base(fixture), func(t *testing.T) {
			inputPath := repoPath(t, fixture+".input.js")
			expectedPath := repoPath(t, fixture+".expected.js")

			input, err := os.ReadFile(inputPath)
			if err != nil {
				t.Fatalf("read input: %v", err)
			}
			expected, err := os.ReadFile(expectedPath)
			if err != nil {
				t.Fatalf("read expected: %v", err)
			}

			got, err := ConvertJestToVitestSource(string(input))
			if err != nil {
				t.Fatalf("ConvertJestToVitestSource returned error: %v", err)
			}
			if got != string(expected) {
				t.Fatalf("converted output mismatch\n--- got ---\n%s\n--- want ---\n%s", got, expected)
			}
		})
	}
}

func TestConvertJestToVitestSource_RemovesJestGlobalsImport(t *testing.T) {
	t.Parallel()

	input := `import { describe, it, expect, jest } from '@jest/globals';
import { createUser } from './factory';

describe('User', () => {
  it('creates a user', () => {
    const callback = jest.fn();
    expect(callback).toBeDefined();
    expect(createUser()).toBeDefined();
  });
});
`

	got, err := ConvertJestToVitestSource(input)
	if err != nil {
		t.Fatalf("ConvertJestToVitestSource returned error: %v", err)
	}
	if strings.Contains(got, "@jest/globals") {
		t.Fatalf("expected @jest/globals import to be removed, got:\n%s", got)
	}
	if !strings.Contains(got, "import { describe, it, expect, vi } from 'vitest';") {
		t.Fatalf("expected vitest import, got:\n%s", got)
	}
	if !strings.Contains(got, "const callback = vi.fn();") {
		t.Fatalf("expected jest.fn to become vi.fn, got:\n%s", got)
	}
}

func TestExecuteJestToVitestDirectory_WritesConvertedAndUnchangedFiles(t *testing.T) {
	t.Parallel()

	sourceDir := t.TempDir()
	outputDir := filepath.Join(t.TempDir(), "converted")
	testPath := filepath.Join(sourceDir, "auth.test.js")
	helperPath := filepath.Join(sourceDir, "helper.js")
	if err := os.WriteFile(testPath, []byte("describe('auth', () => { it('works', () => { const fn = jest.fn(); expect(fn).toBeDefined(); }); });\n"), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}
	if err := os.WriteFile(helperPath, []byte("export function helper() { return 1; }\n"), 0o644); err != nil {
		t.Fatalf("write helper file: %v", err)
	}

	direction, ok := LookupDirection("jest", "vitest")
	if !ok {
		t.Fatal("expected jest -> vitest direction to exist")
	}

	result, err := Execute(sourceDir, direction, ExecuteOptions{Output: outputDir})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.Mode != "directory" {
		t.Fatalf("mode = %q, want directory", result.Mode)
	}
	if result.ConvertedCount == 0 {
		t.Fatal("expected at least one converted file")
	}

	convertedTest, err := os.ReadFile(filepath.Join(outputDir, "auth.test.js"))
	if err != nil {
		t.Fatalf("read converted test: %v", err)
	}
	if !strings.Contains(string(convertedTest), "import { describe, it, expect, vi } from 'vitest';") {
		t.Fatalf("expected converted test to import vitest, got:\n%s", convertedTest)
	}

	convertedHelper, err := os.ReadFile(filepath.Join(outputDir, "helper.js"))
	if err != nil {
		t.Fatalf("read copied helper: %v", err)
	}
	if string(convertedHelper) != "export function helper() { return 1; }\n" {
		t.Fatalf("expected helper file to be preserved, got:\n%s", convertedHelper)
	}
}

func repoPath(t *testing.T, rel string) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	return filepath.Join(wd, "..", "..", rel)
}
