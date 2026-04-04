package convert

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConvertPuppeteerToPlaywrightSource_RemovesLifecycleAndConvertsCoreFlow(t *testing.T) {
	t.Parallel()

	input := `const puppeteer = require('puppeteer');

describe('basic suite', () => {
  let browser, page;

  beforeAll(async () => {
    browser = await puppeteer.launch();
    page = await browser.newPage();
  });

  afterAll(async () => {
    await browser.close();
  });

  it('should work', async () => {
    await page.goto('/test');
    await page.type('#email', 'user@test.com');
    expect(page.url()).toBe('/test');
  });
});
`

	got, err := ConvertPuppeteerToPlaywrightSource(input)
	if err != nil {
		t.Fatalf("ConvertPuppeteerToPlaywrightSource returned error: %v", err)
	}
	for _, want := range []string{
		"import { test, expect } from '@playwright/test';",
		"test.describe('basic suite'",
		"test('should work', async ({ page }) => {",
		"await page.goto('/test')",
		"await page.locator('#email').fill('user@test.com')",
		"await expect(page).toHaveURL('/test')",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in output, got:\n%s", want, got)
		}
	}
	if strings.Contains(got, "puppeteer.launch") || strings.Contains(got, "browser.newPage") || strings.Contains(got, "browser.close") {
		t.Fatalf("expected lifecycle boilerplate to be removed, got:\n%s", got)
	}
}

func TestConvertPuppeteerToPlaywrightSource_ConvertsSelectorsAndWaits(t *testing.T) {
	t.Parallel()

	input := `const puppeteer = require('puppeteer');

describe('selectors', () => {
  let browser, page;

  beforeAll(async () => {
    browser = await puppeteer.launch();
    page = await browser.newPage();
  });

  it('should evaluate', async () => {
    await page.goto('/page');
    const text = await page.$eval('#msg', el => el.textContent);
    const texts = await page.$$eval('.items', els => els.map(el => el.textContent));
    await page.waitForSelector('#loaded');
    await page.click('#loaded');
  });
});
`

	got, err := ConvertPuppeteerToPlaywrightSource(input)
	if err != nil {
		t.Fatalf("ConvertPuppeteerToPlaywrightSource returned error: %v", err)
	}
	for _, want := range []string{
		"const text = await page.locator('#msg').evaluate(el => el.textContent);",
		"const texts = await page.locator('.items').evaluateAll(els => els.map(el => el.textContent));",
		"await page.locator('#loaded').waitFor()",
		"await page.locator('#loaded').click()",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in output, got:\n%s", want, got)
		}
	}
}

func TestExecutePuppeteerToPlaywrightDirectory_PreservesFileNamesAndHelpers(t *testing.T) {
	t.Parallel()

	sourceDir := t.TempDir()
	outputDir := filepath.Join(t.TempDir(), "converted")
	testPath := filepath.Join(sourceDir, "flow.test.js")
	helperPath := filepath.Join(sourceDir, "support.js")
	if err := os.WriteFile(testPath, []byte("const puppeteer = require('puppeteer');\n\ndescribe('flow', () => {\n  let browser, page;\n  beforeAll(async () => { browser = await puppeteer.launch(); page = await browser.newPage(); });\n  afterAll(async () => { await browser.close(); });\n  it('opens', async () => { await page.goto('/flow'); await page.click('#go'); });\n});\n"), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}
	if err := os.WriteFile(helperPath, []byte("export const support = true;\n"), 0o644); err != nil {
		t.Fatalf("write helper file: %v", err)
	}

	direction, ok := LookupDirection("puppeteer", "playwright")
	if !ok {
		t.Fatal("expected puppeteer -> playwright direction to exist")
	}

	result, err := Execute(sourceDir, direction, ExecuteOptions{Output: outputDir})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.Mode != "directory" {
		t.Fatalf("mode = %q, want directory", result.Mode)
	}

	convertedTest, err := os.ReadFile(filepath.Join(outputDir, "flow.test.js"))
	if err != nil {
		t.Fatalf("read converted test: %v", err)
	}
	if !strings.Contains(string(convertedTest), "await page.locator('#go').click()") {
		t.Fatalf("expected converted puppeteer test, got:\n%s", convertedTest)
	}

	convertedHelper, err := os.ReadFile(filepath.Join(outputDir, "support.js"))
	if err != nil {
		t.Fatalf("read copied helper: %v", err)
	}
	if string(convertedHelper) != "export const support = true;\n" {
		t.Fatalf("expected helper file to be preserved, got:\n%s", convertedHelper)
	}
}
