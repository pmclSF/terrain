package convert

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConvertTestCafeToCypressSource_ConvertsFixtureActionsAndAssertions(t *testing.T) {
	t.Parallel()

	input := `import { Selector } from 'testcafe';

fixture` + "`" + `Checkout` + "`" + `.page` + "`" + `/checkout` + "`" + `;

test('submits', async t => {
  await t.click(Selector('#submit'));
  await t.typeText(Selector('#email'), 'user@test.com');
  await t.expect(Selector('.notice').visible).ok();
});
`

	got, err := ConvertTestCafeToCypressSource(input)
	if err != nil {
		t.Fatalf("ConvertTestCafeToCypressSource returned error: %v", err)
	}
	for _, want := range []string{
		"/// <reference types=\"cypress\" />",
		"describe('Checkout', () => {",
		"beforeEach(() => {",
		"cy.visit('/checkout')",
		"it('submits', () => {",
		"cy.get('#submit').click()",
		"cy.get('#email').type('user@test.com')",
		"cy.get('.notice').should('be.visible')",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in output, got:\n%s", want, got)
		}
	}
	if strings.Contains(got, "from 'testcafe'") {
		t.Fatalf("expected TestCafe import to be removed, got:\n%s", got)
	}
}

func TestExecuteTestCafeToCypressDirectory_PreservesFileNamesAndHelpers(t *testing.T) {
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

	direction, ok := LookupDirection("testcafe", "cypress")
	if !ok {
		t.Fatal("expected testcafe -> cypress direction to exist")
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
	if !strings.Contains(string(convertedTest), "cy.get('#submit').click()") {
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

func TestConvertTestCafeToCypressSource_HandlesSelectorParensAndPreservesComments(t *testing.T) {
	t.Parallel()

	input := `import { Selector } from 'testcafe';

fixture` + "`" + `Checkout` + "`" + `;

test('complex selector', async t => {
  // Selector('.btn:nth-child(2)') should stay in this comment
  const note = "Selector('.btn:nth-child(2)') is only documentation";
  await t.click(Selector('.btn:nth-child(2)').find('.label'));
});
`

	got, err := ConvertTestCafeToCypressSource(input)
	if err != nil {
		t.Fatalf("ConvertTestCafeToCypressSource returned error: %v", err)
	}
	if !strings.Contains(got, "// Selector('.btn:nth-child(2)') should stay in this comment") {
		t.Fatalf("expected comment to be preserved, got:\n%s", got)
	}
	if !strings.Contains(got, `const note = "Selector('.btn:nth-child(2)') is only documentation"`) {
		t.Fatalf("expected string literal to remain unchanged, got:\n%s", got)
	}
	if !strings.Contains(got, "cy.get('.btn:nth-child(2)').find('.label').click()") {
		t.Fatalf("expected nested selector with parens to convert, got:\n%s", got)
	}
}

func TestConvertTestCafeToCypressSource_CommentsUnsupportedUseRole(t *testing.T) {
	t.Parallel()

	input := `import { Role } from 'testcafe';

fixture` + "`" + `Checkout` + "`" + `;

test('role', async t => {
  await t.useRole(adminRole);
});
`

	got, err := ConvertTestCafeToCypressSource(input)
	if err != nil {
		t.Fatalf("ConvertTestCafeToCypressSource returned error: %v", err)
	}
	if !strings.Contains(got, "// TERRAIN-TODO: manual TestCafe conversion required") {
		t.Fatalf("expected unsupported useRole line to be commented, got:\n%s", got)
	}
	if !strings.Contains(got, "// await t.useRole(adminRole);") {
		t.Fatalf("expected original useRole line to be preserved as comment, got:\n%s", got)
	}
}

func TestConvertTestCafeToCypressSource_SupportsLiteralFixtureCallSyntax(t *testing.T) {
	t.Parallel()

	input := `import { Selector } from 'testcafe';

fixture('Checkout').page('/checkout');

test('opens', async t => {
  await t.click(Selector('#submit'));
});
`

	got, err := ConvertTestCafeToCypressSource(input)
	if err != nil {
		t.Fatalf("ConvertTestCafeToCypressSource returned error: %v", err)
	}
	for _, want := range []string{
		"describe('Checkout', () => {",
		"beforeEach(() => {",
		"cy.visit('/checkout')",
		"cy.get('#submit').click()",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in output, got:\n%s", want, got)
		}
	}
	if strings.Contains(got, "fixture('Checkout').page('/checkout')") {
		t.Fatalf("expected fixture call syntax to be removed, got:\n%s", got)
	}
}
