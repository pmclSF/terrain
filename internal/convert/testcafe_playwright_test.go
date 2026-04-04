package convert

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConvertTestCafeToPlaywrightSource_ConvertsFixtureActionsAndAssertions(t *testing.T) {
	t.Parallel()

	input := `import { Selector } from 'testcafe';

fixture` + "`" + `Checkout` + "`" + `.page` + "`" + `/checkout` + "`" + `;

test('submits', async t => {
  await t.click(Selector('#submit'));
  await t.typeText(Selector('#email'), 'user@test.com');
  await t.expect(Selector('.notice').visible).ok();
});
`

	got, err := ConvertTestCafeToPlaywrightSource(input)
	if err != nil {
		t.Fatalf("ConvertTestCafeToPlaywrightSource returned error: %v", err)
	}
	for _, want := range []string{
		"import { test, expect } from '@playwright/test';",
		"test.describe('Checkout', () => {",
		"test.beforeEach(async ({ page }) => {",
		"await page.goto('/checkout')",
		"test('submits', async ({ page }) => {",
		"await page.locator('#submit').click()",
		"await page.locator('#email').fill('user@test.com')",
		"await expect(page.locator('.notice')).toBeVisible()",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in output, got:\n%s", want, got)
		}
	}
	if strings.Contains(got, "from 'testcafe'") {
		t.Fatalf("expected TestCafe import to be removed, got:\n%s", got)
	}
}

func TestExecuteTestCafeToPlaywrightDirectory_PreservesFileNamesAndHelpers(t *testing.T) {
	t.Parallel()

	sourceDir := t.TempDir()
	outputDir := filepath.Join(t.TempDir(), "converted")
	testPath := filepath.Join(sourceDir, "checkout.test.js")
	helperPath := filepath.Join(sourceDir, "support.js")
	input := `import { Selector } from 'testcafe';

fixture` + "`" + `Checkout` + "`" + `;

test('opens', async t => {
  await t.click(Selector('#submit'));
});
`
	if err := os.WriteFile(testPath, []byte(input), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}
	if err := os.WriteFile(helperPath, []byte("export const support = true;\n"), 0o644); err != nil {
		t.Fatalf("write helper file: %v", err)
	}

	direction, ok := LookupDirection("testcafe", "playwright")
	if !ok {
		t.Fatal("expected testcafe -> playwright direction to exist")
	}

	result, err := Execute(sourceDir, direction, ExecuteOptions{Output: outputDir})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.Mode != "directory" {
		t.Fatalf("mode = %q, want directory", result.Mode)
	}

	convertedTest, err := os.ReadFile(filepath.Join(outputDir, "checkout.test.js"))
	if err != nil {
		t.Fatalf("read converted test: %v", err)
	}
	if !strings.Contains(string(convertedTest), "await page.locator('#submit').click()") {
		t.Fatalf("expected converted TestCafe test, got:\n%s", convertedTest)
	}

	convertedHelper, err := os.ReadFile(filepath.Join(outputDir, "support.js"))
	if err != nil {
		t.Fatalf("read copied helper: %v", err)
	}
	if string(convertedHelper) != "export const support = true;\n" {
		t.Fatalf("expected helper file to be preserved, got:\n%s", convertedHelper)
	}
}

func TestConvertTestCafeToPlaywrightSource_HandlesSelectorParensAndPreservesComments(t *testing.T) {
	t.Parallel()

	input := `import { Selector } from 'testcafe';

fixture` + "`" + `Checkout` + "`" + `;

test('complex selector', async t => {
  // Selector('.btn:nth-child(2)') should stay in this comment
  const note = "Selector('.btn:nth-child(2)') is only documentation";
  await t.click(Selector('.btn:nth-child(2)'));
});
`

	got, err := ConvertTestCafeToPlaywrightSource(input)
	if err != nil {
		t.Fatalf("ConvertTestCafeToPlaywrightSource returned error: %v", err)
	}
	if !strings.Contains(got, "// Selector('.btn:nth-child(2)') should stay in this comment") {
		t.Fatalf("expected comment to be preserved, got:\n%s", got)
	}
	if !strings.Contains(got, `const note = "Selector('.btn:nth-child(2)') is only documentation"`) {
		t.Fatalf("expected string literal to remain unchanged, got:\n%s", got)
	}
	if !strings.Contains(got, "await page.locator('.btn:nth-child(2)').click()") {
		t.Fatalf("expected selector with nested parens to convert, got:\n%s", got)
	}
}

func TestConvertTestCafeToPlaywrightSource_CommentsUnsupportedUseRole(t *testing.T) {
	t.Parallel()

	input := `import { Role } from 'testcafe';

fixture` + "`" + `Checkout` + "`" + `;

test('role', async t => {
  await t.useRole(adminRole);
});
`

	got, err := ConvertTestCafeToPlaywrightSource(input)
	if err != nil {
		t.Fatalf("ConvertTestCafeToPlaywrightSource returned error: %v", err)
	}
	if !strings.Contains(got, "// TERRAIN-TODO: manual TestCafe conversion required") {
		t.Fatalf("expected unsupported useRole line to be commented, got:\n%s", got)
	}
	if !strings.Contains(got, "// await t.useRole(adminRole);") {
		t.Fatalf("expected original useRole line to be preserved as comment, got:\n%s", got)
	}
}
