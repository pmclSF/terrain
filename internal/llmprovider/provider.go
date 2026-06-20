// Package llmprovider abstracts LLM inference for `terrain explain`
// and other CLI-only LLM-enrichment paths. Three provider families
// are supported:
//
//   - Ollama (default, security-friendly — local-only inference at
//     http://localhost:11434, no network)
//   - BYOK external (OpenAI / Anthropic — adopter provides API key
//     via environment variable, never embedded in terrain.yaml)
//   - Custom OpenAI-compatible endpoint (internal LLM gateways,
//     vLLM / TGI deployments)
//
// The package is intentionally narrow: chat completions only in
// 0.3.0, plus a placeholder for tool calls (returned as
// ErrToolCallsNotImplemented until the consumer wires through). LLM
// enrichment is a CLI luxury; the CI gate works without it.
package llmprovider

import (
	"context"
	"errors"
	"fmt"
)

// ErrProviderDisabled signals that the configured provider is "none"
// or no provider is configured. Callers branch on this to skip LLM
// enrichment without surfacing an error to the user.
var ErrProviderDisabled = errors.New("llmprovider: disabled (provider=none)")

// ErrToolCallsNotImplemented signals that the configured provider
// adapter doesn't expose tool-calling yet. Chat completions work;
// tool-calling lands as a followup.
var ErrToolCallsNotImplemented = errors.New("llmprovider: tool calls not implemented for this provider in 0.3.0")

// Message is one entry in a chat conversation.
type Message struct {
	Role    string `json:"role"` // system | user | assistant | tool
	Content string `json:"content"`
}

// ChatRequest is a chat completion request.
type ChatRequest struct {
	Model       string    `json:"model,omitempty"`
	Messages    []Message `json:"messages"`
	Temperature float64   `json:"temperature,omitempty"`
	MaxTokens   int       `json:"max_tokens,omitempty"`

	// Tools is reserved for the tool-calling extension. In 0.3.0,
	// providers may return ErrToolCallsNotImplemented when this is
	// non-empty.
	Tools []ToolSpec `json:"tools,omitempty"`
}

// ToolSpec describes a tool the model can call.
type ToolSpec struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

// ChatResponse is a chat completion response.
type ChatResponse struct {
	// Content is the assistant's text reply.
	Content string `json:"content"`

	// ToolCalls is populated when the model requested a tool call.
	// Empty when the model returned text only.
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`

	// Model is the actual model that served the request (may differ
	// from the requested model if the provider does fallback routing).
	Model string `json:"model,omitempty"`

	// PromptTokens / CompletionTokens are token-usage counters when
	// the provider exposes them. Zero when not reported.
	PromptTokens     int `json:"prompt_tokens,omitempty"`
	CompletionTokens int `json:"completion_tokens,omitempty"`
}

// ToolCall is one tool invocation requested by the model.
type ToolCall struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

// Provider is the LLM provider abstraction. Implementations are
// stateless per call — callers pass the full conversation each time.
type Provider interface {
	// Name returns the canonical provider identifier
	// (ollama, openai, anthropic, custom).
	Name() string

	// Chat runs one chat completion. Returns ErrProviderDisabled when
	// the provider is configured as "none".
	Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error)
}

// Config carries the configuration for one provider. Mirrors the
// terrain.yaml `explain` section.
type Config struct {
	// Provider names the family (ollama, openai, anthropic, custom, none).
	Provider string

	// Endpoint is the provider URL. For Ollama, defaults to
	// http://localhost:11434. For custom, required (adopter's OpenAI-
	// compatible endpoint).
	Endpoint string

	// Model is the provider-specific model identifier. For Ollama:
	// recommended default "llama3.2:3b".
	Model string

	// APIKeyEnv is the environment variable name holding the API key
	// for hosted providers. The package reads from os.Getenv at call
	// time — keys are NEVER embedded in terrain.yaml.
	APIKeyEnv string
}

// FromConfig dispatches to the right provider implementation based
// on cfg.Provider. Returns a disabled provider when cfg.Provider is
// "none" or empty.
func FromConfig(cfg Config) (Provider, error) {
	switch cfg.Provider {
	case "", "none":
		return disabledProvider{}, nil
	case "ollama":
		ep := cfg.Endpoint
		if ep == "" {
			ep = "http://localhost:11434"
		}
		model := cfg.Model
		if model == "" {
			model = "llama3.2:3b"
		}
		return &OllamaProvider{Endpoint: ep, Model: model}, nil
	case "openai":
		ep := cfg.Endpoint
		if ep == "" {
			ep = "https://api.openai.com/v1"
		}
		return &OpenAIProvider{
			Endpoint:  ep,
			Model:     cfg.Model,
			APIKeyEnv: cfg.APIKeyEnv,
		}, nil
	case "anthropic":
		ep := cfg.Endpoint
		if ep == "" {
			ep = "https://api.anthropic.com/v1"
		}
		return &AnthropicProvider{
			Endpoint:  ep,
			Model:     cfg.Model,
			APIKeyEnv: cfg.APIKeyEnv,
		}, nil
	case "custom":
		// Custom is OpenAI-compatible; we reuse the OpenAI client.
		if cfg.Endpoint == "" {
			return nil, fmt.Errorf("llmprovider: custom provider requires endpoint")
		}
		return &OpenAIProvider{
			Endpoint:  cfg.Endpoint,
			Model:     cfg.Model,
			APIKeyEnv: cfg.APIKeyEnv,
		}, nil
	}
	return nil, fmt.Errorf("llmprovider: unknown provider %q", cfg.Provider)
}

// disabledProvider returns ErrProviderDisabled for every call.
type disabledProvider struct{}

func (disabledProvider) Name() string { return "none" }
func (disabledProvider) Chat(context.Context, ChatRequest) (*ChatResponse, error) {
	return nil, ErrProviderDisabled
}
