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

func TestConvertPuppeteerToPlaywrightSource_DoesNotRewriteStringsOrComments(t *testing.T) {
	t.Parallel()

	input := `const puppeteer = require('puppeteer');

describe('notes', () => {
  let browser, page;

  it('leaves prose alone', async () => {
    // await page.click('#save') should stay in this comment
    const note = "expect(page.url()).toBe('/docs') is only documentation";
    const action = "await page.type('#email', 'user@test.com')";
    await page.click('#save');
    expect(note).toContain("toBe('/docs')");
    expect(action).toContain("page.type('#email'");
  });
});
`

	got, err := ConvertPuppeteerToPlaywrightSource(input)
	if err != nil {
		t.Fatalf("ConvertPuppeteerToPlaywrightSource returned error: %v", err)
	}
	if !strings.Contains(got, "// await page.click('#save') should stay in this comment") {
		t.Fatalf("expected comment to be preserved, got:\n%s", got)
	}
	if !strings.Contains(got, `const note = "expect(page.url()).toBe('/docs') is only documentation"`) {
		t.Fatalf("expected note string to remain unchanged, got:\n%s", got)
	}
	if !strings.Contains(got, `const action = "await page.type('#email', 'user@test.com')"`) {
		t.Fatalf("expected action string to remain unchanged, got:\n%s", got)
	}
	if !strings.Contains(got, "await page.locator('#save').click()") {
		t.Fatalf("expected real Puppeteer action to convert, got:\n%s", got)
	}
}

func TestConvertPuppeteerToPlaywrightSource_CommentsUnsupportedPatterns(t *testing.T) {
	t.Parallel()

	input := `const puppeteer = require('puppeteer');

describe('notes', () => {
  let browser, page;

  beforeAll(async () => {
    browser = await puppeteer.launch();
    page = await browser.newPage();
  });

  it('flags unsupported calls', async () => {
    const note = "page.waitForNavigation() should stay literal";
    await page.waitForNavigation();
  });
});
`

	got, err := ConvertPuppeteerToPlaywrightSource(input)
	if err != nil {
		t.Fatalf("ConvertPuppeteerToPlaywrightSource returned error: %v", err)
	}
	if !strings.Contains(got, "// TERRAIN-TODO: manual Puppeteer conversion required") {
		t.Fatalf("expected TODO comment, got:\n%s", got)
	}
	if !strings.Contains(got, "// await page.waitForNavigation();") {
		t.Fatalf("expected unsupported line to be commented out, got:\n%s", got)
	}
	if !strings.Contains(got, `const note = "page.waitForNavigation() should stay literal";`) {
		t.Fatalf("expected string literal to stay unchanged, got:\n%s", got)
	}
}

func TestConvertPuppeteerToPlaywrightSource_ConvertsRegexURLAndTitleAssertions(t *testing.T) {
	t.Parallel()

	input := `const puppeteer = require('puppeteer');

describe('regex assertions', () => {
  let browser, page;

  it('converts regex matchers', async () => {
    expect(page.url()).toMatch(/dashboard\/\d+/);
    expect(await page.title()).toMatch(/Checkout/);
  });
});
`

	got, err := ConvertPuppeteerToPlaywrightSource(input)
	if err != nil {
		t.Fatalf("ConvertPuppeteerToPlaywrightSource returned error: %v", err)
	}
	if !strings.Contains(got, "await expect(page).toHaveURL(/dashboard\\/\\d+/)") {
		t.Fatalf("expected regex URL matcher to convert, got:\n%s", got)
	}
	if !strings.Contains(got, "await expect(page).toHaveTitle(/Checkout/)") {
		t.Fatalf("expected regex title matcher to convert, got:\n%s", got)
	}
}

func TestConvertPuppeteerToPlaywrightSource_ConvertsViewportAndCookieShapesSafely(t *testing.T) {
	t.Parallel()

	input := `const puppeteer = require('puppeteer');

describe('browser config', () => {
  let browser, page;

  it('converts viewport and cookies', async () => {
    await page.setViewport({ width: 1280, height: 720, deviceScaleFactor: 2 });
    await page.setCookie({ name: 'session', value: 'abc' });
    await page.setCookie(
      { name: 'a', value: '1' },
      { name: 'b', value: '2' },
    );
  });
});
`

	got, err := ConvertPuppeteerToPlaywrightSource(input)
	if err != nil {
		t.Fatalf("ConvertPuppeteerToPlaywrightSource returned error: %v", err)
	}
	if !strings.Contains(got, "await page.setViewportSize({ width: 1280, height: 720 })") {
		t.Fatalf("expected safe viewport conversion, got:\n%s", got)
	}
	if !strings.Contains(got, "await page.context().addCookies([{ name: 'session', value: 'abc' }])") {
		t.Fatalf("expected single cookie object to be wrapped, got:\n%s", got)
	}
	if !strings.Contains(got, "await page.context().addCookies([{ name: 'a', value: '1' }, { name: 'b', value: '2' }])") {
		t.Fatalf("expected multiple cookies to be wrapped as an array, got:\n%s", got)
	}
}

func TestConvertPuppeteerToPlaywrightSource_CommentsAmbiguousCookieDeletion(t *testing.T) {
	t.Parallel()

	input := `const puppeteer = require('puppeteer');

describe('cookies', () => {
  let browser, page;

  it('flags ambiguous cookie deletion', async () => {
    await page.deleteCookie(cookie);
  });
});
`

	got, err := ConvertPuppeteerToPlaywrightSource(input)
	if err != nil {
		t.Fatalf("ConvertPuppeteerToPlaywrightSource returned error: %v", err)
	}
	if !strings.Contains(got, "// TERRAIN-TODO: manual Puppeteer conversion required") {
		t.Fatalf("expected TODO comment, got:\n%s", got)
	}
	if !strings.Contains(got, "// await page.deleteCookie(cookie);") {
		t.Fatalf("expected unsupported deleteCookie call to be commented out, got:\n%s", got)
	}
}
