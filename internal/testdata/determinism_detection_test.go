package testdata

import (
	"encoding/json"
	"testing"

	"github.com/pmclSF/terrain/internal/analysis"
)

const jsPromptFixture = `
const messages = [
	{ role: "system", content: "You are a helpful assistant that answers questions." },
	{ role: "user", content: userInput },
];
const response = await openai.chat.completions.create({
	model: "gpt-4",
	messages,
	temperature: 0.7,
});
const tools = [
	{ type: "function", function: { name: "search", parameters: { type: "object" } } },
];
const systemPrompt = "You are a coding assistant. Always respond with valid JSON.";
`

const pythonPromptFixture = `
from langchain.chat_models import ChatOpenAI
from langchain.prompts import ChatPromptTemplate

SYSTEM_PROMPT = """You are a helpful assistant.
Answer questions based on the provided context.
Always cite your sources."""

template = ChatPromptTemplate.from_messages([
    ("system", SYSTEM_PROMPT),
    ("human", "{question}"),
])

llm = ChatOpenAI(model="gpt-4", temperature=0)
chain = template | llm
`

const jsStructuralFixture = `
const SYSTEM_MESSAGES = [
	{ role: "system", content: "You are a support agent." },
	{ role: "user", content: "{query}" },
];
const FEW_SHOT_EXAMPLES = [
	{ input: "hello", output: "Hi there!" },
	{ input: "help", output: "Sure, what do you need?" },
];
const DEFAULT_PROMPT = "Answer the following question based on the context provided.";
`

func TestDeterminism_ParsePromptAST_JS(t *testing.T) {
	t.Parallel()
	assertDeterministic(t, 10, func() any {
		return analysis.ParsePromptAST("src/chat.js", jsPromptFixture, "js")
	})
}

func TestDeterminism_ParsePromptAST_Python(t *testing.T) {
	t.Parallel()
	assertDeterministic(t, 10, func() any {
		return analysis.ParsePromptAST("src/chat.py", pythonPromptFixture, "python")
	})
}

func TestDeterminism_ParseEmbeddedPrompts_JS(t *testing.T) {
	t.Parallel()
	assertDeterministic(t, 10, func() any {
		return analysis.ParseEmbeddedPrompts("src/chat.js", jsPromptFixture, "js")
	})
}

func TestDeterminism_ParseEmbeddedPrompts_Python(t *testing.T) {
	t.Parallel()
	assertDeterministic(t, 10, func() any {
		return analysis.ParseEmbeddedPrompts("src/chat.py", pythonPromptFixture, "python")
	})
}

func TestDeterminism_ParseStructural_JS(t *testing.T) {
	t.Parallel()
	assertDeterministic(t, 10, func() any {
		return analysis.ParseStructural("src/prompts.js", jsStructuralFixture, "js")
	})
}

// assertDeterministic runs fn n times, JSON-marshals the output, and asserts
// all runs produce identical output.
func assertDeterministic(t *testing.T, n int, fn func() any) {
	t.Helper()
	results := make([]string, n)
	for i := 0; i < n; i++ {
		data, err := json.Marshal(fn())
		if err != nil {
			t.Fatalf("marshal run %d: %v", i, err)
		}
		results[i] = string(data)
	}
	for i := 1; i < n; i++ {
		if results[i] != results[0] {
			t.Errorf("run %d differs from run 0:\n  run 0: %s\n  run %d: %s",
				i, results[0], i, results[i])
		}
	}
}
