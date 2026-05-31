package coverage

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pmclSF/terrain/internal/models"
)

func TestDetectMissingBaseline_Fires(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	snap := &models.TestSuiteSnapshot{
		CodeSurfaces: []models.CodeSurface{
			{SurfaceID: "p:1", Name: "prompt", Kind: models.SurfacePrompt},
		},
	}
	sigs := DetectMissingBaseline(root, snap)
	if len(sigs) != 1 {
		t.Fatalf("expected 1 signal, got %d", len(sigs))
	}
}

func TestDetectMissingBaseline_SuppressedByPopulated(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	dir := filepath.Join(root, ".terrain", "baselines")
	_ = os.MkdirAll(dir, 0o755)
	_ = os.WriteFile(filepath.Join(dir, "latest.json"), []byte("{}"), 0o644)

	snap := &models.TestSuiteSnapshot{
		CodeSurfaces: []models.CodeSurface{
			{SurfaceID: "p:1", Name: "prompt", Kind: models.SurfacePrompt},
		},
	}
	sigs := DetectMissingBaseline(root, snap)
	if len(sigs) != 0 {
		t.Errorf("populated baseline dir should suppress, got %+v", sigs)
	}
}

func TestDetectMissingBaseline_NoAISurfaces(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	snap := &models.TestSuiteSnapshot{
		CodeSurfaces: []models.CodeSurface{
			{SurfaceID: "f:1", Name: "fn", Kind: models.SurfaceFunction},
		},
	}
	sigs := DetectMissingBaseline(root, snap)
	if len(sigs) != 0 {
		t.Errorf("no AI surfaces should not fire, got %+v", sigs)
	}
}
