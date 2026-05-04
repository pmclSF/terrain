package engine

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/pmclSF/terrain/internal/identity"
	"github.com/pmclSF/terrain/internal/models"
)

func TestApplySuppressions_DropsMatchingSignal(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, ".terrain"), 0o755); err != nil {
		t.Fatal(err)
	}

	id := identity.BuildFindingID("weakAssertion", "internal/auth/login_test.go", "TestLogin", 42)
	body := `schema_version: "1"
suppressions:
  - finding_id: ` + id + `
    reason: false positive; sanitized upstream
    owner: "@platform"
`
	suppPath := filepath.Join(tmp, ".terrain", "suppressions.yaml")
	if err := os.WriteFile(suppPath, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	snap := &models.TestSuiteSnapshot{
		Signals: []models.Signal{
			{
				Type:      "weakAssertion",
				FindingID: id,
				Location:  models.SignalLocation{File: "internal/auth/login_test.go", Symbol: "TestLogin", Line: 42},
			},
			{
				Type:      "mockHeavyTest",
				FindingID: "mockHeavyTest@a.go:b#xx",
				Location:  models.SignalLocation{File: "a.go", Line: 1},
			},
		},
	}

	applySuppressions(snap, tmp, "", time.Now())

	if len(snap.Signals) != 1 {
		t.Fatalf("expected 1 surviving signal, got %d", len(snap.Signals))
	}
	if string(snap.Signals[0].Type) != "mockHeavyTest" {
		t.Errorf("wrong signal survived: %+v", snap.Signals[0])
	}
}

func TestApplySuppressions_ExpiredEmitsWarning(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, ".terrain"), 0o755); err != nil {
		t.Fatal(err)
	}

	id := identity.BuildFindingID("weakAssertion", "a.go", "X", 1)
	body := `schema_version: "1"
suppressions:
  - finding_id: ` + id + `
    reason: temporary
    expires: "2025-01-01"
`
	suppPath := filepath.Join(tmp, ".terrain", "suppressions.yaml")
	if err := os.WriteFile(suppPath, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	snap := &models.TestSuiteSnapshot{
		Signals: []models.Signal{
			{
				Type:      "weakAssertion",
				FindingID: id,
				Location:  models.SignalLocation{File: "a.go", Symbol: "X", Line: 1},
			},
		},
	}

	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	applySuppressions(snap, tmp, "", now)

	// Original signal should have survived (expired suppression is
	// not in effect) AND a `suppressionExpired` warning signal appears.
	if len(snap.Signals) != 2 {
		t.Fatalf("expected 2 signals (original + expired warning), got %d", len(snap.Signals))
	}
	var foundExpired bool
	for _, s := range snap.Signals {
		if string(s.Type) == "suppressionExpired" {
			foundExpired = true
			if s.Severity != models.SeverityMedium {
				t.Errorf("expired warning should be medium severity, got %s", s.Severity)
			}
			if s.Metadata == nil || s.Metadata["finding_id"] != id {
				t.Errorf("expired warning should carry finding_id metadata: %+v", s.Metadata)
			}
		}
	}
	if !foundExpired {
		t.Error("expected a suppressionExpired warning signal")
	}
}

func TestApplySuppressions_MissingFileNoOp(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir() // no .terrain/suppressions.yaml present

	snap := &models.TestSuiteSnapshot{
		Signals: []models.Signal{
			{Type: "weakAssertion", FindingID: "w@x:y#z"},
		},
	}
	applySuppressions(snap, tmp, "", time.Now())
	if len(snap.Signals) != 1 {
		t.Errorf("missing file should be a no-op; got %d signals", len(snap.Signals))
	}
}

func TestApplySuppressions_MalformedFileLogsAndContinues(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, ".terrain"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, ".terrain", "suppressions.yaml"), []byte("not: [valid yaml"), 0o644); err != nil {
		t.Fatal(err)
	}

	snap := &models.TestSuiteSnapshot{
		Signals: []models.Signal{
			{Type: "weakAssertion", FindingID: "w@x:y#z"},
		},
	}
	applySuppressions(snap, tmp, "", time.Now())
	// Signals should be untouched; we don't fail the pipeline on
	// malformed files (CI users who fat-finger a YAML edit shouldn't
	// lose their analysis).
	if len(snap.Signals) != 1 {
		t.Errorf("malformed file should leave signals intact; got %d", len(snap.Signals))
	}
}

func TestApplySuppressions_OverridePath(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	custom := filepath.Join(tmp, "custom-suppressions.yaml")
	id := identity.BuildFindingID("weakAssertion", "a.go", "X", 1)
	body := `schema_version: "1"
suppressions:
  - finding_id: ` + id + `
    reason: ok
`
	if err := os.WriteFile(custom, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	snap := &models.TestSuiteSnapshot{
		Signals: []models.Signal{
			{Type: "weakAssertion", FindingID: id},
		},
	}
	applySuppressions(snap, tmp, custom, time.Now())
	if len(snap.Signals) != 0 {
		t.Errorf("override path should suppress; got %d signals", len(snap.Signals))
	}
}
