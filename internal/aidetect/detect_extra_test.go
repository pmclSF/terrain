package aidetect

import (
	"os"
	"path/filepath"
	"testing"
)

// ---------------------------------------------------------------------------
// KnownFrameworks data integrity
// ---------------------------------------------------------------------------

func TestKnownFrameworks_NoDuplicateNames(t *testing.T) {
	t.Parallel()
	seen := map[string]bool{}
	for _, sig := range KnownFrameworks {
		if seen[sig.Name] {
			t.Errorf("duplicate framework name: %q", sig.Name)
		}
		seen[sig.Name] = true
	}
}

func TestKnownFrameworks_AllHaveDetectionMethod(t *testing.T) {
	t.Parallel()
	for _, sig := range KnownFrameworks {
		if len(sig.ConfigFiles) == 0 && len(sig.DependencyKeys) == 0 && len(sig.ImportPatterns) == 0 {
			t.Errorf("framework %q has no detection method (no config files, dependency keys, or import patterns)", sig.Name)
		}
	}
}

func TestKnownFrameworks_NonEmptyNames(t *testing.T) {
	t.Parallel()
	for i, sig := range KnownFrameworks {
		if sig.Name == "" {
			t.Errorf("KnownFrameworks[%d] has empty name", i)
		}
	}
}

// ---------------------------------------------------------------------------
// deduplicateFrameworks edge cases
// ---------------------------------------------------------------------------

func TestDeduplicateFrameworks_Empty(t *testing.T) {
	t.Parallel()
	out := deduplicateFrameworks(nil)
	if len(out) != 0 {
		t.Errorf("expected 0, got %d", len(out))
	}
}

func TestDeduplicateFrameworks_SingleEntry(t *testing.T) {
	t.Parallel()
	fws := []Framework{{Name: "openai", Confidence: 0.9, Source: "dependency"}}
	out := deduplicateFrameworks(fws)
	if len(out) != 1 {
		t.Fatalf("expected 1, got %d", len(out))
	}
	if out[0].Name != "openai" {
		t.Errorf("name = %q, want openai", out[0].Name)
	}
}

func TestDeduplicateFrameworks_SortedByConfidenceThenName(t *testing.T) {
	t.Parallel()
	fws := []Framework{
		{Name: "z-framework", Confidence: 0.5},
		{Name: "a-framework", Confidence: 0.5},
		{Name: "m-framework", Confidence: 0.9},
	}
	out := deduplicateFrameworks(fws)
	if len(out) != 3 {
		t.Fatalf("expected 3, got %d", len(out))
	}
	// Highest confidence first.
	if out[0].Name != "m-framework" {
		t.Errorf("first = %q, want m-framework (highest confidence)", out[0].Name)
	}
	// Same confidence sorted alphabetically.
	if out[1].Name != "a-framework" {
		t.Errorf("second = %q, want a-framework", out[1].Name)
	}
	if out[2].Name != "z-framework" {
		t.Errorf("third = %q, want z-framework", out[2].Name)
	}
}

// ---------------------------------------------------------------------------
// detectFromSource edge cases
// ---------------------------------------------------------------------------

func TestDetect_SkipsNodeModules(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	nm := filepath.Join(root, "node_modules", "langchain")
	os.MkdirAll(nm, 0o755)
	os.WriteFile(filepath.Join(nm, "index.js"), []byte(`import { ChatOpenAI } from "@langchain/openai";`), 0o644)

	result := Detect(root)
	for _, f := range result.Frameworks {
		if f.Source == "import" {
			t.Errorf("should not detect frameworks from node_modules, found %s via %s", f.Name, f.Source)
		}
	}
}

func TestDetect_PromptAndDatasetFiles(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "src"), 0o755)
	os.WriteFile(filepath.Join(root, "src", "prompts.py"), []byte(`
from langchain import PromptTemplate
PROMPT = "You are helpful"
template = PromptTemplate(template=PROMPT)
`), 0o644)
	os.WriteFile(filepath.Join(root, "src", "data.py"), []byte(`
from datasets import load_dataset
training_data = load_dataset("squad")
`), 0o644)

	result := Detect(root)
	if len(result.PromptFiles) == 0 {
		t.Error("expected prompt files detected")
	}
	if len(result.DatasetFiles) == 0 {
		t.Error("expected dataset files detected")
	}
}

func TestDetect_PyprojectToml(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "pyproject.toml"), []byte(`
[project]
dependencies = [
    "anthropic>=0.18.0",
    "instructor>=0.4.0",
]
`), 0o644)

	result := Detect(root)
	names := map[string]bool{}
	for _, f := range result.Frameworks {
		names[f.Name] = true
	}
	if !names["anthropic"] {
		t.Error("expected anthropic detected from pyproject.toml")
	}
	if !names["instructor"] {
		t.Error("expected instructor detected from pyproject.toml")
	}
}

func TestDetect_LargeFile_Skipped(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "src"), 0o755)
	// Create a file larger than maxSourceFileSize.
	big := make([]byte, maxSourceFileSize+1)
	copy(big, []byte(`import { ChatOpenAI } from "@langchain/openai";`))
	os.WriteFile(filepath.Join(root, "src", "huge.ts"), big, 0o644)

	result := Detect(root)
	for _, f := range result.Frameworks {
		if f.Source == "import" && f.ConfigFile == "src/huge.ts" {
			t.Error("should skip files exceeding maxSourceFileSize")
		}
	}
}

// ---------------------------------------------------------------------------
// hasPromptPatterns / hasDatasetPatterns
// ---------------------------------------------------------------------------

func TestHasPromptPatterns_Positive(t *testing.T) {
	t.Parallel()
	cases := []string{
		`const tmpl = new ChatPromptTemplate("hello")`,
		`system_prompt = "You are helpful"`,
		`buildPrompt(input)`,
	}
	for _, c := range cases {
		if !hasPromptPatterns(c) {
			t.Errorf("hasPromptPatterns(%q) = false, want true", c[:30])
		}
	}
}

func TestHasPromptPatterns_Negative(t *testing.T) {
	t.Parallel()
	if hasPromptPatterns("const x = 42; console.log(x);") {
		t.Error("expected false for non-AI code")
	}
}

func TestHasDatasetPatterns_Positive(t *testing.T) {
	t.Parallel()
	cases := []string{
		`data = load_dataset("squad")`,
		`const loader = new DataLoader(batch)`,
		`training_data = prepare()`,
	}
	for _, c := range cases {
		if !hasDatasetPatterns(c) {
			t.Errorf("hasDatasetPatterns(%q) = false, want true", c[:30])
		}
	}
}

func TestHasDatasetPatterns_Negative(t *testing.T) {
	t.Parallel()
	if hasDatasetPatterns("function add(a, b) { return a + b }") {
		t.Error("expected false for non-dataset code")
	}
}

// ---------------------------------------------------------------------------
// sortedKeyList
// ---------------------------------------------------------------------------

func TestSortedKeyList_Empty(t *testing.T) {
	t.Parallel()
	out := sortedKeyList(nil)
	if len(out) != 0 {
		t.Errorf("expected empty, got %v", out)
	}
}

func TestSortedKeyList_Sorted(t *testing.T) {
	t.Parallel()
	m := map[string]bool{"c": true, "a": true, "b": true}
	out := sortedKeyList(m)
	if len(out) != 3 || out[0] != "a" || out[1] != "b" || out[2] != "c" {
		t.Errorf("expected [a b c], got %v", out)
	}
}
