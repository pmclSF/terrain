package convert

import (
	"strings"
	"testing"
)

func TestDetectConfigFramework(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"jest.config.js":       "jest",
		"vitest.config.ts":     "vitest",
		"playwright.config.ts": "playwright",
		"cypress.config.js":    "cypress",
		"wdio.conf.js":         "webdriverio",
		".mocharc.yml":         "mocha",
		"jasmine.json":         "jasmine",
		"selenium.config.js":   "selenium",
	}

	for path, want := range cases {
		path := path
		want := want
		t.Run(path, func(t *testing.T) {
			t.Parallel()
			if got := DetectConfigFramework(path); got != want {
				t.Fatalf("DetectConfigFramework(%q) = %q, want %q", path, got, want)
			}
		})
	}
}

func TestConvertConfig_JestToVitest(t *testing.T) {
	t.Parallel()

	input := `module.exports = { testEnvironment: 'node', testTimeout: 30000, moduleNameMapper: './mapper.js' };`
	output, err := ConvertConfig(input, "jest", "vitest")
	if err != nil {
		t.Fatalf("ConvertConfig returned error: %v", err)
	}

	if !strings.Contains(output, "import { defineConfig } from 'vitest/config';") {
		t.Fatalf("expected vitest import, got:\n%s", output)
	}
	if !strings.Contains(output, "environment: 'node'") {
		t.Fatalf("expected environment mapping, got:\n%s", output)
	}
	if !strings.Contains(output, "testTimeout: 30000") {
		t.Fatalf("expected testTimeout mapping, got:\n%s", output)
	}
	if !strings.Contains(output, "TERRAIN-TODO") || !strings.Contains(output, "moduleNameMapper") {
		t.Fatalf("expected unsupported key todo, got:\n%s", output)
	}
}

func TestConvertConfig_MochaToJestKeepsPlainYAMLStringsQuoted(t *testing.T) {
	t.Parallel()

	input := "timeout: 5000\nspec: ./test/**/*.test.js\n"
	output, err := ConvertConfig(input, "mocha", "jest")
	if err != nil {
		t.Fatalf("ConvertConfig returned error: %v", err)
	}

	if !strings.Contains(output, "testTimeout: 5000") {
		t.Fatalf("expected timeout mapping, got:\n%s", output)
	}
	if !strings.Contains(output, "testMatch: './test/**/*.test.js'") {
		t.Fatalf("expected spec glob to stay quoted, got:\n%s", output)
	}
}

func TestConvertConfig_CypressToPlaywrightAddsProjects(t *testing.T) {
	t.Parallel()

	input := `module.exports = { baseUrl: 'http://localhost:3000', viewportWidth: 1280, viewportHeight: 720, retries: 2 };`
	output, err := ConvertConfig(input, "cypress", "playwright")
	if err != nil {
		t.Fatalf("ConvertConfig returned error: %v", err)
	}

	if !strings.Contains(output, "use: {") || !strings.Contains(output, "baseURL: 'http://localhost:3000'") {
		t.Fatalf("expected nested use.baseURL mapping, got:\n%s", output)
	}
	if strings.Contains(output, "use.baseURL") {
		t.Fatalf("expected nested JS object rendering, got:\n%s", output)
	}
	if !strings.Contains(output, "projects: [") || !strings.Contains(output, "name: 'chromium'") {
		t.Fatalf("expected default Playwright projects, got:\n%s", output)
	}
}

func TestConvertConfig_WdioToPlaywright(t *testing.T) {
	t.Parallel()

	input := `exports.config = { baseUrl: 'http://localhost:3000', waitforTimeout: 10000, maxInstances: 5 };`
	output, err := ConvertConfig(input, "webdriverio", "playwright")
	if err != nil {
		t.Fatalf("ConvertConfig returned error: %v", err)
	}

	if !strings.Contains(output, "use: {") || !strings.Contains(output, "baseURL: 'http://localhost:3000'") {
		t.Fatalf("expected nested use.baseURL mapping, got:\n%s", output)
	}
	if !strings.Contains(output, "timeout: 10000") {
		t.Fatalf("expected timeout mapping, got:\n%s", output)
	}
	if !strings.Contains(output, "workers: 5") {
		t.Fatalf("expected workers mapping, got:\n%s", output)
	}
}
