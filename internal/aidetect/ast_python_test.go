package aidetect

import (
	"testing"
)

func TestDetectPythonAISurfaces_OpenAIClient(t *testing.T) {
	t.Parallel()
	src := []byte(`
from openai import OpenAI

client = OpenAI()

def summarize(text):
    response = client.chat.completions.create(
        model="gpt-4o-mini",
        messages=[{"role": "user", "content": text}],
    )
    return response.choices[0].message.content
`)
	hits := DetectPythonAISurfaces(src, "api/summarize.py")
	if len(hits) == 0 {
		t.Fatal("expected at least one hit")
	}

	var found *AICallSite
	for i, h := range hits {
		if h.Method == "client.chat.completions.create" {
			found = &hits[i]
			break
		}
	}
	if found == nil {
		t.Fatalf("expected client.chat.completions.create call, got %+v", hits)
	}
	// Shape-based detection: client is bound to OpenAI() but the AST
	// detector resolves SDK identity via the call shape since `client`
	// itself isn't tracked through assignment. This is the correct
	// confidence for that path.
	if found.SDK != "openai" {
		t.Errorf("SDK = %q, want openai", found.SDK)
	}
	if found.Model != "gpt-4o-mini" {
		t.Errorf("model = %q, want gpt-4o-mini", found.Model)
	}
	if found.Path != "api/summarize.py" {
		t.Errorf("path = %q", found.Path)
	}
	if found.Line == 0 {
		t.Error("expected non-zero line")
	}
}

func TestDetectPythonAISurfaces_AnthropicSDK(t *testing.T) {
	t.Parallel()
	src := []byte(`
import anthropic

ant = anthropic.Anthropic()
reply = ant.messages.create(
    model="claude-opus-4-7",
    max_tokens=1024,
    messages=[{"role": "user", "content": "Hello"}],
)
`)
	hits := DetectPythonAISurfaces(src, "agent.py")
	// Two calls: anthropic.Anthropic() and ant.messages.create()
	if len(hits) < 2 {
		t.Fatalf("expected ≥2 hits, got %d: %+v", len(hits), hits)
	}
	var ctor, messages *AICallSite
	for i, h := range hits {
		switch h.Method {
		case "anthropic.Anthropic":
			ctor = &hits[i]
		case "ant.messages.create":
			messages = &hits[i]
		}
	}
	if ctor == nil {
		t.Error("missing anthropic.Anthropic() ctor hit")
	} else if ctor.SDK != "anthropic" || ctor.Confidence < 0.9 {
		t.Errorf("ctor: sdk=%q conf=%v, want anthropic / ≥0.9", ctor.SDK, ctor.Confidence)
	}
	if messages == nil {
		t.Error("missing ant.messages.create() hit")
	} else if messages.SDK != "anthropic" {
		t.Errorf("messages.create sdk=%q, want anthropic", messages.SDK)
	} else if messages.Model != "claude-opus-4-7" {
		t.Errorf("messages.create model=%q, want claude-opus-4-7", messages.Model)
	}
}

func TestDetectPythonAISurfaces_LangChainInvoke(t *testing.T) {
	t.Parallel()
	src := []byte(`
from langchain.chat_models import ChatOpenAI
from langchain.prompts import ChatPromptTemplate

llm = ChatOpenAI(model="gpt-4o")
prompt = ChatPromptTemplate.from_template("Summarize: {text}")
chain = prompt | llm
result = chain.invoke({"text": "lorem ipsum"})
`)
	hits := DetectPythonAISurfaces(src, "chain.py")
	// At minimum we expect a langchain hit on chain.invoke.
	var invoke *AICallSite
	for i, h := range hits {
		if h.Method == "chain.invoke" {
			invoke = &hits[i]
		}
	}
	if invoke == nil {
		t.Fatalf("expected chain.invoke hit, got %+v", hits)
	}
	if invoke.SDK != "langchain" {
		t.Errorf("SDK = %q, want langchain", invoke.SDK)
	}
}

func TestDetectPythonAISurfaces_ImportWithAlias(t *testing.T) {
	t.Parallel()
	src := []byte(`
import openai as oai

oai.ChatCompletion.create(model="gpt-3.5-turbo", messages=[])
`)
	hits := DetectPythonAISurfaces(src, "legacy.py")
	if len(hits) == 0 {
		t.Fatal("expected hits")
	}
	var found *AICallSite
	for i, h := range hits {
		if h.Method == "oai.ChatCompletion.create" {
			found = &hits[i]
		}
	}
	if found == nil {
		t.Fatalf("expected oai.ChatCompletion.create hit, got %+v", hits)
	}
	if found.SDK != "openai" {
		t.Errorf("SDK = %q, want openai (resolved via alias)", found.SDK)
	}
	if found.Confidence < 0.9 {
		t.Errorf("confidence = %v, want ≥0.9 for binding-resolved call", found.Confidence)
	}
	if found.Model != "gpt-3.5-turbo" {
		t.Errorf("model = %q", found.Model)
	}
}

func TestDetectPythonAISurfaces_DynamicModel(t *testing.T) {
	t.Parallel()
	src := []byte(`
from openai import OpenAI
import os

client = OpenAI()
client.chat.completions.create(model=os.environ["MODEL"], messages=[])
`)
	hits := DetectPythonAISurfaces(src, "dyn.py")
	var found *AICallSite
	for i, h := range hits {
		if h.Method == "client.chat.completions.create" {
			found = &hits[i]
		}
	}
	if found == nil {
		t.Fatalf("expected hit, got %+v", hits)
	}
	if found.Model != "" {
		t.Errorf("model should be empty for dynamic value, got %q", found.Model)
	}
}

func TestDetectPythonAISurfaces_NoAIImports(t *testing.T) {
	t.Parallel()
	src := []byte(`
import os
import json

def load_config(path):
    with open(path) as f:
        return json.load(f)

# This is just .invoke on a non-AI object — should NOT match.
class Runner:
    def invoke(self, ctx):
        return ctx

Runner().invoke({})
`)
	hits := DetectPythonAISurfaces(src, "plain.py")
	if len(hits) != 0 {
		t.Errorf("expected no hits in pure Python, got %+v", hits)
	}
}

func TestDetectPythonAISurfaces_EmptySource(t *testing.T) {
	t.Parallel()
	if hits := DetectPythonAISurfaces(nil, "x.py"); hits != nil {
		t.Errorf("nil source should yield nil hits, got %+v", hits)
	}
	if hits := DetectPythonAISurfaces([]byte(""), "x.py"); hits != nil {
		t.Errorf("empty source should yield nil hits, got %+v", hits)
	}
}

func TestClassifyPythonModule(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in, want string
	}{
		{"openai", "openai"},
		{"openai.types", "openai"},
		{"anthropic", "anthropic"},
		{"langchain", "langchain"},
		{"langchain_core.messages", "langchain"},
		{"langsmith", "langchain"},
		{"llama_index", "llamaindex"},
		{"llama_index.core", "llamaindex"},
		{"transformers", "huggingface"},
		{"datasets", "huggingface"},
		{"unrelated.lib", ""},
		{"", ""},
	}
	for _, tc := range cases {
		if got := classifyPythonModule(tc.in); got != tc.want {
			t.Errorf("classifyPythonModule(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
