package convert

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConvertWdioToPlaywrightSource_LoginFlowMatchesFixtureShape(t *testing.T) {
	t.Parallel()

	input := `import { browser, $, expect } from '@wdio/globals';

describe('Login Flow', () => {
  beforeEach(async () => {
    await browser.url('/login');
  });

  it('should login successfully', async () => {
    await $('#username').setValue('admin');
    await $('#password').setValue('pass123');
    await $('#login-btn').click();
    await expect(browser).toHaveUrl('http://localhost/dashboard');
    await expect($('#welcome')).toBeDisplayed();
    await expect($('#welcome')).toHaveText('Welcome, admin');
  });
});
`

	want := `import { test, expect } from '@playwright/test';

test.describe('Login Flow', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/login');
  });

  test('should login successfully', async ({ page }) => {
    await page.locator('#username').fill('admin');
    await page.locator('#password').fill('pass123');
    await page.locator('#login-btn').click();
    await expect(page).toHaveURL('http://localhost/dashboard');
    await expect(page.locator('#welcome')).toBeVisible();
    await expect(page.locator('#welcome')).toHaveText('Welcome, admin');
  });
});
`

	got, err := ConvertWdioToPlaywrightSource(input)
	if err != nil {
		t.Fatalf("ConvertWdioToPlaywrightSource returned error: %v", err)
	}
	if got != want {
		t.Fatalf("converted output mismatch\nwant:\n%s\ngot:\n%s", want, got)
	}
}

func TestConvertWdioToPlaywrightSource_ConvertsBrowserAPIAndAssertions(t *testing.T) {
	t.Parallel()

	input := `describe('navigation', () => {
  it('should navigate back and forward', async () => {
    await browser.url('/page1');
    await browser.url('/page2');
    await browser.back();
    await browser.forward();
    await browser.refresh();
    await browser.pause(2000);
    await expect($$('.item')).toBeElementsArrayOfSize(3);
  });
});
`

	got, err := ConvertWdioToPlaywrightSource(input)
	if err != nil {
		t.Fatalf("ConvertWdioToPlaywrightSource returned error: %v", err)
	}
	for _, want := range []string{
		"await page.goto('/page1')",
		"await page.goto('/page2')",
		"await page.goBack()",
		"await page.goForward()",
		"await page.reload()",
		"await page.waitForTimeout(2000)",
		"await expect(page.locator('.item')).toHaveCount(3)",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in output, got:\n%s", want, got)
		}
	}
}

func TestConvertWdioToPlaywrightSource_CommentsUnsupportedPatterns(t *testing.T) {
	t.Parallel()

	input := `it('uses unsupported helpers', async () => {
  await browser.mock('**/api/users');
});
`

	got, err := ConvertWdioToPlaywrightSource(input)
	if err != nil {
		t.Fatalf("ConvertWdioToPlaywrightSource returned error: %v", err)
	}
	if !strings.Contains(got, "TERRAIN-TODO: manual WebdriverIO conversion required") {
		t.Fatalf("expected TODO comment for unsupported helpers, got:\n%s", got)
	}
	if !strings.Contains(got, "// await browser.mock('**/api/users');") {
		t.Fatalf("expected original unsupported line to be commented out, got:\n%s", got)
	}
}

func TestConvertWdioToPlaywrightSource_DoesNotRewriteStringsOrComments(t *testing.T) {
	t.Parallel()

	input := `// await $('#login-btn').click() should stay in comments
const note = "await browser.url('/dashboard') should stay literal";

it('keeps literals intact', async () => {
  await browser.url('/login');
  expect(note).toContain('browser.url');
});
`

	got, err := ConvertWdioToPlaywrightSource(input)
	if err != nil {
		t.Fatalf("ConvertWdioToPlaywrightSource returned error: %v", err)
	}
	if !strings.Contains(got, "// await $('#login-btn').click() should stay in comments") {
		t.Fatalf("expected comment to stay unchanged, got:\n%s", got)
	}
	if !strings.Contains(got, `const note = "await browser.url('/dashboard') should stay literal";`) {
		t.Fatalf("expected string literal to stay unchanged, got:\n%s", got)
	}
	if !strings.Contains(got, "await page.goto('/login')") {
		t.Fatalf("expected runtime browser call to convert, got:\n%s", got)
	}
}

func TestConvertWdioToPlaywrightSource_ConvertsTextSelectorsWithAstPath(t *testing.T) {
	t.Parallel()

	input := `it('uses text selectors', async () => {
  await $('=Save').click();
  await expect($('*=draft')).toBeDisplayed();
});
`

	got, err := ConvertWdioToPlaywrightSource(input)
	if err != nil {
		t.Fatalf("ConvertWdioToPlaywrightSource returned error: %v", err)
	}
	if !strings.Contains(got, "await page.getByText('Save').click()") {
		t.Fatalf("expected exact text selector conversion, got:\n%s", got)
	}
	if !strings.Contains(got, "await expect(page.getByText('draft')).toBeVisible()") {
		t.Fatalf("expected partial text selector conversion, got:\n%s", got)
	}
}

func TestExecuteWdioToPlaywrightDirectory_PreservesFileNamesAndHelpers(t *testing.T) {
	t.Parallel()

	sourceDir := t.TempDir()
	outputDir := filepath.Join(t.TempDir(), "converted")
	testPath := filepath.Join(sourceDir, "login.spec.js")
	helperPath := filepath.Join(sourceDir, "support.js")
	if err := os.WriteFile(testPath, []byte("describe('login', () => { it('opens', async () => { await browser.url('/login'); await $('#submit').click(); }); });\n"), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}
	if err := os.WriteFile(helperPath, []byte("export const support = true;\n"), 0o644); err != nil {
		t.Fatalf("write helper file: %v", err)
	}

	direction, ok := LookupDirection("webdriverio", "playwright")
	if !ok {
		t.Fatal("expected webdriverio -> playwright direction to exist")
	}

	result, err := Execute(sourceDir, direction, ExecuteOptions{Output: outputDir})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.Mode != "directory" {
		t.Fatalf("mode = %q, want directory", result.Mode)
	}

	convertedTest, err := os.ReadFile(filepath.Join(outputDir, "login.spec.js"))
	if err != nil {
		t.Fatalf("read converted test: %v", err)
	}
	if !strings.Contains(string(convertedTest), "await page.goto('/login')") {
		t.Fatalf("expected converted webdriverio test, got:\n%s", convertedTest)
	}

	convertedHelper, err := os.ReadFile(filepath.Join(outputDir, "support.js"))
	if err != nil {
		t.Fatalf("read copied helper: %v", err)
	}
	if string(convertedHelper) != "export const support = true;\n" {
		t.Fatalf("expected helper file to be preserved, got:\n%s", convertedHelper)
	}
}
