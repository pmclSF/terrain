package aidetect

import (
	"testing"
)

func TestDetectJSAISurfaces_OpenAIDefault(t *testing.T) {
	t.Parallel()
	src := []byte(`
import OpenAI from "openai";

const client = new OpenAI();

export async function summarize(text) {
  const response = await client.chat.completions.create({
    model: "gpt-4o-mini",
    messages: [{ role: "user", content: text }],
  });
  return response.choices[0].message.content;
}
`)
	hits := DetectJSAISurfaces(src, "src/api/summarize.ts")
	if len(hits) < 2 {
		t.Fatalf("expected ≥2 hits, got %d: %+v", len(hits), hits)
	}

	var ctor, call *AICallSite
	for i, h := range hits {
		switch h.Method {
		case "OpenAI":
			ctor = &hits[i]
		case "client.chat.completions.create":
			call = &hits[i]
		}
	}
	if ctor == nil {
		t.Error("missing OpenAI() constructor hit")
	} else if ctor.SDK != "openai" || ctor.Confidence < 0.9 {
		t.Errorf("ctor: sdk=%q conf=%v", ctor.SDK, ctor.Confidence)
	}
	if call == nil {
		t.Fatalf("missing chat.completions.create hit, got %+v", hits)
	}
	if call.SDK != "openai" {
		t.Errorf("call SDK = %q, want openai", call.SDK)
	}
	if call.Model != "gpt-4o-mini" {
		t.Errorf("call model = %q, want gpt-4o-mini", call.Model)
	}
}

func TestDetectJSAISurfaces_AnthropicNamedImport(t *testing.T) {
	t.Parallel()
	src := []byte(`
import { Anthropic } from "@anthropic-ai/sdk";

const ant = new Anthropic();
const reply = await ant.messages.create({
  model: "claude-opus-4-7",
  max_tokens: 1024,
  messages: [{ role: "user", content: "Hi" }],
});
`)
	hits := DetectJSAISurfaces(src, "agent.ts")
	if len(hits) < 2 {
		t.Fatalf("expected ≥2 hits, got %d", len(hits))
	}
	var msgCall *AICallSite
	for i, h := range hits {
		if h.Method == "ant.messages.create" {
			msgCall = &hits[i]
		}
	}
	if msgCall == nil {
		t.Fatalf("missing ant.messages.create, got %+v", hits)
	}
	if msgCall.SDK != "anthropic" {
		t.Errorf("SDK = %q, want anthropic", msgCall.SDK)
	}
	if msgCall.Model != "claude-opus-4-7" {
		t.Errorf("model = %q", msgCall.Model)
	}
}

func TestDetectJSAISurfaces_CommonJSRequire(t *testing.T) {
	t.Parallel()
	src := []byte(`
const OpenAI = require("openai");
const client = new OpenAI();
client.chat.completions.create({ model: "gpt-3.5-turbo", messages: [] });
`)
	hits := DetectJSAISurfaces(src, "legacy.js")
	var found *AICallSite
	for i, h := range hits {
		if h.Method == "client.chat.completions.create" {
			found = &hits[i]
		}
	}
	if found == nil {
		t.Fatalf("missing call hit, got %+v", hits)
	}
	if found.SDK != "openai" {
		t.Errorf("SDK = %q, want openai", found.SDK)
	}
	if found.Model != "gpt-3.5-turbo" {
		t.Errorf("model = %q", found.Model)
	}
}

func TestDetectJSAISurfaces_DestructuredRequire(t *testing.T) {
	t.Parallel()
	src := []byte(`
const { Anthropic } = require("@anthropic-ai/sdk");
const ant = new Anthropic();
await ant.messages.create({ model: "claude-opus-4-7", messages: [] });
`)
	hits := DetectJSAISurfaces(src, "cj.js")
	var msg *AICallSite
	for i, h := range hits {
		if h.Method == "ant.messages.create" {
			msg = &hits[i]
		}
	}
	if msg == nil || msg.SDK != "anthropic" || msg.Model != "claude-opus-4-7" {
		t.Errorf("expected anthropic claude-opus-4-7 hit, got %+v", hits)
	}
}

func TestDetectJSAISurfaces_TemplateStringNoInterp(t *testing.T) {
	t.Parallel()
	src := []byte("import OpenAI from \"openai\";\nconst client = new OpenAI();\nclient.chat.completions.create({ model: `gpt-4o`, messages: [] });\n")
	hits := DetectJSAISurfaces(src, "tpl.ts")
	var found *AICallSite
	for i, h := range hits {
		if h.Method == "client.chat.completions.create" {
			found = &hits[i]
		}
	}
	if found == nil {
		t.Fatalf("missing call hit, got %+v", hits)
	}
	if found.Model != "gpt-4o" {
		t.Errorf("model = %q, want gpt-4o (from backtick template)", found.Model)
	}
}

func TestDetectJSAISurfaces_TemplateStringWithInterp(t *testing.T) {
	t.Parallel()
	src := []byte("import OpenAI from \"openai\";\nconst v = \"4o\";\nconst client = new OpenAI();\nclient.chat.completions.create({ model: `gpt-${v}`, messages: [] });\n")
	hits := DetectJSAISurfaces(src, "tpl.ts")
	var found *AICallSite
	for i, h := range hits {
		if h.Method == "client.chat.completions.create" {
			found = &hits[i]
		}
	}
	if found == nil {
		t.Fatalf("missing call hit, got %+v", hits)
	}
	if found.Model != "" {
		t.Errorf("model = %q, want empty (interpolated template)", found.Model)
	}
}

func TestDetectJSAISurfaces_LangChainInvoke(t *testing.T) {
	t.Parallel()
	src := []byte(`
import { ChatOpenAI } from "@langchain/openai";
import { ChatPromptTemplate } from "@langchain/core/prompts";

const llm = new ChatOpenAI({ model: "gpt-4o" });
const prompt = ChatPromptTemplate.fromTemplate("Summarize: {text}");
const chain = prompt.pipe(llm);
const result = await chain.invoke({ text: "lorem ipsum" });
`)
	hits := DetectJSAISurfaces(src, "chain.ts")
	var found *AICallSite
	for i, h := range hits {
		if h.Method == "chain.invoke" {
			found = &hits[i]
		}
	}
	if found == nil {
		t.Fatalf("missing chain.invoke hit, got %+v", hits)
	}
	if found.SDK != "langchain" {
		t.Errorf("SDK = %q, want langchain", found.SDK)
	}
}

func TestDetectJSAISurfaces_NoAIImports(t *testing.T) {
	t.Parallel()
	src := []byte(`
import fs from "fs";

class Runner {
  invoke(ctx) { return ctx; }
}
new Runner().invoke({});
`)
	hits := DetectJSAISurfaces(src, "plain.ts")
	if len(hits) != 0 {
		t.Errorf("expected no hits, got %+v", hits)
	}
}

func TestDetectJSAISurfaces_NamespaceImport(t *testing.T) {
	t.Parallel()
	src := []byte(`
import * as anthropic from "@anthropic-ai/sdk";
const ant = new anthropic.Anthropic();
await ant.messages.create({ model: "claude-opus-4-7", messages: [] });
`)
	hits := DetectJSAISurfaces(src, "ns.ts")
	var ctor, msg *AICallSite
	for i, h := range hits {
		switch h.Method {
		case "anthropic.Anthropic":
			ctor = &hits[i]
		case "ant.messages.create":
			msg = &hits[i]
		}
	}
	if ctor == nil {
		t.Error("missing anthropic.Anthropic ctor hit (namespace import)")
	}
	if msg == nil {
		t.Errorf("missing messages.create hit, got %+v", hits)
	}
}

func TestClassifyJSModule(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in, want string
	}{
		{"openai", "openai"},
		{"openai/resources", "openai"},
		{"@anthropic-ai/sdk", "anthropic"},
		{"@anthropic-ai/bedrock-sdk", "anthropic"},
		{"langchain", "langchain"},
		{"langchain/chat_models", "langchain"},
		{"@langchain/core", "langchain"},
		{"@langchain/openai", "langchain"},
		{"@llamaindex/core", "llamaindex"},
		{"@huggingface/inference", "huggingface"},
		{"react", ""},
		{"", ""},
	}
	for _, tc := range cases {
		if got := classifyJSModule(tc.in); got != tc.want {
			t.Errorf("classifyJSModule(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
