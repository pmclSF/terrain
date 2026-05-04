package signals

import (
	"strings"
	"testing"
	"time"

	"github.com/pmclSF/terrain/internal/models"
)

// Track 9.4 — per-detector budget enforcement tests.

// slowDetector deliberately sleeps past its budget so the timeout
// path in safeDetectWithBudget exercises end-to-end. The work-time
// is parameterized so individual tests can probe boundary cases.
type slowDetector struct {
	work time.Duration
}

func (d *slowDetector) Detect(_ *models.TestSuiteSnapshot) []models.Signal {
	time.Sleep(d.work)
	return []models.Signal{{
		Type:        models.SignalType("slow.work-completed"),
		Category:    models.CategoryQuality,
		Severity:    models.SeverityLow,
		Confidence:  1.0,
		Explanation: "slow detector finished without being abandoned",
	}}
}

// TestSafeDetectWithBudget_BudgetExceeded verifies that a detector
// running past its budget is abandoned and produces a budget-
// exceeded marker signal. The detector's eventual completion
// signal is NOT returned (the contract: when the budget elapses,
// the pipeline moves on).
func TestSafeDetectWithBudget_BudgetExceeded(t *testing.T) {
	t.Parallel()
	reg := DetectorRegistration{
		Meta: DetectorMeta{
			ID:     "test.slow",
			Domain: DomainQuality,
			Budget: 30 * time.Millisecond,
		},
		Detector: &slowDetector{work: 200 * time.Millisecond},
	}

	start := time.Now()
	got := safeDetectWithBudget(reg, func() []models.Signal {
		return reg.Detector.Detect(nil)
	})
	elapsed := time.Since(start)

	// Should return within budget + small overhead, not after the
	// detector's full work duration.
	if elapsed > 100*time.Millisecond {
		t.Errorf("budget exceeded but wrapper waited %v (budget 30ms; detector work 200ms)", elapsed)
	}

	if len(got) != 1 {
		t.Fatalf("expected 1 marker signal, got %d", len(got))
	}
	if got[0].Type != signalTypeDetectorBudgetExceeded {
		t.Errorf("Type = %q, want %q", got[0].Type, signalTypeDetectorBudgetExceeded)
	}
	if !strings.Contains(got[0].Explanation, "test.slow") {
		t.Errorf("explanation should name the detector ID: %q", got[0].Explanation)
	}
	if !strings.Contains(got[0].Explanation, "30ms") {
		t.Errorf("explanation should name the budget: %q", got[0].Explanation)
	}
}

// TestSafeDetectWithBudget_FastDetectorPasses verifies the happy
// path: a detector that completes within its budget returns
// normally. The marker signal does NOT appear.
func TestSafeDetectWithBudget_FastDetectorPasses(t *testing.T) {
	t.Parallel()
	reg := DetectorRegistration{
		Meta: DetectorMeta{
			ID:     "test.fast",
			Domain: DomainQuality,
			Budget: 100 * time.Millisecond,
		},
		Detector: &slowDetector{work: 5 * time.Millisecond},
	}

	got := safeDetectWithBudget(reg, func() []models.Signal {
		return reg.Detector.Detect(nil)
	})

	if len(got) != 1 {
		t.Fatalf("expected 1 signal from completing detector, got %d", len(got))
	}
	if got[0].Type == signalTypeDetectorBudgetExceeded {
		t.Errorf("fast detector should not produce a budget-exceeded marker")
	}
	if got[0].Type != "slow.work-completed" {
		t.Errorf("Type = %q, want slow.work-completed (the detector's own signal)", got[0].Type)
	}
}

// TestSafeDetectWithBudget_ZeroBudgetUsesDefault verifies that a
// detector with Budget=0 picks up DefaultDetectorBudget rather than
// timing out immediately. This is the contract for legacy detectors
// registered before Track 9.4 — Budget defaults to zero, behavior
// stays the same as pre-Track-9.4 (no enforced timeout) but with
// the safety net of the default.
func TestSafeDetectWithBudget_ZeroBudgetUsesDefault(t *testing.T) {
	t.Parallel()
	reg := DetectorRegistration{
		Meta: DetectorMeta{
			ID:     "test.no-budget",
			Domain: DomainQuality,
			// Budget intentionally zero — should use DefaultDetectorBudget.
		},
		Detector: &slowDetector{work: 5 * time.Millisecond},
	}

	got := safeDetectWithBudget(reg, func() []models.Signal {
		return reg.Detector.Detect(nil)
	})

	if len(got) != 1 || got[0].Type == signalTypeDetectorBudgetExceeded {
		t.Errorf("zero-budget detector should pick up the default and complete; got: %+v", got)
	}
}

// TestSafeDetectWithBudget_PanicStillRecovered verifies budget
// enforcement composes with safeDetect's panic recovery. A panicking
// detector inside the budget window should still surface the
// detectorPanic marker, not the budget marker.
func TestSafeDetectWithBudget_PanicStillRecovered(t *testing.T) {
	t.Parallel()
	reg := DetectorRegistration{
		Meta: DetectorMeta{
			ID:     "test.panic",
			Domain: DomainQuality,
			Budget: 100 * time.Millisecond,
		},
	}

	got := safeDetectWithBudget(reg, func() []models.Signal {
		panic("deliberate panic for test")
	})

	if len(got) != 1 {
		t.Fatalf("expected 1 marker signal, got %d", len(got))
	}
	if got[0].Type != "detectorPanic" {
		t.Errorf("panic should produce detectorPanic marker, not %q", got[0].Type)
	}
}

// TestRegistry_Run_BudgetEnforced is the end-to-end integration
// test: register a slow detector with a tight budget; run via the
// registry; verify the snapshot has the budget-exceeded marker, and
// the pipeline didn't hang waiting for the slow detector's eventual
// completion.
func TestRegistry_Run_BudgetEnforced(t *testing.T) {
	t.Parallel()
	r := NewRegistry()
	if err := r.Register(DetectorRegistration{
		Meta: DetectorMeta{
			ID:     "test.slow-in-registry",
			Domain: DomainQuality,
			Budget: 20 * time.Millisecond,
		},
		Detector: &slowDetector{work: 200 * time.Millisecond},
	}); err != nil {
		t.Fatalf("Register: %v", err)
	}

	snap := &models.TestSuiteSnapshot{}
	start := time.Now()
	r.Run(snap)
	elapsed := time.Since(start)

	if elapsed > 100*time.Millisecond {
		t.Errorf("registry Run waited %v for a 20ms-budget detector; budget enforcement broken", elapsed)
	}

	if len(snap.Signals) != 1 {
		t.Fatalf("expected 1 marker signal, got %d", len(snap.Signals))
	}
	if snap.Signals[0].Type != signalTypeDetectorBudgetExceeded {
		t.Errorf("expected budget-exceeded marker, got %q", snap.Signals[0].Type)
	}
}
