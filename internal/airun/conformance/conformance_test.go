// Package conformance holds shape-fixture tests for the airun
// adapters. Each fixture is a small but representative payload of
// one (framework × version) combination Terrain claims to support;
// the tests assert that shape detection identifies the version and
// flags the warnings we expect.
//
// Adding a new fixture is the documented way to extend coverage:
//   1. Drop a JSON file under testdata/<framework>/.
//   2. Add a test case below mapping the file → expected ShapeInfo.
//
// This is the load-bearing test suite for Track 7.1 — adapter
// conformance fixtures per (framework × version) — and Track 7.2
// — warn-on-shape-drift logging — of the 0.2 release plan.
package conformance

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pmclSF/terrain/internal/airun"
)

func TestPromptfooShape_v3Nested(t *testing.T) {
	t.Parallel()
	info := loadAndDetectPromptfoo(t, "promptfoo/v3-nested.json")
	if info.Version != "v3" {
		t.Errorf("Version = %q, want v3", info.Version)
	}
	if info.HasWarnings() {
		t.Errorf("expected no warnings on canonical v3 shape; got: %v", info.Warnings)
	}
}

func TestPromptfooShape_v4Flat(t *testing.T) {
	t.Parallel()
	info := loadAndDetectPromptfoo(t, "promptfoo/v4-flat.json")
	if info.Version != "v4" {
		t.Errorf("Version = %q, want v4", info.Version)
	}
	if info.HasWarnings() {
		t.Errorf("expected no warnings on canonical v4 shape; got: %v", info.Warnings)
	}
}

func TestPromptfooShape_MissingEvalId(t *testing.T) {
	t.Parallel()
	info := loadAndDetectPromptfoo(t, "promptfoo/missing-eval-id.json")
	if !info.HasWarnings() {
		t.Error("expected drift warning for missing evalId")
	}
	if !containsAny(info.Warnings, "missing evalId") {
		t.Errorf("expected evalId-missing warning; got: %v", info.Warnings)
	}
}

func TestDeepEvalShape_CamelCase(t *testing.T) {
	t.Parallel()
	info := loadAndDetectDeepEval(t, "deepeval/1x-camel.json")
	if info.Version != "1.x" {
		t.Errorf("Version = %q, want 1.x", info.Version)
	}
	if info.HasWarnings() {
		t.Errorf("expected no warnings on canonical 1.x camelCase; got: %v", info.Warnings)
	}
}

func TestDeepEvalShape_SnakeCase(t *testing.T) {
	t.Parallel()
	info := loadAndDetectDeepEval(t, "deepeval/1x-snake.json")
	if info.Version != "1.x" {
		t.Errorf("Version = %q, want 1.x", info.Version)
	}
	if !info.HasWarnings() {
		t.Error("expected drift warning for snake_case test_cases")
	}
	if !containsAny(info.Warnings, "snake_case") {
		t.Errorf("expected snake_case warning; got: %v", info.Warnings)
	}
}

func TestDeepEvalShape_BareArray(t *testing.T) {
	t.Parallel()
	info := loadAndDetectDeepEval(t, "deepeval/bare-array.json")
	if info.Version != "1.x" {
		t.Errorf("Version = %q, want 1.x", info.Version)
	}
	if !info.HasWarnings() {
		t.Error("expected drift warning for bare-array shape")
	}
}

func TestRagasShape_Modern(t *testing.T) {
	t.Parallel()
	info := loadAndDetectRagas(t, "ragas/modern.json")
	if info.Version != "modern" {
		t.Errorf("Version = %q, want modern", info.Version)
	}
	if info.HasWarnings() {
		t.Errorf("expected no warnings on canonical modern Ragas; got: %v", info.Warnings)
	}
}

func TestRagasShape_Legacy(t *testing.T) {
	t.Parallel()
	info := loadAndDetectRagas(t, "ragas/legacy.json")
	if info.Version != "legacy" {
		t.Errorf("Version = %q, want legacy", info.Version)
	}
	if info.HasWarnings() {
		t.Errorf("expected no warnings on canonical legacy Ragas array; got: %v", info.Warnings)
	}
}

func TestPromptfooShape_EmptyPayload(t *testing.T) {
	t.Parallel()
	info := airun.DetectPromptfooShape(nil)
	if !info.HasWarnings() {
		t.Error("expected warning for empty payload")
	}
	if info.Framework != "promptfoo" {
		t.Errorf("Framework = %q, want promptfoo", info.Framework)
	}
}

func TestFormatWarnings_StableOrder(t *testing.T) {
	t.Parallel()
	info := airun.ShapeInfo{
		Framework: "promptfoo",
		Warnings:  []string{"first", "second", "third"},
	}
	got := info.FormatWarnings()
	if got != "first; second; third" {
		t.Errorf("FormatWarnings = %q, want stable insertion order", got)
	}
}

// --- helpers ---

func loadAndDetectPromptfoo(t *testing.T, rel string) airun.ShapeInfo {
	t.Helper()
	data := load(t, rel)
	return airun.DetectPromptfooShape(data)
}

func loadAndDetectDeepEval(t *testing.T, rel string) airun.ShapeInfo {
	t.Helper()
	data := load(t, rel)
	return airun.DetectDeepEvalShape(data)
}

func loadAndDetectRagas(t *testing.T, rel string) airun.ShapeInfo {
	t.Helper()
	data := load(t, rel)
	return airun.DetectRagasShape(data)
}

func load(t *testing.T, rel string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", rel))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return data
}

func containsAny(haystack []string, needle string) bool {
	for _, s := range haystack {
		if strings.Contains(s, needle) {
			return true
		}
	}
	return false
}
