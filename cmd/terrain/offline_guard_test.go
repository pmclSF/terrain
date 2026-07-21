package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

// outboundNetworkShapes match code that opens an OUTBOUND network connection.
// Terrain's "runs locally, no API key, no network" guarantee means none of
// these may appear on the analysis/gate path. Inbound listeners (terrain serve,
// the webhook receiver) use http.ResponseWriter / http.HandlerFunc, which are
// not in this set, so they do not match.
var outboundNetworkShapes = regexp.MustCompile(
	`\bhttp\.(Get|Post|Head|PostForm|NewRequest|NewRequestWithContext|DefaultClient|DefaultTransport)\b|` +
		`\bnet\.Dial\b|\bsmtp\.[A-Z]|\bgrpc\.Dial\b|\bwebsocket\.`)

// offlineAllowlist is the set of path prefixes permitted to contain outbound
// network code. Every entry MUST be unreachable from the scan/gate path:
//   - internal/llmprovider: opt-in BYOK adapters; not wired into any shipped
//     scan/gate command (verified: zero non-test importers).
var offlineAllowlist = []string{
	"internal/llmprovider/",
}

// TestOfflineGuarantee_NoOutboundNetworkOnScanPath enforces the shipped
// "runs locally, no API key, no network" guarantee by EVIDENCE rather than a
// one-time manual audit: it fails if any outbound-network call shape appears in
// shipped (non-test) source outside the allowlist. A new outbound call — a
// telemetry beacon, an accidental fetch, a provider call wired onto the scan
// path — trips this test.
func TestOfflineGuarantee_NoOutboundNetworkOnScanPath(t *testing.T) {
	t.Parallel()
	root := moduleRoot(t)

	var offenders []string
	for _, dir := range []string{"internal", "cmd"} {
		_ = filepath.WalkDir(filepath.Join(root, dir), func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			rel := filepath.ToSlash(mustRel(t, root, path))
			for _, a := range offlineAllowlist {
				if strings.HasPrefix(rel, a) {
					return nil
				}
			}
			b, readErr := os.ReadFile(path)
			if readErr != nil {
				return nil
			}
			if outboundNetworkShapes.Match(b) {
				offenders = append(offenders, rel)
			}
			return nil
		})
	}

	if len(offenders) > 0 {
		t.Fatalf("outbound network calls found outside the offline allowlist — the no-network "+
			"guarantee is broken:\n  %s\nIf this is legitimate opt-in/BYOK code that is unreachable "+
			"from the scan/gate path, add its prefix to offlineAllowlist with justification.",
			strings.Join(offenders, "\n  "))
	}
}

func mustRel(t *testing.T, base, target string) string {
	t.Helper()
	rel, err := filepath.Rel(base, target)
	if err != nil {
		t.Fatalf("rel(%q,%q): %v", base, target, err)
	}
	return rel
}

func moduleRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate test file")
	}
	dir := filepath.Dir(file)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found above test file")
		}
		dir = parent
	}
}
