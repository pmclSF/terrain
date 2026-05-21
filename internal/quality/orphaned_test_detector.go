package quality

import (
	"fmt"
	"path/filepath"
	"sort"

	"github.com/pmclSF/terrain/internal/barrelresolver"
	"github.com/pmclSF/terrain/internal/mechanisms"
	"github.com/pmclSF/terrain/internal/models"
)

// OrphanedTestDetector identifies test files that have no linked code units —
// meaning they don't appear to test any source code in the repository. These
// tests may be obsolete, testing external dependencies, or simply disconnected
// from the import graph.
//
// This is a static detector: it uses structural analysis data (linked code
// units), requiring no runtime or coverage artifacts.
type OrphanedTestDetector struct {
	// RepoRoot enables the a7_barrel_resolver mechanism. When set, the
	// detector consults barrelresolver to follow re-export indirection
	// when claiming a test file has no linked source code units.
	RepoRoot string
}

// barrelResolvesAny returns true when at least one of `imports`
// resolves to an in-repo path via the barrel resolver. Used by the
// orphaned-test gate to suppress the orphan claim when re-export
// indirection makes the test reachable.
func barrelResolvesAny(r *barrelresolver.Resolver, fromDir string, imports map[string]bool) bool {
	for imp := range imports {
		results := r.Resolve(mechanisms.Default(), fromDir, imp)
		if len(results) > 0 {
			return true
		}
	}
	return false
}

// Detect scans test files for those with zero linked code units.
func (d *OrphanedTestDetector) Detect(snap *models.TestSuiteSnapshot) []models.Signal {
	var sigs []models.Signal

	type candidate struct {
		path      string
		tests     int
		framework string
	}
	var candidates []candidate

	// Mechanism gate: a7_barrel_resolver. Build a resolver once so a
	// test file claimed-orphaned by the legacy linkage can still be
	// rescued by a barrel-re-export-resolved import. When the
	// mechanism is off, Resolve returns nil and the rescue path is a
	// no-op.
	var resolver *barrelresolver.Resolver
	if d.RepoRoot != "" && mechanisms.Default().State(barrelresolver.MechanismName) != mechanisms.StateOff {
		if r, err := barrelresolver.New(d.RepoRoot); err == nil {
			resolver = r
		}
	}

	for _, tf := range snap.TestFiles {
		if tf.TestCount == 0 {
			continue
		}
		if len(tf.LinkedCodeUnits) == 0 {
			// Rescue path: if barrel resolver can resolve any of this
			// test's imports to an in-repo file, the test isn't
			// orphaned in any meaningful sense.
			if resolver != nil && snap.ImportGraph != nil {
				if resolved := barrelResolvesAny(resolver, filepath.Dir(tf.Path), snap.ImportGraph[tf.Path]); resolved {
					continue
				}
			}
			candidates = append(candidates, candidate{
				path:      tf.Path,
				tests:     tf.TestCount,
				framework: tf.Framework,
			})
		}
	}

	if len(candidates) == 0 {
		return nil
	}

	// Sort by path for deterministic output.
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].path < candidates[j].path
	})

	// Repository-level summary signal.
	total := len(snap.TestFiles)
	ratio := float64(len(candidates)) / float64(total)
	sev := models.SeverityLow
	if ratio > 0.3 {
		sev = models.SeverityMedium
	}

	sigs = append(sigs, models.Signal{
		Type:       "orphanedTestFile",
		Category:   models.CategoryHealth,
		Severity:   sev,
		Confidence: 0.5,
		Location:   models.SignalLocation{Repository: "static"},
		Explanation: fmt.Sprintf(
			"%d of %d test files have no linked source code units (%.0f%%).",
			len(candidates), total, ratio*100,
		),
		SuggestedAction:  "Verify orphaned tests are still relevant or remove them to reduce CI burden.",
		EvidenceStrength: models.EvidenceWeak,
		EvidenceSource:   models.SourceStructuralPattern,
		Metadata: map[string]any{
			"orphanedFiles": len(candidates),
			"totalFiles":    total,
			"ratio":         ratio,
			"scope":         "repository",
		},
	})

	// Per-file signals (capped at 10).
	limit := 10
	if len(candidates) < limit {
		limit = len(candidates)
	}
	for _, c := range candidates[:limit] {
		sigs = append(sigs, models.Signal{
			Type:       "orphanedTestFile",
			Category:   models.CategoryHealth,
			Severity:   models.SeverityLow,
			Confidence: 0.5,
			Location:   models.SignalLocation{File: c.path},
			Explanation: fmt.Sprintf(
				"%s has %d test(s) but no linked source code units.",
				c.path, c.tests,
			),
			SuggestedAction:  "Verify the test is still relevant or remove it.",
			EvidenceStrength: models.EvidenceWeak,
			EvidenceSource:   models.SourceStructuralPattern,
			Metadata: map[string]any{
				"testCount": c.tests,
				"framework": c.framework,
				"scope":     "file",
			},
		})
	}

	return sigs
}
