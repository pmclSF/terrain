package convert

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConvertSeleniumToPlaywrightSource_RemovesBoilerplateAndConvertsCoreFlow(t *testing.T) {
	t.Parallel()

	input := `const { Builder, By, Key, until } = require('selenium-webdriver');

describe('Login Flow', () => {
  let driver;

  beforeAll(async () => {
    driver = await new Builder().forBrowser('chrome').build();
  });

  afterAll(async () => {
    await driver.quit();
  });

  it('logs in', async () => {
    await driver.get('/login');
    await driver.findElement(By.css('#email')).sendKeys('user@test.com');
    await driver.findElement(By.css('#submit')).click();
    expect(await driver.getCurrentUrl()).toBe('/login');
    expect(await (await driver.findElement(By.css('.notice'))).isDisplayed()).toBe(true);
  });
});
`

	got, err := ConvertSeleniumToPlaywrightSource(input)
	if err != nil {
		t.Fatalf("ConvertSeleniumToPlaywrightSource returned error: %v", err)
	}
	for _, want := range []string{
		"import { test, expect } from '@playwright/test';",
		"test.describe('Login Flow', () => {",
		"test('logs in', async ({ page }) => {",
		"await page.goto('/login')",
		"await page.locator('#email').fill('user@test.com')",
		"await page.locator('#submit').click()",
		"await expect(page).toHaveURL('/login')",
		"await expect(page.locator('.notice')).toBeVisible()",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in output, got:\n%s", want, got)
		}
	}
	for _, unwanted := range []string{
		"selenium-webdriver",
		"new Builder()",
		"driver.quit()",
	} {
		if strings.Contains(got, unwanted) {
			t.Fatalf("expected %q to be removed, got:\n%s", unwanted, got)
		}
	}
}

func TestConvertSeleniumToPlaywrightSource_CommentsUnsupportedPatterns(t *testing.T) {
	t.Parallel()

	input := `it('uses waits', async () => {
  await driver.wait(until.elementLocated(By.id('ready')), 5000);
});
`

	got, err := ConvertSeleniumToPlaywrightSource(input)
	if err != nil {
		t.Fatalf("ConvertSeleniumToPlaywrightSource returned error: %v", err)
	}
	if !strings.Contains(got, "TERRAIN-TODO: manual Selenium conversion required") {
		t.Fatalf("expected TODO comment for unsupported Selenium helpers, got:\n%s", got)
	}
	if !strings.Contains(got, "// await driver.wait(until.elementLocated(By.id('ready')), 5000);") {
		t.Fatalf("expected original unsupported line to be commented out, got:\n%s", got)
	}
}

func TestExecuteSeleniumToPlaywrightDirectory_PreservesFileNamesAndHelpers(t *testing.T) {
	t.Parallel()

	sourceDir := t.TempDir()
	outputDir := filepath.Join(t.TempDir(), "converted")
	testPath := filepath.Join(sourceDir, "login.spec.js")
	helperPath := filepath.Join(sourceDir, "support.js")
	if err := os.WriteFile(testPath, []byte("describe('login', () => { it('opens', async () => { await driver.get('/login'); await driver.findElement(By.css('#submit')).click(); }); });\n"), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}
	if err := os.WriteFile(helperPath, []byte("export const support = true;\n"), 0o644); err != nil {
		t.Fatalf("write helper file: %v", err)
	}

	direction, ok := LookupDirection("selenium", "playwright")
	if !ok {
		t.Fatal("expected selenium -> playwright direction to exist")
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
		t.Fatalf("expected converted selenium test, got:\n%s", convertedTest)
	}

	convertedHelper, err := os.ReadFile(filepath.Join(outputDir, "support.js"))
	if err != nil {
		t.Fatalf("read copied helper: %v", err)
	}
	if string(convertedHelper) != "export const support = true;\n" {
		t.Fatalf("expected helper file to be preserved, got:\n%s", convertedHelper)
	}
}
