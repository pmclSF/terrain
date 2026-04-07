package convert

import "testing"

func TestUnsupportedPlaywrightLineRowsAST_OnlyMarksRealUnsupportedCalls(t *testing.T) {
	t.Parallel()

	source := `// const pending = page.waitForEvent('download')
const note = "page.route('**/api') should stay literal";
test('uses unsupported helpers', async ({ page }) => {
  const download = page.waitForEvent('download');
  await page.goto('/ok');
});`

	rows, ok := unsupportedPlaywrightLineRowsAST(source)
	if !ok {
		t.Fatal("unsupportedPlaywrightLineRowsAST returned ok=false")
	}
	if len(rows) != 1 {
		t.Fatalf("rows len = %d, want 1 (%v)", len(rows), rows)
	}
	if !rows[3] {
		t.Fatalf("expected unsupported row 3 to be marked, got %v", rows)
	}
}
