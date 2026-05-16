// Package mcp implements a Model Context Protocol server for Terrain.
//
// The server speaks the JSON-RPC 2.0 transport defined in the MCP
// specification, pinned to version 2025-11-25 per PRODUCT.md §16. It
// runs over stdio (the default MCP transport) and exposes the minimum
// tool inventory listed in §16:
//
//	list_findings        — list findings from the most recent run
//	get_finding          — retrieve one finding by ID
//	get_cause_path       — return the cause-path chain for a finding
//	read_surface         — read an AI/ML surface description
//	read_eval            — read an eval definition
//	read_baseline        — read a baseline run summary
//	suggest_action       — return suggested remediation for a finding
//	reproduction_command — return the CLI command to reproduce locally
//
// Adopter configs for Claude Code and Cursor that wire the server in
// are documented in docs/integrations/mcp.md.
package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"

	"github.com/pmclSF/terrain/internal/findings"
)

// SpecVersion is the MCP spec version this server implements. Pinned
// per PRODUCT.md §16; adopters that pin to a different version on
// their client side will see initialize fail with a version
// mismatch.
const SpecVersion = "2025-11-25"

// ServerName is the server's name as reported in initialize.
const ServerName = "terrain-mcp"

// ServerVersion is the Terrain binary version (overridden at build).
var ServerVersion = "0.2.0"

// Server is an MCP server that reads JSON-RPC messages from a reader
// and writes responses to a writer. The default transport is stdio.
type Server struct {
	in  io.Reader
	out io.Writer

	// Artifacts is the read-only source of truth the tools query.
	// Populated by the caller before Serve is called — the server
	// doesn't run the analyze pipeline itself.
	Artifacts *Artifacts

	mu sync.Mutex
}

// Artifacts is the snapshot of analyze output the MCP tools query.
type Artifacts struct {
	// FindingsArtifact is the findings.json payload (Tier 3 emission).
	FindingsArtifact *findings.Artifact

	// Surfaces is the adopter-declared surface inventory from
	// terrain.yaml plus surfaces auto-derived from source.
	Surfaces map[string]SurfaceDescriptor

	// Evals is the eval inventory.
	Evals map[string]EvalDescriptor

	// Baselines is the baseline summary by name (typically "latest").
	Baselines map[string]json.RawMessage
}

// SurfaceDescriptor is the rendered shape returned by read_surface.
type SurfaceDescriptor struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Type        string `json:"type"`
	FilePath    string `json:"file_path,omitempty"`
	Model       string `json:"model,omitempty"`
}

// EvalDescriptor is the rendered shape returned by read_eval.
type EvalDescriptor struct {
	ID                string   `json:"id"`
	Name              string   `json:"name"`
	Path              string   `json:"path,omitempty"`
	Framework         string   `json:"framework,omitempty"`
	CoveredSurfaceIDs []string `json:"covered_surface_ids,omitempty"`
}

// New constructs a server reading from in / writing to out.
func New(in io.Reader, out io.Writer) *Server {
	return &Server{in: in, out: out}
}

// Serve runs the read-dispatch-write loop until ctx is canceled, EOF,
// or a write error.
func (s *Server) Serve(ctx context.Context) error {
	scanner := bufio.NewScanner(s.in)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		response := s.handle(line)
		if response != nil {
			if err := s.write(response); err != nil {
				return err
			}
		}
	}
	return scanner.Err()
}

// handle processes one incoming JSON-RPC message and returns the
// response message (or nil for notifications, which don't get a
// response).
func (s *Server) handle(raw []byte) *jsonRPCResponse {
	var req jsonRPCRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return errorResponse(nil, codeParseError, "invalid JSON")
	}
	if req.JSONRPC != "2.0" {
		return errorResponse(req.ID, codeInvalidRequest, "jsonrpc must be \"2.0\"")
	}

	switch req.Method {
	case "initialize":
		return s.handleInitialize(req)
	case "initialized":
		// Notification; no response.
		return nil
	case "tools/list":
		return s.handleToolsList(req)
	case "tools/call":
		return s.handleToolsCall(req)
	case "ping":
		return successResponse(req.ID, map[string]any{})
	}
	return errorResponse(req.ID, codeMethodNotFound, "unknown method: "+req.Method)
}

func (s *Server) handleInitialize(req jsonRPCRequest) *jsonRPCResponse {
	return successResponse(req.ID, map[string]any{
		"protocolVersion": SpecVersion,
		"serverInfo": map[string]any{
			"name":    ServerName,
			"version": ServerVersion,
		},
		"capabilities": map[string]any{
			"tools": map[string]any{},
		},
	})
}

func (s *Server) handleToolsList(req jsonRPCRequest) *jsonRPCResponse {
	tools := make([]map[string]any, 0, len(toolRegistry))
	for _, t := range toolRegistry {
		tools = append(tools, map[string]any{
			"name":        t.Name,
			"description": t.Description,
			"inputSchema": t.InputSchema,
		})
	}
	return successResponse(req.ID, map[string]any{"tools": tools})
}

func (s *Server) handleToolsCall(req jsonRPCRequest) *jsonRPCResponse {
	var params struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return errorResponse(req.ID, codeInvalidParams, "invalid params: "+err.Error())
	}
	tool, ok := toolByName(params.Name)
	if !ok {
		return errorResponse(req.ID, codeInvalidParams, "unknown tool: "+params.Name)
	}
	result, err := tool.Handler(s.Artifacts, params.Arguments)
	if err != nil {
		return errorResponse(req.ID, codeInternalError, err.Error())
	}
	// MCP tool results wrap content blocks; we emit one text block
	// carrying the JSON-encoded result.
	body, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return errorResponse(req.ID, codeInternalError, "encode result: "+err.Error())
	}
	return successResponse(req.ID, map[string]any{
		"content": []map[string]any{
			{"type": "text", "text": string(body)},
		},
		"isError": false,
	})
}

func (s *Server) write(resp *jsonRPCResponse) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	body, err := json.Marshal(resp)
	if err != nil {
		return err
	}
	if _, err := s.out.Write(body); err != nil {
		return err
	}
	_, err = s.out.Write([]byte("\n"))
	return err
}

// --- JSON-RPC 2.0 wire types ---

type jsonRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type jsonRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *jsonRPCError   `json:"error,omitempty"`
}

type jsonRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// JSON-RPC standard error codes.
const (
	codeParseError     = -32700
	codeInvalidRequest = -32600
	codeMethodNotFound = -32601
	codeInvalidParams  = -32602
	codeInternalError  = -32603
)

func successResponse(id any, result any) *jsonRPCResponse {
	return &jsonRPCResponse{JSONRPC: "2.0", ID: id, Result: result}
}

func errorResponse(id any, code int, message string) *jsonRPCResponse {
	return &jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &jsonRPCError{Code: code, Message: message},
	}
}

// --- helpers ---

func unmarshalArgs(args json.RawMessage, dst any) error {
	if len(args) == 0 {
		return nil
	}
	return json.Unmarshal(args, dst)
}

// finingByID looks up a finding in the artifact by primary_loc-derived
// or rule-id-derived key. Format: "<rule_id>:<primary_loc.path>:<line>".
func findingByID(a *findings.Artifact, id string) (*findings.Finding, bool) {
	if a == nil {
		return nil, false
	}
	for i, f := range a.Findings {
		if findingID(f) == id {
			return &a.Findings[i], true
		}
	}
	return nil, false
}

func findingID(f findings.Finding) string {
	return fmt.Sprintf("%s:%s:%d", f.RuleID, f.PrimaryLoc.Path, f.PrimaryLoc.Line)
}
