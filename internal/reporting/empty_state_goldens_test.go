package reporting

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// updateEmptyStateGoldens regenerates the golden files instead of
// asserting against them. Run with `-update-empty-state-goldens` after
// intentional empty-state copy changes; commit the resulting goldens
// in the same PR as the message change so reviewers see both.
var updateEmptyStateGoldens = flag.Bool("update-empty-state-goldens", false,
	"regenerate empty-state golden files instead of asserting against them")

// TestEmptyState_Goldens is the Track 10.8 visual regression test for
// every shipped empty state. The contract: byte-identical output
// between `RenderEmptyState(EmptyXxx)` and the committed golden under
// internal/reporting/testdata/empty_state_goldens/<name>.txt.
//
// Empty-state copy is a high-leverage UX surface — first-run, clean
// repos, edge cases. Drift here means adopters experience subtle
// regressions in the messages that introduce them to the product.
// Locking the goldens in CI surfaces the drift immediately.
//
// To intentionally change a message:
//   1. Edit the string in EmptyStateFor (empty_states.go).
//   2. Run: go test ./internal/reporting/... -update-empty-state-goldens
//   3. Inspect the diff in the golden file.
//   4. Commit both the source change and the golden update together.
func TestEmptyState_Goldens(t *testing.T) {
	cases := []struct {
		name string
		kind EmptyStateKind
	}{
		{"zero_findings", EmptyZeroFindings},
		{"no_ai_surfaces", EmptyNoAISurfaces},
		{"no_policy_file", EmptyNoPolicyFile},
		{"first_run", EmptyFirstRun},
		{"no_impact", EmptyNoImpact},
		{"no_test_selection", EmptyNoTestSelection},
		{"no_migration_candidates", EmptyNoMigrationCandidates},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			RenderEmptyState(&buf, tc.kind)
			got := buf.String()

			path := filepath.Join("testdata", "empty_state_goldens", tc.name+".txt")

			if *updateEmptyStateGoldens {
				if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
					t.Fatalf("write golden %s: %v", path, err)
				}
				return
			}

			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read golden %s: %v (run with -update-empty-state-goldens to create it)",
					path, err)
			}

			if got != string(want) {
				t.Errorf("empty-state %s drift:\n--- want (%s) ---\n%s\n--- got ---\n%s",
					tc.name, path, string(want), got)
			}
		})
	}
}

// TestEmptyState_GoldensCoverEveryKind is the drift gate: the goldens
// directory must contain one .txt per shipped EmptyStateKind. Adding
// a new kind without a golden surfaces the gap in CI.
func TestEmptyState_GoldensCoverEveryKind(t *testing.T) {
	t.Parallel()
	allKinds := []EmptyStateKind{
		EmptyZeroFindings,
		EmptyNoAISurfaces,
		EmptyNoPolicyFile,
		EmptyFirstRun,
		EmptyNoImpact,
		EmptyNoTestSelection,
		EmptyNoMigrationCandidates,
	}

	entries, err := os.ReadDir(filepath.Join("testdata", "empty_state_goldens"))
	if err != nil {
		t.Fatalf("read goldens dir: %v", err)
	}
	files := map[string]bool{}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".txt") {
			files[strings.TrimSuffix(e.Name(), ".txt")] = true
		}
	}

	if len(allKinds) != len(files) {
		t.Errorf("kinds=%d goldens=%d — every shipped kind needs a golden, every golden needs a corresponding kind. Files found: %v",
			len(allKinds), len(files), keys(files))
	}
}

func keys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
