package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/pmclSF/terrain/internal/findings"
	"github.com/pmclSF/terrain/internal/mcp"
	"github.com/pmclSF/terrain/internal/models"
	"github.com/pmclSF/terrain/internal/terrainconfig"
)

// runMCPCommand starts the MCP server on stdio. The server reads
// JSON-RPC requests from stdin and writes responses to stdout —
// this is the standard MCP transport.
//
// Wiring artifacts: reads `.terrain/findings.json` (emitted by every
// `terrain analyze` run) plus terrain.yaml surfaces, any
// `.terrain/baselines/*.json` files, and `.terrain/snapshots/latest.json`
// for the eval inventory. Each load step degrades gracefully — a missing
// file just leaves that field empty so the server stays usable on a fresh
// repo.
func runMCPCommand(root string) error {
	fmt.Fprintf(os.Stderr, "terrain-mcp: starting on stdio, spec version %s\n", mcp.SpecVersion)
	fmt.Fprintf(os.Stderr, "terrain-mcp: serving from %s\n", root)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	server := mcp.New(os.Stdin, os.Stdout)
	server.Artifacts = loadMCPArtifacts(root)
	return server.Serve(ctx)
}

// loadMCPArtifacts reads the most-recent analyze artifacts from .terrain/
// into an Artifacts struct. Every load step degrades gracefully — a
// missing file just leaves that field empty so the server stays usable
// even on a fresh repo.
func loadMCPArtifacts(root string) *mcp.Artifacts {
	out := &mcp.Artifacts{
		Surfaces:  map[string]mcp.SurfaceDescriptor{},
		Evals:     map[string]mcp.EvalDescriptor{},
		Baselines: map[string]json.RawMessage{},
	}

	// findings.json — emitted by `terrain analyze`.
	if data, err := os.ReadFile(filepath.Join(root, ".terrain", "findings.json")); err == nil {
		var art findings.Artifact
		if err := json.Unmarshal(data, &art); err == nil {
			out.FindingsArtifact = &art
		} else {
			fmt.Fprintf(os.Stderr, "terrain-mcp: findings.json parse: %v\n", err)
		}
	}

	// terrain.yaml surfaces.
	if cfg, err := terrainconfig.LoadForRoot(root); err == nil && cfg != nil {
		for name, s := range cfg.Surfaces {
			out.Surfaces[name] = mcp.SurfaceDescriptor{
				Name:        name,
				Description: s.Description,
				Type:        s.Type,
				FilePath:    s.FilePath,
				Model:       s.Model,
			}
		}
	}

	// Persisted snapshot — written by `terrain analyze --write-snapshot`.
	// Its eval inventory feeds the read_eval tool.
	if data, err := os.ReadFile(filepath.Join(root, ".terrain", "snapshots", "latest.json")); err == nil {
		var snap models.TestSuiteSnapshot
		if err := json.Unmarshal(data, &snap); err == nil {
			for i := range snap.Evals {
				e := snap.Evals[i]
				out.Evals[e.EvalID] = mcp.EvalDescriptor{
					ID:                e.EvalID,
					Name:              e.Name,
					Path:              e.Path,
					Framework:         e.Framework,
					CoveredSurfaceIDs: e.CoveredSurfaceIDs,
				}
			}
		} else {
			fmt.Fprintf(os.Stderr, "terrain-mcp: snapshots/latest.json parse: %v\n", err)
		}
	}

	// Baselines under .terrain/baselines/*.json — load each as RawMessage.
	baselineDir := filepath.Join(root, ".terrain", "baselines")
	if entries, err := os.ReadDir(baselineDir); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			path := filepath.Join(baselineDir, e.Name())
			data, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			name := e.Name()
			if ext := filepath.Ext(name); ext != "" {
				name = name[:len(name)-len(ext)]
			}
			out.Baselines[name] = json.RawMessage(data)
		}
	}

	return out
}
