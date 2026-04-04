package convert

import "testing"

func TestConverters_EmptyInputReturnsEmptyWithoutError(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		fn   func(string) (string, error)
	}{
		{name: "jest-vitest", fn: ConvertJestToVitestSource},
		{name: "jasmine-jest", fn: ConvertJasmineToJestSource},
		{name: "jest-jasmine", fn: ConvertJestToJasmineSource},
		{name: "mocha-jest", fn: ConvertMochaToJestSource},
		{name: "jest-mocha", fn: ConvertJestToMochaSource},
		{name: "cypress-playwright", fn: ConvertCypressToPlaywrightSource},
		{name: "playwright-cypress", fn: ConvertPlaywrightToCypressSource},
		{name: "playwright-puppeteer", fn: ConvertPlaywrightToPuppeteerSource},
		{name: "puppeteer-playwright", fn: ConvertPuppeteerToPlaywrightSource},
		{name: "testcafe-playwright", fn: ConvertTestCafeToPlaywrightSource},
		{name: "testcafe-cypress", fn: ConvertTestCafeToCypressSource},
		{name: "junit4-junit5", fn: ConvertJUnit4ToJunit5Source},
		{name: "pytest-unittest", fn: ConvertPytestToUnittestSource},
		{name: "nose2-pytest", fn: ConvertNose2ToPytestSource},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := tc.fn("")
			if err != nil {
				t.Fatalf("expected nil error, got %v", err)
			}
			if got != "" {
				t.Fatalf("expected empty output, got %q", got)
			}
		})
	}
}

func TestConverters_MalformedInputDoesNotReturnError(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		input string
		fn    func(string) (string, error)
	}{
		{name: "jest-vitest", input: "describe('x', () => { if (true) {", fn: ConvertJestToVitestSource},
		{name: "jasmine-jest", input: "describe('x', function() { jasmine.clock().install(", fn: ConvertJasmineToJestSource},
		{name: "cypress-playwright", input: "it('x', () => { cy.get('#btn').click(", fn: ConvertCypressToPlaywrightSource},
		{name: "playwright-puppeteer", input: "test('x', async ({ page }) => { await page.locator('#btn').click(", fn: ConvertPlaywrightToPuppeteerSource},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := tc.fn(tc.input)
			if err != nil {
				t.Fatalf("expected nil error, got %v", err)
			}
			if got == "" {
				t.Fatal("expected non-empty output for malformed input fallback")
			}
		})
	}
}
