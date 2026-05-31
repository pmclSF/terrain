package llmprovider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// AnthropicProvider implements Provider against Anthropic's hosted
// Messages API.
type AnthropicProvider struct {
	Endpoint  string
	Model     string
	APIKeyEnv string
	Client    *http.Client
}

// Name implements Provider.
func (p *AnthropicProvider) Name() string { return "anthropic" }

// Chat runs one chat completion against /v1/messages.
//
// Anthropic's shape differs from OpenAI's:
//   - System message is a top-level "system" field, not a role.
//   - max_tokens is required.
//   - The API key is "x-api-key" header, not "Authorization: Bearer".
//   - Response content blocks include text and tool_use shapes.
//   - Tool spec uses input_schema (not parameters) and lives at the
//     top level of each tool object.
func (p *AnthropicProvider) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	key, err := readAPIKey(p.APIKeyEnv)
	if err != nil {
		return nil, err
	}

	model := req.Model
	if model == "" {
		model = p.Model
	}
	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = 4096
	}

	// Anthropic requires system message as a top-level field.
	system, messages := splitSystemMessage(req.Messages)
	payload := map[string]any{
		"model":       model,
		"messages":    messages,
		"system":      system,
		"max_tokens":  maxTokens,
		"temperature": req.Temperature,
	}
	if len(req.Tools) > 0 {
		tools := make([]map[string]any, len(req.Tools))
		for i, t := range req.Tools {
			tools[i] = map[string]any{
				"name":         t.Name,
				"description":  t.Description,
				"input_schema": t.Parameters,
			}
		}
		payload["tools"] = tools
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("anthropic: marshal: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.Endpoint+"/messages", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("anthropic: build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", key)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	client := p.Client
	if client == nil {
		client = &http.Client{Timeout: 60 * time.Second}
	}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("anthropic: request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("anthropic: HTTP %d", resp.StatusCode)
	}

	var raw struct {
		Content []struct {
			Type  string         `json:"type"`
			Text  string         `json:"text"`
			Name  string         `json:"name"`
			Input map[string]any `json:"input"`
		} `json:"content"`
		Model string `json:"model"`
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("anthropic: decode: %w", err)
	}

	var sb strings.Builder
	var toolCalls []ToolCall
	for _, c := range raw.Content {
		switch c.Type {
		case "text":
			sb.WriteString(c.Text)
		case "tool_use":
			toolCalls = append(toolCalls, ToolCall{Name: c.Name, Arguments: c.Input})
		}
	}

	return &ChatResponse{
		Content:          sb.String(),
		ToolCalls:        toolCalls,
		Model:            raw.Model,
		PromptTokens:     raw.Usage.InputTokens,
		CompletionTokens: raw.Usage.OutputTokens,
	}, nil
}

// splitSystemMessage extracts the system message (concatenated when
// multiple) and returns the remaining messages list.
func splitSystemMessage(messages []Message) (system string, rest []Message) {
	var sysParts []string
	for _, m := range messages {
		if m.Role == "system" {
			sysParts = append(sysParts, m.Content)
			continue
		}
		rest = append(rest, m)
	}
	return strings.Join(sysParts, "\n\n"), rest
}
