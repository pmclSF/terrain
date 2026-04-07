package aidetect

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pmclSF/terrain/internal/models"
)

func TestDetect_EmptyDir(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	result := Detect(root)
	if len(result.Frameworks) != 0 {
		t.Errorf("expected 0 frameworks, got %d", len(result.Frameworks))
	}
}

func TestDetect_PackageJSON_LangChain(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "package.json"), []byte(`{
		"dependencies": {"@langchain/core": "^0.1.0", "openai": "^4.0.0"}
	}`), 0o644)

	result := Detect(root)
	names := map[string]bool{}
	for _, f := range result.Frameworks {
		names[f.Name] = true
	}
	if !names["langchain"] {
		t.Error("expected langchain detected from package.json")
	}
	if !names["openai"] {
		t.Error("expected openai detected from package.json")
	}
}

func TestDetect_PythonDeps(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "requirements.txt"), []byte("deepeval>=0.21\nragas>=0.1\nlangchain\n"), 0o644)

	result := Detect(root)
	names := map[string]bool{}
	for _, f := range result.Frameworks {
		names[f.Name] = true
	}
	if !names["deepeval"] {
		t.Error("expected deepeval detected from requirements.txt")
	}
	if !names["ragas"] {
		t.Error("expected ragas detected from requirements.txt")
	}
	if !names["langchain"] {
		t.Error("expected langchain detected from requirements.txt")
	}
}

func TestDetect_ConfigFile_Promptfoo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "promptfooconfig.yaml"), []byte("prompts:\n  - prompt1\n"), 0o644)

	result := Detect(root)
	if len(result.Frameworks) == 0 {
		t.Fatal("expected frameworks detected")
	}
	if result.Frameworks[0].Name != "promptfoo" {
		t.Errorf("expected promptfoo, got %s", result.Frameworks[0].Name)
	}
	if result.Frameworks[0].Source != "config" {
		t.Errorf("expected config source, got %s", result.Frameworks[0].Source)
	}
}

func TestDetect_SourceImports(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "src"), 0o755)
	os.WriteFile(filepath.Join(root, "src", "chain.ts"), []byte(`
import { ChatOpenAI } from "@langchain/openai";
import Anthropic from "@anthropic-ai/sdk";
`), 0o644)

	result := Detect(root)
	names := map[string]bool{}
	for _, f := range result.Frameworks {
		names[f.Name] = true
	}
	if !names["langchain"] {
		t.Error("expected langchain from import")
	}
	if !names["anthropic"] {
		t.Error("expected anthropic from import")
	}
}

func TestDetect_ModelFiles(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "src"), 0o755)
	os.WriteFile(filepath.Join(root, "src", "inference.py"), []byte(`
from openai import OpenAI
client = OpenAI()
response = client.chat.completions.create(model="gpt-4")
`), 0o644)

	result := Detect(root)
	if len(result.ModelFiles) == 0 {
		t.Error("expected model files detected")
	}
}

func TestDeriveScenarios_FromEvalDir(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	// Create eval test file that imports a prompt surface.
	os.MkdirAll(filepath.Join(root, "tests", "eval", "safety"), 0o755)
	os.MkdirAll(filepath.Join(root, "src", "prompts"), 0o755)

	os.WriteFile(filepath.Join(root, "src", "prompts", "builder.py"), []byte(`
def build_safety_prompt(text):
    return "Safety: " + text
`), 0o644)

	os.WriteFile(filepath.Join(root, "tests", "eval", "safety", "test_safety.py"), []byte(`
from src.prompts.builder import build_safety_prompt

def test_safe_input():
    assert build_safety_prompt("hello")
`), 0o644)

	detection := Detect(root)
	surfaces := []models.CodeSurface{
		{SurfaceID: "surface:src/prompts/builder.py:build_safety_prompt", Name: "build_safety_prompt", Path: "src/prompts/builder.py", Kind: models.SurfacePrompt},
	}
	testFiles := []models.TestFile{
		{Path: "tests/eval/safety/test_safety.py", Framework: "pytest"},
	}

	scenarios := DeriveScenarios(root, detection, surfaces, testFiles)
	if len(scenarios) == 0 {
		t.Fatal("expected at least 1 auto-derived scenario")
	}

	found := false
	for _, sc := range scenarios {
		if sc.Category == "safety" {
			found = true
			if len(sc.CoveredSurfaceIDs) == 0 {
				t.Error("scenario should have linked surfaces")
			}
		}
	}
	if !found {
		t.Error("expected safety category scenario from eval/safety/ directory")
	}
}

func TestDeriveScenarios_EmptyRepo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	detection := Detect(root)
	scenarios := DeriveScenarios(root, detection, nil, nil)
	if len(scenarios) != 0 {
		t.Errorf("expected 0 scenarios for empty repo, got %d", len(scenarios))
	}
}

func TestDeriveScenarios_FromPromptfooConfig(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	// Create a promptfoo config with test descriptions.
	os.MkdirAll(filepath.Join(root, "src"), 0o755)
	// Note: deriveFromPromptfooConfig matches lines starting with "description:"
	// after trimming, so we use the indented key format (not YAML list item format).
	os.WriteFile(filepath.Join(root, "promptfooconfig.yaml"), []byte(`prompts:
  - "You are a helpful assistant. {{query}}"
tests:
  - vars:
      query: "hello"
    assert:
      - type: contains
        value: "hello"
    description: "Basic Q&A correctness"
  - vars:
      query: "harm me"
    assert:
      - type: not-contains
        value: "harmful"
    description: "Safety boundary test"
`), 0o644)

	os.WriteFile(filepath.Join(root, "src", "prompt.ts"), []byte(`
export const systemPrompt = "You are a helpful assistant";
`), 0o644)

	detection := Detect(root)
	surfaces := []models.CodeSurface{
		{SurfaceID: "surface:src/prompt.ts:systemPrompt", Name: "systemPrompt", Path: "src/prompt.ts", Kind: models.SurfacePrompt},
	}

	scenarios := DeriveScenarios(root, detection, surfaces, nil)

	// Should derive scenarios from promptfoo config descriptions.
	promptfooScenarios := 0
	for _, sc := range scenarios {
		if sc.Framework == "promptfoo" {
			promptfooScenarios++
			if len(sc.CoveredSurfaceIDs) == 0 {
				t.Error("promptfoo scenario should link to prompt surfaces")
			}
			if sc.Path != "promptfooconfig.yaml" {
				t.Errorf("expected path promptfooconfig.yaml, got %q", sc.Path)
			}
		}
	}
	if promptfooScenarios != 2 {
		t.Errorf("expected 2 promptfoo scenarios, got %d", promptfooScenarios)
	}
}

func TestDeriveScenarios_FromAIImports(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	// Create a test file that imports an AI library but is NOT in an eval directory.
	os.MkdirAll(filepath.Join(root, "tests", "unit"), 0o755)
	os.MkdirAll(filepath.Join(root, "src"), 0o755)

	os.WriteFile(filepath.Join(root, "src", "chat.ts"), []byte(`
import OpenAI from "openai";
export const client = new OpenAI();
`), 0o644)

	os.WriteFile(filepath.Join(root, "tests", "unit", "chat.test.ts"), []byte(`
import OpenAI from "openai";
import { client } from "../../src/chat";

describe("chat", () => {
  it("calls openai", async () => {
    const resp = await client.chat.completions.create({model: "gpt-4"});
    expect(resp).toBeDefined();
  });
});
`), 0o644)

	// Create a package.json so OpenAI framework is detected.
	os.WriteFile(filepath.Join(root, "package.json"), []byte(`{
		"dependencies": {"openai": "^4.0.0"}
	}`), 0o644)

	detection := Detect(root)
	surfaces := []models.CodeSurface{
		{SurfaceID: "surface:src/chat.ts:client", Name: "client", Path: "src/chat.ts", Kind: models.SurfacePrompt},
	}
	testFiles := []models.TestFile{
		{Path: "tests/unit/chat.test.ts", Framework: "jest"},
	}

	scenarios := DeriveScenarios(root, detection, surfaces, testFiles)

	// The test file imports "openai" but is NOT in an eval directory,
	// so it should be picked up by deriveFromAIImports.
	found := false
	for _, sc := range scenarios {
		if sc.Path == "tests/unit/chat.test.ts" {
			found = true
			if sc.Framework != "openai" {
				t.Errorf("expected framework openai, got %q", sc.Framework)
			}
		}
	}
	if !found {
		t.Error("expected scenario derived from AI import in non-eval test file")
	}
}

func TestClassifyScenarioCategory(t *testing.T) {
	t.Parallel()
	cases := []struct {
		path    string
		content string
		want    string
	}{
		{"tests/eval/test_safety.py", "check for harmful output", "safety"},
		{"tests/eval/test_accuracy.py", "precision recall f1", "accuracy"},
		{"tests/eval/test_regression.py", "compare to baseline", "regression"},
		{"tests/eval/test_bias.py", "fairness metric", "bias"},
		{"tests/eval/test_latency.py", "performance benchmark", "performance"},
		{"tests/eval/test_general.py", "run the eval", "eval"},
		{"tests/eval/toxicity_check.py", "toxic content filter", "safety"},
	}
	for _, tc := range cases {
		got := classifyScenarioCategory(tc.path, tc.content)
		if got != tc.want {
			t.Errorf("classifyScenarioCategory(%q, %q) = %q, want %q", tc.path, tc.content, got, tc.want)
		}
	}
}

func TestDeduplicateFrameworks(t *testing.T) {
	t.Parallel()
	fws := []Framework{
		{Name: "openai", Confidence: 0.75, Source: "import"},
		{Name: "openai", Confidence: 0.9, Source: "dependency"},
		{Name: "langchain", Confidence: 0.8, Source: "import"},
	}
	deduped := deduplicateFrameworks(fws)
	if len(deduped) != 2 {
		t.Fatalf("expected 2 unique frameworks, got %d", len(deduped))
	}
	// Highest confidence should win.
	for _, f := range deduped {
		if f.Name == "openai" && f.Confidence != 0.9 {
			t.Errorf("expected openai confidence 0.9 (dependency wins), got %f", f.Confidence)
		}
	}
}
