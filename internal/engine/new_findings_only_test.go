package engine

import (
	"testing"

	"github.com/pmclSF/terrain/internal/identity"
	"github.com/pmclSF/terrain/internal/models"
)

func TestApplyNewFindingsOnly_DropsBaselineMatches(t *testing.T) {
	t.Parallel()

	// Two signals share an ID with the baseline; one is new.
	id1 := identity.BuildFindingID("weakAssertion", "a.go", "X", 1)
	id2 := identity.BuildFindingID("weakAssertion", "b.go", "Y", 2)
	idNew := identity.BuildFindingID("mockHeavyTest", "c.go", "Z", 3)

	snap := &models.TestSuiteSnapshot{
		Signals: []models.Signal{
			{Type: "weakAssertion", FindingID: id1},
			{Type: "weakAssertion", FindingID: id2},
			{Type: "mockHeavyTest", FindingID: idNew},
		},
		Baseline: &models.TestSuiteSnapshot{
			Signals: []models.Signal{
				{Type: "weakAssertion", FindingID: id1},
				{Type: "weakAssertion", FindingID: id2},
			},
		},
	}

	applyNewFindingsOnly(snap)

	if len(snap.Signals) != 1 {
		t.Fatalf("expected 1 surviving signal (the new one), got %d", len(snap.Signals))
	}
	if snap.Signals[0].FindingID != idNew {
		t.Errorf("wrong signal survived: %+v", snap.Signals[0])
	}
}

func TestApplyNewFindingsOnly_NoBaselineLogsWarning(t *testing.T) {
	t.Parallel()
	id := identity.BuildFindingID("weakAssertion", "a.go", "X", 1)
	snap := &models.TestSuiteSnapshot{
		Signals: []models.Signal{
			{Type: "weakAssertion", FindingID: id},
		},
		// Baseline intentionally nil — flag is inert.
	}
	applyNewFindingsOnly(snap)
	// Without a baseline, every signal stays.
	if len(snap.Signals) != 1 {
		t.Errorf("no-baseline case should not filter; got %d signals", len(snap.Signals))
	}
}

func TestApplyNewFindingsOnly_EmptyBaselineKeepsAll(t *testing.T) {
	t.Parallel()
	id := identity.BuildFindingID("weakAssertion", "a.go", "X", 1)
	snap := &models.TestSuiteSnapshot{
		Signals: []models.Signal{
			{Type: "weakAssertion", FindingID: id},
		},
		Baseline: &models.TestSuiteSnapshot{
			Signals: []models.Signal{}, // populated but empty
		},
	}
	applyNewFindingsOnly(snap)
	if len(snap.Signals) != 1 {
		t.Errorf("empty baseline should not filter; got %d signals", len(snap.Signals))
	}
}

func TestApplyNewFindingsOnly_PerFileSignals(t *testing.T) {
	t.Parallel()
	id := identity.BuildFindingID("weakAssertion", "a.go", "X", 1)
	idNew := identity.BuildFindingID("mockHeavyTest", "b.go", "Y", 2)

	snap := &models.TestSuiteSnapshot{
		Signals: []models.Signal{
			{Type: "weakAssertion", FindingID: id},
		},
		TestFiles: []models.TestFile{
			{
				Path: "a.go",
				Signals: []models.Signal{
					{Type: "weakAssertion", FindingID: id},   // existing → drop
					{Type: "mockHeavyTest", FindingID: idNew}, // new → keep
				},
			},
		},
		Baseline: &models.TestSuiteSnapshot{
			Signals: []models.Signal{
				{Type: "weakAssertion", FindingID: id},
			},
		},
	}

	applyNewFindingsOnly(snap)

	if len(snap.Signals) != 0 {
		t.Errorf("top-level matching baseline should be dropped; got %d", len(snap.Signals))
	}
	if len(snap.TestFiles[0].Signals) != 1 {
		t.Fatalf("expected 1 surviving per-file signal, got %d", len(snap.TestFiles[0].Signals))
	}
	if snap.TestFiles[0].Signals[0].FindingID != idNew {
		t.Errorf("wrong signal survived per-file: %+v", snap.TestFiles[0].Signals[0])
	}
}

func TestApplyNewFindingsOnly_KeepsSignalsWithoutFindingID(t *testing.T) {
	t.Parallel()
	// Older or specially-emitted signals may not have a FindingID. The
	// filter shouldn't silently drop them — over-report rather than
	// under-report.
	snap := &models.TestSuiteSnapshot{
		Signals: []models.Signal{
			{Type: "weakAssertion"}, // no FindingID
		},
		Baseline: &models.TestSuiteSnapshot{
			Signals: []models.Signal{
				{Type: "mockHeavyTest", FindingID: "something"},
			},
		},
	}
	applyNewFindingsOnly(snap)
	if len(snap.Signals) != 1 {
		t.Errorf("signals without FindingID should be kept; got %d", len(snap.Signals))
	}
}

func TestApplyNewFindingsOnly_NilSafe(t *testing.T) {
	t.Parallel()
	applyNewFindingsOnly(nil)
	applyNewFindingsOnly(&models.TestSuiteSnapshot{}) // no signals, no baseline
}
