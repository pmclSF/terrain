package main

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

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
//   - Every active endpoint that would be contacted under the current
//     terrain.yaml (none in 0.3.0).
//   - Parsed-but-inactive network settings, so adopters can see future-
//     facing config without mistaking it for active data flow.
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

	audit, err := collectNetworkAudit(abs)
	if err != nil {
		return err
	}

	fmt.Println("Active network endpoints Terrain would contact under the current config:")
	if len(audit.ActiveEndpoints) == 0 {
		fmt.Println("  (none)")
		fmt.Println()
		fmt.Println("Terrain runs entirely local in this configuration. No remote telemetry,")
		fmt.Println("no LLM API calls, no remote analytics, no update checks. The")
		fmt.Println("binary writes only to .terrain/ under the repo root.")
	} else {
		for _, e := range audit.ActiveEndpoints {
			fmt.Printf("  %s %s\n", uitokens.Bullet, e)
		}
	}
	if len(audit.InactiveNetworkSettings) > 0 {
		fmt.Println()
		fmt.Println("Configured but inactive network settings:")
		for _, e := range audit.InactiveNetworkSettings {
			fmt.Printf("  %s %s\n", uitokens.Bullet, e)
		}
		fmt.Println()
		fmt.Println("These settings are parsed for forward compatibility but are not")
		fmt.Println("contacted by Terrain in 0.3.0.")
	}
	return nil
}

type networkAudit struct {
	ActiveEndpoints         []string
	InactiveNetworkSettings []string
}

// collectNetworkAudit enumerates active network endpoints Terrain would
// contact under the configuration at root. Default: no active endpoints.
// It also reports parsed-but-inactive settings so security reviewers can
// distinguish "configured" from "contacted" at 0.3.0.
func collectNetworkAudit(root string) (networkAudit, error) {
	var audit networkAudit
	cfg, err := terrainconfig.LoadForRoot(root)
	if err != nil {
		return audit, err
	}
	if cfg != nil && cfg.Explain != nil {
		endpoint, err := explainNetworkEndpoint(*cfg.Explain)
		if err != nil {
			return audit, err
		}
		if endpoint != "" {
			provider := strings.TrimSpace(cfg.Explain.Provider)
			audit.InactiveNetworkSettings = append(audit.InactiveNetworkSettings, fmt.Sprintf("LLM explain provider (%s): %s", provider, endpoint))
		}
	}
	sort.Strings(audit.ActiveEndpoints)
	sort.Strings(audit.InactiveNetworkSettings)
	return audit, nil
}

func explainNetworkEndpoint(explain terrainconfig.ExplainSection) (string, error) {
	provider := strings.TrimSpace(explain.Provider)
	switch provider {
	case "", "none":
		return "", nil
	case "ollama":
		if strings.TrimSpace(explain.Endpoint) != "" {
			return strings.TrimSpace(explain.Endpoint), nil
		}
		return "http://localhost:11434", nil
	case "openai":
		if strings.TrimSpace(explain.Endpoint) != "" {
			return strings.TrimSpace(explain.Endpoint), nil
		}
		return "https://api.openai.com/v1", nil
	case "anthropic":
		if strings.TrimSpace(explain.Endpoint) != "" {
			return strings.TrimSpace(explain.Endpoint), nil
		}
		return "https://api.anthropic.com/v1", nil
	case "custom":
		endpoint := strings.TrimSpace(explain.Endpoint)
		if endpoint == "" {
			return "", fmt.Errorf("terrainconfig: explain.endpoint is required when explain.provider=custom")
		}
		return endpoint, nil
	default:
		return "", fmt.Errorf("terrainconfig: explain.provider=%q invalid", provider)
	}
}
