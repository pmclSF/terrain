package convert

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConvertJestToJasmineSource_ConvertsSpiesAndTimers(t *testing.T) {
	t.Parallel()

	input := `describe('timers', () => {
  beforeEach(() => {
    jest.useFakeTimers();
  });

  afterEach(() => {
    jest.useRealTimers();
  });

  it('tracks calls', () => {
    const fn = jest.fn();
    setTimeout(fn, 1000);
    jest.advanceTimersByTime(1000);
    expect(expect.any(String)).toBeDefined();
  });
});
`

	got, err := ConvertJestToJasmineSource(input)
	if err != nil {
		t.Fatalf("ConvertJestToJasmineSource returned error: %v", err)
	}
	for _, want := range []string{
		"jasmine.clock().install()",
		"jasmine.clock().uninstall()",
		"const fn = jasmine.createSpy()",
		"jasmine.clock().tick(1000)",
		"jasmine.any(String)",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in output, got:\n%s", want, got)
		}
	}
}

func TestExecuteJestToJasmineDirectory_PreservesFileNamesAndHelpers(t *testing.T) {
	t.Parallel()

	sourceDir := t.TempDir()
	outputDir := filepath.Join(t.TempDir(), "converted")
	testPath := filepath.Join(sourceDir, "service.test.js")
	helperPath := filepath.Join(sourceDir, "support.js")
	input := `describe('service', () => {
  it('works', () => {
    const fn = jest.fn();
    fn();
    expect(fn).toHaveBeenCalled();
  });
});
`
	if err := os.WriteFile(testPath, []byte(input), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}
	if err := os.WriteFile(helperPath, []byte("export const support = true;\n"), 0o644); err != nil {
		t.Fatalf("write helper file: %v", err)
	}

	direction, ok := LookupDirection("jest", "jasmine")
	if !ok {
		t.Fatal("expected jest -> jasmine direction to exist")
	}

	result, err := Execute(sourceDir, direction, ExecuteOptions{Output: outputDir})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.Mode != "directory" {
		t.Fatalf("mode = %q, want directory", result.Mode)
	}

	convertedTest, err := os.ReadFile(filepath.Join(outputDir, "service.test.js"))
	if err != nil {
		t.Fatalf("read converted test: %v", err)
	}
	if !strings.Contains(string(convertedTest), "const fn = jasmine.createSpy()") {
		t.Fatalf("expected converted jest test, got:\n%s", convertedTest)
	}

	convertedHelper, err := os.ReadFile(filepath.Join(outputDir, "support.js"))
	if err != nil {
		t.Fatalf("read copied helper: %v", err)
	}
	if string(convertedHelper) != "export const support = true;\n" {
		t.Fatalf("expected helper file to be preserved, got:\n%s", convertedHelper)
	}
}

func TestConvertJestToJasmineSource_DoesNotRewriteStringsOrComments(t *testing.T) {
	t.Parallel()

	input := `describe('notes', () => {
  it('leaves prose alone', () => {
    // jest.useFakeTimers() should stay in this comment
    const note = "jest.spyOn(service, 'get') is only documentation";
    const matcher = "expect.any(String)";
    const fn = jest.fn().mockImplementation(() => true);
    expect(fn()).toBe(true);
    expect(note).toContain("jest.spyOn(service, 'get')");
    expect(matcher).toContain("expect.any(String)");
  });
});
`

	got, err := ConvertJestToJasmineSource(input)
	if err != nil {
		t.Fatalf("ConvertJestToJasmineSource returned error: %v", err)
	}
	if !strings.Contains(got, "// jest.useFakeTimers() should stay in this comment") {
		t.Fatalf("expected comment to be preserved, got:\n%s", got)
	}
	if !strings.Contains(got, `const note = "jest.spyOn(service, 'get') is only documentation"`) {
		t.Fatalf("expected string literal to remain unchanged, got:\n%s", got)
	}
	if !strings.Contains(got, `const matcher = "expect.any(String)"`) {
		t.Fatalf("expected matcher string to remain unchanged, got:\n%s", got)
	}
	if !strings.Contains(got, "const fn = jasmine.createSpy().and.callFake(() => true)") {
		t.Fatalf("expected real jest.fn call to convert, got:\n%s", got)
	}
}

func TestConvertJestToJasmineSource_CommentsUnsupportedModuleMock(t *testing.T) {
	t.Parallel()

	input := `describe('mocks', () => {
  it('flags manual work', () => {
    jest.mock('./service');
  });
});
`

	got, err := ConvertJestToJasmineSource(input)
	if err != nil {
		t.Fatalf("ConvertJestToJasmineSource returned error: %v", err)
	}
	if !strings.Contains(got, "// TERRAIN-TODO: manual Jest module mock conversion required") {
		t.Fatalf("expected jest.mock to be commented for manual review, got:\n%s", got)
	}
	if !strings.Contains(got, "// jest.mock('./service');") {
		t.Fatalf("expected original jest.mock line to be preserved as comment, got:\n%s", got)
	}
}
