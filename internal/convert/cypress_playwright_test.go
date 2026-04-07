package convert

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConvertCypressToPlaywrightSource_CoreUIFlow(t *testing.T) {
	t.Parallel()

	input := `describe('Login Page', () => {
  beforeEach(() => {
    cy.visit('/login');
  });

  it('logs in', () => {
    cy.get('#email').type('user@example.com');
    cy.get('#password').type('password123');
    cy.get('button[type="submit"]').click();
    cy.url().should('include', '/dashboard');
    cy.get('.result').should('be.visible');
  });
});
`

	got, err := ConvertCypressToPlaywrightSource(input)
	if err != nil {
		t.Fatalf("ConvertCypressToPlaywrightSource returned error: %v", err)
	}
	if !strings.Contains(got, "import { test, expect } from '@playwright/test';") {
		t.Fatalf("expected playwright import, got:\n%s", got)
	}
	if !strings.Contains(got, "test.describe('Login Page'") {
		t.Fatalf("expected describe conversion, got:\n%s", got)
	}
	if !strings.Contains(got, "test.beforeEach(async ({ page }) => {") {
		t.Fatalf("expected hook callback conversion, got:\n%s", got)
	}
	if !strings.Contains(got, "test('logs in', async ({ page }) => {") {
		t.Fatalf("expected test callback conversion, got:\n%s", got)
	}
	if !strings.Contains(got, "await page.goto('/login')") {
		t.Fatalf("expected visit conversion, got:\n%s", got)
	}
	if !strings.Contains(got, "await page.locator('#email').fill('user@example.com')") {
		t.Fatalf("expected type conversion, got:\n%s", got)
	}
	if !strings.Contains(got, "await page.locator('button[type=\"submit\"]').click()") {
		t.Fatalf("expected click conversion, got:\n%s", got)
	}
	if !strings.Contains(got, "expect(page.url()).toContain('/dashboard')") {
		t.Fatalf("expected URL assertion conversion, got:\n%s", got)
	}
	if !strings.Contains(got, "await expect(page.locator('.result')).toBeVisible()") {
		t.Fatalf("expected visibility assertion conversion, got:\n%s", got)
	}
	if strings.Contains(got, "cy.get") || strings.Contains(got, "cy.visit") {
		t.Fatalf("expected core Cypress commands to be removed, got:\n%s", got)
	}
}

func TestConvertCypressToPlaywrightSource_CommentsUnsupportedPatterns(t *testing.T) {
	t.Parallel()

	input := `it('uses unsupported helpers', () => {
  cy.intercept('GET', '/api/users').as('users');
  cy.wait('@users');
});
`

	got, err := ConvertCypressToPlaywrightSource(input)
	if err != nil {
		t.Fatalf("ConvertCypressToPlaywrightSource returned error: %v", err)
	}
	if !strings.Contains(got, "TERRAIN-TODO: manual Cypress conversion required") {
		t.Fatalf("expected TODO comment for unsupported helpers, got:\n%s", got)
	}
	if !strings.Contains(got, "// cy.intercept('GET', '/api/users').as('users');") {
		t.Fatalf("expected original unsupported line to be commented out, got:\n%s", got)
	}
}

func TestConvertCypressToPlaywrightSource_DoesNotRewriteStringsOrComments(t *testing.T) {
	t.Parallel()

	input := `// cy.get('.cta').click() should stay in comments
const note = "cy.visit('/dashboard') should stay literal";

it('keeps literals intact', () => {
  cy.visit('/login');
  expect(note).toContain('cy.visit');
});
`

	got, err := ConvertCypressToPlaywrightSource(input)
	if err != nil {
		t.Fatalf("ConvertCypressToPlaywrightSource returned error: %v", err)
	}
	if !strings.Contains(got, "// cy.get('.cta').click() should stay in comments") {
		t.Fatalf("expected Cypress comment to stay unchanged, got:\n%s", got)
	}
	if !strings.Contains(got, `const note = "cy.visit('/dashboard') should stay literal";`) {
		t.Fatalf("expected Cypress string literal to stay unchanged, got:\n%s", got)
	}
	if !strings.Contains(got, "await page.goto('/login')") {
		t.Fatalf("expected runtime Cypress call to convert, got:\n%s", got)
	}
}

func TestConvertCypressToPlaywrightSource_FunctionCallbacksBecomePlaywrightStyle(t *testing.T) {
	t.Parallel()

	input := `describe('Checkout', function () {
  beforeEach(function () {
    cy.visit('/checkout');
  });

  it('submits', function () {
    cy.get('button[type="submit"]').click();
  });
});
`

	got, err := ConvertCypressToPlaywrightSource(input)
	if err != nil {
		t.Fatalf("ConvertCypressToPlaywrightSource returned error: %v", err)
	}
	if !strings.Contains(got, "test.describe('Checkout', () => {") {
		t.Fatalf("expected describe function callback conversion, got:\n%s", got)
	}
	if !strings.Contains(got, "test.beforeEach(async ({ page }) => {") {
		t.Fatalf("expected hook function callback conversion, got:\n%s", got)
	}
	if !strings.Contains(got, "test('submits', async ({ page }) => {") {
		t.Fatalf("expected test function callback conversion, got:\n%s", got)
	}
	if !strings.Contains(got, "await page.locator('button[type=\"submit\"]').click()") {
		t.Fatalf("expected click conversion inside function callback, got:\n%s", got)
	}
}

func TestExecuteCypressToPlaywrightDirectory_RenamesCyFilesToSpec(t *testing.T) {
	t.Parallel()

	sourceDir := t.TempDir()
	outputDir := filepath.Join(t.TempDir(), "converted")
	testPath := filepath.Join(sourceDir, "checkout.cy.js")
	helperPath := filepath.Join(sourceDir, "support.js")
	if err := os.WriteFile(testPath, []byte("it('works', () => { cy.visit('/'); cy.get('.btn').click(); });\n"), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}
	if err := os.WriteFile(helperPath, []byte("export function helper() { return true; }\n"), 0o644); err != nil {
		t.Fatalf("write helper file: %v", err)
	}

	direction, ok := LookupDirection("cypress", "playwright")
	if !ok {
		t.Fatal("expected cypress -> playwright direction to exist")
	}

	result, err := Execute(sourceDir, direction, ExecuteOptions{Output: outputDir})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.Mode != "directory" {
		t.Fatalf("mode = %q, want directory", result.Mode)
	}

	convertedTest, err := os.ReadFile(filepath.Join(outputDir, "checkout.spec.js"))
	if err != nil {
		t.Fatalf("read converted test: %v", err)
	}
	if !strings.Contains(string(convertedTest), "await page.goto('/')") {
		t.Fatalf("expected converted cypress test, got:\n%s", convertedTest)
	}

	convertedHelper, err := os.ReadFile(filepath.Join(outputDir, "support.js"))
	if err != nil {
		t.Fatalf("read copied helper: %v", err)
	}
	if string(convertedHelper) != "export function helper() { return true; }\n" {
		t.Fatalf("expected helper file to be preserved, got:\n%s", convertedHelper)
	}
}
