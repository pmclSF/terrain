package convert

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConvertPlaywrightToPuppeteerSource_AddsLifecycleAndConvertsCoreFlow(t *testing.T) {
	t.Parallel()

	input := `import { test, expect } from '@playwright/test';

test.describe('suite', () => {
  test('should work', async ({ page }) => {
    await page.goto('/test');
    await page.locator('#email').fill('user@test.com');
    await expect(page).toHaveURL('/test');
  });
});
`

	got, err := ConvertPlaywrightToPuppeteerSource(input)
	if err != nil {
		t.Fatalf("ConvertPlaywrightToPuppeteerSource returned error: %v", err)
	}
	for _, want := range []string{
		"const puppeteer = require('puppeteer');",
		"let browser, page;",
		"browser = await puppeteer.launch();",
		"page = await browser.newPage();",
		"await browser.close();",
		"describe('suite', () => {",
		"it('should work', async () => {",
		"await page.goto('/test')",
		"await page.type('#email', 'user@test.com')",
		"expect(page.url()).toBe('/test')",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in output, got:\n%s", want, got)
		}
	}
}

func TestConvertPlaywrightToPuppeteerSource_ConvertsAssertionsAndWaits(t *testing.T) {
	t.Parallel()

	input := `import { test, expect } from '@playwright/test';

test.describe('assertions', () => {
  test('checks state', async ({ page }) => {
    await page.goto('/page');
    await expect(page.locator('#visible')).toBeVisible();
    await expect(page.locator('#input')).toHaveValue('test');
    await page.locator('#loaded').waitFor();
    await page.locator('#loaded').click();
  });
});
`

	got, err := ConvertPlaywrightToPuppeteerSource(input)
	if err != nil {
		t.Fatalf("ConvertPlaywrightToPuppeteerSource returned error: %v", err)
	}
	for _, want := range []string{
		"expect(await page.$('#visible')).toBeTruthy()",
		"expect(await page.$eval('#input', el => el.value)).toBe('test')",
		"await page.waitForSelector('#loaded')",
		"await page.click('#loaded')",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in output, got:\n%s", want, got)
		}
	}
}

func TestConvertPlaywrightToPuppeteerSource_ConvertsClearSemanticsWithoutBackspaceHack(t *testing.T) {
	t.Parallel()

	input := `import { test } from '@playwright/test';

test('clears input', async ({ page }) => {
  await page.locator('#email').clear();
});
`

	got, err := ConvertPlaywrightToPuppeteerSource(input)
	if err != nil {
		t.Fatalf("ConvertPlaywrightToPuppeteerSource returned error: %v", err)
	}
	if !strings.Contains(got, "await page.$eval('#email', el => { el.value = '';") {
		t.Fatalf("expected clear to use DOM value reset, got:\n%s", got)
	}
	if strings.Contains(got, "clickCount: 3") || strings.Contains(got, "keyboard.press('Backspace')") {
		t.Fatalf("expected legacy triple-click clear hack to be removed, got:\n%s", got)
	}
}

func TestExecutePlaywrightToPuppeteerDirectory_PreservesFileNamesAndHelpers(t *testing.T) {
	t.Parallel()

	sourceDir := t.TempDir()
	outputDir := filepath.Join(t.TempDir(), "converted")
	testPath := filepath.Join(sourceDir, "dashboard.spec.ts")
	helperPath := filepath.Join(sourceDir, "support.ts")
	if err := os.WriteFile(testPath, []byte("import { test } from '@playwright/test';\n\ntest('opens', async ({ page }) => { await page.goto('/dashboard'); await page.locator('#open').click(); });\n"), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}
	if err := os.WriteFile(helperPath, []byte("export const support = true;\n"), 0o644); err != nil {
		t.Fatalf("write helper file: %v", err)
	}

	direction, ok := LookupDirection("playwright", "puppeteer")
	if !ok {
		t.Fatal("expected playwright -> puppeteer direction to exist")
	}

	result, err := Execute(sourceDir, direction, ExecuteOptions{Output: outputDir})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.Mode != "directory" {
		t.Fatalf("mode = %q, want directory", result.Mode)
	}

	convertedTest, err := os.ReadFile(filepath.Join(outputDir, "dashboard.spec.ts"))
	if err != nil {
		t.Fatalf("read converted test: %v", err)
	}
	if !strings.Contains(string(convertedTest), "await page.click('#open')") {
		t.Fatalf("expected converted playwright test, got:\n%s", convertedTest)
	}

	convertedHelper, err := os.ReadFile(filepath.Join(outputDir, "support.ts"))
	if err != nil {
		t.Fatalf("read copied helper: %v", err)
	}
	if string(convertedHelper) != "export const support = true;\n" {
		t.Fatalf("expected helper file to be preserved, got:\n%s", convertedHelper)
	}
}

func TestConvertPlaywrightToPuppeteerSource_DoesNotRewriteStringsOrComments(t *testing.T) {
	t.Parallel()

	input := `import { test, expect } from '@playwright/test';

test.describe('notes', () => {
  test('leaves prose alone', async ({ page }) => {
    // await page.locator('#save').click() should stay in this comment
    const note = "await expect(page).toHaveURL('/docs') is only documentation";
    const action = "page.locator('#save').click()";
    await page.locator('#save').click();
    expect(note).toContain("toHaveURL('/docs')");
    expect(action).toContain("locator('#save').click()");
  });
});
`

	got, err := ConvertPlaywrightToPuppeteerSource(input)
	if err != nil {
		t.Fatalf("ConvertPlaywrightToPuppeteerSource returned error: %v", err)
	}
	if !strings.Contains(got, "// await page.locator('#save').click() should stay in this comment") {
		t.Fatalf("expected comment to be preserved, got:\n%s", got)
	}
	if !strings.Contains(got, `const note = "await expect(page).toHaveURL('/docs') is only documentation"`) {
		t.Fatalf("expected string literal to remain unchanged, got:\n%s", got)
	}
	if !strings.Contains(got, `const action = "page.locator('#save').click()"`) {
		t.Fatalf("expected action string to remain unchanged, got:\n%s", got)
	}
	if !strings.Contains(got, "await page.click('#save')") {
		t.Fatalf("expected real Playwright action to convert, got:\n%s", got)
	}
}

func TestConvertPlaywrightToPuppeteerSource_CommentsUnsupportedGetByRole(t *testing.T) {
	t.Parallel()

	input := `import { test } from '@playwright/test';

test('manual', async ({ page }) => {
  await page.getByRole('button', { name: 'Save' }).click();
});
`

	got, err := ConvertPlaywrightToPuppeteerSource(input)
	if err != nil {
		t.Fatalf("ConvertPlaywrightToPuppeteerSource returned error: %v", err)
	}
	if !strings.Contains(got, "// TERRAIN-TODO: manual Playwright conversion required") {
		t.Fatalf("expected unsupported getByRole line to be commented, got:\n%s", got)
	}
	if !strings.Contains(got, "// await page.getByRole('button', { name: 'Save' }).click();") {
		t.Fatalf("expected original getByRole line to be preserved as comment, got:\n%s", got)
	}
}
