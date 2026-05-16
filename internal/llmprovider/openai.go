package llmprovider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
)

// OpenAIProvider implements Provider against an OpenAI-compatible
// endpoint. Used directly for OpenAI's hosted API and also as the
// implementation for `provider: custom` (adopter's internal LLM
// gateway, vLLM / TGI deployment, anything that speaks the
// /v1/chat/completions shape).
type OpenAIProvider struct {
	Endpoint  string
	Model     string
	APIKeyEnv string
	Client    *http.Client
}

// Name implements Provider.
func (p *OpenAIProvider) Name() string { return "openai" }

// Chat runs one chat completion against /v1/chat/completions.
// Tool calls translate to OpenAI's `tools` parameter shape; when the
// model returns tool_calls in the response, they're surfaced as
// ChatResponse.ToolCalls so the caller can dispatch.
func (p *OpenAIProvider) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	key, err := readAPIKey(p.APIKeyEnv)
	if err != nil {
		return nil, err
	}

	model := req.Model
	if model == "" {
		model = p.Model
	}

	payload := map[string]any{
		"model":       model,
		"messages":    req.Messages,
		"temperature": req.Temperature,
		"max_tokens":  req.MaxTokens,
	}
	if len(req.Tools) > 0 {
		// OpenAI tool spec: {type:"function", function:{name, description, parameters}}.
		tools := make([]map[string]any, len(req.Tools))
		for i, t := range req.Tools {
			tools[i] = map[string]any{
				"type": "function",
				"function": map[string]any{
					"name":        t.Name,
					"description": t.Description,
					"parameters":  t.Parameters,
				},
			}
		}
		payload["tools"] = tools
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("openai: marshal: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.Endpoint+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("openai: build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+key)

	client := p.Client
	if client == nil {
		client = &http.Client{Timeout: 60 * time.Second}
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("openai: request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("openai: HTTP %d", resp.StatusCode)
	}

	var raw struct {
		Choices []struct {
			Message struct {
				Content   string `json:"content"`
				ToolCalls []struct {
					Function struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls"`
			} `json:"message"`
		} `json:"choices"`
		Model string `json:"model"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("openai: decode: %w", err)
	}
	if len(raw.Choices) == 0 {
		return nil, fmt.Errorf("openai: empty choices")
	}

	out := &ChatResponse{
		Content:          raw.Choices[0].Message.Content,
		Model:            raw.Model,
		PromptTokens:     raw.Usage.PromptTokens,
		CompletionTokens: raw.Usage.CompletionTokens,
	}
	for _, tc := range raw.Choices[0].Message.ToolCalls {
		var args map[string]any
		// Arguments is a string-encoded JSON object per OpenAI's spec.
		if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
			args = map[string]any{"_raw": tc.Function.Arguments}
		}
		out.ToolCalls = append(out.ToolCalls, ToolCall{
			Name:      tc.Function.Name,
			Arguments: args,
		})
	}
	return out, nil
}

// readAPIKey loads the API key from the configured env var.
// Returns a clear error when the env var name isn't configured or
// the value is empty — never leaks the env-var name into stderr.
func readAPIKey(envName string) (string, error) {
	if envName == "" {
		return "", fmt.Errorf("api_key_env is required for this provider")
	}
	key := os.Getenv(envName)
	if key == "" {
		return "", fmt.Errorf("environment variable %q is empty", envName)
	}
	return key, nil
}
