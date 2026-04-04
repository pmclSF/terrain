package convert

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConvertSeleniumToCypressSource_RemovesBoilerplateAndConvertsCoreFlow(t *testing.T) {
	t.Parallel()

	input := `const { Builder, By, Key, until } = require('selenium-webdriver');
const { expect } = require('@jest/globals');

describe('Orders', () => {
  let driver;

  beforeAll(async () => {
    driver = await new Builder().forBrowser('chrome').build();
  });

  afterAll(async () => {
    await driver.quit();
  });

  it('opens', async () => {
    await driver.get('/orders');
    await driver.findElement(By.css('#submit')).click();
    expect(await (await driver.findElement(By.css('.notice'))).isDisplayed()).toBe(true);
  });
});
`

	got, err := ConvertSeleniumToCypressSource(input)
	if err != nil {
		t.Fatalf("ConvertSeleniumToCypressSource returned error: %v", err)
	}
	for _, want := range []string{
		"/// <reference types=\"cypress\" />",
		"cy.visit('/orders')",
		"cy.get('#submit').click()",
		"cy.get('.notice').should('be.visible')",
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

func TestConvertSeleniumToCypressSource_CommentsUnsupportedPatterns(t *testing.T) {
	t.Parallel()

	input := `it('uses waits', async () => {
  await driver.wait(until.elementLocated(By.id('ready')), 5000);
});
`

	got, err := ConvertSeleniumToCypressSource(input)
	if err != nil {
		t.Fatalf("ConvertSeleniumToCypressSource returned error: %v", err)
	}
	if !strings.Contains(got, "TERRAIN-TODO: manual Selenium conversion required") {
		t.Fatalf("expected TODO comment for unsupported Selenium helpers, got:\n%s", got)
	}
	if !strings.Contains(got, "// await driver.wait(until.elementLocated(By.id('ready')), 5000);") {
		t.Fatalf("expected original unsupported line to be commented out, got:\n%s", got)
	}
}

func TestExecuteSeleniumToCypressDirectory_PreservesFileNamesAndHelpers(t *testing.T) {
	t.Parallel()

	sourceDir := t.TempDir()
	outputDir := filepath.Join(t.TempDir(), "converted")
	testPath := filepath.Join(sourceDir, "account.spec.js")
	helperPath := filepath.Join(sourceDir, "support.js")
	if err := os.WriteFile(testPath, []byte("describe('account', () => { it('opens', async () => { await driver.get('/account'); await driver.findElement(By.css('#save')).click(); }); });\n"), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}
	if err := os.WriteFile(helperPath, []byte("export const support = true;\n"), 0o644); err != nil {
		t.Fatalf("write helper file: %v", err)
	}

	direction, ok := LookupDirection("selenium", "cypress")
	if !ok {
		t.Fatal("expected selenium -> cypress direction to exist")
	}

	result, err := Execute(sourceDir, direction, ExecuteOptions{Output: outputDir})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.Mode != "directory" {
		t.Fatalf("mode = %q, want directory", result.Mode)
	}

	convertedTest, err := os.ReadFile(filepath.Join(outputDir, "account.spec.js"))
	if err != nil {
		t.Fatalf("read converted test: %v", err)
	}
	if !strings.Contains(string(convertedTest), "cy.visit('/account')") {
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
