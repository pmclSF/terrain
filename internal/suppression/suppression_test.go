package suppression

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pmclSF/terrain/internal/aliases"
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
    expires: 2099-08-01
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
		t.Error("expiresAt should be parsed for 2099-08-01")
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

// ── ApplyWithAliases (alias-aware suppression) ────────────────────

// TestApplyWithAliases_NilRegistryEquivalentToApply confirms the
// no-registry path matches literal SignalType only (same as Apply).
func TestApplyWithAliases_NilRegistryEquivalentToApply(t *testing.T) {
	snap := &models.TestSuiteSnapshot{
		Signals: []models.Signal{
			{Type: "ruleA", Location: models.SignalLocation{File: "a.go"}},
			{Type: "ruleB", Location: models.SignalLocation{File: "b.go"}},
		},
	}
	entries := []Entry{
		{SignalType: "ruleA", File: "**", Reason: "test"},
	}
	matched, expired := ApplyWithAliases(snap, entries, nil, time.Now())
	if len(matched) != 1 {
		t.Errorf("matched = %d, want 1", len(matched))
	}
	if len(expired) != 0 {
		t.Errorf("expired = %d, want 0", len(expired))
	}
	if len(snap.Signals) != 1 || snap.Signals[0].Type != "ruleB" {
		t.Errorf("post-Apply signals = %+v, want only ruleB", snap.Signals)
	}
}

// TestApplyWithAliases_OldIDSuppressesNew validates the deprecation-
// window contract: a suppression on the old rule_id continues to
// suppress findings emitted under any new rule_id from the alias's
// ReplacesWith list.
func TestApplyWithAliases_OldIDSuppressesNew(t *testing.T) {
	yamlBody := []byte(`
version: 1
aliases:
  oldRule:
    replaces_with: [newRuleA, newRuleB]
    why: "split for clarity"
`)
	reg, err := aliases.LoadFromBytes(yamlBody)
	if err != nil {
		t.Fatalf("LoadFromBytes: %v", err)
	}

	snap := &models.TestSuiteSnapshot{
		Signals: []models.Signal{
			{Type: "newRuleA", Location: models.SignalLocation{File: "a.go"}},
			{Type: "newRuleB", Location: models.SignalLocation{File: "b.go"}},
			{Type: "oldRule", Location: models.SignalLocation{File: "c.go"}},
			{Type: "unrelated", Location: models.SignalLocation{File: "d.go"}},
		},
	}
	entries := []Entry{
		{SignalType: "oldRule", File: "**", Reason: "deprecation window"},
	}
	matched, _ := ApplyWithAliases(snap, entries, reg, time.Now())

	// The single entry should match (counted once, not three times).
	if len(matched) != 1 {
		t.Errorf("matched = %d, want 1 (single entry hit)", len(matched))
	}
	// Only the unrelated signal should remain.
	if len(snap.Signals) != 1 || snap.Signals[0].Type != "unrelated" {
		got := []string{}
		for _, s := range snap.Signals {
			got = append(got, string(s.Type))
		}
		t.Errorf("post-Apply types = %v, want [\"unrelated\"]", got)
	}
}

// TestApplyWithAliases_NewIDSuppressionDoesNotMatchOld is the
// one-way-contract test: a suppression written against a NEW id does
// not auto-suppress findings still emitted under the OLD id. Adopters
// who write suppressions against new IDs are post-migration; the OLD
// id will have stopped firing.
func TestApplyWithAliases_NewIDSuppressionDoesNotMatchOld(t *testing.T) {
	yamlBody := []byte(`
version: 1
aliases:
  oldRule:
    replaces_with: [newRule]
`)
	reg, err := aliases.LoadFromBytes(yamlBody)
	if err != nil {
		t.Fatalf("LoadFromBytes: %v", err)
	}

	snap := &models.TestSuiteSnapshot{
		Signals: []models.Signal{
			{Type: "oldRule", Location: models.SignalLocation{File: "a.go"}},
		},
	}
	entries := []Entry{
		{SignalType: "newRule", File: "**", Reason: "post-migration"},
	}
	matched, _ := ApplyWithAliases(snap, entries, reg, time.Now())

	if len(matched) != 0 {
		t.Errorf("matched = %d, want 0 (new ID suppression does not match old)", len(matched))
	}
	if len(snap.Signals) != 1 {
		t.Errorf("post-Apply signals = %d, want 1 (old ID retained)", len(snap.Signals))
	}
}

// TestApplyWithAliases_FindingIDPathUnaffected: alias expansion only
// applies to SignalType matching. FindingID-based suppressions
// continue to require exact match (no alias rewriting).
func TestApplyWithAliases_FindingIDPathUnaffected(t *testing.T) {
	yamlBody := []byte(`
version: 1
aliases:
  oldRule:
    replaces_with: [newRule]
`)
	reg, _ := aliases.LoadFromBytes(yamlBody)

	snap := &models.TestSuiteSnapshot{
		Signals: []models.Signal{
			{Type: "newRule", FindingID: "newRule@a.go:Sym#abc12345", Location: models.SignalLocation{File: "a.go"}},
		},
	}
	entries := []Entry{
		// FindingID-only entry against a different id — must not match
		// even though "oldRule" is an alias for "newRule".
		{FindingID: "oldRule@a.go:Sym#abc12345", Reason: "stale pin"},
	}
	matched, _ := ApplyWithAliases(snap, entries, reg, time.Now())
	if len(matched) != 0 {
		t.Errorf("matched = %d, want 0 (FindingID requires exact match)", len(matched))
	}
}

// TestApplyWithAliases_ExpiryRespected confirms that expired entries
// are partitioned out before the alias-aware match runs.
func TestApplyWithAliases_ExpiryRespected(t *testing.T) {
	yamlBody := []byte(`
version: 1
aliases:
  oldRule:
    replaces_with: [newRule]
`)
	reg, _ := aliases.LoadFromBytes(yamlBody)

	expiredEntry := Entry{SignalType: "oldRule", File: "**", Reason: "x"}
	expiredEntry.Expires = "2020-01-01"
	expiredEntry.expiresAt, _ = time.Parse("2006-01-02", expiredEntry.Expires)

	snap := &models.TestSuiteSnapshot{
		Signals: []models.Signal{
			{Type: "newRule", Location: models.SignalLocation{File: "a.go"}},
		},
	}
	now := time.Now()
	matched, expired := ApplyWithAliases(snap, []Entry{expiredEntry}, reg, now)
	if len(matched) != 0 {
		t.Errorf("matched = %d, want 0 (entry expired)", len(matched))
	}
	if len(expired) != 1 {
		t.Errorf("expired = %d, want 1", len(expired))
	}
	if len(snap.Signals) != 1 {
		t.Errorf("post-Apply signals = %d, want 1 (suppression expired)", len(snap.Signals))
	}
}

// ── Schema v2 — content_hash + scope ──────────────────────────────

// TestContextHash_Stable confirms the hash is deterministic across
// repeat calls and identical for the same window content.
func TestContextHash_Stable(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "src.go")
	body := "line1\nline2\nline3 // finding\nline4\nline5\nline6\nline7\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	h1, err := ContextHash(path, 3)
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	h2, err := ContextHash(path, 3)
	if err != nil {
		t.Fatalf("repeat: %v", err)
	}
	if h1 != h2 {
		t.Errorf("non-deterministic: %s vs %s", h1, h2)
	}
	if len(h1) != 64 {
		t.Errorf("expected 64-char SHA-256 hex, got %d chars", len(h1))
	}
}

// TestContextHash_WhitespaceTolerance pins the spec: trailing
// whitespace on each line should not change the hash. Leading
// whitespace and internal whitespace are part of the hash.
func TestContextHash_WhitespaceTolerance(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.go")
	b := filepath.Join(dir, "b.go")
	clean := "line1\nline2\nline3\nline4\nline5\n"
	trailing := "line1   \nline2\t\nline3 \nline4   \nline5\t\t\n"
	if err := os.WriteFile(a, []byte(clean), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(b, []byte(trailing), 0o644); err != nil {
		t.Fatal(err)
	}
	ha, _ := ContextHash(a, 3)
	hb, _ := ContextHash(b, 3)
	if ha != hb {
		t.Errorf("trailing-whitespace variation changed hash: %s vs %s", ha, hb)
	}
}

// TestContextHash_LineChangeInvalidates pins the spec: editing the
// suppressed line itself must change the hash.
func TestContextHash_LineChangeInvalidates(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "v1.go")
	if err := os.WriteFile(path, []byte("a\nb\nORIGINAL\nd\ne\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	h1, _ := ContextHash(path, 3)

	path2 := filepath.Join(dir, "v2.go")
	if err := os.WriteFile(path2, []byte("a\nb\nCHANGED\nd\ne\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	h2, _ := ContextHash(path2, 3)
	if h1 == h2 {
		t.Errorf("line change did not invalidate hash: both %s", h1)
	}
}

// TestContextHash_SurroundingLineEditsAllowed: edits to non-suppressed
// lines OUTSIDE the 5-line window should not change the hash. Edits
// WITHIN the window do change it (window is 5 lines = ±2).
func TestContextHash_SurroundingLineEditsAllowed(t *testing.T) {
	dir := t.TempDir()
	pathA := filepath.Join(dir, "a.go")
	pathB := filepath.Join(dir, "b.go")
	// File A: 9 lines, finding on line 5 (window covers lines 3-7).
	if err := os.WriteFile(pathA, []byte("L1\nL2\nL3\nL4\nFINDING\nL6\nL7\nL8\nL9\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// File B: line 1 and line 9 changed (outside the window).
	if err := os.WriteFile(pathB, []byte("XX\nL2\nL3\nL4\nFINDING\nL6\nL7\nL8\nYY\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	hA, _ := ContextHash(pathA, 5)
	hB, _ := ContextHash(pathB, 5)
	if hA != hB {
		t.Errorf("outside-window edits invalidated hash: %s vs %s", hA, hB)
	}
}

// TestContextHash_MissingFileReturnsEmpty pins the spec: a finding
// against a non-existent file produces "" (not an error). The matcher
// treats "" as "skip hash check, fail to match" so the suppression
// doesn't fire on a deleted file.
func TestContextHash_MissingFileReturnsEmpty(t *testing.T) {
	h, err := ContextHash(filepath.Join(t.TempDir(), "does-not-exist.go"), 1)
	if err != nil {
		t.Errorf("missing file should not error: %v", err)
	}
	if h != "" {
		t.Errorf("missing file should return \"\", got %q", h)
	}
}

// TestContextHash_NearStartOfFile: when line < ContextHashRadius+1
// the window is clamped to the file start. The hash is still stable.
func TestContextHash_NearStartOfFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tiny.go")
	if err := os.WriteFile(path, []byte("L1\nL2\nL3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	h1, _ := ContextHash(path, 1)
	h1again, _ := ContextHash(path, 1)
	if h1 != h1again {
		t.Errorf("non-deterministic near start: %s vs %s", h1, h1again)
	}
	if h1 == "" {
		t.Error("near-start hash should not be empty")
	}
}

// TestLoad_SchemaV2 confirms v2 fields load cleanly.
func TestLoad_SchemaV2(t *testing.T) {
	t.Parallel()
	yaml := `
schema_version: "2"
suppressions:
  - signal_type: ruleA
    file: src/a.go
    scope: instance
    content_hash: deadbeef
    reason: "test"
`
	path := writeTemp(t, yaml)
	res, err := Load(path)
	if err != nil {
		t.Fatalf("load v2: %v", err)
	}
	if len(res.Entries) != 1 {
		t.Fatalf("entries = %d, want 1", len(res.Entries))
	}
	e := res.Entries[0]
	if e.Scope != ScopeInstance {
		t.Errorf("scope = %q, want %q", e.Scope, ScopeInstance)
	}
	if e.ContentHash != "deadbeef" {
		t.Errorf("content_hash = %q, want %q", e.ContentHash, "deadbeef")
	}
}

// TestLoad_RepoScopeAllowsNoFile confirms scope=repo with no file
// loads cleanly (the rule-disabled-for-the-whole-repo shape).
func TestLoad_RepoScopeAllowsNoFile(t *testing.T) {
	t.Parallel()
	yaml := `
schema_version: "2"
suppressions:
  - signal_type: ruleA
    scope: repo
    reason: "disable until 0.3"
`
	res, err := Load(writeTemp(t, yaml))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(res.Entries) != 1 || res.Entries[0].Scope != ScopeRepo {
		t.Fatalf("entries = %+v", res.Entries)
	}
}

// TestLoad_RejectsUnknownScope validates the scope allowlist.
func TestLoad_RejectsUnknownScope(t *testing.T) {
	t.Parallel()
	yaml := `
schema_version: "2"
suppressions:
  - signal_type: ruleA
    file: src/a.go
    scope: bogus
    reason: "test"
`
	if _, err := Load(writeTemp(t, yaml)); err == nil {
		t.Error("expected error for unknown scope")
	}
}

// TestLoad_RejectsContentHashWithoutFile pins the schema rule.
func TestLoad_RejectsContentHashWithoutFile(t *testing.T) {
	t.Parallel()
	yaml := `
schema_version: "2"
suppressions:
  - signal_type: ruleA
    content_hash: abc123
    reason: "test"
`
	if _, err := Load(writeTemp(t, yaml)); err == nil {
		t.Error("expected error for content_hash without file")
	}
}

// TestApplyWithAliases_ContentHashMatches: when the entry's
// content_hash matches the current file content, the suppression fires.
func TestApplyWithAliases_ContentHashMatches(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "src.go")
	if err := os.WriteFile(path, []byte("L1\nL2\nL3\nL4\nL5\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	hash, _ := ContextHash(path, 3)

	snap := &models.TestSuiteSnapshot{
		Signals: []models.Signal{
			{
				Type:     "ruleA",
				Location: models.SignalLocation{File: path, Line: 3},
			},
		},
	}
	entry := Entry{
		SignalType:  "ruleA",
		File:        path,
		ContentHash: hash,
		Reason:      "test",
	}
	matched, _ := ApplyWithAliases(snap, []Entry{entry}, nil, time.Now())
	if len(matched) != 1 {
		t.Errorf("matched = %d, want 1 (hash matches current content)", len(matched))
	}
	if len(snap.Signals) != 0 {
		t.Errorf("signal not suppressed: %+v", snap.Signals)
	}
}

// TestApplyWithAliases_ContentHashMismatchSkipsSuppression: when the
// entry's content_hash does NOT match (file was edited), the
// suppression does not fire — the assumption is the user's rationale
// was tied to the old code.
func TestApplyWithAliases_ContentHashMismatchSkipsSuppression(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "src.go")
	if err := os.WriteFile(path, []byte("L1\nL2\nNEW_CONTENT\nL4\nL5\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	snap := &models.TestSuiteSnapshot{
		Signals: []models.Signal{
			{
				Type:     "ruleA",
				Location: models.SignalLocation{File: path, Line: 3},
			},
		},
	}
	entry := Entry{
		SignalType:  "ruleA",
		File:        path,
		ContentHash: "0000000000000000000000000000000000000000000000000000000000000000",
		Reason:      "test",
	}
	matched, _ := ApplyWithAliases(snap, []Entry{entry}, nil, time.Now())
	if len(matched) != 0 {
		t.Errorf("matched = %d, want 0 (hash should not match)", len(matched))
	}
	if len(snap.Signals) != 1 {
		t.Errorf("signal was suppressed despite hash mismatch")
	}
}

// TestApplyWithAliases_RepoScopeNoFile: scope=repo + no file applies
// to every finding of the rule.
func TestApplyWithAliases_RepoScopeNoFile(t *testing.T) {
	snap := &models.TestSuiteSnapshot{
		Signals: []models.Signal{
			{Type: "ruleA", Location: models.SignalLocation{File: "a.go"}},
			{Type: "ruleA", Location: models.SignalLocation{File: "b.go"}},
			{Type: "ruleA", Location: models.SignalLocation{File: "c.go"}},
			{Type: "ruleB", Location: models.SignalLocation{File: "d.go"}},
		},
	}
	entry := Entry{
		SignalType: "ruleA",
		Scope:      ScopeRepo,
		Reason:     "disable everywhere",
	}
	matched, _ := ApplyWithAliases(snap, []Entry{entry}, nil, time.Now())
	if len(matched) != 1 {
		t.Errorf("matched entries = %d, want 1", len(matched))
	}
	if len(snap.Signals) != 1 || snap.Signals[0].Type != "ruleB" {
		t.Errorf("post-Apply types should be [ruleB], got %+v", snap.Signals)
	}
}

// TestDefaultExpiryForScope pins the per-scope defaults.
func TestDefaultExpiryForScope(t *testing.T) {
	day := 24 * time.Hour
	cases := []struct {
		scope Scope
		want  time.Duration
	}{
		{ScopeInstance, 90 * day},
		{ScopeFile, 180 * day},
		{ScopeDirectory, 180 * day},
		{ScopeRepo, 365 * day},
		{"", 90 * day}, // unknown defaults to instance-like
	}
	for _, c := range cases {
		if got := DefaultExpiryForScope(c.scope); got != c.want {
			t.Errorf("DefaultExpiryForScope(%q) = %v, want %v", c.scope, got, c.want)
		}
	}
}
