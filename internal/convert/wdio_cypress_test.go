package convert

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConvertWdioToCypressSource_LoginFlowMatchesFixtureShape(t *testing.T) {
	t.Parallel()

	input := `import { browser, $, expect } from '@wdio/globals';

describe('Login Flow', () => {
  beforeEach(async () => {
    await browser.url('/login');
  });

  it('should login', async () => {
    await $('#username').setValue('admin');
    await $('#password').setValue('pass123');
    await $('#login-btn').click();
    await expect(browser).toHaveUrl('http://localhost/dashboard');
    await expect($('#welcome')).toBeDisplayed();
  });
});
`

	want := `describe('Login Flow', () => {
  beforeEach(() => {
    cy.visit('/login');
  });

  it('should login', () => {
    cy.get('#username').clear().type('admin');
    cy.get('#password').clear().type('pass123');
    cy.get('#login-btn').click();
    cy.url().should('eq', 'http://localhost/dashboard');
    cy.get('#welcome').should('be.visible');
  });
});
`

	got, err := ConvertWdioToCypressSource(input)
	if err != nil {
		t.Fatalf("ConvertWdioToCypressSource returned error: %v", err)
	}
	if got != want {
		t.Fatalf("converted output mismatch\nwant:\n%s\ngot:\n%s", want, got)
	}
}

func TestConvertWdioToCypressSource_ConvertsTextSelectorsAndNavigation(t *testing.T) {
	t.Parallel()

	input := `describe('navigation', () => {
  it('should move around', async () => {
    await browser.url('/home');
    await $('*=Submit').click();
    await browser.refresh();
    await browser.back();
    await browser.forward();
    await browser.pause(2000);
  });
});
`

	got, err := ConvertWdioToCypressSource(input)
	if err != nil {
		t.Fatalf("ConvertWdioToCypressSource returned error: %v", err)
	}
	for _, want := range []string{
		"cy.visit('/home')",
		"cy.contains('Submit').click()",
		"cy.reload()",
		"cy.go('back')",
		"cy.go('forward')",
		"cy.wait(2000)",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in output, got:\n%s", want, got)
		}
	}
}

func TestConvertWdioToCypressSource_CommentsUnsupportedPatterns(t *testing.T) {
	t.Parallel()

	input := `it('uses unsupported helpers', async () => {
  await browser.mock('**/api/users');
});
`

	got, err := ConvertWdioToCypressSource(input)
	if err != nil {
		t.Fatalf("ConvertWdioToCypressSource returned error: %v", err)
	}
	if !strings.Contains(got, "TERRAIN-TODO: manual WebdriverIO conversion required") {
		t.Fatalf("expected TODO comment for unsupported helpers, got:\n%s", got)
	}
	if !strings.Contains(got, "// await browser.mock('**/api/users');") {
		t.Fatalf("expected original unsupported line to be commented out, got:\n%s", got)
	}
}

func TestExecuteWdioToCypressDirectory_PreservesFileNamesAndHelpers(t *testing.T) {
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

	direction, ok := LookupDirection("webdriverio", "cypress")
	if !ok {
		t.Fatal("expected webdriverio -> cypress direction to exist")
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
	if !strings.Contains(string(convertedTest), "cy.visit('/login')") {
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
