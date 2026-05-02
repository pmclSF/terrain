// Package server implements a local HTTP server for Terrain.
//
// The server renders the analysis HTML report at / and provides JSON API
// endpoints at /api/*. By default it binds to localhost only (127.0.0.1)
// and is intended for local development use, not production deployment.
//
// Security posture (0.1.2):
//
//   - Bind address defaults to 127.0.0.1; opt-in via Config.Host to bind
//     elsewhere (with a stderr warning).
//   - Origin / Referer validation on every request rejects cross-origin
//     access, even on localhost. Browser-based attacks that hit
//     http://127.0.0.1:8421/api/analyze from an unrelated tab return 403.
//   - Security response headers (CSP, X-Frame-Options, X-Content-Type-Options,
//     Referrer-Policy) are set on every response.
//   - Optional read-only flag disables future state-changing endpoints
//     before they ship in 0.2; today every handler is read-only so the
//     flag is a no-op gate.
//
// Sandboxing AI eval execution and authentication for shared dev hosts is
// 0.3 work; until then, do not expose `terrain serve` on a multi-user
// machine without external auth (e.g. an SSH tunnel).
package server

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/pmclSF/terrain/internal/analyze"
	"github.com/pmclSF/terrain/internal/engine"
)

// DefaultPort is the default port for the Terrain server.
const DefaultPort = 8421

// DefaultHost is the default bind host. Localhost-only by design.
const DefaultHost = "127.0.0.1"

// Config controls server behavior. The zero value is safe; New applies
// sensible defaults for any field that is left empty.
type Config struct {
	// Host is the bind address. Defaults to "127.0.0.1". Setting this to
	// "0.0.0.0" or a specific external IP makes the server reachable from
	// the network and emits a stderr warning at startup.
	Host string

	// Port is the bind port. Defaults to DefaultPort.
	Port int

	// ReadOnly, when true, rejects any non-GET/HEAD/OPTIONS request with
	// HTTP 405 in the security middleware. Every endpoint shipped in 0.2
	// is read-only (GET-only routes), so this is a contract gate for
	// future state-changing endpoints rather than a behavior change for
	// today's traffic.
	ReadOnly bool
}

// Server is a local HTTP server for Terrain analysis.
type Server struct {
	root string
	cfg  Config

	mu           sync.Mutex
	cachedAt     time.Time
	cachedResult *engine.PipelineResult
	cachedReport *analyze.Report
}

// New creates a new Server for the given repository root with default
// configuration (DefaultHost, DefaultPort, read-only off).
//
// Existing callers that pass a port positionally continue to work; for
// finer-grained control use NewWithConfig.
func New(root string, port int) *Server {
	return NewWithConfig(root, Config{Port: port})
}

// NewWithConfig creates a new Server with explicit configuration. Empty
// fields fall back to defaults.
func NewWithConfig(root string, cfg Config) *Server {
	if cfg.Host == "" {
		cfg.Host = DefaultHost
	}
	if cfg.Port <= 0 {
		cfg.Port = DefaultPort
	}
	return &Server{root: root, cfg: cfg}
}

// ListenAndServe starts the HTTP server and blocks until the context is canceled.
func (s *Server) ListenAndServe(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleRoot)
	mux.HandleFunc("/api/health", s.handleHealth)
	mux.HandleFunc("/api/analyze", s.handleAnalyze)

	// Wrap mux with security middleware: Origin/Referer validation and
	// response-header hardening.
	handler := s.withSecurity(mux)

	addr := fmt.Sprintf("%s:%d", s.cfg.Host, s.cfg.Port)
	srv := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	// Shutdown gracefully when context is canceled.
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(shutdownCtx)
	}()

	if s.cfg.Host != DefaultHost {
		fmt.Fprintf(os.Stderr,
			"WARNING: terrain serve is bound to %q (not localhost). The server\n"+
				"         has no built-in authentication; do not expose it on a\n"+
				"         shared or untrusted network.\n",
			s.cfg.Host,
		)
	}
	fmt.Fprintf(os.Stderr, "Terrain server listening on http://%s\n", addr)
	fmt.Fprintf(os.Stderr, "Press Ctrl+C to stop.\n")

	err := srv.ListenAndServe()
	if err == http.ErrServerClosed {
		return nil
	}
	return err
}

// withSecurity wraps a handler with Origin/Referer validation and
// security-related response headers. Browser-based attacks against
// localhost endpoints (e.g. drive-by JavaScript on an open tab making
// fetch() calls to 127.0.0.1) are rejected with 403.
func (s *Server) withSecurity(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// ReadOnly enforcement: when set, only GET / HEAD / OPTIONS are
		// allowed. 0.2.0 promotes this from "reserved no-op" to active
		// enforcement so users who set --read-only get the contract
		// they ticked the box for, even though every current handler
		// is GET. Any future state-changing endpoint will be rejected
		// here without the handler needing per-route logic.
		if s.cfg.ReadOnly {
			switch r.Method {
			case http.MethodGet, http.MethodHead, http.MethodOptions:
				// allowed
			default:
				w.Header().Set("Allow", "GET, HEAD, OPTIONS")
				w.WriteHeader(http.StatusMethodNotAllowed)
				fmt.Fprintln(w, "method not allowed: server is in --read-only mode")
				return
			}
		}
		// Reject requests whose Origin/Referer don't match the bind host.
		// Empty Origin/Referer (e.g. curl, server-to-server) is allowed
		// because the only attacker we're filtering here is a browser.
		if !s.originAllowed(r) {
			w.WriteHeader(http.StatusForbidden)
			fmt.Fprintln(w, "forbidden: origin not allowed")
			return
		}

		// Standard hardening headers. CSP is intentionally strict — the
		// HTML report currently contains an inline reload script which we
		// permit via the script-src 'unsafe-inline'; the broader plan in
		// 0.2 is to extract that to an external file and tighten further.
		w.Header().Set(
			"Content-Security-Policy",
			"default-src 'self'; script-src 'self' 'unsafe-inline'; "+
				"style-src 'self' 'unsafe-inline'; img-src 'self' data:; "+
				"connect-src 'self'; frame-ancestors 'none';",
		)
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")

		next.ServeHTTP(w, r)
	})
}

// originAllowed returns true if a request's Origin and/or Referer header
// is consistent with the server's bind host. Empty headers are allowed
// (non-browser clients); a header that names a different host is blocked.
func (s *Server) originAllowed(r *http.Request) bool {
	expected := fmt.Sprintf("http://%s:%d", s.cfg.Host, s.cfg.Port)
	// Localhost-bound servers should also accept the http://localhost:PORT form
	// because browsers normalize 127.0.0.1 vs localhost differently.
	expectedLocalhost := fmt.Sprintf("http://localhost:%d", s.cfg.Port)

	if origin := r.Header.Get("Origin"); origin != "" {
		if origin != expected && origin != expectedLocalhost {
			return false
		}
	}
	if ref := r.Header.Get("Referer"); ref != "" {
		if !strings.HasPrefix(ref, expected+"/") &&
			!strings.HasPrefix(ref, expectedLocalhost+"/") &&
			ref != expected && ref != expectedLocalhost {
			return false
		}
	}
	return true
}

// cacheTTL is how long a cached pipeline result is considered fresh.
const cacheTTL = 5 * time.Second

// getResult returns a cached or fresh pipeline result and report.
func (s *Server) getResult() (*engine.PipelineResult, *analyze.Report, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cachedResult != nil && time.Since(s.cachedAt) < cacheTTL {
		return s.cachedResult, s.cachedReport, nil
	}

	result, err := engine.RunPipeline(s.root, engine.PipelineOptions{
		EngineVersion: "serve",
	})
	if err != nil {
		return nil, nil, err
	}

	report := analyze.Build(&analyze.BuildInput{
		Snapshot:  result.Snapshot,
		HasPolicy: result.HasPolicy,
	})

	s.cachedResult = result
	s.cachedReport = report
	s.cachedAt = time.Now()

	return result, report, nil
}
