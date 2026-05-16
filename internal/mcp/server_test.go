package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/pmclSF/terrain/internal/findings"
)

func TestServer_Initialize(t *testing.T) {
	t.Parallel()
	req := `{"jsonrpc":"2.0","id":1,"method":"initialize"}` + "\n"
	out, err := runRequest(req, nil)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	var resp jsonRPCResponse
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		t.Fatalf("decode: %v\noutput: %s", err, out)
	}
	if resp.Error != nil {
		t.Fatalf("error: %+v", resp.Error)
	}
	result := resp.Result.(map[string]any)
	if result["protocolVersion"] != SpecVersion {
		t.Errorf("version = %v", result["protocolVersion"])
	}
}

func TestServer_ToolsList(t *testing.T) {
	t.Parallel()
	req := `{"jsonrpc":"2.0","id":1,"method":"tools/list"}` + "\n"
	out, err := runRequest(req, nil)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if !strings.Contains(out, "list_findings") ||
		!strings.Contains(out, "get_finding") ||
		!strings.Contains(out, "get_cause_path") ||
		!strings.Contains(out, "read_surface") ||
		!strings.Contains(out, "read_eval") ||
		!strings.Contains(out, "read_baseline") ||
		!strings.Contains(out, "suggest_action") ||
		!strings.Contains(out, "reproduction_command") {
		t.Errorf("missing tool from tools/list output:\n%s", out)
	}
}

func TestServer_ListFindings(t *testing.T) {
	t.Parallel()
	art := findings.NewArtifact([]findings.Finding{
		{
			Version: 1, RuleID: "terrain/coverage/no-tests", Severity: findings.SeverityWarning,
			PrimaryLoc: findings.Location{Path: "a.go", Line: 7}, ShortMessage: "x",
			DocsURL: "https://x",
		},
	})
	a := &Artifacts{FindingsArtifact: art}

	req := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"list_findings","arguments":{}}}` + "\n"
	out, err := runRequest(req, a)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	// The tool response wraps the JSON in an MCP content block (escaped),
	// so the rule_id appears as \"rule_id\" in the wire format.
	if !strings.Contains(out, `rule_id`) {
		t.Errorf("response missing rule_id: %s", out)
	}
}

func TestServer_GetFinding(t *testing.T) {
	t.Parallel()
	art := findings.NewArtifact([]findings.Finding{
		{
			Version: 1, RuleID: "terrain/coverage/no-tests", Severity: findings.SeverityWarning,
			PrimaryLoc:   findings.Location{Path: "a.go", Line: 7},
			ShortMessage: "untested",
			DocsURL:      "https://x",
			Reproduction: "terrain test",
		},
	})
	a := &Artifacts{FindingsArtifact: art}

	id := "terrain/coverage/no-tests:a.go:7"
	req := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"get_finding","arguments":{"id":"` + id + `"}}}` + "\n"
	out, err := runRequest(req, a)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if !strings.Contains(out, `untested`) {
		t.Errorf("response missing short_message: %s", out)
	}
}

func TestServer_GetFindingNotFound(t *testing.T) {
	t.Parallel()
	a := &Artifacts{FindingsArtifact: findings.NewArtifact(nil)}
	req := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"get_finding","arguments":{"id":"bogus"}}}` + "\n"
	out, _ := runRequest(req, a)

	// Our server surfaces tool execution errors as the JSON-RPC error
	// envelope (codeInternalError).
	if !strings.Contains(out, `no finding`) {
		t.Errorf("expected error mentioning missing finding, got: %s", out)
	}
}

func TestServer_UnknownMethod(t *testing.T) {
	t.Parallel()
	req := `{"jsonrpc":"2.0","id":1,"method":"bogus"}` + "\n"
	out, _ := runRequest(req, nil)
	if !strings.Contains(out, "unknown method") {
		t.Errorf("missing error: %s", out)
	}
}

func TestServer_ReadSurface(t *testing.T) {
	t.Parallel()
	a := &Artifacts{
		Surfaces: map[string]SurfaceDescriptor{
			"summarizer": {Name: "summarizer", Description: "summarizes input", Type: "llm"},
		},
	}
	req := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"read_surface","arguments":{"name":"summarizer"}}}` + "\n"
	out, _ := runRequest(req, a)
	if !strings.Contains(out, "summarizes input") {
		t.Errorf("missing description: %s", out)
	}
}

func TestServer_Notification(t *testing.T) {
	t.Parallel()
	// "initialized" is a notification — no response should be sent.
	req := `{"jsonrpc":"2.0","method":"initialized"}` + "\n"
	out, err := runRequest(req, nil)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if out != "" {
		t.Errorf("notification produced response: %q", out)
	}
}

// runRequest spins up the server, sends one request, and returns its
// stdout output.
func runRequest(req string, a *Artifacts) (string, error) {
	in := strings.NewReader(req)
	var out bytes.Buffer
	s := New(in, &out)
	s.Artifacts = a
	if err := s.Serve(context.Background()); err != nil {
		return out.String(), err
	}
	return strings.TrimRight(out.String(), "\n"), nil
}
