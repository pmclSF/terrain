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

// buildBaselineFile synthesizes a baseline snapshot from any framework
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
// in-tree fixture corpus and confirms the runner reports sane numbers.
//
// Each fixture's labels.yaml declares the signals the suite should fire.
// New labels caught here trip the test until the corresponding detector
// is updated, which is exactly the regression gate we want.
//
// The gate is LOAD-BEARING: any unmatched expected label fails the test.
// Adding a new fixture with a label that doesn't fire is a regression
// that blocks merge.
func TestCalibration_CorpusRunner(t *testing.T) {
	t.Parallel()

	corpusRoot := corpusPath(t)
	dirs, err := calibration.FindFixtures(corpusRoot)
	if err != nil {
		t.Fatalf("FindFixtures: %v", err)
	}
	// Hard-fail on missing corpus rather than skip, so a rename or
	// move can't silently bypass the load-bearing gate.
	if len(dirs) == 0 {
		t.Fatalf("fixture corpus missing at %s — gate cannot be bypassed by deletion", corpusRoot)
	}
	const minFixtures = 25
	if len(dirs) < minFixtures {
		t.Fatalf("fixture corpus has %d fixtures; require at least %d", len(dirs), minFixtures)
	}

	analyze := func(fixturePath string) ([]models.Signal, error) {
		opts := engine.PipelineOptions{
			// The fixture corpus exercises every detector, including
			// those that ship disabled-by-default. The override opts
			// them back in for the duration of the test.
			EnabledDetectorsOverride: []string{
				"aiPromptInjectionRisk",
				"aiHardcodedAPIKey",
			},
		}
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

	corpus, err := calibration.Run(corpusRoot, analyze)
	if err != nil {
		t.Fatalf("corpus run: %v", err)
	}

	// The gate is load-bearing: every labeled fixture must still
	// fire its expected detector. Any future detector change that
	// drops a labeled signal trips this block.
	rec := corpus.RecallByType()
	for _, ftr := range corpus.Fixtures {
		for _, m := range ftr.Matches {
			if m.Outcome == calibration.OutcomeFalseNegative {
				t.Errorf(
					"regression: fixture %q expected %s on %s but detector did not fire (notes: %s)",
					ftr.Fixture, m.Type, m.File, m.Notes,
				)
			}
		}
	}

	// Surface counts in test output so reviewers can spot-check
	// without re-running by hand.
	t.Logf("manifest: %d fixtures, %d detector types observed",
		len(corpus.Fixtures), len(corpus.SortedDetectorTypes()))
	for _, typ := range corpus.SortedDetectorTypes() {
		prec := corpus.PrecisionByType()[typ]
		r := rec[typ]
		t.Logf("  %-30s  precision=%.2f  recall=%.2f  TP=%d FP=%d FN=%d",
			typ, prec, r,
			corpus.TP[typ], corpus.FP[typ], corpus.FN[typ])
	}
}

// corpusPath resolves the fixture root relative to this test file so
// the test runs the same whether `go test` is invoked from the repo
// root or a subdirectory.
func corpusPath(t *testing.T) string {
	t.Helper()
	_, thisFile, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(thisFile), "..", "..", "tests", "calibration")
}
