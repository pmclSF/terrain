package convert

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConvertMochaToJestSource_ConvertsAssertionsMocksAndHooks(t *testing.T) {
	t.Parallel()

	input := `const { expect } = require('chai');
const sinon = require('sinon');

describe('service', () => {
  before(() => {
    // setup
  });

  after(() => {
    sinon.restore();
  });

  it('tracks calls', () => {
    const fn = sinon.stub();
    fn();
    sinon.assert.calledOnce(fn);
    expect({ a: 1 }).to.deep.equal({ a: 1 });
    expect([1, 2, 3]).to.have.lengthOf(3);
  });
});
`

	got, err := ConvertMochaToJestSource(input)
	if err != nil {
		t.Fatalf("ConvertMochaToJestSource returned error: %v", err)
	}
	for _, want := range []string{
		"beforeAll(() => {",
		"afterAll(() => {",
		"jest.restoreAllMocks()",
		"const fn = jest.fn()",
		"expect(fn).toHaveBeenCalledTimes(1)",
		"expect({ a: 1 }).toEqual({ a: 1 })",
		"expect([1, 2, 3]).toHaveLength(3)",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in output, got:\n%s", want, got)
		}
	}
	if strings.Contains(got, "require('chai')") || strings.Contains(got, "require('sinon')") {
		t.Fatalf("expected Mocha prelude imports to be removed, got:\n%s", got)
	}
}

func TestExecuteMochaToJestDirectory_PreservesFileNamesAndHelpers(t *testing.T) {
	t.Parallel()

	sourceDir := t.TempDir()
	outputDir := filepath.Join(t.TempDir(), "converted")
	testPath := filepath.Join(sourceDir, "service.test.js")
	helperPath := filepath.Join(sourceDir, "support.js")
	input := `const { expect } = require('chai');

describe('service', () => {
  it('works', () => {
    expect(true).to.be.true;
  });
});
`
	if err := os.WriteFile(testPath, []byte(input), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}
	if err := os.WriteFile(helperPath, []byte("module.exports = { support: true };\n"), 0o644); err != nil {
		t.Fatalf("write helper file: %v", err)
	}

	direction, ok := LookupDirection("mocha", "jest")
	if !ok {
		t.Fatal("expected mocha -> jest direction to exist")
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
	if !strings.Contains(string(convertedTest), "expect(true).toBe(true)") {
		t.Fatalf("expected converted mocha test, got:\n%s", convertedTest)
	}

	convertedHelper, err := os.ReadFile(filepath.Join(outputDir, "support.js"))
	if err != nil {
		t.Fatalf("read copied helper: %v", err)
	}
	if string(convertedHelper) != "module.exports = { support: true };\n" {
		t.Fatalf("expected helper file to be preserved, got:\n%s", convertedHelper)
	}
}
