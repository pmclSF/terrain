package analysis

import (
	"io"
	"os"
	"path/filepath"
	"strings"
)

// hasVitestInSourceMarker returns true when the JS/TS file at absPath
// contains a Vitest in-source-test marker:
//
//	if (import.meta.vitest) { ... }
//	import.meta.vitest && describe(...)
//
// Vitest's in-source pattern lets a regular `add.ts` carry tests inline
// (https://vitest.dev/guide/in-source.html). These files don't match
// the `.test.ts` / `.spec.ts` naming convention so the path-based
// isTestFile check skips them. This helper is the content-based
// fallback that pulls them into the test-file set.
//
// We bound the read at 64 KB — the marker virtually always appears
// in the top of the file (or wouldn't have been intentional). Files
// that fail to open are silently treated as non-marker.
func hasVitestInSourceMarker(relPath, absPath string) bool {
	if !vitestSourceLanguages[strings.ToLower(filepath.Ext(relPath))] {
		return false
	}

	f, err := os.Open(absPath)
	if err != nil {
		return false
	}
	defer f.Close()

	const probeBytes = 64 * 1024
	buf := make([]byte, probeBytes)
	n, _ := io.ReadFull(f, buf)
	if n == 0 {
		return false
	}
	src := string(buf[:n])

	// The two canonical Vitest in-source shapes. Either substring
	// alone is enough — the patterns are specific to Vitest's
	// documented API.
	if strings.Contains(src, "import.meta.vitest") {
		return true
	}
	return false
}

// vitestSourceLanguages is the file-extension allowlist for the
// in-source marker scan. Keep it tight so we don't probe every text
// file in a repo.
var vitestSourceLanguages = map[string]bool{
	".js":  true,
	".jsx": true,
	".ts":  true,
	".tsx": true,
	".mjs": true,
	".mts": true,
}
