package coverage

import (
	"testing"

	"github.com/pmclSF/terrain/internal/impact"
	"github.com/pmclSF/terrain/internal/models"
)

func TestDetectNoIntegrationTest_FiresWhenEntryPointUncovered(t *testing.T) {
	t.Parallel()
	snap := &models.TestSuiteSnapshot{
		CodeSurfaces: []models.CodeSurface{
			{SurfaceID: "h:1", Name: "handleRefund", Path: "backend/refund.go", Kind: models.SurfaceHandler},
		},
		TestFiles: []models.TestFile{
			{Path: "tests/unit/refund_test.go"},                  // unit test
			{Path: "tests/integration/checkout_test.go"},          // integration test, doesn't cover h:1
		},
	}
	g := &impact.ImpactGraph{
		UnitToTests: map[string][]string{
			"h:1": {"tests/unit/refund_test.go"},
		},
	}
	sigs := DetectNoIntegrationTest(snap, g)
	if len(sigs) != 1 {
		t.Fatalf("expected 1 signal, got %d", len(sigs))
	}
}

func TestDetectNoIntegrationTest_SuppressedWhenCovered(t *testing.T) {
	t.Parallel()
	snap := &models.TestSuiteSnapshot{
		CodeSurfaces: []models.CodeSurface{
			{SurfaceID: "h:1", Name: "handleRefund", Path: "backend/refund.go", Kind: models.SurfaceHandler},
		},
		TestFiles: []models.TestFile{
			{Path: "tests/integration/refund_test.go"},
		},
	}
	g := &impact.ImpactGraph{
		UnitToTests: map[string][]string{
			"h:1": {"tests/integration/refund_test.go"},
		},
	}
	sigs := DetectNoIntegrationTest(snap, g)
	if len(sigs) != 0 {
		t.Errorf("expected suppression, got %+v", sigs)
	}
}

func TestDetectNoIntegrationTest_NoIntegrationTestsAnywhere(t *testing.T) {
	t.Parallel()
	// Repo has no integration tests; the rule stays silent for 0.2.0.
	snap := &models.TestSuiteSnapshot{
		CodeSurfaces: []models.CodeSurface{
			{SurfaceID: "h:1", Name: "h", Kind: models.SurfaceHandler},
		},
		TestFiles: []models.TestFile{
			{Path: "tests/unit/x_test.go"},
		},
	}
	g := &impact.ImpactGraph{UnitToTests: map[string][]string{}}
	sigs := DetectNoIntegrationTest(snap, g)
	if len(sigs) != 0 {
		t.Errorf("expected silent when no integration tests exist, got %+v", sigs)
	}
}

func TestDetectNoIntegrationTest_NonEntryPointSkipped(t *testing.T) {
	t.Parallel()
	snap := &models.TestSuiteSnapshot{
		CodeSurfaces: []models.CodeSurface{
			{SurfaceID: "fn", Name: "fn", Kind: models.SurfaceFunction},
		},
		TestFiles: []models.TestFile{
			{Path: "tests/integration/x_test.go"},
		},
	}
	g := &impact.ImpactGraph{UnitToTests: map[string][]string{}}
	sigs := DetectNoIntegrationTest(snap, g)
	if len(sigs) != 0 {
		t.Errorf("non-entry-point should not fire, got %+v", sigs)
	}
}
