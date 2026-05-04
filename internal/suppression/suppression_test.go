package suppression

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pmclSF/terrain/internal/identity"
	"github.com/pmclSF/terrain/internal/models"
)

// ── Load ────────────────────────────────────────────────────────────

func TestLoad_Missing(t *testing.T) {
	t.Parallel()
	r, err := Load(filepath.Join(t.TempDir(), "does-not-exist.yaml"))
	if err != nil {
		t.Fatalf("missing file should not error: %v", err)
	}
	if r == nil || len(r.Entries) != 0 {
		t.Errorf("expected empty result, got %+v", r)
	}
}

func TestLoad_ValidFindingID(t *testing.T) {
	t.Parallel()
	body := `schema_version: "1"
suppressions:
  - finding_id: weakAssertion@internal/auth/login_test.go:TestLogin#a1b2c3d4
    reason: false positive; sanitized upstream
    expires: 2026-08-01
    owner: "@platform-team"
`
	path := writeTemp(t, body)
	r, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(r.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(r.Entries))
	}
	e := r.Entries[0]
	if e.FindingID == "" || e.Reason == "" || e.Owner == "" {
		t.Errorf("expected populated entry, got %+v", e)
	}
	if e.expiresAt.IsZero() {
		t.Error("expiresAt should be parsed for 2026-08-01")
	}
}

func TestLoad_ValidSignalTypeFile(t *testing.T) {
	t.Parallel()
	body := `schema_version: "1"
suppressions:
  - signal_type: aiPromptInjectionRisk
    file: internal/legacy/**
    reason: rewriting in 0.3
`
	r, err := Load(writeTemp(t, body))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(r.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(r.Entries))
	}
}

func TestLoad_RejectsCombinedModes(t *testing.T) {
	t.Parallel()
	body := `schema_version: "1"
suppressions:
  - finding_id: weakAssertion@a.go:b#hash
    signal_type: weakAssertion
    file: "*.go"
    reason: bad
`
	_, err := Load(writeTemp(t, body))
	if err == nil || !strings.Contains(err.Error(), "cannot combine") {
		t.Errorf("expected 'cannot combine' error, got %v", err)
	}
}

func TestLoad_RejectsNoMatchMode(t *testing.T) {
	t.Parallel()
	body := `schema_version: "1"
suppressions:
  - reason: bad — neither finding_id nor signal_type+file
`
	_, err := Load(writeTemp(t, body))
	if err == nil || !strings.Contains(err.Error(), "must set either finding_id") {
		t.Errorf("expected 'must set either' error, got %v", err)
	}
}

func TestLoad_RejectsMissingReason(t *testing.T) {
	t.Parallel()
	body := `schema_version: "1"
suppressions:
  - finding_id: weakAssertion@a.go:b#hash
`
	_, err := Load(writeTemp(t, body))
	if err == nil || !strings.Contains(err.Error(), "reason is required") {
		t.Errorf("expected 'reason is required' error, got %v", err)
	}
}

func TestLoad_RejectsBadSchemaVersion(t *testing.T) {
	t.Parallel()
	body := `schema_version: "999"
suppressions: []
`
	_, err := Load(writeTemp(t, body))
	if err == nil || !strings.Contains(err.Error(), "schema_version") {
		t.Errorf("expected schema_version error, got %v", err)
	}
}

func TestLoad_WarnsOnBadExpiry(t *testing.T) {
	t.Parallel()
	body := `schema_version: "1"
suppressions:
  - finding_id: weakAssertion@a.go:b#hash
    reason: ok
    expires: not-a-date
`
	r, err := Load(writeTemp(t, body))
	if err != nil {
		t.Fatalf("should not error on bad expiry, got %v", err)
	}
	if len(r.Warnings) != 1 || !strings.Contains(r.Warnings[0], "unparseable expires") {
		t.Errorf("expected unparseable-expires warning, got %v", r.Warnings)
	}
	// Entry still loaded; treated as no expiry.
	if len(r.Entries) != 1 || !r.Entries[0].expiresAt.IsZero() {
		t.Errorf("entry should load with zero expiresAt, got %+v", r.Entries)
	}
}

// ── Apply ───────────────────────────────────────────────────────────

func TestApply_FindingIDExactMatch(t *testing.T) {
	t.Parallel()
	id := identity.BuildFindingID("weakAssertion", "internal/auth/login_test.go", "TestLogin", 42)
	snap := &models.TestSuiteSnapshot{
		Signals: []models.Signal{
			{
				Type:      "weakAssertion",
				FindingID: id,
				Location:  models.SignalLocation{File: "internal/auth/login_test.go", Symbol: "TestLogin", Line: 42},
			},
			{
				Type:      "mockHeavyTest",
				FindingID: "mockHeavyTest@a.go:b#diff",
				Location:  models.SignalLocation{File: "a.go", Line: 1},
			},
		},
	}
	entries := []Entry{
		{FindingID: id, Reason: "fp"},
	}
	matched, expired := Apply(snap, entries, time.Now())
	if len(expired) != 0 {
		t.Errorf("no expired entries expected, got %v", expired)
	}
	if len(matched) != 1 {
		t.Errorf("expected 1 matched entry, got %v", matched)
	}
	if len(snap.Signals) != 1 {
		t.Errorf("expected 1 surviving signal, got %d", len(snap.Signals))
	}
	if string(snap.Signals[0].Type) != "mockHeavyTest" {
		t.Errorf("wrong signal survived: %+v", snap.Signals[0])
	}
}

func TestApply_SignalTypeFileGlob(t *testing.T) {
	t.Parallel()
	snap := &models.TestSuiteSnapshot{
		Signals: []models.Signal{
			{
				Type:     "aiPromptInjectionRisk",
				Location: models.SignalLocation{File: "internal/legacy/foo.go"},
			},
			{
				Type:     "aiPromptInjectionRisk",
				Location: models.SignalLocation{File: "internal/auth/foo.go"},
			},
			{
				Type:     "weakAssertion",
				Location: models.SignalLocation{File: "internal/legacy/foo.go"},
			},
		},
	}
	entries := []Entry{
		{SignalType: "aiPromptInjectionRisk", File: "internal/legacy/**", Reason: "rewriting"},
	}
	Apply(snap, entries, time.Now())
	if len(snap.Signals) != 2 {
		t.Errorf("expected 2 surviving signals, got %d", len(snap.Signals))
	}
	for _, s := range snap.Signals {
		if string(s.Type) == "aiPromptInjectionRisk" && strings.HasPrefix(s.Location.File, "internal/legacy/") {
			t.Errorf("legacy aiPromptInjectionRisk should be suppressed: %+v", s)
		}
	}
}

func TestApply_ExpiredEntryDoesNotMatch(t *testing.T) {
	t.Parallel()
	id := identity.BuildFindingID("weakAssertion", "a.go", "X", 1)
	snap := &models.TestSuiteSnapshot{
		Signals: []models.Signal{
			{Type: "weakAssertion", FindingID: id, Location: models.SignalLocation{File: "a.go", Symbol: "X", Line: 1}},
		},
	}
	entries := []Entry{
		{FindingID: id, Reason: "fp", expiresAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)},
	}
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	matched, expired := Apply(snap, entries, now)
	if len(matched) != 0 {
		t.Errorf("expired entry should not match; got %v", matched)
	}
	if len(expired) != 1 {
		t.Errorf("expected 1 expired entry, got %v", expired)
	}
	if len(snap.Signals) != 1 {
		t.Errorf("expired entry should not suppress; signal should remain. got %d signals", len(snap.Signals))
	}
}

func TestApply_PerFileSignals(t *testing.T) {
	t.Parallel()
	id := identity.BuildFindingID("weakAssertion", "a.go", "X", 1)
	snap := &models.TestSuiteSnapshot{
		Signals: []models.Signal{
			{Type: "weakAssertion", FindingID: id, Location: models.SignalLocation{File: "a.go", Symbol: "X", Line: 1}},
		},
		TestFiles: []models.TestFile{
			{
				Path: "a.go",
				Signals: []models.Signal{
					{Type: "weakAssertion", FindingID: id, Location: models.SignalLocation{File: "a.go", Symbol: "X", Line: 1}},
				},
			},
		},
	}
	entries := []Entry{{FindingID: id, Reason: "fp"}}
	Apply(snap, entries, time.Now())
	if len(snap.Signals) != 0 {
		t.Errorf("top-level signal should be suppressed")
	}
	if len(snap.TestFiles[0].Signals) != 0 {
		t.Errorf("per-file signal should be suppressed")
	}
}

func TestApply_NilSafe(t *testing.T) {
	t.Parallel()
	matched, expired := Apply(nil, []Entry{{FindingID: "x", Reason: "y"}}, time.Now())
	if matched != nil || expired != nil {
		t.Error("nil snapshot should produce nil returns")
	}
}

func TestApply_EmptyEntriesNoop(t *testing.T) {
	t.Parallel()
	snap := &models.TestSuiteSnapshot{
		Signals: []models.Signal{{Type: "x", FindingID: "y"}},
	}
	matched, expired := Apply(snap, nil, time.Now())
	if matched != nil || expired != nil {
		t.Error("nil entries should produce nil returns")
	}
	if len(snap.Signals) != 1 {
		t.Error("nil entries should not modify snapshot")
	}
}

// ── pathMatch ───────────────────────────────────────────────────────

func TestPathMatch_RecursiveStarStar(t *testing.T) {
	t.Parallel()
	cases := []struct {
		pattern, path string
		want          bool
	}{
		{"internal/legacy/**", "internal/legacy/foo.go", true},
		{"internal/legacy/**", "internal/legacy/sub/foo.go", true},
		{"internal/legacy/**", "internal/auth/foo.go", false},
		{"**/legacy/*.go", "internal/legacy/foo.go", true},
		{"**/legacy/*.go", "deep/nested/legacy/foo.go", true},
		{"**/legacy/*.go", "internal/legacy/sub/foo.go", false}, // single star doesn't cross /
		{"*.go", "foo.go", true},
		{"*.go", "sub/foo.go", false},
	}
	for _, tc := range cases {
		got, err := pathMatch(tc.pattern, tc.path)
		if err != nil {
			t.Errorf("pathMatch(%q, %q) error: %v", tc.pattern, tc.path, err)
			continue
		}
		if got != tc.want {
			t.Errorf("pathMatch(%q, %q) = %v, want %v", tc.pattern, tc.path, got, tc.want)
		}
	}
}

// ── helpers ─────────────────────────────────────────────────────────

func writeTemp(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "suppressions.yaml")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}
