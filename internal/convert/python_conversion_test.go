package convert

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConvertPytestToUnittestSource_ConvertsAutouseFixtureAndAssert(t *testing.T) {
	t.Parallel()

	input := `import pytest

@pytest.fixture(autouse=True)
def setup_data():
    print("setting up")

def test_example():
    assert True
`

	got, err := ConvertPytestToUnittestSource(input)
	if err != nil {
		t.Fatalf("ConvertPytestToUnittestSource returned error: %v", err)
	}
	for _, want := range []string{
		"import unittest",
		"class TestExample(unittest.TestCase):",
		"def setUp(self):",
		"print(\"setting up\")",
		"def test_example(self):",
		"self.assertTrue(True)",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in output, got:\n%s", want, got)
		}
	}
}

func TestConvertUnittestToPytestSource_ConvertsFixtureAndRaises(t *testing.T) {
	t.Parallel()

	input := `import unittest

class TestExample(unittest.TestCase):
    def setUp(self):
        print("setting up")

    def tearDown(self):
        print("cleaning up")

    def test_example(self):
        with self.assertRaises(ValueError):
            int("abc")
`

	got, err := ConvertUnittestToPytestSource(input)
	if err != nil {
		t.Fatalf("ConvertUnittestToPytestSource returned error: %v", err)
	}
	for _, want := range []string{
		"import pytest",
		"@pytest.fixture(autouse=True)",
		"def setup_teardown():",
		"yield",
		"def test_example():",
		"with pytest.raises(ValueError):",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in output, got:\n%s", want, got)
		}
	}
}

func TestConvertNose2ToPytestSource_ConvertsParamsAndAssertions(t *testing.T) {
	t.Parallel()

	input := `from nose2.tools import params
from nose.tools import assert_equal, assert_true

@params(1, 2, 3)
def test_param(value):
    assert_equal(value, value)
    assert_true(value > 0)
`

	got, err := ConvertNose2ToPytestSource(input)
	if err != nil {
		t.Fatalf("ConvertNose2ToPytestSource returned error: %v", err)
	}
	for _, want := range []string{
		"import pytest",
		"@pytest.mark.parametrize(\"value\", [1, 2, 3])",
		"assert value == value",
		"assert value > 0",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in output, got:\n%s", want, got)
		}
	}
}

func TestExecuteUnittestToPytestDirectory_ConvertsPythonFilesAndPreservesHelpers(t *testing.T) {
	t.Parallel()

	sourceDir := t.TempDir()
	outputDir := filepath.Join(t.TempDir(), "converted")
	testPath := filepath.Join(sourceDir, "test_example.py")
	helperPath := filepath.Join(sourceDir, "helpers.py")
	if err := os.WriteFile(testPath, []byte("import unittest\n\nclass TestExample(unittest.TestCase):\n    def test_math(self):\n        self.assertEqual(2 + 2, 4)\n"), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}
	if err := os.WriteFile(helperPath, []byte("VALUE = 42\n"), 0o644); err != nil {
		t.Fatalf("write helper file: %v", err)
	}

	direction, ok := LookupDirection("unittest", "pytest")
	if !ok {
		t.Fatal("expected unittest -> pytest direction to exist")
	}

	result, err := Execute(sourceDir, direction, ExecuteOptions{Output: outputDir})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.Mode != "directory" {
		t.Fatalf("mode = %q, want directory", result.Mode)
	}

	convertedTest, err := os.ReadFile(filepath.Join(outputDir, "test_example.py"))
	if err != nil {
		t.Fatalf("read converted test: %v", err)
	}
	if !strings.Contains(string(convertedTest), "assert 2 + 2 == 4") {
		t.Fatalf("expected converted pytest assert, got:\n%s", convertedTest)
	}

	convertedHelper, err := os.ReadFile(filepath.Join(outputDir, "helpers.py"))
	if err != nil {
		t.Fatalf("read copied helper: %v", err)
	}
	if string(convertedHelper) != "VALUE = 42\n" {
		t.Fatalf("expected helper file to be preserved, got:\n%s", convertedHelper)
	}
}
