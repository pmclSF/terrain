package models

import (
	"testing"
)

// ---------------------------------------------------------------------------
// BuildSurfaceID
// ---------------------------------------------------------------------------

func TestBuildSurfaceID_NoParent(t *testing.T) {
	t.Parallel()
	got := BuildSurfaceID("src/handler.go", "HandleRequest", "")
	want := "surface:src/handler.go:HandleRequest"
	if got != want {
		t.Errorf("BuildSurfaceID = %q, want %q", got, want)
	}
}

func TestBuildSurfaceID_WithParent(t *testing.T) {
	t.Parallel()
	got := BuildSurfaceID("src/handler.go", "ServeHTTP", "Server")
	want := "surface:src/handler.go:Server.ServeHTTP"
	if got != want {
		t.Errorf("BuildSurfaceID = %q, want %q", got, want)
	}
}

func TestBuildSurfaceID_EmptyName(t *testing.T) {
	t.Parallel()
	got := BuildSurfaceID("path.go", "", "")
	want := "surface:path.go:"
	if got != want {
		t.Errorf("BuildSurfaceID = %q, want %q", got, want)
	}
}

func TestBuildSurfaceID_EmptyPath(t *testing.T) {
	t.Parallel()
	got := BuildSurfaceID("", "Foo", "")
	want := "surface::Foo"
	if got != want {
		t.Errorf("BuildSurfaceID = %q, want %q", got, want)
	}
}

// ---------------------------------------------------------------------------
// extractDetectorID
// ---------------------------------------------------------------------------

func TestExtractDetectorID_ValidBrackets(t *testing.T) {
	t.Parallel()
	cases := []struct {
		input string
		want  string
	}{
		{"[bracket-message-array] detected message array", "bracket-message-array"},
		{"[langchain-constructor] LangChain message constructor", "langchain-constructor"},
		{"[content-string] assigned prompt constant", "content-string"},
	}
	for _, tc := range cases {
		got := extractDetectorID(tc.input)
		if got != tc.want {
			t.Errorf("extractDetectorID(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestExtractDetectorID_NoBrackets(t *testing.T) {
	t.Parallel()
	cases := []string{
		"no brackets here",
		"",
		"ab",
	}
	for _, input := range cases {
		got := extractDetectorID(input)
		if got != "" {
			t.Errorf("extractDetectorID(%q) = %q, want empty", input, got)
		}
	}
}

func TestExtractDetectorID_UnclosedBracket(t *testing.T) {
	t.Parallel()
	got := extractDetectorID("[unclosed detector")
	if got != "" {
		t.Errorf("extractDetectorID with unclosed bracket = %q, want empty", got)
	}
}

// ---------------------------------------------------------------------------
// CodeSurface.Evidence
// ---------------------------------------------------------------------------

func TestCodeSurface_Evidence(t *testing.T) {
	t.Parallel()
	cs := CodeSurface{
		SurfaceID:     "surface:src/prompts.ts:buildPrompt_L42",
		Name:          "buildPrompt",
		Path:          "src/prompts.ts",
		Kind:          SurfacePrompt,
		Language:      "js",
		Line:          42,
		DetectionTier: TierStructural,
		Confidence:    0.95,
		Reason:        "[bracket-message-array] bracket-matched message array",
	}

	ev := cs.Evidence()

	if ev.DetectorID != "bracket-message-array" {
		t.Errorf("DetectorID = %q, want %q", ev.DetectorID, "bracket-message-array")
	}
	if ev.Tier != TierStructural {
		t.Errorf("Tier = %q, want %q", ev.Tier, TierStructural)
	}
	if ev.Confidence != 0.95 {
		t.Errorf("Confidence = %f, want 0.95", ev.Confidence)
	}
	if ev.FilePath != "src/prompts.ts" {
		t.Errorf("FilePath = %q, want %q", ev.FilePath, "src/prompts.ts")
	}
	if ev.Symbol != "buildPrompt" {
		t.Errorf("Symbol = %q, want %q", ev.Symbol, "buildPrompt")
	}
	if ev.Line != 42 {
		t.Errorf("Line = %d, want 42", ev.Line)
	}
	if ev.Reason != cs.Reason {
		t.Errorf("Reason mismatch")
	}
}

func TestCodeSurface_Evidence_NoReason(t *testing.T) {
	t.Parallel()
	cs := CodeSurface{
		Name:          "HandleRequest",
		Path:          "src/handler.go",
		Kind:          SurfaceHandler,
		DetectionTier: TierPattern,
		Confidence:    0.85,
	}

	ev := cs.Evidence()
	if ev.DetectorID != "" {
		t.Errorf("DetectorID = %q, want empty for no-reason surface", ev.DetectorID)
	}
}

// ---------------------------------------------------------------------------
// CodeSurfaceKind constants
// ---------------------------------------------------------------------------

func TestCodeSurfaceKind_Values(t *testing.T) {
	t.Parallel()
	// Ensure key surface kinds have expected string values for JSON stability.
	expectations := map[CodeSurfaceKind]string{
		SurfaceFunction:  "function",
		SurfaceMethod:    "method",
		SurfaceHandler:   "handler",
		SurfaceRoute:     "route",
		SurfacePrompt:    "prompt",
		SurfaceContext:   "context",
		SurfaceDataset:   "dataset",
		SurfaceToolDef:   "tool_definition",
		SurfaceRetrieval: "retrieval",
		SurfaceAgent:     "agent",
		SurfaceEvalDef:   "eval_definition",
		SurfaceFixture:   "fixture",
	}
	for kind, want := range expectations {
		if string(kind) != want {
			t.Errorf("CodeSurfaceKind %v = %q, want %q", kind, string(kind), want)
		}
	}
}

// ---------------------------------------------------------------------------
// Detection tier constants
// ---------------------------------------------------------------------------

func TestDetectionTier_Values(t *testing.T) {
	t.Parallel()
	expectations := map[string]string{
		TierStructural: "structural",
		TierSemantic:   "semantic",
		TierPattern:    "pattern",
		TierContent:    "content",
	}
	for got, want := range expectations {
		if got != want {
			t.Errorf("tier %q != %q", got, want)
		}
	}
}
