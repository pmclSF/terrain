package convert

import (
	"strings"
	"testing"
)

func TestValidateConvertedOutput_DetectsLeftoverSourceFrameworkCode(t *testing.T) {
	t.Parallel()

	direction, ok := LookupDirection("cypress", "playwright")
	if !ok {
		t.Fatal("expected cypress -> playwright direction")
	}

	output := `import { test } from '@playwright/test';
test('example', async ({ page }) => {
  cy.get('#status').click()
})
`

	err := ValidateConvertedOutput("converted.spec.ts", direction, output)
	if err == nil {
		t.Fatal("expected semantic validation error, got nil")
	}
	if !strings.Contains(err.Error(), "leftover Cypress API detected") {
		t.Fatalf("expected Cypress semantic validation failure, got %v", err)
	}
}

func TestValidateConvertedOutput_IgnoresCommentsAndStrings(t *testing.T) {
	t.Parallel()

	direction, ok := LookupDirection("cypress", "playwright")
	if !ok {
		t.Fatal("expected cypress -> playwright direction")
	}

	output := `import { test } from '@playwright/test';
// cy.get('#status') should stay in comments
const keep = "cy.get('#status') should stay literal";
test('example', async ({ page }) => {
  await page.locator('#status').click();
})
`

	if err := ValidateConvertedOutput("converted.spec.ts", direction, output); err != nil {
		t.Fatalf("expected comments and strings to be ignored, got %v", err)
	}
}
