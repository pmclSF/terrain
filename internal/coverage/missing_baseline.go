package coverage

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pmclSF/terrain/internal/models"
	"github.com/pmclSF/terrain/internal/signals"
)

// DetectMissingBaseline fires when the snapshot has AI surfaces but
// `.terrain/baselines/` is absent or empty. The structural twin of
// regression/baseline-not-set — that rule fires after evals run and
// finds no comparator; this rule fires at the coverage layer before
// evals run so adopters lock a baseline deliberately.
//
// Implements terrain/coverage/missing-baseline.
//
// Fires at most once per snapshot (it's a repo-level condition).
func DetectMissingBaseline(repoRoot string, snap *models.TestSuiteSnapshot) []models.Signal {
	if snap == nil {
		return nil
	}
	if !hasAnyAISurface(snap) {
		return nil
	}
	if baselineDirIsPopulated(repoRoot) {
		return nil
	}
	return []models.Signal{{
		Type:             signals.SignalMissingBaseline,
		Category:         models.CategoryAI,
		Severity:         models.SeverityMedium,
		Confidence:       1.0,
		EvidenceStrength: models.EvidenceStrong,
		EvidenceSource:   models.SourceStructuralPattern,
		Location:         models.SignalLocation{File: ".terrain/baselines/"},
		Explanation: fmt.Sprintf(
			"Repository has %d AI surfaces but .terrain/baselines/ is absent or empty. Eval regression detection at the coverage layer is disabled until a baseline exists.",
			countAISurfaces(snap),
		),
		SuggestedAction: "Run `terrain ai record` from the main-branch state to create .terrain/baselines/latest.json, then commit it.",
		RuleID:          "terrain/coverage/missing-baseline",
		RuleURI:         "docs/rules/coverage/missing-baseline.md",
		DetectorVersion: "0.2.0",
	}}
}

func hasAnyAISurface(snap *models.TestSuiteSnapshot) bool {
	for _, cs := range snap.CodeSurfaces {
		if isAISurface(cs.Kind) {
			return true
		}
	}
	return false
}

func countAISurfaces(snap *models.TestSuiteSnapshot) int {
	n := 0
	for _, cs := range snap.CodeSurfaces {
		if isAISurface(cs.Kind) {
			n++
		}
	}
	return n
}

// baselineDirIsPopulated returns true when .terrain/baselines/ has
// at least one regular file.
func baselineDirIsPopulated(repoRoot string) bool {
	dir := filepath.Join(repoRoot, ".terrain", "baselines")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if !e.IsDir() {
			return true
		}
	}
	return false
}
