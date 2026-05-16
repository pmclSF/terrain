package aidetect

import "testing"

func TestDetectGoAISurfaces_OpenAINewClient(t *testing.T) {
	t.Parallel()
	src := []byte(`package main

import (
	"context"
	openai "github.com/sashabaranov/go-openai"
)

func main() {
	client := openai.NewClient("sk-...")
	resp, err := client.CreateChatCompletion(context.Background(),
		openai.ChatCompletionRequest{
			Model: "gpt-4o-mini",
			Messages: []openai.ChatCompletionMessage{
				{Role: "user", Content: "hi"},
			},
		})
	_ = resp
	_ = err
}
`)
	hits := DetectGoAISurfaces(src, "main.go")
	if len(hits) < 2 {
		t.Fatalf("hits = %d, want ≥2 (openai.NewClient + client.CreateChatCompletion): %+v", len(hits), hits)
	}

	var newClient, chatCall *AICallSite
	for i, h := range hits {
		switch h.Method {
		case "openai.NewClient":
			newClient = &hits[i]
		case "client.CreateChatCompletion":
			chatCall = &hits[i]
		}
	}
	if newClient == nil {
		t.Errorf("missing openai.NewClient hit, got %+v", hits)
	} else if newClient.SDK != "openai" || newClient.Confidence < 0.9 {
		t.Errorf("newClient: sdk=%q conf=%v", newClient.SDK, newClient.Confidence)
	}
	if chatCall == nil {
		t.Fatalf("missing client.CreateChatCompletion hit, got %+v", hits)
	}
	if chatCall.SDK != "openai" {
		t.Errorf("chatCall sdk = %q, want openai", chatCall.SDK)
	}
	if chatCall.Model != "gpt-4o-mini" {
		t.Errorf("chatCall model = %q", chatCall.Model)
	}
}

func TestDetectGoAISurfaces_ModelConstant(t *testing.T) {
	t.Parallel()
	// Model field assigned to a qualified constant (openai.GPT4oMini)
	// rather than a string literal. Inside a call argument so the
	// detector reaches it.
	src := []byte(`package main

import (
	"context"
	openai "github.com/sashabaranov/go-openai"
)

func chat(client *openai.Client) {
	client.CreateChatCompletion(context.Background(), openai.ChatCompletionRequest{
		Model: openai.GPT4oMini,
	})
}
`)
	hits := DetectGoAISurfaces(src, "x.go")
	if len(hits) == 0 {
		t.Fatalf("no hits, got %+v", hits)
	}
	var found *AICallSite
	for i, h := range hits {
		if h.Method == "client.CreateChatCompletion" {
			found = &hits[i]
		}
	}
	if found == nil {
		t.Fatalf("missing call hit: %+v", hits)
	}
	// Qualified constants are surfaced verbatim — resolving them to a
	// model name would need cross-file type analysis.
	if found.Model != "openai.GPT4oMini" {
		t.Errorf("model = %q, want openai.GPT4oMini (raw constant text)", found.Model)
	}
}

func TestDetectGoAISurfaces_BlankImportFlags(t *testing.T) {
	t.Parallel()
	// A blank import doesn't bind a name we can call through, but it
	// still flags the file as touching the SDK. We don't emit hits for
	// blank imports, but we shouldn't crash on them either.
	src := []byte(`package main

import (
	_ "github.com/sashabaranov/go-openai"
)

func main() { println("hello") }
`)
	hits := DetectGoAISurfaces(src, "x.go")
	// Expect zero hits (no call sites). Should not crash.
	if len(hits) != 0 {
		t.Errorf("expected 0 hits for blank-import file, got %+v", hits)
	}
}

func TestDetectGoAISurfaces_NoAIImports(t *testing.T) {
	t.Parallel()
	src := []byte(`package main

import "fmt"

type Runner struct{}

func (r *Runner) CreateChatCompletion() {} // shape-match candidate

func main() {
	r := &Runner{}
	r.CreateChatCompletion() // should NOT fire — no AI imports
	fmt.Println("ok")
}
`)
	hits := DetectGoAISurfaces(src, "x.go")
	if len(hits) != 0 {
		t.Errorf("expected 0 hits, got %+v", hits)
	}
}

func TestClassifyGoModule(t *testing.T) {
	t.Parallel()
	cases := []struct{ in, want string }{
		{"github.com/sashabaranov/go-openai", "openai"},
		{"github.com/openai/openai-go", "openai"},
		{"github.com/openai/openai-go/responses", "openai"},
		{"github.com/anthropic/anthropic-sdk-go", "anthropic"},
		{"github.com/tmc/langchaingo", "langchain"},
		{"github.com/x/unrelated", ""},
		{"", ""},
	}
	for _, tc := range cases {
		if got := classifyGoModule(tc.in); got != tc.want {
			t.Errorf("classifyGoModule(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
