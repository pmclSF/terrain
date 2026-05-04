package engine

import (
	"strings"
	"testing"

	"github.com/pmclSF/terrain/internal/models"
)

func TestAssignFindingIDs_TopLevelAndPerFile(t *testing.T) {
	t.Parallel()
	snap := &models.TestSuiteSnapshot{
		Signals: []models.Signal{
			{
				Type: "weakAssertion",
				Location: models.SignalLocation{
					File: "internal/auth/login_test.go", Symbol: "TestLogin", Line: 42,
				},
			},
		},
		TestFiles: []models.TestFile{
			{
				Path: "internal/auth/login_test.go",
				Signals: []models.Signal{
					{
						Type: "mockHeavyTest",
						Location: models.SignalLocation{
							File: "internal/auth/login_test.go", Symbol: "TestLogin", Line: 100,
						},
					},
				},
			},
		},
	}
	assignFindingIDs(snap)

	if snap.Signals[0].FindingID == "" {
		t.Error("top-level signal FindingID was not populated")
	}
	if snap.TestFiles[0].Signals[0].FindingID == "" {
		t.Error("per-file signal FindingID was not populated")
	}
	if !strings.HasPrefix(snap.Signals[0].FindingID, "weakAssertion@") {
		t.Errorf("top-level FindingID has wrong shape: %q", snap.Signals[0].FindingID)
	}
	if !strings.HasPrefix(snap.TestFiles[0].Signals[0].FindingID, "mockHeavyTest@") {
		t.Errorf("per-file FindingID has wrong shape: %q", snap.TestFiles[0].Signals[0].FindingID)
	}
}

func TestAssignFindingIDs_PreservesPreSetID(t *testing.T) {
	t.Parallel()
	snap := &models.TestSuiteSnapshot{
		Signals: []models.Signal{
			{
				Type:      "weakAssertion",
				FindingID: "custom@id",
				Location: models.SignalLocation{
					File: "internal/auth/login_test.go", Symbol: "TestLogin", Line: 42,
				},
			},
		},
	}
	assignFindingIDs(snap)
	if snap.Signals[0].FindingID != "custom@id" {
		t.Errorf("pre-set FindingID was overwritten: %q", snap.Signals[0].FindingID)
	}
}

func TestAssignFindingIDs_Idempotent(t *testing.T) {
	t.Parallel()
	snap := &models.TestSuiteSnapshot{
		Signals: []models.Signal{
			{
				Type: "weakAssertion",
				Location: models.SignalLocation{
					File: "internal/auth/login_test.go", Symbol: "TestLogin", Line: 42,
				},
			},
		},
	}
	assignFindingIDs(snap)
	first := snap.Signals[0].FindingID
	assignFindingIDs(snap)
	second := snap.Signals[0].FindingID
	if first != second {
		t.Errorf("non-idempotent: first=%q, second=%q", first, second)
	}
}

func TestAssignFindingIDs_NilSafe(t *testing.T) {
	t.Parallel()
	// Should not panic on nil snapshot.
	assignFindingIDs(nil)
	// Should not panic on snapshot with no signals.
	assignFindingIDs(&models.TestSuiteSnapshot{})
}
