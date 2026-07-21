package analysis

import (
	"testing"

	"github.com/pmclSF/terrain/internal/models"
)

func surfacePathSet(s []models.CodeSurface) map[string]bool {
	m := make(map[string]bool, len(s))
	for _, cs := range s {
		m[cs.Path] = true
	}
	return m
}

// TestModelArtifactDirs_Augments verifies ml.artifacts_dir recognizes
// extensionless weight blobs under the configured dir, without weakening
// the always-on extension match and without sweeping in non-artifacts.
func TestModelArtifactDirs_Augments(t *testing.T) {
	defer SetModelArtifactDirs(nil) // reset package global

	src := []string{
		"models/weights",   // extensionless blob — only recognized via artifacts_dir
		"models/readme.md", // doc under the dir — must NOT be recognized
		"ckpt/model.pt",    // known extension — recognized everywhere
		"src/main.py",      // unrelated source — never recognized
	}
	testPaths := map[string]bool{}

	// Without config: known extension matches; extensionless does not.
	got := surfacePathSet(detectModelArtifactSurfaces(testPaths, src))
	if got["models/weights"] {
		t.Error("extensionless file recognized as a model surface without artifacts_dir set")
	}
	if !got["ckpt/model.pt"] {
		t.Error(".pt file should always be recognized as a model surface")
	}

	// With artifacts_dir=models: the extensionless blob is now recognized;
	// the .md doc is not (only extensionless); .pt still matches.
	SetModelArtifactDirs([]string{"models"})
	got = surfacePathSet(detectModelArtifactSurfaces(testPaths, src))
	if !got["models/weights"] {
		t.Error("extensionless file under artifacts_dir should be recognized as a model surface")
	}
	if got["models/readme.md"] {
		t.Error(".md under artifacts_dir wrongly recognized (only extensionless blobs should be)")
	}
	if !got["ckpt/model.pt"] {
		t.Error(".pt should still match with artifacts_dir set (augment, not replace)")
	}
	if got["src/main.py"] {
		t.Error("unrelated source file wrongly recognized as a model surface")
	}

	// Normalization: a messy spec (caps + surrounding slashes) must still
	// match — guards against the dir spec being compared without normalizing.
	SetModelArtifactDirs([]string{"/Models/"})
	got = surfacePathSet(detectModelArtifactSurfaces(testPaths, src))
	if !got["models/weights"] {
		t.Error("artifacts_dir spec '/Models/' should normalize and match models/weights")
	}
}

// TestModelArtifactSurfaces_Exclusions pins the exclusion contract of
// detectModelArtifactSurfaces:
//
//	E1. node_modules/ and vendor/ files are never model surfaces, even with
//	    a known model extension.
//	E2. test-path files are never model surfaces.
func TestModelArtifactSurfaces_Exclusions(t *testing.T) {
	defer SetModelArtifactDirs(nil)
	src := []string{
		"node_modules/pkg/model.pt", // dependency — excluded
		"vendor/lib/model.onnx",     // vendored — excluded
		"tests/fixtures/model.pkl",  // test path — excluded
		"models/real.pt",            // genuine artifact — included
	}
	testPaths := map[string]bool{"tests/fixtures/model.pkl": true}
	got := surfacePathSet(detectModelArtifactSurfaces(testPaths, src))

	for _, excluded := range []string{"node_modules/pkg/model.pt", "vendor/lib/model.onnx", "tests/fixtures/model.pkl"} {
		if got[excluded] {
			t.Errorf("%s must be excluded from model surfaces", excluded)
		}
	}
	if !got["models/real.pt"] {
		t.Error("models/real.pt is a genuine model artifact and must be included")
	}
}
