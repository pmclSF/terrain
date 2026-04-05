package convert

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEstimateMigration_PytestProject(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	testDir := filepath.Join(root, "tests")
	if err := os.MkdirAll(testDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(testDir, "test_example.py"), []byte("import pytest\n\ndef test_example():\n    assert True\n"), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(testDir, "helpers.py"), []byte("VALUE = 42\n"), 0o644); err != nil {
		t.Fatalf("write helper file: %v", err)
	}

	estimate, err := EstimateMigration(root, "pytest", "unittest")
	if err != nil {
		t.Fatalf("EstimateMigration returned error: %v", err)
	}
	if estimate.Summary.TotalFiles != 2 {
		t.Fatalf("total files = %d, want 2", estimate.Summary.TotalFiles)
	}
	if estimate.Summary.TestFiles != 1 {
		t.Fatalf("test files = %d, want 1", estimate.Summary.TestFiles)
	}
	if estimate.Summary.HelperFiles != 1 {
		t.Fatalf("helper files = %d, want 1", estimate.Summary.HelperFiles)
	}
	if estimate.Summary.PredictedHigh == 0 {
		t.Fatal("expected at least one high-confidence file")
	}
}

func TestMigrateProject_WritesOutputsStateAndChecklist(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	testDir := filepath.Join(root, "tests")
	outputDir := filepath.Join(root, "converted")
	if err := os.MkdirAll(testDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	input := `import pytest

@pytest.fixture(autouse=True)
def setup_data():
    print("setting up")

def test_example():
    assert True
`
	if err := os.WriteFile(filepath.Join(testDir, "test_example.py"), []byte(input), 0o644); err != nil {
		t.Fatalf("write input: %v", err)
	}

	result, err := MigrateProject(root, "pytest", "unittest", MigrationRunOptions{
		Output:      outputDir,
		Concurrency: 2,
	})
	if err != nil {
		t.Fatalf("MigrateProject returned error: %v", err)
	}
	if result.State.Converted != 1 {
		t.Fatalf("converted = %d, want 1", result.State.Converted)
	}

	output, err := os.ReadFile(filepath.Join(outputDir, "tests", "test_example.py"))
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	text := string(output)
	if !strings.Contains(text, "class TestExample(unittest.TestCase):") {
		t.Fatalf("expected unittest output, got:\n%s", text)
	}
	if !strings.Contains(text, "def setUp(self):") {
		t.Fatalf("expected setUp conversion, got:\n%s", text)
	}

	statePath := filepath.Join(root, ".terrain", "migration", "state.json")
	if _, err := os.Stat(statePath); err != nil {
		t.Fatalf("expected state file at %s: %v", statePath, err)
	}
	if !strings.Contains(result.Checklist, "# Migration Checklist") {
		t.Fatalf("expected checklist header, got:\n%s", result.Checklist)
	}
}

func TestMigrateProject_ConvertsConfigAlongsideTests(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	testDir := filepath.Join(root, "tests")
	outputDir := filepath.Join(root, "converted")
	if err := os.MkdirAll(testDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "jest.config.js"), []byte(`module.exports = { testEnvironment: 'node' };`), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(testDir, "auth.test.js"), []byte("describe('auth', () => { expect(true).toBe(true) })\n"), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	result, err := MigrateProject(root, "jest", "vitest", MigrationRunOptions{
		Output:         outputDir,
		Concurrency:    2,
		StrictValidate: true,
	})
	if err != nil {
		t.Fatalf("MigrateProject returned error: %v", err)
	}
	if result.State.Converted != 2 {
		t.Fatalf("converted = %d, want 2", result.State.Converted)
	}

	configOutput := filepath.Join(outputDir, "vitest.config.ts")
	content, err := os.ReadFile(configOutput)
	if err != nil {
		t.Fatalf("read config output: %v", err)
	}
	if !strings.Contains(string(content), "defineConfig") {
		t.Fatalf("expected vitest config output, got:\n%s", content)
	}
}

func TestRunMigrationDoctor_ReturnsChecks(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "package.json"), []byte(`{"devDependencies":{"pytest":"8.0.0"}}`), 0o644); err != nil {
		t.Fatalf("write package.json: %v", err)
	}
	testDir := filepath.Join(root, "tests")
	if err := os.MkdirAll(testDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(testDir, "test_example.py"), []byte("import pytest\n\ndef test_example():\n    assert True\n"), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	result, err := RunMigrationDoctor(root)
	if err != nil {
		t.Fatalf("RunMigrationDoctor returned error: %v", err)
	}
	if len(result.Checks) < 4 {
		t.Fatalf("expected several doctor checks, got %d", len(result.Checks))
	}
	if result.Summary.Total != len(result.Checks) {
		t.Fatalf("summary total = %d, want %d", result.Summary.Total, len(result.Checks))
	}
}
