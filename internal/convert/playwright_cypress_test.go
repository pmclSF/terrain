package convert

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConvertPlaywrightToCypressSource_CoreUIFlow(t *testing.T) {
	t.Parallel()

	input := `import { test, expect } from '@playwright/test';

test.describe('Dashboard', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/dashboard');
  });

  test('opens details', async ({ page }) => {
    await page.locator('[data-testid="details"]').click();
    await expect(page.locator('.panel')).toBeVisible();
    await expect(page).toHaveURL('/dashboard/details');
  });
});
`

	got, err := ConvertPlaywrightToCypressSource(input)
	if err != nil {
		t.Fatalf("ConvertPlaywrightToCypressSource returned error: %v", err)
	}
	if !strings.Contains(got, "/// <reference types=\"cypress\" />") {
		t.Fatalf("expected cypress reference, got:\n%s", got)
	}
	if strings.Contains(got, "@playwright/test") {
		t.Fatalf("expected playwright import removal, got:\n%s", got)
	}
	if !strings.Contains(got, "describe('Dashboard'") {
		t.Fatalf("expected describe conversion, got:\n%s", got)
	}
	if !strings.Contains(got, "beforeEach(() => {") {
		t.Fatalf("expected hook conversion, got:\n%s", got)
	}
	if !strings.Contains(got, "it('opens details', () => {") {
		t.Fatalf("expected test conversion, got:\n%s", got)
	}
	if !strings.Contains(got, "cy.visit('/dashboard')") {
		t.Fatalf("expected goto conversion, got:\n%s", got)
	}
	if !strings.Contains(got, "cy.get('[data-testid=\"details\"]').click()") {
		t.Fatalf("expected locator click conversion, got:\n%s", got)
	}
	if !strings.Contains(got, "cy.get('.panel').should('be.visible')") {
		t.Fatalf("expected assertion conversion, got:\n%s", got)
	}
	if !strings.Contains(got, "cy.url().should('include', '/dashboard/details')") {
		t.Fatalf("expected URL conversion, got:\n%s", got)
	}
}

func TestConvertPlaywrightToCypressSource_CommentsUnsupportedPatterns(t *testing.T) {
	t.Parallel()

	input := `test('downloads', async ({ page }) => {
  const downloadPromise = page.waitForEvent('download');
  await downloadPromise;
});
`

	got, err := ConvertPlaywrightToCypressSource(input)
	if err != nil {
		t.Fatalf("ConvertPlaywrightToCypressSource returned error: %v", err)
	}
	if !strings.Contains(got, "TERRAIN-TODO: manual Playwright conversion required") {
		t.Fatalf("expected TODO comment, got:\n%s", got)
	}
	if !strings.Contains(got, "//   const downloadPromise = page.waitForEvent('download');") && !strings.Contains(got, "// const downloadPromise = page.waitForEvent('download');") {
		t.Fatalf("expected original line to be commented out, got:\n%s", got)
	}
}

func TestConvertPlaywrightToCypressSource_DoesNotRewriteStringsOrComments(t *testing.T) {
	t.Parallel()

	input := `// await page.locator('.panel').click() should stay in comments
const note = "await page.goto('/dashboard') should stay literal";

test('keeps literals intact', async ({ page }) => {
  await page.goto('/login');
  expect(note).toContain('page.goto');
});
`

	got, err := ConvertPlaywrightToCypressSource(input)
	if err != nil {
		t.Fatalf("ConvertPlaywrightToCypressSource returned error: %v", err)
	}
	if !strings.Contains(got, "// await page.locator('.panel').click() should stay in comments") {
		t.Fatalf("expected comment to stay unchanged, got:\n%s", got)
	}
	if !strings.Contains(got, `const note = "await page.goto('/dashboard') should stay literal";`) {
		t.Fatalf("expected string literal to stay unchanged, got:\n%s", got)
	}
	if !strings.Contains(got, "cy.visit('/login')") {
		t.Fatalf("expected runtime playwright call to convert, got:\n%s", got)
	}
}

func TestConvertPlaywrightToCypressSource_RemovesFixtureArgs(t *testing.T) {
	t.Parallel()

	input := `test.describe('Checkout', () => {
  test.beforeEach(async ({ page, request }) => {
    await page.goto('/checkout');
  });

  test('submits', async ({ page }) => {
    await page.locator('button[type="submit"]').click();
  });
});
`

	got, err := ConvertPlaywrightToCypressSource(input)
	if err != nil {
		t.Fatalf("ConvertPlaywrightToCypressSource returned error: %v", err)
	}
	if !strings.Contains(got, "describe('Checkout', () => {") {
		t.Fatalf("expected describe conversion, got:\n%s", got)
	}
	if !strings.Contains(got, "beforeEach(() => {") {
		t.Fatalf("expected fixture args to be removed from hook, got:\n%s", got)
	}
	if !strings.Contains(got, "it('submits', () => {") {
		t.Fatalf("expected fixture args to be removed from test, got:\n%s", got)
	}
	if !strings.Contains(got, "cy.get('button[type=\"submit\"]').click()") {
		t.Fatalf("expected click conversion inside fixture callback, got:\n%s", got)
	}
}

func TestConvertPlaywrightToCypressSource_PreservesRegexURLAndTitleExpectations(t *testing.T) {
	t.Parallel()

	input := `import { test, expect } from '@playwright/test';

test('regex expectations', async ({ page }) => {
  await expect(page).toHaveURL(/dashboard\/\d+/);
  await expect(page).toHaveTitle(/Checkout/);
});
`

	got, err := ConvertPlaywrightToCypressSource(input)
	if err != nil {
		t.Fatalf("ConvertPlaywrightToCypressSource returned error: %v", err)
	}
	if !strings.Contains(got, "cy.url().should('match', /dashboard\\/\\d+/)") {
		t.Fatalf("expected regex URL assertion to convert with match, got:\n%s", got)
	}
	if !strings.Contains(got, "cy.title().should('match', /Checkout/)") {
		t.Fatalf("expected regex title assertion to convert with match, got:\n%s", got)
	}
}

func TestConvertPlaywrightToCypressSource_FallbackPreservesRegexURLAndTitleExpectations(t *testing.T) {
	t.Parallel()

	input := `import { test, expect } from '@playwright/test';

test('regex expectations', async ({ page }) => {
  await expect(page).toHaveURL(/dashboard\/\d+/);
  await expect(page).toHaveTitle(/Checkout/);
  if (
});
`

	got, err := ConvertPlaywrightToCypressSource(input)
	if err != nil {
		t.Fatalf("ConvertPlaywrightToCypressSource returned error: %v", err)
	}
	if !strings.Contains(got, "cy.url().should('match', /dashboard\\/\\d+/)") {
		t.Fatalf("expected fallback regex URL assertion to convert with match, got:\n%s", got)
	}
	if !strings.Contains(got, "cy.title().should('match', /Checkout/)") {
		t.Fatalf("expected fallback regex title assertion to convert with match, got:\n%s", got)
	}
	if strings.Contains(got, "cy.url().should('include', /dashboard") {
		t.Fatalf("expected fallback path not to downgrade regex URL assertions to include, got:\n%s", got)
	}
}

func TestExecutePlaywrightToCypressDirectory_RenamesSpecFilesToCy(t *testing.T) {
	t.Parallel()

	sourceDir := t.TempDir()
	outputDir := filepath.Join(t.TempDir(), "converted")
	testPath := filepath.Join(sourceDir, "dashboard.spec.ts")
	helperPath := filepath.Join(sourceDir, "support.ts")
	if err := os.WriteFile(testPath, []byte("test('works', async ({ page }) => { await page.goto('/'); await expect(page.locator('.ok')).toBeVisible(); });\n"), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}
	if err := os.WriteFile(helperPath, []byte("export const support = true;\n"), 0o644); err != nil {
		t.Fatalf("write helper file: %v", err)
	}

	direction, ok := LookupDirection("playwright", "cypress")
	if !ok {
		t.Fatal("expected playwright -> cypress direction to exist")
	}

	result, err := Execute(sourceDir, direction, ExecuteOptions{Output: outputDir})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.Mode != "directory" {
		t.Fatalf("mode = %q, want directory", result.Mode)
	}

	convertedTest, err := os.ReadFile(filepath.Join(outputDir, "dashboard.cy.ts"))
	if err != nil {
		t.Fatalf("read converted test: %v", err)
	}
	if !strings.Contains(string(convertedTest), "cy.visit('/')") {
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
