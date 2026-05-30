package main

import (
	"os/exec"
	"strings"
	"testing"
)

// TestLLMFreeContract is the structural guard for terrain's foundational
// principle: the analyze pipeline, the change-scope renderer, the
// check-runs bundler, and the slash receiver MUST NOT depend on the
// internal/llmprovider package. LLM enrichment is an opt-in CLI luxury;
// the CI gate (analyze → render → check-runs → slash) runs without any
// API key.
//
// Saved-memory provenance: see `terrain_llm_free_principle.md` —
// "runs without your API key" is the positioning hook. If a future
// change wires llmprovider into any of these paths, this test fails
// loudly so the regression cannot ship.
//
// We use `go list -deps` rather than reflection because Go's compiler
// inlines references — a structurally-imported but never-called package
// still appears in the dep graph, and that's what we want to forbid.
func TestLLMFreeContract(t *testing.T) {
	t.Parallel()

	// Every gate-path package must appear here. The list is reviewed
	// at every release; adding a new package to the analyze /
	// PR-comment / check-runs / slash paths without listing it below
	// would let a regression slip through. See cmd_llm_free_guard_test
	// commit history for prior expansions and the rationale.
	llmFreePackages := []string{
		"github.com/pmclSF/terrain/internal/engine",
		"github.com/pmclSF/terrain/internal/changescope",
		"github.com/pmclSF/terrain/internal/checkruns",
		"github.com/pmclSF/terrain/internal/slash",
		"github.com/pmclSF/terrain/internal/findinghistory",
		"github.com/pmclSF/terrain/internal/suppression",
		"github.com/pmclSF/terrain/internal/prtemplates",
		"github.com/pmclSF/terrain/internal/promptflow",
		"github.com/pmclSF/terrain/internal/prompttemplate",
		"github.com/pmclSF/terrain/internal/schemadiff",
		"github.com/pmclSF/terrain/internal/injection",
		"github.com/pmclSF/terrain/internal/scaffold",
		"github.com/pmclSF/terrain/internal/plugin",
	}

	const forbidden = "github.com/pmclSF/terrain/internal/llmprovider"

	for _, pkg := range llmFreePackages {
		t.Run(pkg, func(t *testing.T) {
			out, err := exec.Command("go", "list", "-deps", pkg).Output()
			if err != nil {
				t.Fatalf("go list -deps %s: %v", pkg, err)
			}
			for _, line := range strings.Split(string(out), "\n") {
				if strings.TrimSpace(line) == forbidden {
					t.Errorf("%s transitively depends on %s — this breaks the LLM-free contract.\n"+
						"If you intentionally wired LLM enrichment into a gate path, this guard "+
						"needs an explicit exception with the reason documented in CHANGELOG.",
						pkg, forbidden)
				}
			}
		})
	}
}
