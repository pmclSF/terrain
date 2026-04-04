package convert

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConvertCypressToSeleniumSource_AddsLifecycleAndConvertsCoreFlow(t *testing.T) {
	t.Parallel()

	input := `/// <reference types="cypress" />

describe('Checkout', () => {
  it('submits', () => {
    cy.visit('/checkout');
    cy.get('#email').type('user@test.com');
    cy.get('#submit').click();
    cy.get('.notice').should('be.visible');
  });
});
`

	got, err := ConvertCypressToSeleniumSource(input)
	if err != nil {
		t.Fatalf("ConvertCypressToSeleniumSource returned error: %v", err)
	}
	for _, want := range []string{
		"const { Builder, By, Key, until } = require('selenium-webdriver');",
		"const { expect } = require('@jest/globals');",
		"driver = await new Builder().forBrowser('chrome').build();",
		"await driver.quit();",
		"it('submits', async () => {",
		"await driver.get('/checkout')",
		"await driver.findElement(By.css('#email')).sendKeys('user@test.com')",
		"await driver.findElement(By.css('#submit')).click()",
		"expect(await (await driver.findElement(By.css('.notice'))).isDisplayed()).toBe(true)",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in output, got:\n%s", want, got)
		}
	}
	if strings.Contains(got, "reference types=\"cypress\"") {
		t.Fatalf("expected Cypress reference to be removed, got:\n%s", got)
	}
}

func TestConvertCypressToSeleniumSource_CommentsUnsupportedPatterns(t *testing.T) {
	t.Parallel()

	input := `it('uses intercept', () => {
  cy.intercept('GET', '/api/orders');
});
`

	got, err := ConvertCypressToSeleniumSource(input)
	if err != nil {
		t.Fatalf("ConvertCypressToSeleniumSource returned error: %v", err)
	}
	if !strings.Contains(got, "TERRAIN-TODO: manual Cypress conversion required") {
		t.Fatalf("expected TODO comment for unsupported Cypress helpers, got:\n%s", got)
	}
	if !strings.Contains(got, "// cy.intercept('GET', '/api/orders');") {
		t.Fatalf("expected original unsupported line to be commented out, got:\n%s", got)
	}
}

func TestExecuteCypressToSeleniumDirectory_PreservesFileNamesAndHelpers(t *testing.T) {
	t.Parallel()

	sourceDir := t.TempDir()
	outputDir := filepath.Join(t.TempDir(), "converted")
	testPath := filepath.Join(sourceDir, "checkout.cy.js")
	helperPath := filepath.Join(sourceDir, "support.js")
	if err := os.WriteFile(testPath, []byte("describe('checkout', () => { it('opens', () => { cy.visit('/checkout'); cy.get('#submit').click(); }); });\n"), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}
	if err := os.WriteFile(helperPath, []byte("export const support = true;\n"), 0o644); err != nil {
		t.Fatalf("write helper file: %v", err)
	}

	direction, ok := LookupDirection("cypress", "selenium")
	if !ok {
		t.Fatal("expected cypress -> selenium direction to exist")
	}

	result, err := Execute(sourceDir, direction, ExecuteOptions{Output: outputDir})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.Mode != "directory" {
		t.Fatalf("mode = %q, want directory", result.Mode)
	}

	convertedTest, err := os.ReadFile(filepath.Join(outputDir, "checkout.cy.js"))
	if err != nil {
		t.Fatalf("read converted test: %v", err)
	}
	if !strings.Contains(string(convertedTest), "await driver.get('/checkout')") {
		t.Fatalf("expected converted cypress test, got:\n%s", convertedTest)
	}

	convertedHelper, err := os.ReadFile(filepath.Join(outputDir, "support.js"))
	if err != nil {
		t.Fatalf("read copied helper: %v", err)
	}
	if string(convertedHelper) != "export const support = true;\n" {
		t.Fatalf("expected helper file to be preserved, got:\n%s", convertedHelper)
	}
}
