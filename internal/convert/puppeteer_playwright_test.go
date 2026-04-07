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

func TestConvertPuppeteerToPlaywrightSource_ConvertsOnlySafeWaitForSelectorOptions(t *testing.T) {
	t.Parallel()

	input := `const puppeteer = require('puppeteer');

describe('selector waits', () => {
  let browser, page;

  it('converts safe wait options', async () => {
    await page.waitForSelector('#loaded', { visible: true, timeout: 5000 });
    await page.waitForSelector('#gone', { hidden: true });
    await page.waitForSelector('#manual', { root: frame });
  });
});
`

	got, err := ConvertPuppeteerToPlaywrightSource(input)
	if err != nil {
		t.Fatalf("ConvertPuppeteerToPlaywrightSource returned error: %v", err)
	}
	if !strings.Contains(got, "await page.locator('#loaded').waitFor({ state: 'visible', timeout: 5000 })") {
		t.Fatalf("expected visible waitForSelector options to convert, got:\n%s", got)
	}
	if !strings.Contains(got, "await page.locator('#gone').waitFor({ state: 'hidden' })") {
		t.Fatalf("expected hidden waitForSelector options to convert, got:\n%s", got)
	}
	if !strings.Contains(got, "// TERRAIN-TODO: manual Puppeteer conversion required") {
		t.Fatalf("expected unsupported waitForSelector options to be flagged, got:\n%s", got)
	}
	if !strings.Contains(got, "// await page.waitForSelector('#manual', { root: frame });") {
		t.Fatalf("expected unsupported waitForSelector call to be preserved as comment, got:\n%s", got)
	}
}

func TestConvertPuppeteerToPlaywrightSource_ConvertsMultiSelectAndCommentsOptionedActions(t *testing.T) {
	t.Parallel()

	input := `const puppeteer = require('puppeteer');

describe('actions', () => {
  let browser, page;

  it('converts only safe action shapes', async () => {
    await page.select('#colors', 'red', 'blue');
    await page.click('#save', { button: 'right' });
    await page.type('#name', 'terrain', { delay: 25 });
  });
});
`

	got, err := ConvertPuppeteerToPlaywrightSource(input)
	if err != nil {
		t.Fatalf("ConvertPuppeteerToPlaywrightSource returned error: %v", err)
	}
	if !strings.Contains(got, "await page.locator('#colors').selectOption(['red', 'blue'])") {
		t.Fatalf("expected multi-select to convert to an array-based selectOption call, got:\n%s", got)
	}
	if !strings.Contains(got, "// TERRAIN-TODO: manual Puppeteer conversion required") {
		t.Fatalf("expected optioned action calls to be flagged, got:\n%s", got)
	}
	if !strings.Contains(got, "// await page.click('#save', { button: 'right' });") {
		t.Fatalf("expected optioned click call to be preserved as comment, got:\n%s", got)
	}
	if !strings.Contains(got, "// await page.type('#name', 'terrain', { delay: 25 });") {
		t.Fatalf("expected optioned type call to be preserved as comment, got:\n%s", got)
	}
}

func TestConvertPuppeteerToPlaywrightSource_ConvertsOnlySafeEvalShapes(t *testing.T) {
	t.Parallel()

	input := `const puppeteer = require('puppeteer');

describe('evals', () => {
  let browser, page;

  it('converts only safe eval shapes', async () => {
    const text = await page.$eval('#msg', el => el.textContent);
    const texts = await page.$$eval('.items', els => els.map(el => el.textContent));
    const value = await page.$eval('#count', (el, add) => Number(el.textContent) + add, 2);
  });
});
`

	got, err := ConvertPuppeteerToPlaywrightSource(input)
	if err != nil {
		t.Fatalf("ConvertPuppeteerToPlaywrightSource returned error: %v", err)
	}
	if !strings.Contains(got, "const text = await page.locator('#msg').evaluate(el => el.textContent);") {
		t.Fatalf("expected safe $eval call to convert, got:\n%s", got)
	}
	if !strings.Contains(got, "const texts = await page.locator('.items').evaluateAll(els => els.map(el => el.textContent));") {
		t.Fatalf("expected safe $$eval call to convert, got:\n%s", got)
	}
	if !strings.Contains(got, "// TERRAIN-TODO: manual Puppeteer conversion required") {
		t.Fatalf("expected extra-arg eval call to be flagged, got:\n%s", got)
	}
	if !strings.Contains(got, "// const value = await page.$eval('#count', (el, add) => Number(el.textContent) + add, 2);") {
		t.Fatalf("expected extra-arg $eval call to be preserved as comment, got:\n%s", got)
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

func TestConvertPuppeteerToPlaywrightSource_FallbackConvertsLocatorCallsWithoutTouchingLiterals(t *testing.T) {
	t.Parallel()

	input := `const puppeteer = require('puppeteer');

describe('notes', () => {
  let browser, page;

  it('rewrites only real locator calls', async () => {
    // page.$('#save') should stay in this comment
    const note = "page.$('#save') should stay literal";
    const listNote = "page.$$('.row') should stay literal";
    const button = await page.$('#save');
    const rows = await page.$$('.row');
    const alias = page.$('#name');
    if (
  });
});
`

	got, err := ConvertPuppeteerToPlaywrightSource(input)
	if err != nil {
		t.Fatalf("ConvertPuppeteerToPlaywrightSource returned error: %v", err)
	}
	if !strings.Contains(got, "// page.$('#save') should stay in this comment") {
		t.Fatalf("expected comment to stay unchanged in fallback path, got:\n%s", got)
	}
	if !strings.Contains(got, `const note = "page.$('#save') should stay literal"`) {
		t.Fatalf("expected single-element locator string to stay unchanged in fallback path, got:\n%s", got)
	}
	if !strings.Contains(got, `const listNote = "page.$$('.row') should stay literal"`) {
		t.Fatalf("expected multi-element locator string to stay unchanged in fallback path, got:\n%s", got)
	}
	if !strings.Contains(got, "const button = page.locator('#save');") {
		t.Fatalf("expected awaited page.$ call to convert to a locator, got:\n%s", got)
	}
	if !strings.Contains(got, "const rows = page.locator('.row');") {
		t.Fatalf("expected awaited page.$$ call to convert to a locator, got:\n%s", got)
	}
	if !strings.Contains(got, "const alias = page.locator('#name');") {
		t.Fatalf("expected non-awaited page.$ call to convert to a locator, got:\n%s", got)
	}
}

func TestConvertPuppeteerToPlaywrightSource_FallbackConvertsActionsWithoutTouchingLiterals(t *testing.T) {
	t.Parallel()

	input := `const puppeteer = require('puppeteer');

describe('actions', () => {
  let browser, page;

  it('rewrites only real action calls', async () => {
    // await page.click('#save') should stay in this comment
    // await page.waitForSelector('#loaded') should stay in this comment
    const clickNote = "await page.click('#save') should stay literal";
    const typeNote = "await page.type('#name', 'terrain') should stay literal";
    await page.click('#save');
    await page.type('#name', 'terrain');
    await page.select('#colors', 'red');
    await page.waitForSelector('#loaded');
    if (
  });
});
`

	got, err := ConvertPuppeteerToPlaywrightSource(input)
	if err != nil {
		t.Fatalf("ConvertPuppeteerToPlaywrightSource returned error: %v", err)
	}
	if !strings.Contains(got, "// await page.click('#save') should stay in this comment") {
		t.Fatalf("expected click comment to stay unchanged in fallback path, got:\n%s", got)
	}
	if !strings.Contains(got, "// await page.waitForSelector('#loaded') should stay in this comment") {
		t.Fatalf("expected waitForSelector comment to stay unchanged in fallback path, got:\n%s", got)
	}
	if !strings.Contains(got, `const clickNote = "await page.click('#save') should stay literal"`) {
		t.Fatalf("expected click string to stay unchanged in fallback path, got:\n%s", got)
	}
	if !strings.Contains(got, `const typeNote = "await page.type('#name', 'terrain') should stay literal"`) {
		t.Fatalf("expected type string to stay unchanged in fallback path, got:\n%s", got)
	}
	if !strings.Contains(got, "await page.locator('#save').click()") {
		t.Fatalf("expected real click call to convert in fallback path, got:\n%s", got)
	}
	if !strings.Contains(got, "await page.locator('#name').fill('terrain')") {
		t.Fatalf("expected real type call to convert in fallback path, got:\n%s", got)
	}
	if !strings.Contains(got, "await page.locator('#colors').selectOption('red')") {
		t.Fatalf("expected real select call to convert in fallback path, got:\n%s", got)
	}
	if !strings.Contains(got, "await page.locator('#loaded').waitFor()") {
		t.Fatalf("expected real waitForSelector call to convert in fallback path, got:\n%s", got)
	}
}

func TestConvertPuppeteerToPlaywrightSource_FallbackConvertsMultilineLocatorViewportAndCookieCalls(t *testing.T) {
	t.Parallel()

	input := `const puppeteer = require('puppeteer');

describe('multiline fallback', () => {
  let browser, page;

  it('converts multiline locator and browser calls safely', async () => {
    // await page.$('#save') should stay in this comment
    const note = "page.$('#save') should stay literal";
    const button = await page.$(
      '#save',
    );
    const cookies = await page.cookies(
    );
    await page.deleteCookie(
    );
    await page.setViewport(
      { width: 1280, height: 720, deviceScaleFactor: 2 },
    );
    if (
  });
});
`

	got, err := ConvertPuppeteerToPlaywrightSource(input)
	if err != nil {
		t.Fatalf("ConvertPuppeteerToPlaywrightSource returned error: %v", err)
	}
	if !strings.Contains(got, "// await page.$('#save') should stay in this comment") {
		t.Fatalf("expected locator comment to stay unchanged in fallback path, got:\n%s", got)
	}
	if !strings.Contains(got, `const note = "page.$('#save') should stay literal"`) {
		t.Fatalf("expected locator string to stay unchanged in fallback path, got:\n%s", got)
	}
	if !strings.Contains(got, "const button = page.locator('#save');") {
		t.Fatalf("expected multiline page.$ call to convert in fallback path, got:\n%s", got)
	}
	if !strings.Contains(got, "const cookies = await page.context().cookies();") {
		t.Fatalf("expected multiline page.cookies call to convert in fallback path, got:\n%s", got)
	}
	if !strings.Contains(got, "await page.context().clearCookies();") {
		t.Fatalf("expected multiline deleteCookie call to convert in fallback path, got:\n%s", got)
	}
	if !strings.Contains(got, "await page.setViewportSize({ width: 1280, height: 720 })") {
		t.Fatalf("expected multiline setViewport call to convert in fallback path, got:\n%s", got)
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

func TestConvertPuppeteerToPlaywrightSource_ConvertsContainedURLAndTitleAssertions(t *testing.T) {
	t.Parallel()

	input := `const puppeteer = require('puppeteer');

describe('substring assertions', () => {
  let browser, page;

  it('converts contains matchers', async () => {
    expect(page.url()).toContain('/dashboard');
    expect(await page.title()).toContain('Checkout');
  });
});
`

	got, err := ConvertPuppeteerToPlaywrightSource(input)
	if err != nil {
		t.Fatalf("ConvertPuppeteerToPlaywrightSource returned error: %v", err)
	}
	if !strings.Contains(got, "await expect(page).toHaveURL(new RegExp('/dashboard'))") {
		t.Fatalf("expected URL contains matcher to convert, got:\n%s", got)
	}
	if !strings.Contains(got, "await expect(page).toHaveTitle(new RegExp('Checkout'))") {
		t.Fatalf("expected title contains matcher to convert, got:\n%s", got)
	}
	if strings.Contains(got, "expect(page.url()).toContain('/dashboard')") || strings.Contains(got, "expect(await page.title()).toContain('Checkout')") {
		t.Fatalf("expected raw Puppeteer contain assertions to be removed, got:\n%s", got)
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

func TestConvertPuppeteerToPlaywrightSource_FallbackConvertsSafeViewportAndCommentsAmbiguousViewport(t *testing.T) {
	t.Parallel()

	input := `const puppeteer = require('puppeteer');

describe('browser config', () => {
  let browser, page;

  it('handles fallback viewport safely', async () => {
    await page.setViewport({ width: 1280, height: 720, deviceScaleFactor: 2 });
    await page.setViewport(viewport);
    if (
  });
});
`

	got, err := ConvertPuppeteerToPlaywrightSource(input)
	if err != nil {
		t.Fatalf("ConvertPuppeteerToPlaywrightSource returned error: %v", err)
	}
	if !strings.Contains(got, "await page.setViewportSize({ width: 1280, height: 720 })") {
		t.Fatalf("expected fallback safe viewport conversion, got:\n%s", got)
	}
	if !strings.Contains(got, "// TERRAIN-TODO: manual Puppeteer conversion required") {
		t.Fatalf("expected unsupported fallback viewport call to be flagged, got:\n%s", got)
	}
	if !strings.Contains(got, "// await page.setViewport(viewport);") {
		t.Fatalf("expected ambiguous fallback viewport call to be preserved as comment, got:\n%s", got)
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

func TestConvertPuppeteerToPlaywrightSource_FallbackConvertsContainedURLAndTitleAssertions(t *testing.T) {
	t.Parallel()

	input := `const puppeteer = require('puppeteer');

describe('substring assertions', () => {
  let browser, page;

  it('converts contains matchers', async () => {
    expect(page.url()).toContain('/dashboard');
    expect(await page.title()).toContain('Checkout');
    if (
  });
});
`

	got, err := ConvertPuppeteerToPlaywrightSource(input)
	if err != nil {
		t.Fatalf("ConvertPuppeteerToPlaywrightSource returned error: %v", err)
	}
	if !strings.Contains(got, "await expect(page).toHaveURL(new RegExp('/dashboard'))") {
		t.Fatalf("expected fallback URL contains matcher to convert, got:\n%s", got)
	}
	if !strings.Contains(got, "await expect(page).toHaveTitle(new RegExp('Checkout'))") {
		t.Fatalf("expected fallback title contains matcher to convert, got:\n%s", got)
	}
	if strings.Contains(got, "expect(page.url()).toContain('/dashboard')") || strings.Contains(got, "expect(await page.title()).toContain('Checkout')") {
		t.Fatalf("expected fallback path not to leave raw Puppeteer contain assertions behind, got:\n%s", got)
	}
}

func TestConvertPuppeteerToPlaywrightSource_FallbackWrapsSafeCookiesAndCommentsUnsupportedCalls(t *testing.T) {
	t.Parallel()

	input := `const puppeteer = require('puppeteer');

describe('fallback safety', () => {
  let browser, page;

  it('handles fallback safely', async () => {
    const note = "page.waitForNavigation() should stay literal";
    await page.setCookie({ name: 'session', value: 'abc' });
    await page.cookies();
    await page.cookies(currentURL);
    await page.waitForNavigation();
    await page.deleteCookie(cookie);
    if (
  });
});
`

	got, err := ConvertPuppeteerToPlaywrightSource(input)
	if err != nil {
		t.Fatalf("ConvertPuppeteerToPlaywrightSource returned error: %v", err)
	}
	if !strings.Contains(got, "await page.context().addCookies([{ name: 'session', value: 'abc' }])") {
		t.Fatalf("expected fallback setCookie to wrap a single cookie object, got:\n%s", got)
	}
	if !strings.Contains(got, "await page.context().cookies()") {
		t.Fatalf("expected fallback zero-arg cookies call to convert, got:\n%s", got)
	}
	if !strings.Contains(got, "// TERRAIN-TODO: manual Puppeteer conversion required") {
		t.Fatalf("expected unsupported fallback calls to be flagged, got:\n%s", got)
	}
	if !strings.Contains(got, "// await page.cookies(currentURL);") {
		t.Fatalf("expected fallback argumented cookies call to be preserved as comment, got:\n%s", got)
	}
	if !strings.Contains(got, "// await page.waitForNavigation();") {
		t.Fatalf("expected fallback waitForNavigation call to be preserved as comment, got:\n%s", got)
	}
	if !strings.Contains(got, "// await page.deleteCookie(cookie);") {
		t.Fatalf("expected fallback deleteCookie call to be preserved as comment, got:\n%s", got)
	}
	if !strings.Contains(got, `const note = "page.waitForNavigation() should stay literal";`) {
		t.Fatalf("expected string literal to stay unchanged in fallback path, got:\n%s", got)
	}
}

func TestConvertPuppeteerToPlaywrightSource_FallbackConvertsMultilineCallsSafely(t *testing.T) {
	t.Parallel()

	input := `const puppeteer = require('puppeteer');

describe('multiline fallback', () => {
  let browser, page;

  it('converts multiline calls safely', async () => {
    await page.waitForSelector(
      '#loaded',
      { visible: true, timeout: 5000 },
    );
    await page.setCookie(
      { name: 'a', value: '1' },
      { name: 'b', value: '2' },
    );
    if (
  });
});
`

	got, err := ConvertPuppeteerToPlaywrightSource(input)
	if err != nil {
		t.Fatalf("ConvertPuppeteerToPlaywrightSource returned error: %v", err)
	}
	if !strings.Contains(got, "await page.locator('#loaded').waitFor({ state: 'visible', timeout: 5000 })") {
		t.Fatalf("expected multiline waitForSelector call to convert in fallback path, got:\n%s", got)
	}
	if !strings.Contains(got, "await page.context().addCookies([{ name: 'a', value: '1' }, { name: 'b', value: '2' }])") {
		t.Fatalf("expected multiline setCookie call to convert in fallback path, got:\n%s", got)
	}
}

func TestConvertPuppeteerToPlaywrightSource_FallbackConvertsEvalCallsWithoutTouchingLiterals(t *testing.T) {
	t.Parallel()

	input := `const puppeteer = require('puppeteer');

describe('eval fallback', () => {
  let browser, page;

  it('rewrites only real eval calls', async () => {
    // await page.$eval('#msg', el => el.textContent) should stay in this comment
    const note = "await page.$eval('#msg', el => el.textContent) should stay literal";
    const text = await page.$eval(
      '#msg',
      el => el.textContent,
    );
    const texts = await page.$$eval(
      '.items',
      els => els.map(el => el.textContent),
    );
    if (
  });
});
`

	got, err := ConvertPuppeteerToPlaywrightSource(input)
	if err != nil {
		t.Fatalf("ConvertPuppeteerToPlaywrightSource returned error: %v", err)
	}
	if !strings.Contains(got, "// await page.$eval('#msg', el => el.textContent) should stay in this comment") {
		t.Fatalf("expected eval comment to stay unchanged in fallback path, got:\n%s", got)
	}
	if !strings.Contains(got, `const note = "await page.$eval('#msg', el => el.textContent) should stay literal"`) {
		t.Fatalf("expected eval string to stay unchanged in fallback path, got:\n%s", got)
	}
	if !strings.Contains(got, "const text = await page.locator('#msg').evaluate(el => el.textContent);") {
		t.Fatalf("expected multiline $eval call to convert in fallback path, got:\n%s", got)
	}
	if !strings.Contains(got, "const texts = await page.locator('.items').evaluateAll(els => els.map(el => el.textContent));") {
		t.Fatalf("expected multiline $$eval call to convert in fallback path, got:\n%s", got)
	}
}

func TestConvertPuppeteerToPlaywrightSource_FallbackConvertsAssertionsWithoutTouchingLiterals(t *testing.T) {
	t.Parallel()

	input := `const puppeteer = require('puppeteer');

describe('assertion fallback', () => {
  let browser, page;

  it('rewrites only real assertions', async () => {
    // expect(page.url()).toContain('/docs') should stay in this comment
    const note = "expect(page.url()).toContain('/docs') should stay literal";
    const titleNote = "expect(await page.title()).toMatch(/Checkout/) should stay literal";
    expect(page.url()).toContain('/dashboard');
    expect(await page.title()).toMatch(/Checkout/);
    expect(await page.$('#save')).toBeTruthy();
    if (
  });
});
`

	got, err := ConvertPuppeteerToPlaywrightSource(input)
	if err != nil {
		t.Fatalf("ConvertPuppeteerToPlaywrightSource returned error: %v", err)
	}
	if !strings.Contains(got, "// expect(page.url()).toContain('/docs') should stay in this comment") {
		t.Fatalf("expected assertion comment to stay unchanged in fallback path, got:\n%s", got)
	}
	if !strings.Contains(got, `const note = "expect(page.url()).toContain('/docs') should stay literal"`) {
		t.Fatalf("expected URL assertion string to stay unchanged in fallback path, got:\n%s", got)
	}
	if !strings.Contains(got, `const titleNote = "expect(await page.title()).toMatch(/Checkout/) should stay literal"`) {
		t.Fatalf("expected title assertion string to stay unchanged in fallback path, got:\n%s", got)
	}
	if !strings.Contains(got, "await expect(page).toHaveURL(new RegExp('/dashboard'))") {
		t.Fatalf("expected real URL assertion to convert in fallback path, got:\n%s", got)
	}
	if !strings.Contains(got, "await expect(page).toHaveTitle(/Checkout/)") {
		t.Fatalf("expected real title assertion to convert in fallback path, got:\n%s", got)
	}
	if !strings.Contains(got, "await expect(page.locator('#save')).toBeVisible()") {
		t.Fatalf("expected real element assertion to convert in fallback path, got:\n%s", got)
	}
}

func TestConvertPuppeteerToPlaywrightSource_FallbackConvertsTestStructureWithoutTouchingLiterals(t *testing.T) {
	t.Parallel()

	input := `const puppeteer = require('puppeteer');

const note = "describe('docs') and beforeEach(async () => {}) should stay literal";

describe('suite', () => {
  // it('docs') and beforeEach(async () => {}) should stay in this comment
  beforeEach(async () => {
    const hookNote = "beforeEach(async () => {}) should stay literal";
  });

  it('works', () => {
    if (
  });
});
`

	got, err := ConvertPuppeteerToPlaywrightSource(input)
	if err != nil {
		t.Fatalf("ConvertPuppeteerToPlaywrightSource returned error: %v", err)
	}
	if !strings.Contains(got, `const note = "describe('docs') and beforeEach(async () => {}) should stay literal"`) {
		t.Fatalf("expected describe/beforeEach string to stay unchanged in fallback path, got:\n%s", got)
	}
	if !strings.Contains(got, "// it('docs') and beforeEach(async () => {}) should stay in this comment") {
		t.Fatalf("expected structure comment to stay unchanged in fallback path, got:\n%s", got)
	}
	if !strings.Contains(got, `const hookNote = "beforeEach(async () => {}) should stay literal"`) {
		t.Fatalf("expected hook string to stay unchanged in fallback path, got:\n%s", got)
	}
	if !strings.Contains(got, "test.describe('suite', () => {") {
		t.Fatalf("expected describe block to convert in fallback path, got:\n%s", got)
	}
	if !strings.Contains(got, "test.beforeEach(async ({ page }) => {") {
		t.Fatalf("expected beforeEach hook to convert in fallback path, got:\n%s", got)
	}
	if !strings.Contains(got, "test('works', async ({ page }) => {") {
		t.Fatalf("expected test callback to convert in fallback path, got:\n%s", got)
	}
}

func TestConvertPuppeteerToPlaywrightSource_FallbackConvertsFunctionCallbacksWithoutTouchingLiterals(t *testing.T) {
	t.Parallel()

	input := `const puppeteer = require('puppeteer');

const note = "describe('docs', function () {}) should stay literal";

describe('suite', function () {
  // it('docs', function () {}) should stay in this comment
  beforeEach(async function () {
    const hookNote = "beforeEach(async function () {}) should stay literal";
  });

  it('works', function () {
    if (
  });
});
`

	got, err := ConvertPuppeteerToPlaywrightSource(input)
	if err != nil {
		t.Fatalf("ConvertPuppeteerToPlaywrightSource returned error: %v", err)
	}
	if !strings.Contains(got, `const note = "describe('docs', function () {}) should stay literal"`) {
		t.Fatalf("expected describe function string to stay unchanged in fallback path, got:\n%s", got)
	}
	if !strings.Contains(got, "// it('docs', function () {}) should stay in this comment") {
		t.Fatalf("expected function-callback comment to stay unchanged in fallback path, got:\n%s", got)
	}
	if !strings.Contains(got, `const hookNote = "beforeEach(async function () {}) should stay literal"`) {
		t.Fatalf("expected hook function string to stay unchanged in fallback path, got:\n%s", got)
	}
	if !strings.Contains(got, "test.describe('suite', () => {") {
		t.Fatalf("expected describe function callback to convert in fallback path, got:\n%s", got)
	}
	if !strings.Contains(got, "test.beforeEach(async ({ page }) => {") {
		t.Fatalf("expected beforeEach function callback to convert in fallback path, got:\n%s", got)
	}
	if !strings.Contains(got, "test('works', async ({ page }) => {") {
		t.Fatalf("expected test function callback to convert in fallback path, got:\n%s", got)
	}
}

func TestConvertPuppeteerToPlaywrightSource_FallbackConvertsOnlySafeWaitForSelectorOptions(t *testing.T) {
	t.Parallel()

	input := `const puppeteer = require('puppeteer');

describe('selector waits', () => {
  let browser, page;

  it('converts safe wait options', async () => {
    await page.waitForSelector('#loaded', { visible: true, timeout: 5000 });
    await page.waitForSelector('#gone', { hidden: true });
    await page.waitForSelector('#manual', { root: frame });
    if (
  });
});
`

	got, err := ConvertPuppeteerToPlaywrightSource(input)
	if err != nil {
		t.Fatalf("ConvertPuppeteerToPlaywrightSource returned error: %v", err)
	}
	if !strings.Contains(got, "await page.locator('#loaded').waitFor({ state: 'visible', timeout: 5000 })") {
		t.Fatalf("expected fallback visible waitForSelector options to convert, got:\n%s", got)
	}
	if !strings.Contains(got, "await page.locator('#gone').waitFor({ state: 'hidden' })") {
		t.Fatalf("expected fallback hidden waitForSelector options to convert, got:\n%s", got)
	}
	if !strings.Contains(got, "// TERRAIN-TODO: manual Puppeteer conversion required") {
		t.Fatalf("expected unsupported fallback waitForSelector options to be flagged, got:\n%s", got)
	}
	if !strings.Contains(got, "// await page.waitForSelector('#manual', { root: frame });") {
		t.Fatalf("expected unsupported fallback waitForSelector call to be preserved as comment, got:\n%s", got)
	}
}

func TestConvertPuppeteerToPlaywrightSource_FallbackConvertsMultiSelectAndCommentsOptionedActions(t *testing.T) {
	t.Parallel()

	input := `const puppeteer = require('puppeteer');

describe('actions', () => {
  let browser, page;

  it('converts only safe action shapes', async () => {
    await page.select('#colors', 'red', 'blue');
    await page.click('#save', { button: 'right' });
    await page.type('#name', 'terrain', { delay: 25 });
    if (
  });
});
`

	got, err := ConvertPuppeteerToPlaywrightSource(input)
	if err != nil {
		t.Fatalf("ConvertPuppeteerToPlaywrightSource returned error: %v", err)
	}
	if !strings.Contains(got, "await page.locator('#colors').selectOption(['red', 'blue'])") {
		t.Fatalf("expected fallback multi-select to convert to an array-based selectOption call, got:\n%s", got)
	}
	if !strings.Contains(got, "// TERRAIN-TODO: manual Puppeteer conversion required") {
		t.Fatalf("expected unsupported fallback action calls to be flagged, got:\n%s", got)
	}
	if !strings.Contains(got, "// await page.click('#save', { button: 'right' });") {
		t.Fatalf("expected fallback optioned click call to be preserved as comment, got:\n%s", got)
	}
	if !strings.Contains(got, "// await page.type('#name', 'terrain', { delay: 25 });") {
		t.Fatalf("expected fallback optioned type call to be preserved as comment, got:\n%s", got)
	}
}

func TestConvertPuppeteerToPlaywrightSource_FallbackConvertsOnlySafeEvalShapes(t *testing.T) {
	t.Parallel()

	input := `const puppeteer = require('puppeteer');

describe('evals', () => {
  let browser, page;

  it('converts only safe eval shapes', async () => {
    const text = await page.$eval('#msg', el => el.textContent);
    const texts = await page.$$eval('.items', els => els.map(el => el.textContent));
    const value = await page.$eval('#count', (el, add) => Number(el.textContent) + add, 2);
    if (
  });
});
`

	got, err := ConvertPuppeteerToPlaywrightSource(input)
	if err != nil {
		t.Fatalf("ConvertPuppeteerToPlaywrightSource returned error: %v", err)
	}
	if !strings.Contains(got, "const text = await page.locator('#msg').evaluate(el => el.textContent);") {
		t.Fatalf("expected fallback safe $eval call to convert, got:\n%s", got)
	}
	if !strings.Contains(got, "const texts = await page.locator('.items').evaluateAll(els => els.map(el => el.textContent));") {
		t.Fatalf("expected fallback safe $$eval call to convert, got:\n%s", got)
	}
	if !strings.Contains(got, "// TERRAIN-TODO: manual Puppeteer conversion required") {
		t.Fatalf("expected fallback extra-arg eval call to be flagged, got:\n%s", got)
	}
	if !strings.Contains(got, "// const value = await page.$eval('#count', (el, add) => Number(el.textContent) + add, 2);") {
		t.Fatalf("expected fallback extra-arg $eval call to be preserved as comment, got:\n%s", got)
	}
}
