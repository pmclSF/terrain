package coverage

import (
	"testing"

	"github.com/pmclSF/terrain/internal/impact"
	"github.com/pmclSF/terrain/internal/models"
)

func TestDetectNoTestsForCodeUnit_FiresOnUncoveredExported(t *testing.T) {
	t.Parallel()
	snap := &models.TestSuiteSnapshot{
		CodeUnits: []models.CodeUnit{
			{Name: "ProcessRefund", Path: "src/payments.go", Kind: models.CodeUnitKindFunction, Exported: true},
			{Name: "FormatCurrency", Path: "src/util.go", Kind: models.CodeUnitKindFunction, Exported: true},
		},
	}
	g := &impact.ImpactGraph{
		UnitToTests: map[string][]string{
			"src/util.go:FormatCurrency": {"src/util_test.go"},
		},
	}

	sigs := DetectNoTestsForCodeUnit(snap, g)
	if len(sigs) != 1 {
		t.Fatalf("signals = %d, want 1: %+v", len(sigs), sigs)
	}
	if sigs[0].Location.Symbol != "ProcessRefund" {
		t.Errorf("fired on %q, want ProcessRefund", sigs[0].Location.Symbol)
	}
	if sigs[0].RuleID != "terrain/coverage/no-tests" {
		t.Errorf("rule ID = %q", sigs[0].RuleID)
	}
}

func TestDetectNoTestsForCodeUnit_SkipsUnexported(t *testing.T) {
	t.Parallel()
	snap := &models.TestSuiteSnapshot{
		CodeUnits: []models.CodeUnit{
			{Name: "helper", Path: "src/internal.go", Exported: false},
			{Name: "Public", Path: "src/api.go", Exported: true},
		},
	}
	g := &impact.ImpactGraph{UnitToTests: map[string][]string{}}

	sigs := DetectNoTestsForCodeUnit(snap, g)
	if len(sigs) != 1 {
		t.Fatalf("signals = %d, want 1 (unexported skipped): %+v", len(sigs), sigs)
	}
	if sigs[0].Location.Symbol != "Public" {
		t.Errorf("fired on %q", sigs[0].Location.Symbol)
	}
}

func TestDetectNoTestsForCodeUnit_SkipsGeneratedAndTrivial(t *testing.T) {
	t.Parallel()
	snap := &models.TestSuiteSnapshot{
		CodeUnits: []models.CodeUnit{
			{Name: "init", Path: "src/foo.go", Exported: false},
			{Name: "GeneratedFn", Path: "src/__generated__/api.go", Exported: true},
			{Name: "ProtobufFn", Path: "src/messages.pb.go", Exported: true},
			{Name: "VendorFn", Path: "vendor/lib/x.go", Exported: true},
			{Name: "VendorFn2", Path: "node_modules/y.js", Exported: true},
		},
	}
	g := &impact.ImpactGraph{UnitToTests: map[string][]string{}}

	sigs := DetectNoTestsForCodeUnit(snap, g)
	if len(sigs) != 0 {
		t.Errorf("expected 0 signals (all filtered), got %d: %+v", len(sigs), sigs)
	}
}

func TestDetectNoTestsForCodeUnit_NilInputs(t *testing.T) {
	t.Parallel()
	if got := DetectNoTestsForCodeUnit(nil, nil); len(got) != 0 {
		t.Errorf("nil inputs should yield no signals, got %d", len(got))
	}
	if got := DetectNoTestsForCodeUnit(&models.TestSuiteSnapshot{}, nil); len(got) != 0 {
		t.Errorf("nil graph should yield no signals, got %d", len(got))
	}
}

func TestDetectNoTestsForCodeUnit_NameOnlyFallback(t *testing.T) {
	t.Parallel()
	// Graph stores unit ID without path qualifier — happens when
	// coverage attribution can't path-qualify the source.
	snap := &models.TestSuiteSnapshot{
		CodeUnits: []models.CodeUnit{
			{Name: "Foo", Path: "src/foo.go", Exported: true},
		},
	}
	g := &impact.ImpactGraph{
		UnitToTests: map[string][]string{
			"Foo": {"src/foo_test.go"},
		},
	}

	sigs := DetectNoTestsForCodeUnit(snap, g)
	if len(sigs) != 0 {
		t.Errorf("name-only graph entry should suppress signal, got %d: %+v", len(sigs), sigs)
	}
}
