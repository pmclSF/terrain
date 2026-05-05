package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// pillarStatus is a per-pillar maturity check the doctor command
// renders before the migration-specific checks. The intent is to
// answer "is this repo set up to use Terrain end-to-end" at a glance,
// without running analyze.
type pillarStatus struct {
	Name    string
	Symbol  string // "✓", "⚠", "?"
	Detail  string
	Hint    string
}

// assessPillars returns one status per pillar. Each check is local —
// no analyze run, no network, no AST work — so doctor stays fast.
func assessPillars(root string) []pillarStatus {
	return []pillarStatus{
		assessUnderstand(root),
		assessAlign(root),
		assessGate(root),
	}
}

// assessUnderstand checks whether Terrain has anything to look at:
// at least one well-known test framework config or test file pattern
// in the repo. Empty result means analyze will produce an empty
// snapshot, and the user should be told that up-front.
func assessUnderstand(root string) pillarStatus {
	indicators := []string{
		"jest.config.js", "jest.config.ts", "jest.config.cjs",
		"vitest.config.js", "vitest.config.ts",
		"playwright.config.js", "playwright.config.ts",
		"cypress.config.js", "cypress.config.ts",
		"pytest.ini", "pyproject.toml",
		"go.mod",
	}
	for _, ind := range indicators {
		if fileExists(filepath.Join(root, ind)) {
			return pillarStatus{
				Name:   "Understand",
				Symbol: "✓",
				Detail: fmt.Sprintf("test framework detected (%s)", ind),
			}
		}
	}
	return pillarStatus{
		Name:   "Understand",
		Symbol: "?",
		Detail: "no recognized test framework config in repo root",
		Hint:   "Add tests with your framework of choice, then re-run.",
	}
}

// assessAlign checks for the multi-repo manifest. Absence isn't a
// problem — most repos are single-repo — but presence indicates
// portfolio adoption.
func assessAlign(root string) pillarStatus {
	if fileExists(filepath.Join(root, ".terrain", "repos.yaml")) {
		return pillarStatus{
			Name:   "Align",
			Symbol: "✓",
			Detail: "multi-repo manifest present",
		}
	}
	return pillarStatus{
		Name:   "Align",
		Symbol: "?",
		Detail: "no multi-repo manifest (single-repo workflow assumed)",
	}
}

// assessGate checks for a CI workflow file that references terrain,
// plus the suppressions / baseline files that the gating flow uses.
// Missing CI workflow is the most common adoption gap.
func assessGate(root string) pillarStatus {
	hasCI := hasTerrainCIWorkflow(root)
	hasSuppress := fileExists(filepath.Join(root, ".terrain", "suppressions.yaml"))
	switch {
	case hasCI && hasSuppress:
		return pillarStatus{
			Name:   "Gate",
			Symbol: "✓",
			Detail: "CI workflow + suppressions configured",
		}
	case hasCI:
		return pillarStatus{
			Name:   "Gate",
			Symbol: "✓",
			Detail: "CI workflow detected",
		}
	default:
		return pillarStatus{
			Name:   "Gate",
			Symbol: "⚠",
			Detail: "no CI workflow references terrain",
			Hint:   "See docs/examples/gate/github-action.yml for the recommended template.",
		}
	}
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

// hasTerrainCIWorkflow scans .github/workflows for any YAML file
// that mentions "terrain" anywhere. Cheap heuristic, but false
// positives are harmless here — the doctor is informational.
func hasTerrainCIWorkflow(root string) bool {
	dir := filepath.Join(root, ".github", "workflows")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !endsWithAny(name, ".yml", ".yaml") {
			continue
		}
		f, err := os.Open(filepath.Join(dir, name))
		if err != nil {
			continue
		}
		buf := make([]byte, 8192)
		n, _ := f.Read(buf)
		f.Close()
		if containsTerrain(buf[:n]) {
			return true
		}
	}
	return false
}

func endsWithAny(s string, suffixes ...string) bool {
	for _, sx := range suffixes {
		if len(s) >= len(sx) && s[len(s)-len(sx):] == sx {
			return true
		}
	}
	return false
}

func containsTerrain(b []byte) bool {
	target := []byte("terrain")
	for i := 0; i+len(target) <= len(b); i++ {
		match := true
		for j, c := range target {
			bc := b[i+j]
			if bc >= 'A' && bc <= 'Z' {
				bc += 'a' - 'A'
			}
			if bc != c {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

// renderPillarStatuses writes the per-pillar maturity block to w.
func renderPillarStatuses(w io.Writer, statuses []pillarStatus) {
	fmt.Fprintln(w, "Pillar maturity:")
	for _, ps := range statuses {
		fmt.Fprintf(w, "  [%s] %s: %s\n", ps.Symbol, ps.Name, ps.Detail)
		if ps.Hint != "" {
			fmt.Fprintf(w, "         -> %s\n", ps.Hint)
		}
	}
	fmt.Fprintln(w)
}
