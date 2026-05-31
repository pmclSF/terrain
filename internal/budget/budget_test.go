package budget

import (
	"testing"

	"github.com/pmclSF/terrain/internal/models"
)

func TestApplyCapsHighVolumeRule(t *testing.T) {
	snap := &models.TestSuiteSnapshot{
		Signals: []models.Signal{
			{RuleID: "terrain/coverage/untested-export", Severity: "medium", Confidence: 0.9, Location: models.SignalLocation{File: "a.go"}},
			{RuleID: "terrain/coverage/untested-export", Severity: "high", Confidence: 0.95, Location: models.SignalLocation{File: "b.go"}},
			{RuleID: "terrain/coverage/untested-export", Severity: "low", Confidence: 0.5, Location: models.SignalLocation{File: "c.go"}},
			{RuleID: "terrain/coverage/untested-export", Severity: "medium", Confidence: 0.8, Location: models.SignalLocation{File: "d.go"}},
			{RuleID: "terrain/ai/hardcoded-api-key", Severity: "high", Confidence: 0.9, Location: models.SignalLocation{File: "x.go"}},
		},
	}
	pruned := Apply(snap, map[string]int{
		"terrain/coverage/untested-export": 2,
	})
	if pruned["terrain/coverage/untested-export"] != 2 {
		t.Fatalf("expected 2 pruned, got %d", pruned["terrain/coverage/untested-export"])
	}
	if len(snap.Signals) != 3 {
		t.Fatalf("expected 3 signals kept (2 untestedExport + 1 unrelated), got %d", len(snap.Signals))
	}
	// The high-severity one must survive.
	foundHigh := false
	for _, s := range snap.Signals {
		if s.RuleID == "terrain/coverage/untested-export" && s.Severity == "high" {
			foundHigh = true
		}
	}
	if !foundHigh {
		t.Fatal("highest-severity untestedExport finding was pruned (priority order broken)")
	}
}

func TestApplyNoBudgetIsNoop(t *testing.T) {
	snap := &models.TestSuiteSnapshot{
		Signals: []models.Signal{
			{RuleID: "terrain/coverage/untested-export"},
			{RuleID: "terrain/coverage/untested-export"},
		},
	}
	pruned := Apply(snap, nil)
	if len(pruned) != 0 || len(snap.Signals) != 2 {
		t.Fatalf("nil budget should be a no-op")
	}
}

func TestApplyZeroMaxIsIgnored(t *testing.T) {
	snap := &models.TestSuiteSnapshot{
		Signals: []models.Signal{
			{RuleID: "terrain/coverage/untested-export"},
		},
	}
	pruned := Apply(snap, map[string]int{"terrain/coverage/untested-export": 0})
	if len(pruned) != 0 || len(snap.Signals) != 1 {
		t.Fatal("max=0 must be treated as unlimited, not as a zero-cap")
	}
}
