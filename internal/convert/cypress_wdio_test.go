package convert

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConvertCypressToWdioSource_LoginFlowMatchesFixtureShape(t *testing.T) {
	t.Parallel()

	input := `describe('Login Flow', () => {
  beforeEach(() => {
    cy.visit('/login');
  });

  it('should login', () => {
    cy.get('#username').type('admin');
    cy.get('#password').type('pass123');
    cy.get('#login-btn').click();
    cy.url().should('eq', 'http://localhost/dashboard');
    cy.get('#welcome').should('be.visible');
  });
});
`

	want := `describe('Login Flow', () => {
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

	got, err := ConvertCypressToWdioSource(input)
	if err != nil {
		t.Fatalf("ConvertCypressToWdioSource returned error: %v", err)
	}
	if got != want {
		t.Fatalf("converted output mismatch\nwant:\n%s\ngot:\n%s", want, got)
	}
}

func TestConvertCypressToWdioSource_ConvertsTextSelectorsAndNavigation(t *testing.T) {
	t.Parallel()

	input := `describe('navigation', () => {
  it('should move around', () => {
    cy.visit('/home');
    cy.contains('Submit').click();
    cy.reload();
    cy.go('back');
    cy.go('forward');
    cy.wait(2000);
  });
});
`

	got, err := ConvertCypressToWdioSource(input)
	if err != nil {
		t.Fatalf("ConvertCypressToWdioSource returned error: %v", err)
	}
	for _, want := range []string{
		"await browser.url('/home')",
		"await $(`*=Submit`).click()",
		"await browser.refresh()",
		"await browser.back()",
		"await browser.forward()",
		"await browser.pause(2000)",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in output, got:\n%s", want, got)
		}
	}
}

func TestConvertCypressToWdioSource_CommentsUnsupportedPatterns(t *testing.T) {
	t.Parallel()

	input := `it('uses unsupported helpers', () => {
  cy.intercept('GET', '/api/users').as('users');
});
`

	got, err := ConvertCypressToWdioSource(input)
	if err != nil {
		t.Fatalf("ConvertCypressToWdioSource returned error: %v", err)
	}
	if !strings.Contains(got, "TERRAIN-TODO: manual Cypress conversion required") {
		t.Fatalf("expected TODO comment for unsupported helpers, got:\n%s", got)
	}
	if !strings.Contains(got, "// cy.intercept('GET', '/api/users').as('users');") {
		t.Fatalf("expected original unsupported line to be commented out, got:\n%s", got)
	}
}

func TestExecuteCypressToWdioDirectory_PreservesFileNamesAndHelpers(t *testing.T) {
	t.Parallel()

	sourceDir := t.TempDir()
	outputDir := filepath.Join(t.TempDir(), "converted")
	testPath := filepath.Join(sourceDir, "checkout.cy.js")
	helperPath := filepath.Join(sourceDir, "support.js")
	if err := os.WriteFile(testPath, []byte("it('works', () => { cy.visit('/'); cy.get('.btn').click(); });\n"), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}
	if err := os.WriteFile(helperPath, []byte("export const support = true;\n"), 0o644); err != nil {
		t.Fatalf("write helper file: %v", err)
	}

	direction, ok := LookupDirection("cypress", "webdriverio")
	if !ok {
		t.Fatal("expected cypress -> webdriverio direction to exist")
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
	if !strings.Contains(string(convertedTest), "await browser.url('/')") {
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

func TestConvertCypressToWdioSource_HandlesSelectorParensAndPreservesComments(t *testing.T) {
	t.Parallel()

	input := `describe('selectors', () => {
  it('keeps prose intact', () => {
    // cy.get('.btn:nth-child(2)').click() should stay in this comment
    const note = "cy.get('.btn:nth-child(2)').click() is only documentation";
    cy.get('.btn:nth-child(2)').click();
  });
});
`

	got, err := ConvertCypressToWdioSource(input)
	if err != nil {
		t.Fatalf("ConvertCypressToWdioSource returned error: %v", err)
	}
	if !strings.Contains(got, "// cy.get('.btn:nth-child(2)').click() should stay in this comment") {
		t.Fatalf("expected comment to be preserved, got:\n%s", got)
	}
	if !strings.Contains(got, `const note = "cy.get('.btn:nth-child(2)').click() is only documentation"`) {
		t.Fatalf("expected string literal to remain unchanged, got:\n%s", got)
	}
	if !strings.Contains(got, "await $('.btn:nth-child(2)').click()") {
		t.Fatalf("expected selector with nested parens to convert, got:\n%s", got)
	}
}
