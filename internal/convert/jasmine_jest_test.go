package convert

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConvertJasmineToJestSource_ConvertsSpiesAndTimers(t *testing.T) {
	t.Parallel()

	input := `describe('clock', () => {
  beforeEach(() => {
    jasmine.clock().install();
  });

  afterEach(() => {
    jasmine.clock().uninstall();
  });

  it('tracks spies', () => {
    const service = jasmine.createSpyObj('service', ['get', 'post']);
    spyOn(service, 'get').and.returnValue('ok');
    expect(jasmine.any(String)).toBeDefined();
  });
});
`

	got, err := ConvertJasmineToJestSource(input)
	if err != nil {
		t.Fatalf("ConvertJasmineToJestSource returned error: %v", err)
	}
	for _, want := range []string{
		"jest.useFakeTimers()",
		"jest.useRealTimers()",
		"const service = { get: jest.fn(), post: jest.fn() }",
		"jest.spyOn(service, 'get').mockReturnValue('ok')",
		"expect.any(String)",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in output, got:\n%s", want, got)
		}
	}
}

func TestExecuteJasmineToJestDirectory_PreservesFileNamesAndHelpers(t *testing.T) {
	t.Parallel()

	sourceDir := t.TempDir()
	outputDir := filepath.Join(t.TempDir(), "converted")
	testPath := filepath.Join(sourceDir, "clock.spec.js")
	helperPath := filepath.Join(sourceDir, "support.js")
	input := `describe('clock', () => {
  it('ticks', () => {
    const fn = jasmine.createSpy('fn');
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

	direction, ok := LookupDirection("jasmine", "jest")
	if !ok {
		t.Fatal("expected jasmine -> jest direction to exist")
	}

	result, err := Execute(sourceDir, direction, ExecuteOptions{Output: outputDir})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.Mode != "directory" {
		t.Fatalf("mode = %q, want directory", result.Mode)
	}

	convertedTest, err := os.ReadFile(filepath.Join(outputDir, "clock.spec.js"))
	if err != nil {
		t.Fatalf("read converted test: %v", err)
	}
	if !strings.Contains(string(convertedTest), "const fn = jest.fn()") {
		t.Fatalf("expected converted jasmine test, got:\n%s", convertedTest)
	}

	convertedHelper, err := os.ReadFile(filepath.Join(outputDir, "support.js"))
	if err != nil {
		t.Fatalf("read copied helper: %v", err)
	}
	if string(convertedHelper) != "export const support = true;\n" {
		t.Fatalf("expected helper file to be preserved, got:\n%s", convertedHelper)
	}
}
