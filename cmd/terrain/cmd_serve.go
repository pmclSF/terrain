package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"

	"github.com/pmclSF/terrain/internal/server"
)

// runServe starts the local Terrain HTTP server.
//
// The server is intended for single-developer use on a trusted machine.
// For 0.1.2 it is marked [experimental] in feature-status.md because the
// HTML dashboard surface is still minimal; flags exist now so 0.2 work
// can extend behavior without breaking the CLI contract.
//
// Flags wired through to internal/server.Config:
//
//	--root      repository root to analyze
//	--port      bind port (default 8421)
//	--host      bind host (default 127.0.0.1; opt-in for non-localhost)
//	--read-only forbid future state-changing API endpoints (today a no-op,
//	             reserved so users who flip it now keep their guarantees
//	             when 0.2 introduces write APIs)
func runServe(root string, port int, host string, readOnly bool) error {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return fmt.Errorf("resolving root path: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	srv := server.NewWithConfig(absRoot, server.Config{
		Host:     host,
		Port:     port,
		ReadOnly: readOnly,
	})
	return srv.ListenAndServe(ctx)
}
