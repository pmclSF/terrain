package engine_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/pmclSF/terrain/internal/calibration"
	"github.com/pmclSF/terrain/internal/engine"
	"github.com/pmclSF/terrain/internal/models"
)

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// buildBaselineFile synthesises a baseline snapshot from any framework
// artifacts found under baselineDir/eval-runs/ and writes it to a temp
// file under outDir. Returns the temp file path or "" when no artifacts
// were found. Author convenience: regression-shaped detector fixtures
// (aiCostRegression, aiRetrievalRegression) only need to drop the
// previous run's framework JSON into baseline/eval-runs/, not hand-author
// a snapshot.
func buildBaselineFile(t *testing.T, baselineDir, outDir string) string {
	t.Helper()

	opts := engine.PipelineOptions{}
	if exists(filepath.Join(baselineDir, "eval-runs/promptfoo.json")) {
		opts.PromptfooPaths = []string{filepath.Join(baselineDir, "eval-runs/promptfoo.json")}
	}
	if exists(filepath.Join(baselineDir, "eval-runs/deepeval.json")) {
		opts.DeepEvalPaths = []string{filepath.Join(baselineDir, "eval-runs/deepeval.json")}
	}
	if exists(filepath.Join(baselineDir, "eval-runs/ragas.json")) {
		opts.RagasPaths = []string{filepath.Join(baselineDir, "eval-runs/ragas.json")}
	}
	if len(opts.PromptfooPaths)+len(opts.DeepEvalPaths)+len(opts.RagasPaths) == 0 {
		return ""
	}

	result, err := engine.RunPipeline(baselineDir, opts)
	if err != nil {
		t.Fatalf("buildBaselineFile: %v", err)
	}

	// The baseline snapshot only needs the EvalRuns the regression
	// detectors look at; the rest is harmless to include.
	bytes, err := json.Marshal(result.Snapshot)
	if err != nil {
		t.Fatalf("marshal baseline: %v", err)
	}
	out := filepath.Join(outDir, "baseline.synthesized.json")
	if err := os.WriteFile(out, bytes, 0o644); err != nil {
		t.Fatalf("write baseline: %v", err)
	}
	return out
}

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
	// Pre-0.2.x this called t.Skipf if the corpus dir was empty, so
	// anyone who renamed or moved tests/calibration/ silently bypassed
	// the load-bearing gate. Now hard-fail on missing corpus and
	// require at least 25 fixtures (per docs/release/0.2.md).
	if len(dirs) == 0 {
		t.Fatalf("calibration corpus missing at %s — gate cannot be bypassed by deletion", corpusRoot)
	}
	const minFixtures = 25
	if len(dirs) < minFixtures {
		t.Fatalf("calibration corpus has %d fixtures; require at least %d", len(dirs), minFixtures)
	}

	analyse := func(fixturePath string) ([]models.Signal, error) {
		opts := engine.PipelineOptions{}
		// Auto-discover per-fixture eval artifacts. Each path is added to
		// PipelineOptions only when the file exists; fixtures without
		// these artifacts behave exactly as before.
		fixtureFile := func(rel string) string { return filepath.Join(fixturePath, rel) }
		if exists(fixtureFile("eval-runs/promptfoo.json")) {
			opts.PromptfooPaths = []string{fixtureFile("eval-runs/promptfoo.json")}
		}
		if exists(fixtureFile("eval-runs/deepeval.json")) {
			opts.DeepEvalPaths = []string{fixtureFile("eval-runs/deepeval.json")}
		}
		if exists(fixtureFile("eval-runs/ragas.json")) {
			opts.RagasPaths = []string{fixtureFile("eval-runs/ragas.json")}
		}
		if exists(fixtureFile("baseline.json")) {
			opts.BaselineSnapshotPath = fixtureFile("baseline.json")
		} else if exists(fixtureFile("baseline")) {
			// Synthesise the baseline snapshot from baseline/eval-runs/
			// framework artifacts. Cheaper to author than a hand-written
			// snapshot JSON with base64-encoded payloads.
			tmpDir := t.TempDir()
			synth := buildBaselineFile(t, fixtureFile("baseline"), tmpDir)
			if synth != "" {
				opts.BaselineSnapshotPath = synth
			}
		}

		result, err := engine.RunPipeline(fixturePath, opts)
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
