package performance

import (
	"testing"

	"github.com/pmclSF/terrain/internal/impact"
	"github.com/pmclSF/terrain/internal/models"
)

func TestDetectMissingPerfTest_FiresOnUncoveredAISurface(t *testing.T) {
	t.Parallel()
	snap := &models.TestSuiteSnapshot{
		CodeSurfaces: []models.CodeSurface{
			{SurfaceID: "p:1", Name: "summarize_prompt", Kind: models.SurfacePrompt},
		},
		TestFiles: []models.TestFile{
			{Path: "benchmarks/throughput_test.py"},
			{Path: "tests/unit/x_test.py"},
		},
	}
	g := &impact.ImpactGraph{UnitToTests: map[string][]string{}}
	sigs := DetectMissingPerfTest(snap, g)
	if len(sigs) != 1 {
		t.Fatalf("expected 1 signal, got %d", len(sigs))
	}
}

func TestDetectMissingPerfTest_SuppressedByPerfTest(t *testing.T) {
	t.Parallel()
	snap := &models.TestSuiteSnapshot{
		CodeSurfaces: []models.CodeSurface{
			{SurfaceID: "p:1", Name: "summarize_prompt", Kind: models.SurfacePrompt},
		},
		TestFiles: []models.TestFile{
			{Path: "benchmarks/summarize_test.py"},
		},
	}
	g := &impact.ImpactGraph{
		UnitToTests: map[string][]string{
			"p:1": {"benchmarks/summarize_test.py"},
		},
	}
	sigs := DetectMissingPerfTest(snap, g)
	if len(sigs) != 0 {
		t.Errorf("perf test coverage should suppress, got %+v", sigs)
	}
}

func TestDetectMissingPerfTest_NoPerfTestsAnywhere(t *testing.T) {
	t.Parallel()
	snap := &models.TestSuiteSnapshot{
		CodeSurfaces: []models.CodeSurface{
			{SurfaceID: "p:1", Name: "p", Kind: models.SurfacePrompt},
		},
		TestFiles: []models.TestFile{
			{Path: "tests/unit/x.py"},
		},
	}
	g := &impact.ImpactGraph{UnitToTests: map[string][]string{}}
	sigs := DetectMissingPerfTest(snap, g)
	if len(sigs) != 0 {
		t.Errorf("expected silent, got %+v", sigs)
	}
}

func TestDetectMissingPerfTest_NonLatencyCriticalSkipped(t *testing.T) {
	t.Parallel()
	snap := &models.TestSuiteSnapshot{
		CodeSurfaces: []models.CodeSurface{
			{SurfaceID: "ds:1", Name: "dataset", Kind: models.SurfaceDataset},
		},
		TestFiles: []models.TestFile{
			{Path: "benchmarks/x.py"},
		},
	}
	g := &impact.ImpactGraph{UnitToTests: map[string][]string{}}
	sigs := DetectMissingPerfTest(snap, g)
	if len(sigs) != 0 {
		t.Errorf("dataset surface should not be latency-critical, got %+v", sigs)
	}
}
