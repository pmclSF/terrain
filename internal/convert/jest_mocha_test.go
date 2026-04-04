package convert

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConvertJestToMochaSource_ConvertsAssertionsMocksAndHooks(t *testing.T) {
	t.Parallel()

	input := `describe('timers', () => {
  beforeAll(() => {
    jest.useFakeTimers();
  });

  afterAll(() => {
    jest.useRealTimers();
  });

  it('tracks calls', () => {
    const fn = jest.fn();
    fn();
    jest.advanceTimersByTime(1000);
    expect(fn).toHaveBeenCalled();
    expect(true).toBe(true);
    expect({ a: 1 }).toEqual({ a: 1 });
  });
});
`

	got, err := ConvertJestToMochaSource(input)
	if err != nil {
		t.Fatalf("ConvertJestToMochaSource returned error: %v", err)
	}
	for _, want := range []string{
		"const { expect } = require('chai');",
		"const sinon = require('sinon');",
		"before(() => {",
		"after(() => {",
		"sinon.useFakeTimers()",
		"clock.tick(1000)",
		"const fn = sinon.stub()",
		"expect(fn).to.have.been.called",
		"expect(true).to.be.true",
		"expect({ a: 1 }).to.deep.equal({ a: 1 })",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in output, got:\n%s", want, got)
		}
	}
}

func TestExecuteJestToMochaDirectory_PreservesFileNamesAndHelpers(t *testing.T) {
	t.Parallel()

	sourceDir := t.TempDir()
	outputDir := filepath.Join(t.TempDir(), "converted")
	testPath := filepath.Join(sourceDir, "service.test.js")
	helperPath := filepath.Join(sourceDir, "support.js")
	input := `describe('service', () => {
  it('works', () => {
    expect(true).toBe(true);
  });
});
`
	if err := os.WriteFile(testPath, []byte(input), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}
	if err := os.WriteFile(helperPath, []byte("module.exports = { support: true };\n"), 0o644); err != nil {
		t.Fatalf("write helper file: %v", err)
	}

	direction, ok := LookupDirection("jest", "mocha")
	if !ok {
		t.Fatal("expected jest -> mocha direction to exist")
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
	if !strings.Contains(string(convertedTest), "expect(true).to.be.true") {
		t.Fatalf("expected converted jest test, got:\n%s", convertedTest)
	}

	convertedHelper, err := os.ReadFile(filepath.Join(outputDir, "support.js"))
	if err != nil {
		t.Fatalf("read copied helper: %v", err)
	}
	if string(convertedHelper) != "module.exports = { support: true };\n" {
		t.Fatalf("expected helper file to be preserved, got:\n%s", convertedHelper)
	}
}
