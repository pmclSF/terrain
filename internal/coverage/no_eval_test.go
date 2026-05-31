package coverage

import (
	"testing"

	"github.com/pmclSF/terrain/internal/models"
)

func TestDetectNoEvalForAISurface_FiresOnUncovered(t *testing.T) {
	t.Parallel()
	snap := &models.TestSuiteSnapshot{
		CodeSurfaces: []models.CodeSurface{
			{SurfaceID: "p:1", Name: "summarizer_prompt", Path: "prompts/summarize.txt", Kind: models.SurfacePrompt},
			{SurfaceID: "m:1", Name: "classifier.pt", Path: "models/classifier.pt", Kind: models.SurfaceModel},
			{SurfaceID: "f:1", Name: "regular_function", Path: "src/util.go", Kind: models.SurfaceFunction},
		},
		Evals: []models.Eval{
			{EvalID: "e:1", CoveredSurfaceIDs: []string{"p:1"}},
		},
	}
	sigs := DetectNoEvalForAISurface(snap)

	// Should fire for m:1 (SurfaceModel uncovered) but not p:1 (covered)
	// nor f:1 (not AI-typed).
	if len(sigs) != 1 {
		t.Fatalf("signals = %d, want 1: %+v", len(sigs), sigs)
	}
	if sigs[0].Metadata["surfaceId"] != "m:1" {
		t.Errorf("fired on %v, want m:1", sigs[0].Metadata["surfaceId"])
	}
}

func TestDetectNoEvalForAISurface_AllAIKinds(t *testing.T) {
	t.Parallel()
	kinds := []models.CodeSurfaceKind{
		models.SurfacePrompt,
		models.SurfaceContext,
		models.SurfaceDataset,
		models.SurfaceToolDef,
		models.SurfaceRetrieval,
		models.SurfaceAgent,
		models.SurfaceEvalDef,
		models.SurfaceModel,
	}
	snap := &models.TestSuiteSnapshot{}
	for i, k := range kinds {
		snap.CodeSurfaces = append(snap.CodeSurfaces, models.CodeSurface{
			SurfaceID: string(k),
			Name:      string(k) + "_x",
			Path:      "src/x.py",
			Kind:      k,
		})
		_ = i
	}
	sigs := DetectNoEvalForAISurface(snap)
	if len(sigs) != len(kinds) {
		t.Errorf("expected one signal per AI kind (%d), got %d", len(kinds), len(sigs))
	}
}

func TestDetectNoEvalForAISurface_NonAIKindsIgnored(t *testing.T) {
	t.Parallel()
	snap := &models.TestSuiteSnapshot{
		CodeSurfaces: []models.CodeSurface{
			{SurfaceID: "fn", Name: "fn", Path: "src/x.go", Kind: models.SurfaceFunction},
			{SurfaceID: "method", Name: "m", Path: "src/x.go", Kind: models.SurfaceMethod},
			{SurfaceID: "handler", Name: "h", Path: "src/x.go", Kind: models.SurfaceHandler},
			{SurfaceID: "route", Name: "r", Path: "src/x.go", Kind: models.SurfaceRoute},
			{SurfaceID: "class", Name: "C", Path: "src/x.go", Kind: models.SurfaceClass},
			{SurfaceID: "fixture", Name: "f", Path: "src/x.go", Kind: models.SurfaceFixture},
		},
	}
	sigs := DetectNoEvalForAISurface(snap)
	if len(sigs) != 0 {
		t.Errorf("non-AI kinds should not fire, got %d", len(sigs))
	}
}

func TestDetectNoEvalForAISurface_NilSnapshot(t *testing.T) {
	t.Parallel()
	if got := DetectNoEvalForAISurface(nil); len(got) != 0 {
		t.Errorf("nil snap should yield no signals, got %d", len(got))
	}
}
