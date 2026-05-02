// Package server implements a local HTTP server for Terrain.
//
// The server renders the analysis HTML report at / and provides JSON API
// endpoints at /api/*. By default it binds to localhost only (127.0.0.1)
// and is intended for local development use, not production deployment.
//
// Security posture (0.2.0):
//
//   - **No authentication.** Security relies entirely on localhost-only
//     binding plus origin/referer validation. Adopters running on
//     multi-user hosts must front the server with external auth
//     (e.g. an SSH tunnel).
//   - Bind address defaults to 127.0.0.1; opt-in via Config.Host to bind
//     elsewhere (with a stderr warning).
//   - Origin / Referer validation on every request rejects cross-origin
//     access, even on localhost. Browser-based attacks that hit
//     http://127.0.0.1:8421/api/analyze from an unrelated tab return 403.
//   - Security response headers (CSP, X-Frame-Options, X-Content-Type-Options,
//     Referrer-Policy) are set on every response.
//   - Read-only flag enforces HTTP 405 on state-changing endpoints.
//
// Concurrency model:
//   - Cache reads use a sync.RWMutex; warm-cache hits don't block writers.
//   - The slow path runs the analysis under singleflight so concurrent
//     callers wait on a single in-flight analysis instead of stacking up.
//   - Each handler threads r.Context() through getResult; a client
//     disconnect returns ctx.Err() immediately, but the underlying
//     analysis continues for any other waiters. (A future iteration
//     could ref-count waiters and cancel when none remain.)
//
// Sandboxing AI eval execution and an actual auth model are 0.3 work;
// until then, this is a *local development tool*, not a team dashboard.
package server

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"

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

	// flight deduplicates concurrent in-flight analyses. Multiple
	// pending requests for the same root share one analysis call;
	// other handlers (e.g. /api/health) are not blocked.
	flight singleflight.Group

	mu           sync.RWMutex
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
//
// The fast path is read-locked: cache hits don't block writers or each
// other. The slow path runs the analysis once per cache window even
// under concurrent load via singleflight; additional callers wait on
// the in-flight analysis instead of running their own. The caller's
// context (typically r.Context()) controls how long this function
// blocks: when the client disconnects, the function returns with
// ctx.Err() and the analysis continues in the background for any other
// waiters. A future iteration could reference-count waiters and cancel
// the analysis when none remain.
func (s *Server) getResult(ctx context.Context) (*engine.PipelineResult, *analyze.Report, error) {
	// Fast path: cached and fresh.
	s.mu.RLock()
	if s.cachedResult != nil && time.Since(s.cachedAt) < cacheTTL {
		result, report := s.cachedResult, s.cachedReport
		s.mu.RUnlock()
		return result, report, nil
	}
	s.mu.RUnlock()

	type cached struct {
		result *engine.PipelineResult
		report *analyze.Report
	}

	ch := s.flight.DoChan("analyze", func() (any, error) {
		// Re-check the cache under singleflight: another caller might
		// have populated it while we were queued.
		s.mu.RLock()
		if s.cachedResult != nil && time.Since(s.cachedAt) < cacheTTL {
			c := &cached{result: s.cachedResult, report: s.cachedReport}
			s.mu.RUnlock()
			return c, nil
		}
		s.mu.RUnlock()

		// The shared analysis runs with context.Background() so a single
		// caller's disconnect doesn't cancel an analysis that other
		// waiters depend on. Per-caller cancellation is handled by the
		// select below.
		result, err := engine.RunPipelineContext(context.Background(), s.root, engine.PipelineOptions{
			EngineVersion: "serve",
		})
		if err != nil {
			return nil, err
		}
		report := analyze.Build(&analyze.BuildInput{
			Snapshot:  result.Snapshot,
			HasPolicy: result.HasPolicy,
		})

		s.mu.Lock()
		s.cachedResult = result
		s.cachedReport = report
		s.cachedAt = time.Now()
		s.mu.Unlock()

		return &cached{result: result, report: report}, nil
	})

	select {
	case res := <-ch:
		if res.Err != nil {
			return nil, nil, res.Err
		}
		c := res.Val.(*cached)
		return c.result, c.report, nil
	case <-ctx.Done():
		return nil, nil, ctx.Err()
	}
}
