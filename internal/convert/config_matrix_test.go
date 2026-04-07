package convert

import (
	"strings"
	"testing"
)

func TestLegacyConfigConversionMatrix(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		from       string
		to         string
		wantMarker string
	}{
		{name: "jest-vitest", from: "jest", to: "vitest", wantMarker: "defineConfig"},
		{name: "vitest-jest", from: "vitest", to: "jest", wantMarker: "module.exports = {"},
		{name: "cypress-playwright", from: "cypress", to: "playwright", wantMarker: "@playwright/test"},
		{name: "playwright-cypress", from: "playwright", to: "cypress", wantMarker: "defineConfig"},
		{name: "webdriverio-playwright", from: "webdriverio", to: "playwright", wantMarker: "@playwright/test"},
		{name: "playwright-webdriverio", from: "playwright", to: "webdriverio", wantMarker: "exports.config = {"},
		{name: "webdriverio-cypress", from: "webdriverio", to: "cypress", wantMarker: "defineConfig"},
		{name: "cypress-webdriverio", from: "cypress", to: "webdriverio", wantMarker: "exports.config = {"},
		{name: "cypress-selenium", from: "cypress", to: "selenium", wantMarker: "selenium-webdriver"},
		{name: "selenium-cypress", from: "selenium", to: "cypress", wantMarker: "defineConfig"},
		{name: "playwright-selenium", from: "playwright", to: "selenium", wantMarker: "selenium-webdriver"},
		{name: "selenium-playwright", from: "selenium", to: "playwright", wantMarker: "@playwright/test"},
		{name: "jest-mocha", from: "jest", to: "mocha", wantMarker: "module.exports = {"},
		{name: "jest-jasmine", from: "jest", to: "jasmine", wantMarker: "module.exports = {"},
		{name: "mocha-jest", from: "mocha", to: "jest", wantMarker: "module.exports = {"},
		{name: "jasmine-jest", from: "jasmine", to: "jest", wantMarker: "module.exports = {"},
		{name: "playwright-puppeteer", from: "playwright", to: "puppeteer", wantMarker: "module.exports = {"},
		{name: "puppeteer-playwright", from: "puppeteer", to: "playwright", wantMarker: "@playwright/test"},
		{name: "testcafe-playwright", from: "testcafe", to: "playwright", wantMarker: "@playwright/test"},
		{name: "testcafe-cypress", from: "testcafe", to: "cypress", wantMarker: "defineConfig"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if !SupportsConfigConversion(tc.from, tc.to) {
				t.Fatalf("SupportsConfigConversion(%q, %q) = false, want true", tc.from, tc.to)
			}

			input, ok := legacyConfigFixture(tc.from)
			if !ok {
				t.Fatalf("missing config fixture for %s", tc.from)
			}

			output, err := ConvertConfig(input, tc.from, tc.to)
			if err != nil {
				t.Fatalf("ConvertConfig returned error: %v", err)
			}
			if strings.TrimSpace(output) == "" {
				t.Fatal("expected non-empty converted config")
			}
			if !strings.Contains(output, tc.wantMarker) {
				t.Fatalf("expected output to contain %q, got:\n%s", tc.wantMarker, output)
			}
		})
	}
}

func legacyConfigFixture(framework string) (string, bool) {
	fixture, ok := legacyConfigFixtures[NormalizeFramework(framework)]
	return fixture, ok
}

var legacyConfigFixtures = map[string]string{
	"jest": `module.exports = { testEnvironment: 'node', testTimeout: 30000, clearMocks: true };`,
	"vitest": `import { defineConfig } from 'vitest/config';

export default defineConfig({
  test: {
    environment: 'node',
    testTimeout: 30000,
    clearMocks: true
  }
});
`,
	"cypress": `const { defineConfig } = require('cypress');

module.exports = defineConfig({
  e2e: {
    baseUrl: 'http://localhost:3000',
    viewportWidth: 1280,
    viewportHeight: 720,
    defaultCommandTimeout: 10000,
    specPattern: 'cypress/e2e/**/*.cy.js'
  }
});
`,
	"playwright": `import { defineConfig } from '@playwright/test';

export default defineConfig({
  testDir: './tests',
  testMatch: 'tests/**/*.spec.ts',
  timeout: 30000,
  use: {
    baseURL: 'http://localhost:3000'
  }
});
`,
	"webdriverio": `exports.config = {
  baseUrl: 'http://localhost:3000',
  waitforTimeout: 10000,
  specs: ['./test/**/*.spec.js'],
  maxInstances: 2
};
`,
	"selenium": `module.exports = {
  baseUrl: 'http://localhost:3000',
  implicitWait: 10000,
  browserName: 'chrome'
};
`,
	"puppeteer": `module.exports = {
  baseURL: 'http://localhost:3000',
  timeout: 30000,
  defaultViewport: { width: 1280, height: 720 },
  headless: true
};
`,
	"mocha": "timeout: 5000\nspec: ./test/**/*.spec.js\nrequire: ./test/setup.js\n",
	"jasmine": `{
  "spec_dir": "spec",
  "spec_files": ["**/*[sS]pec.?(m)js"],
  "helpers": ["helpers/**/*.js"]
}
`,
	"testcafe": `module.exports = {
  src: ['tests/**/*.js'],
  baseUrl: 'http://localhost:3000',
  selectorTimeout: 5000,
  assertionTimeout: 7000
};
`,
}
