package engine_test

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/pmclSF/terrain/internal/calibration"
	"github.com/pmclSF/terrain/internal/engine"
	"github.com/pmclSF/terrain/internal/models"
)

// TestCalibration_CorpusRunner runs the real engine pipeline against the
// in-tree calibration corpus and confirms the runner reports sane
// precision/recall numbers. This is the integration path that 0.2
// promises: adding a labeled fixture under tests/calibration/ and
// running `make calibrate` (which delegates to this code path).
//
// Each fixture's labels.yaml declares the signals the suite should fire.
// New labels caught here trip the test until the corresponding detector
// is updated, which is exactly the regression gate we want.
//
// As of 0.2 the corpus covers 24 fixtures and 30 distinct signal
// types spanning AI, quality, health, migration, structural, and
// runtime domains at 1.00 precision/recall, and the gate is now
// LOAD-BEARING: any unmatched expected label fails the test. Adding
// a new fixture with a label that doesn't fire is a regression that
// blocks merge.
func TestCalibration_CorpusRunner(t *testing.T) {
	t.Parallel()

	corpusRoot := corpusPath(t)
	dirs, err := calibration.FindFixtures(corpusRoot)
	if err != nil {
		t.Fatalf("FindFixtures: %v", err)
	}
	if len(dirs) == 0 {
		t.Skipf("no calibration fixtures under %s; corpus is empty", corpusRoot)
	}

	analyse := func(fixturePath string) ([]models.Signal, error) {
		result, err := engine.RunPipeline(fixturePath)
		if err != nil {
			return nil, err
		}
		if result == nil || result.Snapshot == nil {
			return nil, nil
		}
		return result.Snapshot.Signals, nil
	}

	corpus, err := calibration.Run(corpusRoot, analyse)
	if err != nil {
		t.Fatalf("calibration.Run: %v", err)
	}

	// 0.2's gate is load-bearing: every labelled fixture must still
	// fire its expected detector. We crossed the 25-fixture milestone
	// from docs/release/0.2.md with 24 fixtures × 30 detector types
	// at 100% precision/recall and zero misses — the corpus is now a
	// regression gate. Any future detector change that drops a
	// labelled signal trips this block.
	rec := corpus.RecallByType()
	for _, ftr := range corpus.Fixtures {
		for _, m := range ftr.Matches {
			if m.Outcome == calibration.OutcomeFalseNegative {
				t.Errorf(
					"calibration regression: fixture %q expected %s on %s but detector did not fire (notes: %s)",
					ftr.Fixture, m.Type, m.File, m.Notes,
				)
			}
		}
	}

	// Surface the precision numbers in test output so reviewers can
	// eyeball calibration health without re-running by hand.
	t.Logf("calibration: %d fixtures, %d detector types observed",
		len(corpus.Fixtures), len(corpus.SortedDetectorTypes()))
	for _, typ := range corpus.SortedDetectorTypes() {
		prec := corpus.PrecisionByType()[typ]
		r := rec[typ]
		t.Logf("  %-30s  precision=%.2f  recall=%.2f  TP=%d FP=%d FN=%d",
			typ, prec, r,
			corpus.TP[typ], corpus.FP[typ], corpus.FN[typ])
	}
}

// corpusPath resolves tests/calibration relative to this test file so
// the test runs the same whether `go test` is invoked from the repo
// root or a subdirectory.
func corpusPath(t *testing.T) string {
	t.Helper()
	_, thisFile, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(thisFile), "..", "..", "tests", "calibration")
}
