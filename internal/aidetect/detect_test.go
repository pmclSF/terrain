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

func TestDetect_ClassicalML(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "src"), 0o755)

	os.WriteFile(filepath.Join(root, "src", "train_sklearn.py"), []byte(`
from sklearn.ensemble import RandomForestClassifier
from sklearn.metrics import f1_score
import joblib

clf = RandomForestClassifier(n_estimators=100)
joblib.dump(clf, "model.joblib")
`), 0o644)

	os.WriteFile(filepath.Join(root, "src", "train_xgb.py"), []byte(`
import xgboost as xgb
model = xgb.XGBClassifier()
`), 0o644)

	os.WriteFile(filepath.Join(root, "src", "torch_model.py"), []byte(`
import torch
from torch import nn
class Net(nn.Module): pass
`), 0o644)

	os.WriteFile(filepath.Join(root, "src", "tf_pipeline.py"), []byte(`
import tensorflow as tf
model = tf.keras.Sequential()
`), 0o644)

	result := Detect(root)
	got := map[string]bool{}
	for _, fw := range result.Frameworks {
		got[fw.Name] = true
	}
	for _, want := range []string{"sklearn", "xgboost", "pytorch", "tensorflow"} {
		if !got[want] {
			t.Errorf("expected %s framework detected, frameworks: %v", want, frameworkNames(result.Frameworks))
		}
	}
}

func frameworkNames(fws []Framework) []string {
	out := make([]string, len(fws))
	for i, f := range fws {
		out[i] = f.Name
	}
	return out
}

func TestDetect_GoAndJavaSDK(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	// Go: detection via go.mod dependency entry.
	os.WriteFile(filepath.Join(root, "go.mod"), []byte(`module example.com/demo

go 1.23

require github.com/sashabaranov/go-openai v1.20.0
`), 0o644)
	// Java: detection via Maven pom.xml dependency entry.
	os.WriteFile(filepath.Join(root, "pom.xml"), []byte(`<?xml version="1.0"?>
<project>
  <modelVersion>4.0.0</modelVersion>
  <groupId>com.example</groupId>
  <artifactId>demo</artifactId>
  <version>1.0.0</version>
  <dependencies>
    <dependency>
      <groupId>com.theokanning.openai-gpt3-java</groupId>
      <artifactId>service</artifactId>
      <version>0.18.0</version>
    </dependency>
  </dependencies>
</project>
`), 0o644)

	result := Detect(root)
	got := map[string]bool{}
	for _, fw := range result.Frameworks {
		got[fw.Name] = true
	}
	if !got["openai-go"] {
		t.Errorf("expected openai-go detected from go.mod, frameworks: %v", frameworkNames(result.Frameworks))
	}
	if !got["openai-java"] {
		t.Errorf("expected openai-java detected from pom.xml, frameworks: %v", frameworkNames(result.Frameworks))
	}
}

func TestDetect_HuggingFaceLLM(t *testing.T) {
	t.Parallel()
	// File that uses the LLM-specific entry point should fire both
	// "huggingface" (broad) AND "huggingface-llm" (narrow).
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "src"), 0o755)
	os.WriteFile(filepath.Join(root, "src", "generate.py"), []byte(`
from transformers import AutoModelForCausalLM, AutoTokenizer

model = AutoModelForCausalLM.from_pretrained("mistralai/Mistral-7B-v0.1")
tokenizer = AutoTokenizer.from_pretrained("mistralai/Mistral-7B-v0.1")
`), 0o644)

	result := Detect(root)
	got := map[string]bool{}
	for _, fw := range result.Frameworks {
		got[fw.Name] = true
	}
	if !got["huggingface"] {
		t.Error("expected huggingface (broad) detected")
	}
	if !got["huggingface-llm"] {
		t.Errorf("expected huggingface-llm (narrow) detected, frameworks: %v", frameworkNames(result.Frameworks))
	}
}

func TestDetect_HuggingFaceNonLLM(t *testing.T) {
	t.Parallel()
	// File that uses only non-LLM transformers should fire "huggingface"
	// but NOT "huggingface-llm".
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "src"), 0o755)
	os.WriteFile(filepath.Join(root, "src", "embed.py"), []byte(`
from transformers import AutoModel, AutoTokenizer

model = AutoModel.from_pretrained("bert-base-uncased")
tokenizer = AutoTokenizer.from_pretrained("bert-base-uncased")
`), 0o644)

	result := Detect(root)
	got := map[string]bool{}
	for _, fw := range result.Frameworks {
		got[fw.Name] = true
	}
	if !got["huggingface"] {
		t.Error("expected huggingface detected for BERT embedding usage")
	}
	if got["huggingface-llm"] {
		t.Errorf("did NOT expect huggingface-llm for BERT-only file, frameworks: %v", frameworkNames(result.Frameworks))
	}
}

func TestDetect_Pydantic(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "src"), 0o755)
	os.WriteFile(filepath.Join(root, "src", "models.py"), []byte(`
from pydantic import BaseModel, Field

class User(BaseModel):
    name: str = Field(min_length=1)
    age: int
`), 0o644)

	result := Detect(root)
	got := map[string]bool{}
	for _, fw := range result.Frameworks {
		got[fw.Name] = true
	}
	if !got["pydantic"] {
		t.Errorf("expected pydantic detected, frameworks: %v", frameworkNames(result.Frameworks))
	}
}

func TestDetect_CallSites_PythonAndJS(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "src"), 0o755)

	os.WriteFile(filepath.Join(root, "src", "inference.py"), []byte(`
from openai import OpenAI
client = OpenAI()
client.chat.completions.create(model="gpt-4o-mini", messages=[])
`), 0o644)

	os.WriteFile(filepath.Join(root, "src", "agent.ts"), []byte(`
import { Anthropic } from "@anthropic-ai/sdk";
const ant = new Anthropic();
ant.messages.create({ model: "claude-opus-4-7", messages: [] });
`), 0o644)

	result := Detect(root)
	if len(result.CallSites) < 2 {
		t.Fatalf("expected ≥2 call sites, got %d: %+v", len(result.CallSites), result.CallSites)
	}

	bySDK := map[string]int{}
	byModel := map[string]string{}
	for _, cs := range result.CallSites {
		bySDK[cs.SDK]++
		if cs.Model != "" {
			byModel[cs.SDK] = cs.Model
		}
	}
	if bySDK["openai"] == 0 {
		t.Error("expected at least one openai call site")
	}
	if bySDK["anthropic"] == 0 {
		t.Error("expected at least one anthropic call site")
	}
	if byModel["openai"] != "gpt-4o-mini" {
		t.Errorf("openai model = %q, want gpt-4o-mini", byModel["openai"])
	}
	if byModel["anthropic"] != "claude-opus-4-7" {
		t.Errorf("anthropic model = %q, want claude-opus-4-7", byModel["anthropic"])
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

	scenarios := DeriveEvals(root, detection, surfaces, testFiles)
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
	scenarios := DeriveEvals(root, detection, nil, nil)
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

	scenarios := DeriveEvals(root, detection, surfaces, nil)

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

	scenarios := DeriveEvals(root, detection, surfaces, testFiles)

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
