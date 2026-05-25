package llmprovider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// OllamaProvider implements Provider against the Ollama local server.
// Default endpoint http://localhost:11434, default model llama3.2:3b —
// a security-friendly default (local-only, no network).
type OllamaProvider struct {
	Endpoint string
	Model    string
	Client   *http.Client
}

// Name implements Provider.
func (p *OllamaProvider) Name() string { return "ollama" }

// Chat runs one chat completion against the Ollama /api/chat endpoint.
// Ollama's protocol differs from OpenAI's in a few specifics:
//   - Streaming defaults to true; we explicitly set stream=false.
//   - Message format identical to OpenAI's {role, content}.
//   - No tool-calling at the protocol level (yet); we return
//     ErrToolCallsNotImplemented when req.Tools is set.
func (p *OllamaProvider) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	if len(req.Tools) > 0 {
		return nil, ErrToolCallsNotImplemented
	}

	model := req.Model
	if model == "" {
		model = p.Model
	}
	body, err := json.Marshal(map[string]any{
		"model":    model,
		"messages": req.Messages,
		"stream":   false,
		"options": map[string]any{
			"temperature": req.Temperature,
			"num_predict": req.MaxTokens,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("ollama: marshal: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.Endpoint+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("ollama: build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	client := p.Client
	if client == nil {
		client = &http.Client{Timeout: 60 * time.Second}
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("ollama: request failed (is Ollama running on %s?): %w", p.Endpoint, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("ollama: HTTP %d", resp.StatusCode)
	}

	var raw struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
		Model           string `json:"model"`
		PromptEvalCount int    `json:"prompt_eval_count"`
		EvalCount       int    `json:"eval_count"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("ollama: decode: %w", err)
	}

	return &ChatResponse{
		Content:          raw.Message.Content,
		Model:            raw.Model,
		PromptTokens:     raw.PromptEvalCount,
		CompletionTokens: raw.EvalCount,
	}, nil
}
