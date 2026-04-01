package analyze

import (
	"strings"
	"testing"

	"github.com/pmclSF/terrain/internal/stability"
)

func TestDeriveHeadline_CriticalSignals(t *testing.T) {
	r := &Report{
		SignalSummary: SignalBreakdown{Critical: 5},
	}
	h := deriveHeadline(r)
	if !strings.Contains(h, "5 critical") {
		t.Errorf("expected critical mention, got: %s", h)
	}
}

func TestDeriveHeadline_Redundancy(t *testing.T) {
	r := &Report{
		DuplicateClusters: DuplicateSummary{
			RedundantTestCount: 200,
			ClusterCount:       15,
		},
	}
	h := deriveHeadline(r)
	if !strings.Contains(h, "200 tests") || !strings.Contains(h, "15 clusters") {
		t.Errorf("expected redundancy mention, got: %s", h)
	}
}

func TestDeriveHeadline_Fanout(t *testing.T) {
	r := &Report{
		HighFanout: FanoutSummary{FlaggedCount: 3},
	}
	h := deriveHeadline(r)
	if !strings.Contains(h, "3 shared fixtures") {
		t.Errorf("expected fanout mention, got: %s", h)
	}
}

func TestDeriveHeadline_WeakCoverage(t *testing.T) {
	r := &Report{
		WeakCoverageAreas: []WeakArea{{Path: "src/api/"}},
	}
	h := deriveHeadline(r)
	if !strings.Contains(h, "1 source area") {
		t.Errorf("expected coverage mention, got: %s", h)
	}
}

func TestDeriveHeadline_Unstable(t *testing.T) {
	r := &Report{
		StabilityClusters: &stability.ClusterResult{
			UnstableTestCount: 8,
			Clusters: []stability.Cluster{
				{ID: "c1"},
				{ID: "c2"},
			},
		},
	}
	h := deriveHeadline(r)
	if !strings.Contains(h, "8 tests are unstable") || !strings.Contains(h, "2 shared root") {
		t.Errorf("expected stability mention, got: %s", h)
	}
}

func TestDeriveHeadline_Healthy(t *testing.T) {
	r := &Report{
		TestsDetected: TestSummary{
			TestFileCount: 42,
			Frameworks:    []FrameworkCount{{Name: "jest"}, {Name: "pytest"}},
		},
	}
	h := deriveHeadline(r)
	if !strings.Contains(h, "healthy") || !strings.Contains(h, "42 test files") {
		t.Errorf("expected healthy mention, got: %s", h)
	}
}

func TestDeriveHeadline_PriorityOrder(t *testing.T) {
	// When multiple conditions match, critical signals should win.
	r := &Report{
		SignalSummary:     SignalBreakdown{Critical: 2},
		DuplicateClusters: DuplicateSummary{RedundantTestCount: 100, ClusterCount: 5},
		HighFanout:        FanoutSummary{FlaggedCount: 3},
	}
	h := deriveHeadline(r)
	if !strings.Contains(h, "critical") {
		t.Errorf("critical should take priority, got: %s", h)
	}
}
