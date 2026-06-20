package analysis

import (
	"fmt"
	"strings"
	"testing"

	"github.com/pmclSF/terrain/internal/models"
)

// --- JS/TS AST tests ---

func TestASTJS_MessageArray(t *testing.T) {
	t.Parallel()
	src := `
const messages = [
  { role: "system", content: "You are a helpful coding assistant." },
  { role: "user", content: userQuery },
  { role: "assistant", content: "I can help with that." },
];
`
	surfaces := ParsePromptAST("src/chat.ts", src, "js")

	found := findSurfaceByPrefix(surfaces, "message_array")
	if found == nil {
		t.Fatalf("expected message_array surface, got %v", surfaceNames(surfaces))
	}
	if found.Kind != models.SurfaceContext {
		t.Errorf("kind: want context (has system role), got %s", found.Kind)
	}
	if found.DetectionTier != models.TierStructural {
		t.Errorf("tier: want structural, got %s", found.DetectionTier)
	}
	if found.Confidence < 0.95 {
		t.Errorf("confidence: want >= 0.95, got %.2f", found.Confidence)
	}
	if !strings.Contains(found.Reason, DetectorASTMessageArray) {
		t.Errorf("reason should contain detector ID, got: %s", found.Reason)
	}
	if !strings.Contains(found.Reason, "3 objects") {
		t.Errorf("reason should mention object count, got: %s", found.Reason)
	}
}

func TestASTJS_MessageArrayNoSystemRole(t *testing.T) {
	t.Parallel()
	src := `
const conversation = [
  { role: "user", content: "Hello" },
  { role: "assistant", content: "Hi there!" },
];
`
	surfaces := ParsePromptAST("src/convo.ts", src, "js")

	found := findSurfaceByPrefix(surfaces, "message_array")
	if found == nil {
		t.Fatalf("expected message_array surface, got %v", surfaceNames(surfaces))
	}
	// No system role → SurfacePrompt, not SurfaceContext.
	if found.Kind != models.SurfacePrompt {
		t.Errorf("kind: want prompt (no system role), got %s", found.Kind)
	}
}

func TestASTJS_FrameworkConstructors(t *testing.T) {
	t.Parallel()
	src := `
import { SystemMessage, HumanMessage } from "@langchain/core/messages";

const messages = [
  new SystemMessage("You are a helpful assistant."),
  new HumanMessage(userQuery),
];
`
	surfaces := ParsePromptAST("src/chain.ts", src, "js")

	var systemFound, humanFound bool
	for _, s := range surfaces {
		if strings.Contains(s.Name, "SystemMessage") {
			systemFound = true
			if s.Kind != models.SurfaceContext {
				t.Errorf("SystemMessage kind: want context, got %s", s.Kind)
			}
		}
		if strings.Contains(s.Name, "HumanMessage") {
			humanFound = true
			if s.Kind != models.SurfacePrompt {
				t.Errorf("HumanMessage kind: want prompt, got %s", s.Kind)
			}
		}
	}
	if !systemFound {
		t.Error("expected SystemMessage constructor to be detected")
	}
	if !humanFound {
		t.Error("expected HumanMessage constructor to be detected")
	}
}

func TestASTJS_TemplateFactory(t *testing.T) {
	t.Parallel()
	src := `
const prompt = ChatPromptTemplate.fromMessages([
  ["system", "You are a {role} assistant."],
  ["human", "{input}"],
]);
`
	surfaces := ParsePromptAST("src/template.ts", src, "js")

	found := findSurfaceByPrefix(surfaces, "template_prompt")
	if found == nil {
		t.Fatalf("expected template_prompt surface, got %v", surfaceNames(surfaces))
	}
	if found.Kind != models.SurfacePrompt {
		t.Errorf("kind: want prompt, got %s", found.Kind)
	}
	if found.Confidence < 0.93 {
		t.Errorf("confidence: want >= 0.93, got %.2f", found.Confidence)
	}
}

func TestASTJS_SystemPromptAssignment(t *testing.T) {
	t.Parallel()
	src := `
import OpenAI from 'openai';
const systemPrompt = "You are a helpful assistant. Your role is to answer questions accurately.";
`
	surfaces := ParsePromptAST("src/config.ts", src, "js")

	found := findSurfaceByPrefix(surfaces, "system_prompt")
	if found == nil {
		t.Fatalf("expected system_prompt surface, got %v", surfaceNames(surfaces))
	}
	if found.Kind != models.SurfaceContext {
		t.Errorf("kind: want context, got %s", found.Kind)
	}
	if !strings.Contains(found.Reason, DetectorASTSystemPrompt) {
		t.Errorf("reason should contain detector ID, got: %s", found.Reason)
	}
}

func TestASTJS_TemplateLiteralPrompt(t *testing.T) {
	t.Parallel()
	src := "import OpenAI from 'openai';\nconst chatPrompt = `You are a helpful coding assistant. Your task is to help the user write clean code.\nGiven the context, always respond with clear explanations.`;\n"

	surfaces := ParsePromptAST("src/prompts.ts", src, "js")

	found := findSurfaceByPrefix(surfaces, "template_prompt")
	if found == nil {
		t.Fatalf("expected template_prompt surface, got %v", surfaceNames(surfaces))
	}
	if found.Kind != models.SurfacePrompt {
		t.Errorf("kind: want prompt, got %s", found.Kind)
	}
}

func TestASTJS_OpenAICall(t *testing.T) {
	t.Parallel()
	src := `
const response = await openai.chat.completions.create({
  model: "gpt-4",
  messages: [
    { role: "system", content: systemPrompt },
    { role: "user", content: userMessage },
  ],
});
`
	surfaces := ParsePromptAST("src/api.ts", src, "js")

	found := findSurfaceByPrefix(surfaces, "api_prompt")
	if found == nil {
		t.Fatalf("expected api_prompt surface, got %v", surfaceNames(surfaces))
	}
	if found.Kind != models.SurfacePrompt {
		t.Errorf("kind: want prompt, got %s", found.Kind)
	}
}

func TestASTJS_RejectsNonAIStrings(t *testing.T) {
	t.Parallel()
	src := `
const emailTemplate = "Hello {name}, your order {orderId} has been shipped.";
const htmlTemplate = "<div class='container'><h1>Welcome</h1></div>";
const sqlQuery = "SELECT * FROM users WHERE id = ?";
const config = { apiKey: "sk-12345", baseUrl: "https://api.example.com" };
`
	surfaces := ParsePromptAST("src/templates.ts", src, "js")

	if len(surfaces) != 0 {
		t.Errorf("expected 0 surfaces for non-AI strings, got %d: %v",
			len(surfaces), surfaceNames(surfaces))
	}
}

func TestASTJS_RejectsShortStrings(t *testing.T) {
	t.Parallel()
	src := `const systemPrompt = "Hi";`

	surfaces := ParsePromptAST("src/short.ts", src, "js")
	if len(surfaces) != 0 {
		t.Errorf("expected 0 surfaces for short strings, got %d", len(surfaces))
	}
}

// --- Python AST tests ---

func TestASTPython_MessageList(t *testing.T) {
	t.Parallel()
	src := `
messages = [
    {"role": "system", "content": "You are a helpful assistant."},
    {"role": "user", "content": user_input},
]
`
	surfaces := ParsePromptAST("src/chat.py", src, "python")

	found := findSurfaceByPrefix(surfaces, "message_list")
	if found == nil {
		t.Fatalf("expected message_list surface, got %v", surfaceNames(surfaces))
	}
	if found.Kind != models.SurfaceContext {
		t.Errorf("kind: want context, got %s", found.Kind)
	}
	if found.Confidence < 0.95 {
		t.Errorf("confidence: want >= 0.95, got %.2f", found.Confidence)
	}
}

func TestASTPython_FrameworkConstructors(t *testing.T) {
	t.Parallel()
	src := `
from langchain.schema import SystemMessage, HumanMessage

messages = [
    SystemMessage(content="You are a helpful assistant."),
    HumanMessage(content=user_query),
]
`
	surfaces := ParsePromptAST("src/chain.py", src, "python")

	var systemFound bool
	for _, s := range surfaces {
		if strings.Contains(s.Name, "SystemMessage") {
			systemFound = true
			if s.Kind != models.SurfaceContext {
				t.Errorf("SystemMessage kind: want context, got %s", s.Kind)
			}
		}
	}
	if !systemFound {
		t.Error("expected SystemMessage constructor to be detected")
	}
}

func TestASTPython_TemplateFactory(t *testing.T) {
	t.Parallel()
	src := `
from langchain.prompts import ChatPromptTemplate

prompt = ChatPromptTemplate.from_messages([
    ("system", "You are a {role} assistant."),
    ("human", "{input}"),
])
`
	surfaces := ParsePromptAST("src/template.py", src, "python")

	found := findSurfaceByPrefix(surfaces, "template_prompt")
	if found == nil {
		t.Fatalf("expected template_prompt surface, got %v", surfaceNames(surfaces))
	}
	if found.Kind != models.SurfacePrompt {
		t.Errorf("kind: want prompt, got %s", found.Kind)
	}
	if found.Confidence < 0.93 {
		t.Errorf("confidence: want >= 0.93, got %.2f", found.Confidence)
	}
}

func TestASTPython_TripleQuotePrompt(t *testing.T) {
	t.Parallel()
	src := `
import openai

system_prompt = """You are a helpful assistant.
Your role is to answer questions accurately.
Always respond with clear explanations.
Do not make up information."""
`
	surfaces := ParsePromptAST("src/prompts.py", src, "python")

	found := findSurfaceByPrefix(surfaces, "system_prompt")
	if found == nil {
		// Could also be template_prompt.
		found = findSurfaceByPrefix(surfaces, "template_prompt")
	}
	if found == nil {
		t.Fatalf("expected system_prompt or template_prompt surface, got %v", surfaceNames(surfaces))
	}
	if found.Kind != models.SurfaceContext {
		t.Errorf("kind: want context, got %s", found.Kind)
	}
}

func TestASTPython_FewShotArray(t *testing.T) {
	t.Parallel()
	src := `
import openai

examples = [
    {"input": "What is 2+2?", "output": "4"},
    {"input": "What is the capital of France?", "output": "Paris"},
    {"input": "Who wrote Hamlet?", "output": "Shakespeare"},
]
`
	surfaces := ParsePromptAST("src/examples.py", src, "python")

	found := findSurfaceByPrefix(surfaces, "few_shot")
	if found == nil {
		t.Fatalf("expected few_shot surface, got %v", surfaceNames(surfaces))
	}
	if found.Confidence < 0.90 {
		t.Errorf("confidence: want >= 0.90, got %.2f", found.Confidence)
	}
	if !strings.Contains(found.Reason, DetectorASTFewShot) {
		t.Errorf("reason should contain detector ID, got: %s", found.Reason)
	}
}

func TestASTPython_FewShotQuestionAnswer(t *testing.T) {
	t.Parallel()
	src := `
import openai

qa_examples = [
    {"question": "What is gravity?", "answer": "A fundamental force of nature."},
    {"question": "What is DNA?", "answer": "Deoxyribonucleic acid."},
]
`
	surfaces := ParsePromptAST("src/qa.py", src, "python")

	found := findSurfaceByPrefix(surfaces, "few_shot")
	if found == nil {
		t.Fatalf("expected few_shot surface, got %v", surfaceNames(surfaces))
	}
}

func TestASTPython_RejectsNonAILists(t *testing.T) {
	t.Parallel()
	emailA := "alice" + "@" + "example.test"
	emailB := "bob" + "@" + "example.test"
	src := fmt.Sprintf(`
users = [
    {"name": "Alice", "email": %q},
    {"name": "Bob", "email": %q},
]

config = {"database": "postgres", "port": 5432}
items = [1, 2, 3, 4, 5]
`, emailA, emailB)
	surfaces := ParsePromptAST("src/data.py", src, "python")

	if len(surfaces) != 0 {
		t.Errorf("expected 0 surfaces for non-AI data, got %d: %v",
			len(surfaces), surfaceNames(surfaces))
	}
}

func TestASTPython_OpenAICall(t *testing.T) {
	t.Parallel()
	src := `
response = client.chat.completions.create(
    model="gpt-4",
    messages=[
        {"role": "system", "content": system_prompt},
        {"role": "user", "content": user_message},
    ],
)
`
	surfaces := ParsePromptAST("src/api.py", src, "python")

	found := findSurfaceByPrefix(surfaces, "api_prompt")
	if found == nil {
		t.Fatalf("expected api_prompt surface, got %v", surfaceNames(surfaces))
	}
}

// --- Go AST tests ---

func TestASTGo_MessageSlice(t *testing.T) {
	t.Parallel()
	src := `package chat

type Message struct {
	Role    string
	Content string
}

var messages = []Message{
	{Role: "system", Content: "You are a helpful assistant."},
	{Role: "user", Content: userQuery},
}
`
	surfaces := ParsePromptAST("src/chat.go", src, "go")

	found := findSurfaceByPrefix(surfaces, "message_slice")
	if found == nil {
		t.Fatalf("expected message_slice surface, got %v", surfaceNames(surfaces))
	}
	if found.Kind != models.SurfaceContext {
		t.Errorf("kind: want context, got %s", found.Kind)
	}
	if found.Confidence < 0.95 {
		t.Errorf("confidence: want >= 0.95, got %.2f", found.Confidence)
	}
}

func TestASTGo_PromptConst(t *testing.T) {
	t.Parallel()
	src := "package ai\n\nconst SystemPrompt = `You are a helpful assistant. Your role is to answer questions. Always respond clearly.`\n"

	surfaces := ParsePromptAST("src/ai.go", src, "go")

	found := findSurfaceByPrefix(surfaces, "prompt_const")
	if found == nil {
		t.Fatalf("expected prompt_const surface, got %v", surfaceNames(surfaces))
	}
	if found.Kind != models.SurfaceContext {
		t.Errorf("kind: want context, got %s", found.Kind)
	}
}

func TestASTGo_RejectsNonAI(t *testing.T) {
	t.Parallel()
	src := `package main

var greeting = "Hello, World!"
var config = map[string]string{"key": "value"}
`
	surfaces := ParsePromptAST("src/main.go", src, "go")

	if len(surfaces) != 0 {
		t.Errorf("expected 0 surfaces for non-AI code, got %d: %v",
			len(surfaces), surfaceNames(surfaces))
	}
}

// --- Evidence metadata tests ---

func TestASTPrompt_EvidenceMetadata(t *testing.T) {
	t.Parallel()
	src := `
const messages = [
  { role: "system", content: "You are helpful." },
  { role: "user", content: input },
];
`
	surfaces := ParsePromptAST("src/chat.ts", src, "js")

	for _, s := range surfaces {
		if s.DetectionTier == "" {
			t.Errorf("surface %q missing DetectionTier", s.Name)
		}
		if s.Confidence == 0 {
			t.Errorf("surface %q has zero Confidence", s.Name)
		}
		if s.Reason == "" {
			t.Errorf("surface %q missing Reason", s.Name)
		}
		if s.SurfaceID == "" {
			t.Errorf("surface %q missing SurfaceID", s.Name)
		}
		if s.Path == "" {
			t.Errorf("surface %q missing Path", s.Name)
		}
		if s.Line == 0 {
			t.Errorf("surface %q missing Line", s.Name)
		}
	}
}

func TestASTPrompt_KindAssignment(t *testing.T) {
	t.Parallel()

	// System message → SurfaceContext.
	src1 := `
messages = [
    {"role": "system", "content": "You are an assistant."},
    {"role": "user", "content": query},
]
`
	surfaces1 := ParsePromptAST("src/a.py", src1, "python")
	for _, s := range surfaces1 {
		if strings.Contains(s.Name, "message_list") {
			if s.Kind != models.SurfaceContext {
				t.Errorf("message array with system role: want context, got %s", s.Kind)
			}
		}
	}

	// User-only message → SurfacePrompt.
	src2 := `
const msgs = [
  { role: "user", content: "Hello" },
  { role: "assistant", content: "Hi" },
];
`
	surfaces2 := ParsePromptAST("src/b.ts", src2, "js")
	for _, s := range surfaces2 {
		if strings.Contains(s.Name, "message_array") {
			if s.Kind != models.SurfacePrompt {
				t.Errorf("message array without system role: want prompt, got %s", s.Kind)
			}
		}
	}
}

// --- AI-context gate tests for the infrastructure pattern loop ---
//
// These tests cover the regression we found via the 80-repo non-AI
// OSS corpus: jsStreamingPattern / pyEvalMetricPattern / etc. were
// firing on plain HTTP streaming, statistics primitives, and schema
// validators in non-AI code, producing 814 false-positive
// uncoveredAISurface signals on django/angular/k8s/etc. Gate added
// in prompt_ast_parser.go requires AI-import / SDK-call evidence
// per file before the infrastructure loop runs.

func TestASTJS_InfraGate_NoAIContext_SkipsHTTPStreaming(t *testing.T) {
	t.Parallel()
	// Angular-shaped fetch wrapper — uses standard Web Streams API
	// but has zero AI SDK references. Pre-fix this emitted a
	// "streaming_handler" surface.
	src := `
export class FetchBackend {
	handle(req: HttpRequest): Observable<HttpEvent> {
		return new Observable(observer => {
			fetch(req.url).then(response => {
				const reader = response.body?.getReader();
				const stream = new ReadableStream({
					start(controller) {
						function pump(): Promise<void> {
							return reader!.read().then(({ done, value }) => {
								if (done) { controller.close(); return; }
								controller.enqueue(value);
								return pump();
							});
						}
						return pump();
					},
				});
			});
		});
	}
}
`
	surfaces := ParsePromptAST("packages/common/http/src/fetch.ts", src, "js")
	for _, s := range surfaces {
		if s.Name == "streaming_handler" {
			t.Fatalf("non-AI file emitted streaming_handler surface: %+v", s)
		}
	}
}

func TestASTJS_InfraGate_NoAIContext_SkipsStatisticsLib(t *testing.T) {
	t.Parallel()
	// Generic statistics library — F1 score, cosine similarity.
	// Pre-fix this emitted an "eval_metric" surface.
	src := `
export function f1Score(predictions: number[], labels: number[]): number {
	let tp = 0, fp = 0, fn = 0;
	for (let i = 0; i < predictions.length; i++) {
		if (predictions[i] === 1 && labels[i] === 1) tp++;
		else if (predictions[i] === 1 && labels[i] === 0) fp++;
		else if (predictions[i] === 0 && labels[i] === 1) fn++;
	}
	const precision = tp / (tp + fp);
	const recall = tp / (tp + fn);
	return 2 * (precision * recall) / (precision + recall);
}

export function cosineSimilarity(a: number[], b: number[]): number {
	let dot = 0, magA = 0, magB = 0;
	for (let i = 0; i < a.length; i++) {
		dot += a[i] * b[i];
		magA += a[i] * a[i];
		magB += b[i] * b[i];
	}
	return dot / (Math.sqrt(magA) * Math.sqrt(magB));
}
`
	surfaces := ParsePromptAST("src/stats.ts", src, "js")
	for _, s := range surfaces {
		if s.Name == "eval_metric" {
			t.Fatalf("non-AI statistics file emitted eval_metric surface: %+v", s)
		}
	}
}

func TestASTJS_InfraGate_WithAIImport_EmitsStreamingHandler(t *testing.T) {
	t.Parallel()
	// Same streaming pattern as above, but with an OpenAI import.
	// Gate should allow the infra detection through.
	src := `
import OpenAI from 'openai';

const client = new OpenAI({ apiKey: process.env.OPENAI_API_KEY });

async function streamResponse(prompt: string) {
	const stream = await client.chat.completions.create({
		model: 'gpt-4',
		messages: [{ role: 'user', content: prompt }],
		stream: true,
	});
	for await (const chunk of stream) {
		console.log(chunk.choices[0].delta.content);
	}
}
`
	surfaces := ParsePromptAST("src/llm.ts", src, "js")
	var hasStreamingHandler bool
	for _, s := range surfaces {
		if s.Name == "streaming_handler" {
			hasStreamingHandler = true
		}
	}
	if !hasStreamingHandler {
		t.Errorf("AI-context file should emit streaming_handler; got %v", surfaceNames(surfaces))
	}
}

func TestASTPython_InfraGate_NoAIContext_SkipsRequests(t *testing.T) {
	t.Parallel()
	// requests-based HTTP code with iter_lines streaming — without
	// the AI-context infra gate this would emit streaming_handler
	// because iter_lines + for-loop matches the py streaming pattern.
	src := `
import requests

def stream_csv(url):
    with requests.get(url, stream=True) as r:
        for line in r.iter_lines():
            yield line.decode("utf-8")

def compute_bleu(reference, hypothesis):
    # generic BLEU implementation, no AI imports
    return bleu_score(reference, hypothesis)
`
	surfaces := ParsePromptAST("data/csv_stream.py", src, "python")
	for _, s := range surfaces {
		if s.Name == "streaming_handler" || s.Name == "eval_metric" {
			t.Fatalf("non-AI file emitted infra surface: %+v", s)
		}
	}
}

func TestASTPython_InfraGate_WithTransformersImport_EmitsEvalMetric(t *testing.T) {
	t.Parallel()
	// transformers import counts as AI context — BLEU usage in this
	// file is therefore legitimately an eval metric.
	src := `
from transformers import AutoTokenizer

def evaluate_translation(reference, hypothesis):
    return bleu_score(reference, hypothesis)
`
	surfaces := ParsePromptAST("ml/eval.py", src, "python")
	var hasEvalMetric bool
	for _, s := range surfaces {
		if s.Name == "eval_metric" {
			hasEvalMetric = true
		}
	}
	if !hasEvalMetric {
		t.Errorf("AI-context Python file should emit eval_metric; got %v", surfaceNames(surfaces))
	}
}

func TestASTJS_InfraGate_AcceptsLangChainImport(t *testing.T) {
	t.Parallel()
	// LangChain import (no SDK constructor) should still corroborate
	// AI context for the gate.
	src := `
import { ChatOpenAI } from '@langchain/openai';

export function buildGuardrail() {
	return { contentFilter: true };
}
`
	surfaces := ParsePromptAST("src/agent.ts", src, "js")
	var hasGuardrail bool
	for _, s := range surfaces {
		if s.Name == "guardrail" {
			hasGuardrail = true
		}
	}
	if !hasGuardrail {
		t.Errorf("LangChain import should corroborate AI context; got %v", surfaceNames(surfaces))
	}
}

func TestASTJS_InfraGate_AcceptsRequireForm(t *testing.T) {
	t.Parallel()
	// CommonJS require shape: const OpenAI = require('openai');
	src := `
const OpenAI = require('openai');

function handleStream() {
	return new ReadableStream({ start(c) {} });
}
`
	if !HasAIContextJS(src) {
		t.Errorf("require('openai') should corroborate AI context")
	}
}

// --- Helpers ---

func findSurfaceByPrefix(surfaces []models.CodeSurface, prefix string) *models.CodeSurface {
	for i, s := range surfaces {
		if strings.HasPrefix(s.Name, prefix) {
			return &surfaces[i]
		}
	}
	return nil
}

func TestSetCustomAIMarkers_ExtendsAIContextGate(t *testing.T) {
	// NOTE: NOT t.Parallel — mutates package-global customAIMarkerPatterns.
	defer SetCustomAIMarkers(nil) // reset after test

	// Without custom markers: a file that uses only a private SDK is
	// treated as non-AI, so streaming_handler is not emitted.
	src := `
import { LLMClient } from '@acme/llm-client';

const c = new LLMClient();
const stream = new ReadableStream();
`
	beforeSurfaces := ParsePromptAST("src/private.ts", src, "js")
	for _, s := range beforeSurfaces {
		if s.Name == "streaming_handler" {
			t.Fatalf("pre-marker: streaming_handler unexpectedly emitted")
		}
	}

	// Register the private import pattern as an AI marker. Now the
	// same source is treated as AI context and the infra patterns fire.
	SetCustomAIMarkers([]string{`@acme/llm-client`})
	afterSurfaces := ParsePromptAST("src/private.ts", src, "js")
	var hasStreamingHandler bool
	for _, s := range afterSurfaces {
		if s.Name == "streaming_handler" {
			hasStreamingHandler = true
		}
	}
	if !hasStreamingHandler {
		t.Errorf("post-marker: expected streaming_handler emitted; got %v", surfaceNames(afterSurfaces))
	}
}

func TestSetCustomAIMarkers_InvalidRegexIsSkipped(t *testing.T) {
	defer SetCustomAIMarkers(nil)
	SetCustomAIMarkers([]string{"valid_pattern", "(invalid["})
	if !matchesCustomAIMarker("valid_pattern here") {
		t.Errorf("valid pattern should match")
	}
	// Invalid regex was silently skipped; no panic, no crash.
}
