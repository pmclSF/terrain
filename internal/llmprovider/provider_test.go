package llmprovider

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestFromConfig_Disabled(t *testing.T) {
	t.Parallel()
	p, err := FromConfig(Config{Provider: "none"})
	if err != nil {
		t.Fatalf("FromConfig: %v", err)
	}
	if p.Name() != "none" {
		t.Errorf("name = %q", p.Name())
	}
	_, err = p.Chat(context.Background(), ChatRequest{})
	if err != ErrProviderDisabled {
		t.Errorf("expected ErrProviderDisabled, got %v", err)
	}
}

func TestFromConfig_EmptyProviderDisabled(t *testing.T) {
	t.Parallel()
	p, _ := FromConfig(Config{})
	if p.Name() != "none" {
		t.Errorf("empty provider should default to disabled, got %q", p.Name())
	}
}

func TestFromConfig_OllamaDefaults(t *testing.T) {
	t.Parallel()
	p, _ := FromConfig(Config{Provider: "ollama"})
	op := p.(*OllamaProvider)
	if op.Endpoint != "http://localhost:11434" {
		t.Errorf("endpoint = %q", op.Endpoint)
	}
	if op.Model != "llama3.2:3b" {
		t.Errorf("model = %q", op.Model)
	}
}

func TestFromConfig_CustomRequiresEndpoint(t *testing.T) {
	t.Parallel()
	_, err := FromConfig(Config{Provider: "custom"})
	if err == nil {
		t.Error("expected error when custom provider has no endpoint")
	}
}

func TestFromConfig_UnknownProvider(t *testing.T) {
	t.Parallel()
	_, err := FromConfig(Config{Provider: "bogus"})
	if err == nil {
		t.Error("expected error for unknown provider")
	}
}

func TestOllamaProvider_Chat(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/chat" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"model": "llama3.2:3b",
			"message": map[string]string{
				"content": "Hello from Ollama.",
			},
			"prompt_eval_count": 12,
			"eval_count":        8,
		})
	}))
	defer server.Close()

	p := &OllamaProvider{Endpoint: server.URL, Model: "llama3.2:3b"}
	resp, err := p.Chat(context.Background(), ChatRequest{
		Messages: []Message{
			{Role: "user", Content: "hi"},
		},
	})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if !strings.Contains(resp.Content, "Hello from Ollama") {
		t.Errorf("content = %q", resp.Content)
	}
	if resp.PromptTokens != 12 || resp.CompletionTokens != 8 {
		t.Errorf("token counts: %d / %d", resp.PromptTokens, resp.CompletionTokens)
	}
}

func TestOllamaProvider_RejectsTools(t *testing.T) {
	t.Parallel()
	p := &OllamaProvider{Endpoint: "http://localhost:11434", Model: "x"}
	_, err := p.Chat(context.Background(), ChatRequest{
		Tools: []ToolSpec{{Name: "x"}},
	})
	if err != ErrToolCallsNotImplemented {
		t.Errorf("expected ErrToolCallsNotImplemented, got %v", err)
	}
}

func TestToolCallsNotImplementedMessageIsClearAndVersionFree(t *testing.T) {
	t.Parallel()
	msg := ErrToolCallsNotImplemented.Error()
	if !strings.Contains(msg, "tool calls not implemented") {
		t.Fatalf("message %q should clearly describe the limitation", msg)
	}
	// A version number in an error string is a maintenance trap (it goes stale
	// every release); the message must not embed one.
	if strings.Contains(msg, "0.") {
		t.Fatalf("message %q must not embed a version number", msg)
	}
}

func TestOpenAIProvider_ToolCalls(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), `"tools"`) {
			t.Errorf("request missing tools: %s", body)
		}
		if !strings.Contains(string(body), `"type":"function"`) {
			t.Errorf("request missing function wrapper: %s", body)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []any{
				map[string]any{
					"message": map[string]any{
						"content": "",
						"tool_calls": []any{
							map[string]any{
								"function": map[string]any{
									"name":      "list_findings",
									"arguments": `{"severity":"error"}`,
								},
							},
						},
					},
				},
			},
		})
	}))
	defer server.Close()
	os.Setenv("TERRAIN_TC_OPENAI_KEY", "sk-test")
	defer os.Unsetenv("TERRAIN_TC_OPENAI_KEY")

	p := &OpenAIProvider{Endpoint: server.URL, Model: "gpt-4o-mini", APIKeyEnv: "TERRAIN_TC_OPENAI_KEY"}
	resp, err := p.Chat(context.Background(), ChatRequest{
		Messages: []Message{{Role: "user", Content: "list errors"}},
		Tools: []ToolSpec{
			{Name: "list_findings", Description: "list", Parameters: map[string]any{"type": "object"}},
		},
	})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if len(resp.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(resp.ToolCalls))
	}
	if resp.ToolCalls[0].Name != "list_findings" {
		t.Errorf("name = %q", resp.ToolCalls[0].Name)
	}
	if resp.ToolCalls[0].Arguments["severity"] != "error" {
		t.Errorf("args = %+v", resp.ToolCalls[0].Arguments)
	}
}

func TestOpenAIProvider_Chat(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); !strings.HasPrefix(got, "Bearer ") {
			t.Errorf("missing bearer auth, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"model": "gpt-4o-mini",
			"choices": []any{
				map[string]any{
					"message": map[string]string{"content": "Hello from OpenAI."},
				},
			},
			"usage": map[string]int{"prompt_tokens": 5, "completion_tokens": 4},
		})
	}))
	defer server.Close()

	os.Setenv("TERRAIN_TEST_OPENAI_KEY", "sk-test")
	defer os.Unsetenv("TERRAIN_TEST_OPENAI_KEY")

	p := &OpenAIProvider{Endpoint: server.URL, Model: "gpt-4o-mini", APIKeyEnv: "TERRAIN_TEST_OPENAI_KEY"}
	resp, err := p.Chat(context.Background(), ChatRequest{
		Messages: []Message{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if !strings.Contains(resp.Content, "Hello from OpenAI") {
		t.Errorf("content = %q", resp.Content)
	}
}

func TestOpenAIProvider_MissingKey(t *testing.T) {
	t.Parallel()
	os.Unsetenv("TERRAIN_TEST_NOSUCH_KEY")
	p := &OpenAIProvider{Endpoint: "http://x", APIKeyEnv: "TERRAIN_TEST_NOSUCH_KEY"}
	_, err := p.Chat(context.Background(), ChatRequest{})
	if err == nil {
		t.Error("expected error for missing key env")
	}
}

func TestAnthropicProvider_SplitSystem(t *testing.T) {
	t.Parallel()
	sys, rest := splitSystemMessage([]Message{
		{Role: "system", Content: "you are helpful"},
		{Role: "user", Content: "hi"},
		{Role: "system", Content: "be concise"},
	})
	if sys != "you are helpful\n\nbe concise" {
		t.Errorf("system = %q", sys)
	}
	if len(rest) != 1 || rest[0].Role != "user" {
		t.Errorf("rest = %+v", rest)
	}
}
