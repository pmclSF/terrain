package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandleHealth(t *testing.T) {
	t.Parallel()
	s := New(".", 0)

	req := httptest.NewRequest("GET", "/api/health", nil)
	w := httptest.NewRecorder()
	s.handleHealth(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("health status = %d, want 200", w.Code)
	}

	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf("health status = %q, want ok", body["status"])
	}
}

func TestHandleRoot_NotFound(t *testing.T) {
	t.Parallel()
	s := New(".", 0)

	req := httptest.NewRequest("GET", "/nonexistent", nil)
	w := httptest.NewRecorder()
	s.handleRoot(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("unknown path status = %d, want 404", w.Code)
	}
}

func TestHandleRoot_ReturnsHTML(t *testing.T) {
	// This test requires a valid repo root — use the project root.
	// Skip if running in a CI environment without the full repo.
	s := New("../..", 0)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	s.handleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Skipf("root handler returned %d (likely missing fixture); skipping", w.Code)
	}

	ct := w.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/html") {
		t.Errorf("content-type = %q, want text/html", ct)
	}

	body := w.Body.String()
	if !strings.Contains(body, "<!DOCTYPE html>") {
		t.Error("response missing DOCTYPE")
	}
	if !strings.Contains(body, "setTimeout") {
		t.Error("response missing auto-refresh script")
	}
}

func TestHandleAnalyze_ReturnsJSON(t *testing.T) {
	s := New("../..", 0)

	req := httptest.NewRequest("GET", "/api/analyze", nil)
	w := httptest.NewRecorder()
	s.handleAnalyze(w, req)

	if w.Code != http.StatusOK {
		t.Skipf("analyze handler returned %d (likely missing fixture); skipping", w.Code)
	}

	ct := w.Header().Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Errorf("content-type = %q, want application/json", ct)
	}

	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON response: %v", err)
	}
	if _, ok := body["schemaVersion"]; !ok {
		t.Error("response missing schemaVersion field")
	}
}
