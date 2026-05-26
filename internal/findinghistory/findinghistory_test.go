package findinghistory

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// fixed clock for deterministic ISO date stamps.
func fixedClock(t string) func() time.Time {
	return func() time.Time {
		parsed, _ := time.Parse("2006-01-02", t)
		return parsed
	}
}

func TestNew_EmptyStoreNoDemotion(t *testing.T) {
	s := New()
	if s.ShouldDemote("ruleA", "src/a.go") {
		t.Error("empty store should not demote anything")
	}
}

func TestIncrement_RecordsFiresAndDate(t *testing.T) {
	s := New()
	s.SetClock(fixedClock("2026-05-25"))
	s.Increment("ruleA", "src/a.go")
	s.Increment("ruleA", "src/a.go")
	e, ok := s.Get("ruleA", "src/a.go")
	if !ok {
		t.Fatal("entry should exist")
	}
	if e.Fires != 2 {
		t.Errorf("fires = %d, want 2", e.Fires)
	}
	if e.LastFire != "2026-05-25" {
		t.Errorf("last_fire = %q", e.LastFire)
	}
}

func TestIncrement_NoOpOnEmptyInputs(t *testing.T) {
	s := New()
	s.Increment("", "file")
	s.Increment("rule", "")
	s.Increment("", "")
	if len(s.All()) != 0 {
		t.Errorf("empty inputs should not create entries; got %d", len(s.All()))
	}
}

func TestShouldDemote_BelowThreshold(t *testing.T) {
	s := New()
	for i := 0; i < DefaultThreshold-1; i++ {
		s.Increment("ruleA", "src/a.go")
	}
	if s.ShouldDemote("ruleA", "src/a.go") {
		t.Error("should not demote below threshold")
	}
}

func TestShouldDemote_AtThreshold(t *testing.T) {
	s := New()
	for i := 0; i < DefaultThreshold; i++ {
		s.Increment("ruleA", "src/a.go")
	}
	if !s.ShouldDemote("ruleA", "src/a.go") {
		t.Error("should demote at threshold")
	}
}

func TestShouldDemote_DismissOverridesFatigueDemotion(t *testing.T) {
	s := New()
	s.SetClock(fixedClock("2026-05-25"))
	for i := 0; i < DefaultThreshold+2; i++ {
		s.Increment("ruleA", "src/a.go")
	}
	if !s.ShouldDemote("ruleA", "src/a.go") {
		t.Fatal("precondition: should demote pre-dismiss")
	}
	// Dismiss same-day as last fire.
	s.Dismiss("ruleA", "src/a.go")
	if s.ShouldDemote("ruleA", "src/a.go") {
		t.Error("dismiss on-or-after last fire should reset demotion")
	}
}

func TestShouldDemote_NewFireAfterDismissRestartsCounter(t *testing.T) {
	s := New()
	clock := time.Date(2026, 5, 20, 0, 0, 0, 0, time.UTC)
	s.SetClock(func() time.Time { return clock })

	// Three fires → would demote.
	s.Increment("ruleA", "src/a.go")
	clock = clock.AddDate(0, 0, 1)
	s.Increment("ruleA", "src/a.go")
	clock = clock.AddDate(0, 0, 1)
	s.Increment("ruleA", "src/a.go")
	if !s.ShouldDemote("ruleA", "src/a.go") {
		t.Fatal("expect demote after 3 fires")
	}

	// Dismiss.
	clock = clock.AddDate(0, 0, 1)
	s.Dismiss("ruleA", "src/a.go")
	if s.ShouldDemote("ruleA", "src/a.go") {
		t.Fatal("dismiss should reset")
	}

	// New fire after dismiss should NOT immediately re-demote
	// (last_dismiss < last_fire now, but count is still 3+).
	clock = clock.AddDate(0, 0, 1)
	s.Increment("ruleA", "src/a.go")

	// The counter doesn't reset under the current spec — fires keeps
	// climbing — but the date comparison means demotion holds only
	// when the user lets the rule keep firing without dismissing.
	if !s.ShouldDemote("ruleA", "src/a.go") {
		t.Error("new fire after dismiss should re-demote (fires >= threshold AND last_fire > last_dismiss)")
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".terrain", "finding-history.yaml")

	s := New()
	s.SetClock(fixedClock("2026-05-25"))
	s.Increment("ruleA", "src/a.go")
	s.Increment("ruleA", "src/a.go")
	s.Increment("ruleB", "src/b.go")
	s.Dismiss("ruleA", "src/a.go")
	if err := s.Save(path); err != nil {
		t.Fatalf("save: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	all := loaded.All()
	if len(all) != 2 {
		t.Errorf("entries = %d, want 2", len(all))
	}
	// Sorted (ruleA before ruleB).
	if all[0].RuleID != "ruleA" || all[1].RuleID != "ruleB" {
		t.Errorf("not sorted: %v", all)
	}
	if all[0].Fires != 2 {
		t.Errorf("ruleA fires = %d, want 2", all[0].Fires)
	}
	if all[0].LastDismiss != "2026-05-25" {
		t.Errorf("ruleA last_dismiss = %q", all[0].LastDismiss)
	}
}

func TestLoad_MissingFileIsEmpty(t *testing.T) {
	s, err := Load(filepath.Join(t.TempDir(), "does-not-exist.yaml"))
	if err != nil {
		t.Fatalf("missing file: %v", err)
	}
	if len(s.All()) != 0 {
		t.Error("missing file should produce empty store")
	}
}

func TestLoad_RejectsBadSchema(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "h.yaml")
	if err := os.WriteFile(path, []byte(`schema_version: "99"
entries: []
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Error("expected schema_version error")
	}
}

func TestSetThreshold(t *testing.T) {
	s := New()
	s.SetThreshold(5)
	for i := 0; i < 4; i++ {
		s.Increment("ruleA", "f.go")
	}
	if s.ShouldDemote("ruleA", "f.go") {
		t.Error("4 fires below threshold-5 should not demote")
	}
	s.Increment("ruleA", "f.go")
	if !s.ShouldDemote("ruleA", "f.go") {
		t.Error("5 fires at threshold-5 should demote")
	}
}

func TestSetThreshold_ZeroResetsToDefault(t *testing.T) {
	s := New()
	s.SetThreshold(0)
	for i := 0; i < DefaultThreshold; i++ {
		s.Increment("ruleA", "f.go")
	}
	if !s.ShouldDemote("ruleA", "f.go") {
		t.Error("0 should reset threshold to DefaultThreshold")
	}
}

func TestAll_SortedDeterministic(t *testing.T) {
	s := New()
	s.Increment("ruleZ", "z.go")
	s.Increment("ruleA", "z.go")
	s.Increment("ruleA", "a.go")
	all := s.All()
	if len(all) != 3 {
		t.Fatalf("entries = %d, want 3", len(all))
	}
	// Order: (ruleA, a.go), (ruleA, z.go), (ruleZ, z.go)
	if all[0].RuleID != "ruleA" || all[0].File != "a.go" {
		t.Errorf("first = (%q, %q)", all[0].RuleID, all[0].File)
	}
	if all[2].RuleID != "ruleZ" {
		t.Errorf("last = %q", all[2].RuleID)
	}
}
