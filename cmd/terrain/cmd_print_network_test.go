package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCollectNetworkAudit_DefaultConfigHasNoActiveEndpoints(t *testing.T) {
	t.Parallel()

	got, err := collectNetworkAudit(t.TempDir())
	if err != nil {
		t.Fatalf("collectNetworkAudit: %v", err)
	}
	if len(got.ActiveEndpoints) != 0 {
		t.Fatalf("active endpoints = %+v, want none", got.ActiveEndpoints)
	}
	if len(got.InactiveNetworkSettings) != 0 {
		t.Fatalf("inactive settings = %+v, want none", got.InactiveNetworkSettings)
	}
}

func TestCollectNetworkAudit_ExplainProvidersAreInactiveIn030(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		yaml string
		want string
	}{
		{
			name: "ollama provider default endpoint",
			yaml: "version: 1\nexplain:\n  provider: ollama\n",
			want: "LLM explain provider (ollama): http://localhost:11434",
		},
		{
			name: "openai default",
			yaml: "version: 1\nexplain:\n  provider: openai\n",
			want: "LLM explain provider (openai): https://api.openai.com/v1",
		},
		{
			name: "anthropic custom endpoint",
			yaml: "version: 1\nexplain:\n  provider: anthropic\n  endpoint: https://anthropic.internal/v1\n",
			want: "LLM explain provider (anthropic): https://anthropic.internal/v1",
		},
		{
			name: "custom endpoint",
			yaml: "version: 1\nexplain:\n  provider: custom\n  endpoint: https://llm-gateway.internal/v1\n",
			want: "LLM explain provider (custom): https://llm-gateway.internal/v1",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root := writeTerrainConfig(t, tt.yaml)
			got, err := collectNetworkAudit(root)
			if err != nil {
				t.Fatalf("collectNetworkAudit: %v", err)
			}
			if len(got.ActiveEndpoints) != 0 {
				t.Fatalf("active endpoints = %+v, want none", got.ActiveEndpoints)
			}
			if len(got.InactiveNetworkSettings) != 1 || got.InactiveNetworkSettings[0] != tt.want {
				t.Fatalf("inactive settings = %+v, want [%s]", got.InactiveNetworkSettings, tt.want)
			}
		})
	}
}

func TestCollectNetworkAudit_CustomEndpointRequired(t *testing.T) {
	t.Parallel()

	root := writeTerrainConfig(t, "version: 1\nexplain:\n  provider: custom\n")
	_, err := collectNetworkAudit(root)
	if err == nil {
		t.Fatal("collectNetworkAudit succeeded with custom provider and no endpoint")
	}
	if !strings.Contains(err.Error(), "explain.endpoint is required") {
		t.Fatalf("error = %q, want endpoint requirement", err)
	}
}

func TestRunPrintNetwork_PrintsConfiguredLLMEndpointAsInactive(t *testing.T) {
	root := writeTerrainConfig(t, "version: 1\nexplain:\n  provider: openai\n")

	out, err := captureRun(func() error {
		return runPrintNetwork(root)
	})
	if err != nil {
		t.Fatalf("runPrintNetwork: %v", err)
	}
	text := string(out)
	if !strings.Contains(text, "Active network endpoints Terrain would contact under the current config:\n  (none)") {
		t.Fatalf("output should report no active endpoints:\n%s", text)
	}
	if !strings.Contains(text, "Configured but inactive network settings:") {
		t.Fatalf("output missing inactive settings section:\n%s", text)
	}
	if !strings.Contains(text, "LLM explain provider (openai): https://api.openai.com/v1") {
		t.Fatalf("output missing configured endpoint:\n%s", text)
	}
	if !strings.Contains(text, "not\ncontacted by Terrain in 0.3.0") {
		t.Fatalf("output should say inactive settings are not contacted:\n%s", text)
	}
}

func writeTerrainConfig(t *testing.T, body string) string {
	t.Helper()
	root := t.TempDir()
	configDir := filepath.Join(root, ".terrain")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("mkdir .terrain: %v", err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "terrain.yaml"), []byte(body), 0o644); err != nil {
		t.Fatalf("write terrain.yaml: %v", err)
	}
	return root
}
