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
// It is marked [experimental] in feature-status.md and ships with
// **no authentication** — security relies on localhost-only binding
// (127.0.0.1 by default) plus origin/referer checks. Do not expose it
// on a multi-user machine without external auth (e.g. an SSH tunnel).
// Not production-ready; not a "team dashboard."
//
// Flags wired through to internal/server.Config:
//
//	--root      repository root to analyze
//	--port      bind port (default 8421)
//	--host      bind host (default 127.0.0.1; opt-in for non-localhost)
//	--read-only enforce HTTP 405 on state-changing endpoints (active in 0.2)
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
