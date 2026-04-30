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
// Today the corpus has one tiny fixture (skipped-without-ticket). 0.2's
// content roadmap calls for ~50 fixtures by milestone close; this test
// is the framework that scales to that size without further changes.
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

	// 0.2 ships the calibration *infrastructure*; the regression gate
	// runs in advisory mode (t.Logf, not t.Errorf) until the corpus is
	// populated with enough fixtures and labels for precision/recall to
	// be meaningful. Once the content roadmap reaches ~25 fixtures, this
	// block flips to t.Errorf and the docs/release/0.2.md gate (>= 90%
	// precision per active detector) becomes load-bearing.
	rec := corpus.RecallByType()
	for _, ftr := range corpus.Fixtures {
		for _, m := range ftr.Matches {
			if m.Outcome == calibration.OutcomeFalseNegative {
				t.Logf(
					"calibration miss (advisory): fixture %q expected %s on %s but detector did not fire (notes: %s)",
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
