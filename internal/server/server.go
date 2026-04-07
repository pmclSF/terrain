// Package server implements a local HTTP server for Terrain.
//
// The server renders the analysis HTML report at / and provides JSON API
// endpoints at /api/*. It binds to localhost only (127.0.0.1) and is
// intended for local development use, not production deployment.
package server

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/pmclSF/terrain/internal/analyze"
	"github.com/pmclSF/terrain/internal/engine"
)

// DefaultPort is the default port for the Terrain server.
const DefaultPort = 8421

// Server is a local HTTP server for Terrain analysis.
type Server struct {
	root string
	port int

	mu           sync.Mutex
	cachedAt     time.Time
	cachedResult *engine.PipelineResult
	cachedReport *analyze.Report
}

// New creates a new Server for the given repository root.
func New(root string, port int) *Server {
	if port <= 0 {
		port = DefaultPort
	}
	return &Server{root: root, port: port}
}

// ListenAndServe starts the HTTP server and blocks until the context is cancelled.
func (s *Server) ListenAndServe(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleRoot)
	mux.HandleFunc("/api/health", s.handleHealth)
	mux.HandleFunc("/api/analyze", s.handleAnalyze)

	addr := fmt.Sprintf("127.0.0.1:%d", s.port)
	srv := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	// Shutdown gracefully when context is cancelled.
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(shutdownCtx)
	}()

	fmt.Fprintf(os.Stderr, "Terrain server listening on http://%s\n", addr)
	fmt.Fprintf(os.Stderr, "Press Ctrl+C to stop.\n")

	err := srv.ListenAndServe()
	if err == http.ErrServerClosed {
		return nil
	}
	return err
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
