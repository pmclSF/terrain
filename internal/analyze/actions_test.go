package analyze

import (
	"testing"
)

func TestDeriveNextActions_DataCompletenessPriority(t *testing.T) {
	r := &Report{
		DataCompleteness: []DataSource{
			{Name: "coverage", Available: false},
			{Name: "runtime", Available: false},
		},
		DuplicateClusters: DuplicateSummary{ClusterCount: 5, RedundantTestCount: 100},
	}
	actions := deriveNextActions(r)
	if len(actions) < 2 {
		t.Fatalf("expected at least 2 actions, got %d", len(actions))
	}
	// Data completeness actions should come first.
	if actions[0].Title != "Unlock coverage analysis" {
		t.Errorf("first action should be coverage, got: %s", actions[0].Title)
	}
	if actions[1].Title != "Unlock health signals" {
		t.Errorf("second action should be runtime, got: %s", actions[1].Title)
	}
}

func TestDeriveNextActions_MaxThree(t *testing.T) {
	r := &Report{
		DataCompleteness: []DataSource{
			{Name: "coverage", Available: false},
			{Name: "runtime", Available: false},
		},
		DuplicateClusters: DuplicateSummary{ClusterCount: 5, RedundantTestCount: 100},
		WeakCoverageAreas: []WeakArea{{Path: "src/"}},
		HighFanout:        FanoutSummary{FlaggedCount: 3},
	}
	actions := deriveNextActions(r)
	if len(actions) > 3 {
		t.Errorf("expected max 3 actions, got %d", len(actions))
	}
}

func TestDeriveNextActions_AllDataAvailable(t *testing.T) {
	r := &Report{
		DataCompleteness: []DataSource{
			{Name: "coverage", Available: true},
			{Name: "runtime", Available: true},
		},
		DuplicateClusters: DuplicateSummary{ClusterCount: 3, RedundantTestCount: 50},
	}
	actions := deriveNextActions(r)
	if len(actions) == 0 {
		t.Fatal("expected at least 1 action")
	}
	// First action should be finding-based, not data-completeness.
	if actions[0].Title == "Unlock coverage analysis" || actions[0].Title == "Unlock health signals" {
		t.Errorf("should not suggest data actions when data is available, got: %s", actions[0].Title)
	}
}

func TestDeriveNextActions_EmptyReport(t *testing.T) {
	r := &Report{}
	actions := deriveNextActions(r)
	if len(actions) == 0 {
		t.Fatal("expected at least 1 fallback action")
	}
	// Fallback should be the impact command.
	if actions[0].Command != "terrain impact --base main" {
		t.Errorf("expected impact fallback, got: %s", actions[0].Command)
	}
}

func TestDeriveNextActions_CommandsAreRunnable(t *testing.T) {
	r := &Report{
		DuplicateClusters: DuplicateSummary{ClusterCount: 5, RedundantTestCount: 100},
		WeakCoverageAreas: []WeakArea{{Path: "src/"}},
	}
	actions := deriveNextActions(r)
	for _, a := range actions {
		if a.Command == "" {
			t.Errorf("action %q has empty command", a.Title)
		}
		if a.Title == "" {
			t.Error("action has empty title")
		}
		if a.Explanation == "" {
			t.Error("action has empty explanation")
		}
	}
}
