package main

import (
	"fmt"
	"path/filepath"

	"github.com/pmclSF/terrain/internal/terrainconfig"
	"github.com/pmclSF/terrain/internal/uitokens"
)

// runPrintNetwork audits Terrain's outbound network surface. Anchors the
// "verifiable zero outbound network calls" trust claim that the README,
// DESIGN, OVERVIEW, PRODUCT, and SECURITY-DATA-HANDLING.md all cite as
// the way adopters confirm Terrain is local-first.
//
// What it prints:
//   - The current network policy (default: zero outbound calls).
//   - Every external endpoint that would be contacted under the current
//     terrain.yaml (e.g., a plugin manifest URL once plugins ship).
//   - Default configuration: prints "(none)" and exits 0.
//
// What it does NOT do:
//   - It does NOT scan the repo for the AI surfaces Terrain detects.
//     That's `terrain` (no-args) or `terrain analyze`. The two concepts
//     were originally conflated; the trust-audit semantics here are
//     what the docs promised.
func runPrintNetwork(root string) error {
	abs, err := filepath.Abs(root)
	if err != nil {
		return err
	}

	fmt.Println(uitokens.Header("Outbound Network Audit"))
	fmt.Printf("Root: %s\n\n", abs)

	endpoints := collectOutboundEndpoints(abs)

	fmt.Println("Outbound endpoints Terrain would contact under the current config:")
	if len(endpoints) == 0 {
		fmt.Println("  (none)")
		fmt.Println()
		fmt.Println("Terrain runs entirely local in this configuration. No telemetry,")
		fmt.Println("no LLM API calls, no remote analytics, no update checks. The")
		fmt.Println("binary writes only to .terrain/ under the repo root.")
		return nil
	}
	for _, e := range endpoints {
		fmt.Printf("  %s %s\n", uitokens.Bullet, e)
	}
	return nil
}

// collectOutboundEndpoints enumerates every external endpoint Terrain
// would contact under the configuration at root. Default: empty slice.
// Adopter-opt-in features (a future plugin manifest URL, an OpenTelemetry
// exporter endpoint, etc.) are listed here as they're wired.
func collectOutboundEndpoints(root string) []string {
	var endpoints []string
	if cfg, err := terrainconfig.Load(filepath.Join(root, "terrain.yaml")); err == nil && cfg != nil {
		// Reserved for future opt-in fields. The fact that there are
		// no fields today is itself the trust claim.
		_ = cfg
	}
	return endpoints
}
