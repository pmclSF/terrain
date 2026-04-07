package convert

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConvertPlaywrightToWdioSource_HooksAndAssertionsMatchFixtureShape(t *testing.T) {
	t.Parallel()

	input := `import { test, expect } from '@playwright/test';

test.describe('hooks', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/setup');
  });

  test.afterEach(async ({ page }) => {
    await page.goto('/teardown');
  });

  test('should use hooks', async ({ page }) => {
    await page.locator('#btn').click();
    await expect(page.locator('#btn')).toBeVisible();
  });
});
`

	want := `describe('hooks', () => {
  beforeEach(async () => {
    await browser.url('/setup');
  });

  afterEach(async () => {
    await browser.url('/teardown');
  });

  it('should use hooks', async () => {
    await $('#btn').click();
    await expect($('#btn')).toBeDisplayed();
  });
});
`

	got, err := ConvertPlaywrightToWdioSource(input)
	if err != nil {
		t.Fatalf("ConvertPlaywrightToWdioSource returned error: %v", err)
	}
	if got != want {
		t.Fatalf("converted output mismatch\nwant:\n%s\ngot:\n%s", want, got)
	}
}

func TestConvertPlaywrightToWdioSource_ConvertsSelectorsAndBrowserAPI(t *testing.T) {
	t.Parallel()

	input := `import { test, expect } from '@playwright/test';

test.describe('selectors', () => {
  test('should find by text', async ({ page }) => {
    await page.goto('/home');
    await page.getByText('Sign In').click();
    await page.waitForTimeout(2000);
    await page.reload();
    await page.goBack();
    await page.goForward();
  });
});
`

	got, err := ConvertPlaywrightToWdioSource(input)
	if err != nil {
		t.Fatalf("ConvertPlaywrightToWdioSource returned error: %v", err)
	}
	for _, want := range []string{
		"await browser.url('/home')",
		"await $(`*=Sign In`).click()",
		"await browser.pause(2000)",
		"await browser.refresh()",
		"await browser.back()",
		"await browser.forward()",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in output, got:\n%s", want, got)
		}
	}
}

func TestConvertPlaywrightToWdioSource_CommentsUnsupportedPatterns(t *testing.T) {
	t.Parallel()

	input := `import { test, expect } from '@playwright/test';

test('uses route', async ({ page }) => {
  await page.route('**/api/users', async () => {});
});
`

	got, err := ConvertPlaywrightToWdioSource(input)
	if err != nil {
		t.Fatalf("ConvertPlaywrightToWdioSource returned error: %v", err)
	}
	if !strings.Contains(got, "TERRAIN-TODO: manual Playwright conversion required") {
		t.Fatalf("expected TODO comment for unsupported helpers, got:\n%s", got)
	}
	if !strings.Contains(got, "// await page.route('**/api/users', async () => {});") {
		t.Fatalf("expected original unsupported line to be commented out, got:\n%s", got)
	}
}

func TestConvertPlaywrightToWdioSource_DoesNotRewriteStringsOrComments(t *testing.T) {
	t.Parallel()

	input := `// await page.locator('#save').click() should stay in comments
const note = "await page.goto('/dashboard') should stay literal";

test('keeps literals intact', async ({ page }) => {
  await page.goto('/login');
  expect(note).toContain('page.goto');
});
`

	got, err := ConvertPlaywrightToWdioSource(input)
	if err != nil {
		t.Fatalf("ConvertPlaywrightToWdioSource returned error: %v", err)
	}
	if !strings.Contains(got, "// await page.locator('#save').click() should stay in comments") {
		t.Fatalf("expected comment to stay unchanged, got:\n%s", got)
	}
	if !strings.Contains(got, `const note = "await page.goto('/dashboard') should stay literal";`) {
		t.Fatalf("expected string literal to stay unchanged, got:\n%s", got)
	}
	if !strings.Contains(got, "await browser.url('/login')") {
		t.Fatalf("expected runtime playwright call to convert, got:\n%s", got)
	}
}

func TestConvertPlaywrightToWdioSource_RemovesFixtureArgsAndConvertsGetByText(t *testing.T) {
	t.Parallel()

	input := `test.describe('selectors', () => {
  test('should find by text', async ({ page }) => {
    await page.getByText('Sign In').click();
  });
});
`

	got, err := ConvertPlaywrightToWdioSource(input)
	if err != nil {
		t.Fatalf("ConvertPlaywrightToWdioSource returned error: %v", err)
	}
	if !strings.Contains(got, "describe('selectors', async () => {") && !strings.Contains(got, "describe('selectors', () => {") {
		t.Fatalf("expected describe conversion, got:\n%s", got)
	}
	if !strings.Contains(got, "it('should find by text', async () => {") {
		t.Fatalf("expected fixture args to be removed from test callback, got:\n%s", got)
	}
	if !strings.Contains(got, "await $(`*=Sign In`).click()") {
		t.Fatalf("expected getByText conversion, got:\n%s", got)
	}
}

func TestConvertPlaywrightToWdioSource_FallbackUsesSingularSelectors(t *testing.T) {
	t.Parallel()

	input := `import { test, expect } from '@playwright/test';

test('broken fallback', async ({ page }) => {
  await page.locator('#email').fill('user@example.com');
  await page.locator('#save').click();
  await expect(page.locator('#save')).toBeVisible();
`

	got, err := ConvertPlaywrightToWdioSource(input)
	if err != nil {
		t.Fatalf("ConvertPlaywrightToWdioSource returned error: %v", err)
	}
	if strings.Contains(got, "$$('#email')") || strings.Contains(got, "$$('#save')") {
		t.Fatalf("expected fallback path to use singular WDIO selectors, got:\n%s", got)
	}
	for _, want := range []string{
		"await $('#email').setValue('user@example.com')",
		"await $('#save').click()",
		"await expect($('#save')).toBeDisplayed()",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in output, got:\n%s", want, got)
		}
	}
}

func TestConvertPlaywrightToWdioSource_CommentsFilteredContextCookieCalls(t *testing.T) {
	t.Parallel()

	input := `import { test } from '@playwright/test';

test('cookies', async ({ page }) => {
  await page.context().addCookies([{ name: 'session', value: 'abc' }]);
  await page.context().cookies(urls);
  await page.context().clearCookies({ name: 'session' });
  await page.context().clearCookies();
});
`

	got, err := ConvertPlaywrightToWdioSource(input)
	if err != nil {
		t.Fatalf("ConvertPlaywrightToWdioSource returned error: %v", err)
	}
	if !strings.Contains(got, "await browser.setCookies([{ name: 'session', value: 'abc' }])") {
		t.Fatalf("expected addCookies to convert when safe, got:\n%s", got)
	}
	if !strings.Contains(got, "await browser.deleteCookies()") {
		t.Fatalf("expected zero-arg clearCookies to convert, got:\n%s", got)
	}
	if !strings.Contains(got, "// TERRAIN-TODO: manual Playwright conversion required") {
		t.Fatalf("expected filtered cookie calls to be flagged, got:\n%s", got)
	}
	if !strings.Contains(got, "// await page.context().cookies(urls);") {
		t.Fatalf("expected filtered cookies call to be preserved as comment, got:\n%s", got)
	}
	if !strings.Contains(got, "// await page.context().clearCookies({ name: 'session' });") {
		t.Fatalf("expected filtered clearCookies call to be preserved as comment, got:\n%s", got)
	}
}

func TestConvertPlaywrightToWdioSource_ConvertsStandaloneContextCookieCalls(t *testing.T) {
	t.Parallel()

	input := `import { test } from '@playwright/test';

test('context cookies', async ({ context }) => {
  await context.addCookies([{ name: 'session', value: 'abc' }]);
  await context.cookies();
  await context.clearCookies();
});
`

	got, err := ConvertPlaywrightToWdioSource(input)
	if err != nil {
		t.Fatalf("ConvertPlaywrightToWdioSource returned error: %v", err)
	}
	if !strings.Contains(got, "await browser.setCookies([{ name: 'session', value: 'abc' }])") {
		t.Fatalf("expected standalone context.addCookies to convert, got:\n%s", got)
	}
	if !strings.Contains(got, "await browser.getCookies()") {
		t.Fatalf("expected standalone context.cookies to convert, got:\n%s", got)
	}
	if !strings.Contains(got, "await browser.deleteCookies()") {
		t.Fatalf("expected standalone context.clearCookies to convert, got:\n%s", got)
	}
}

func TestExecutePlaywrightToWdioDirectory_PreservesFileNamesAndHelpers(t *testing.T) {
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

	direction, ok := LookupDirection("playwright", "webdriverio")
	if !ok {
		t.Fatal("expected playwright -> webdriverio direction to exist")
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
	if !strings.Contains(string(convertedTest), "await browser.url('/dashboard')") {
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
