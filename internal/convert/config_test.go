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
		".mocharc.cjs":         "mocha",
		"jasmine.json":         "jasmine",
		"jasmine.config.js":    "jasmine",
		".puppeteerrc.cjs":     "puppeteer",
		"testcafe.config.js":   "testcafe",
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

func TestConvertConfig_JestToMocha(t *testing.T) {
	t.Parallel()

	input := `module.exports = { testTimeout: 12000, testMatch: ['tests/**/*.spec.js'], setupFiles: ['./tests/setup.js'], bail: true };`
	output, err := ConvertConfig(input, "jest", "mocha")
	if err != nil {
		t.Fatalf("ConvertConfig returned error: %v", err)
	}

	if !strings.Contains(output, "module.exports = {") {
		t.Fatalf("expected JS mocha config output, got:\n%s", output)
	}
	if !strings.Contains(output, "timeout: 12000") {
		t.Fatalf("expected timeout mapping, got:\n%s", output)
	}
	if !strings.Contains(output, "spec: ['tests/**/*.spec.js']") {
		t.Fatalf("expected spec mapping, got:\n%s", output)
	}
	if !strings.Contains(output, "require: ['./tests/setup.js']") {
		t.Fatalf("expected setupFiles mapping, got:\n%s", output)
	}
}

func TestConvertConfig_PuppeteerToPlaywrightAddsProjects(t *testing.T) {
	t.Parallel()

	input := `module.exports = { baseURL: 'http://localhost:3000', timeout: 30000, defaultViewport: { width: 1280, height: 720 }, headless: true };`
	output, err := ConvertConfig(input, "puppeteer", "playwright")
	if err != nil {
		t.Fatalf("ConvertConfig returned error: %v", err)
	}

	if !strings.Contains(output, "baseURL: 'http://localhost:3000'") {
		t.Fatalf("expected baseURL mapping, got:\n%s", output)
	}
	if !strings.Contains(output, "viewport: { width: 1280, height: 720 }") {
		t.Fatalf("expected viewport mapping, got:\n%s", output)
	}
	if !strings.Contains(output, "projects: [") {
		t.Fatalf("expected default Playwright projects, got:\n%s", output)
	}
}

func TestConvertConfig_TestCafeToCypress(t *testing.T) {
	t.Parallel()

	input := `module.exports = { src: ['tests/**/*.js'], baseUrl: 'http://localhost:3000', selectorTimeout: 5000, assertionTimeout: 7000 };`
	output, err := ConvertConfig(input, "testcafe", "cypress")
	if err != nil {
		t.Fatalf("ConvertConfig returned error: %v", err)
	}

	if !strings.Contains(output, "specPattern: ['tests/**/*.js']") {
		t.Fatalf("expected src mapping, got:\n%s", output)
	}
	if !strings.Contains(output, "baseUrl: 'http://localhost:3000'") {
		t.Fatalf("expected baseUrl mapping, got:\n%s", output)
	}
	if !strings.Contains(output, "defaultCommandTimeout: 7000") {
		t.Fatalf("expected max timeout mapping, got:\n%s", output)
	}
}

func TestTargetConfigFileName_UsesJSNativeTargetsForSupportedJSConfigs(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"mocha":     ".mocharc.cjs",
		"jasmine":   "jasmine.config.js",
		"puppeteer": ".puppeteerrc.cjs",
	}

	for framework, want := range cases {
		framework := framework
		want := want
		t.Run(framework, func(t *testing.T) {
			t.Parallel()
			if got := TargetConfigFileName(framework, "fallback.config"); got != want {
				t.Fatalf("TargetConfigFileName(%q) = %q, want %q", framework, got, want)
			}
		})
	}
}
