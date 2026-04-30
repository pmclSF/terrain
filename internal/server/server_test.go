package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/pmclSF/terrain/internal/analyze"
	"github.com/pmclSF/terrain/internal/engine"
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
	t.Parallel()
	s := newServerWithCachedReport()

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	s.handleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("root handler returned %d, want 200", w.Code)
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
	t.Parallel()
	s := newServerWithCachedReport()

	req := httptest.NewRequest("GET", "/api/analyze", nil)
	w := httptest.NewRecorder()
	s.handleAnalyze(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("analyze handler returned %d, want 200", w.Code)
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

func newServerWithCachedReport() *Server {
	s := New(".", 0)
	s.cachedAt = time.Now()
	s.cachedResult = &engine.PipelineResult{}
	s.cachedReport = &analyze.Report{
		SchemaVersion: analyze.AnalyzeReportSchemaVersion,
		Repository: analyze.RepositoryInfo{
			Name: "server-test-repo",
		},
		Headline: "Server test report",
		TestsDetected: analyze.TestSummary{
			TestFileCount: 1,
			TestCaseCount: 2,
			CodeUnitCount: 1,
			Frameworks: []analyze.FrameworkCount{
				{Name: "vitest", FileCount: 1, Type: "unit"},
			},
		},
		SignalSummary: analyze.SignalBreakdown{
			Total: 1,
			High:  1,
		},
		KeyFindings: []analyze.KeyFinding{
			{
				Title:    "Example finding",
				Severity: "high",
				Category: "coverage_debt",
			},
		},
	}
	return s
}

func TestNewWithConfig_DefaultsApplied(t *testing.T) {
	t.Parallel()

	s := NewWithConfig(".", Config{})
	if s.cfg.Host != DefaultHost {
		t.Errorf("default Host = %q, want %q", s.cfg.Host, DefaultHost)
	}
	if s.cfg.Port != DefaultPort {
		t.Errorf("default Port = %d, want %d", s.cfg.Port, DefaultPort)
	}
}

func TestNewWithConfig_HonoursOverrides(t *testing.T) {
	t.Parallel()

	s := NewWithConfig(".", Config{Host: "0.0.0.0", Port: 9999, ReadOnly: true})
	if s.cfg.Host != "0.0.0.0" {
		t.Errorf("Host override not applied: got %q", s.cfg.Host)
	}
	if s.cfg.Port != 9999 {
		t.Errorf("Port override not applied: got %d", s.cfg.Port)
	}
	if !s.cfg.ReadOnly {
		t.Errorf("ReadOnly override not applied")
	}
}

func TestNew_BackwardCompat(t *testing.T) {
	t.Parallel()

	// Old signature: New(root, port). Must still work and pick the default
	// host. This protects existing callers from a breaking signature change.
	s := New(".", 0)
	if s.cfg.Host != DefaultHost {
		t.Errorf("New(\".\", 0).Host = %q, want %q", s.cfg.Host, DefaultHost)
	}
	if s.cfg.Port != DefaultPort {
		t.Errorf("New(\".\", 0).Port = %d, want %d", s.cfg.Port, DefaultPort)
	}
}

func TestSecurityHeaders_PresentOnEveryResponse(t *testing.T) {
	t.Parallel()

	s := NewWithConfig(".", Config{Host: "127.0.0.1", Port: 8421})
	handler := s.withSecurity(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	required := map[string]string{
		"X-Frame-Options":         "DENY",
		"X-Content-Type-Options":  "nosniff",
		"Referrer-Policy":         "no-referrer",
	}
	for header, want := range required {
		if got := w.Header().Get(header); got != want {
			t.Errorf("missing header %q (got %q, want %q)", header, got, want)
		}
	}
	if w.Header().Get("Content-Security-Policy") == "" {
		t.Errorf("missing Content-Security-Policy header")
	}
}

func TestOriginAllowed(t *testing.T) {
	t.Parallel()

	s := NewWithConfig(".", Config{Host: "127.0.0.1", Port: 8421})

	cases := []struct {
		name    string
		origin  string
		referer string
		want    bool
	}{
		{"empty headers (curl, server-to-server)", "", "", true},
		{"matching 127.0.0.1 origin", "http://127.0.0.1:8421", "", true},
		{"matching localhost origin", "http://localhost:8421", "", true},
		{"matching referer", "", "http://127.0.0.1:8421/api/health", true},
		{"hostile cross-origin", "https://evil.example.com", "", false},
		{"malformed referer", "", "https://attacker.example.com/", false},
		{"wrong port", "http://127.0.0.1:9999", "", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/health", nil)
			if tc.origin != "" {
				req.Header.Set("Origin", tc.origin)
			}
			if tc.referer != "" {
				req.Header.Set("Referer", tc.referer)
			}
			got := s.originAllowed(req)
			if got != tc.want {
				t.Errorf("origin=%q referer=%q allowed=%v, want %v",
					tc.origin, tc.referer, got, tc.want)
			}
		})
	}
}

func TestSecurityMiddleware_BlocksHostileOrigin(t *testing.T) {
	t.Parallel()

	s := NewWithConfig(".", Config{Host: "127.0.0.1", Port: 8421})
	handler := s.withSecurity(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("inner"))
	}))

	req := httptest.NewRequest("GET", "/api/health", nil)
	req.Header.Set("Origin", "https://evil.example.com")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("hostile-origin status = %d, want 403", w.Code)
	}
	if strings.Contains(w.Body.String(), "inner") {
		t.Errorf("hostile-origin should not reach the inner handler")
	}
}
