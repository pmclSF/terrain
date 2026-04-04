package convert

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConvertPlaywrightToSeleniumSource_AddsLifecycleAndConvertsCoreFlow(t *testing.T) {
	t.Parallel()

	input := `import { test, expect } from '@playwright/test';

test.describe('Account', () => {
  test('opens', async ({ page }) => {
    await page.goto('/account');
    await page.locator('#email').fill('user@test.com');
    await page.locator('#submit').click();
    await expect(page).toHaveURL('/account');
  });
});
`

	got, err := ConvertPlaywrightToSeleniumSource(input)
	if err != nil {
		t.Fatalf("ConvertPlaywrightToSeleniumSource returned error: %v", err)
	}
	for _, want := range []string{
		"const { Builder, By, Key, until } = require('selenium-webdriver');",
		"let driver;",
		"driver = await new Builder().forBrowser('chrome').build();",
		"await driver.quit();",
		"describe('Account', () => {",
		"it('opens', async () => {",
		"await driver.get('/account')",
		"await driver.findElement(By.css('#email')).sendKeys('user@test.com')",
		"await driver.findElement(By.css('#submit')).click()",
		"expect(await driver.getCurrentUrl()).toBe('/account')",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in output, got:\n%s", want, got)
		}
	}
	if strings.Contains(got, "@playwright/test") {
		t.Fatalf("expected Playwright import to be removed, got:\n%s", got)
	}
}

func TestConvertPlaywrightToSeleniumSource_ConvertsAssertionsAndWaits(t *testing.T) {
	t.Parallel()

	input := `import { test, expect } from '@playwright/test';

test.describe('assertions', () => {
  test('checks state', async ({ page }) => {
    await expect(page.locator('#visible')).toBeVisible();
    await expect(page.locator('#items')).toHaveCount(3);
    await page.waitForTimeout(2000);
  });
});
`

	got, err := ConvertPlaywrightToSeleniumSource(input)
	if err != nil {
		t.Fatalf("ConvertPlaywrightToSeleniumSource returned error: %v", err)
	}
	for _, want := range []string{
		"expect(await (await driver.findElement(By.css('#visible'))).isDisplayed()).toBe(true)",
		"expect((await driver.findElements(By.css('#items'))).length).toBe(3)",
		"await driver.sleep(2000)",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in output, got:\n%s", want, got)
		}
	}
}

func TestExecutePlaywrightToSeleniumDirectory_PreservesFileNamesAndHelpers(t *testing.T) {
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

	direction, ok := LookupDirection("playwright", "selenium")
	if !ok {
		t.Fatal("expected playwright -> selenium direction to exist")
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
	if !strings.Contains(string(convertedTest), "await driver.get('/dashboard')") {
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
