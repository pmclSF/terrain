package quality

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pmclSF/terrain/internal/mechanisms"
	"github.com/pmclSF/terrain/internal/models"
)

// SS8: stripLineComments removes //, #, and /* */ comments so a gate pattern
// hidden in a comment is NOT mistaken for a real runtime gate (the false-
// positive guard for conditional-vs-unconditional skip classification). It
// was completely untested.
func TestStripLineComments(t *testing.T) {
	t.Parallel()
	cases := []struct{ name, in, mustNotContain, mustContain string }{
		{"c-line", "code(); // process.env.CI", "process.env.CI", "code()"},
		{"py-line", "skip()  # os.getenv('X')", "os.getenv", "skip()"},
		{"block", "a /* os.getenv */ b", "os.getenv", "a"},
	}
	for _, c := range cases {
		got := stripLineComments(c.in)
		if strings.Contains(got, c.mustNotContain) {
			t.Errorf("%s: comment not stripped; %q still present in %q", c.name, c.mustNotContain, got)
		}
		if !strings.Contains(got, c.mustContain) {
			t.Errorf("%s: stripped too much; %q missing from %q", c.name, c.mustContain, got)
		}
	}
}

// SS4/SS11: per-file signals carry severity derived from each file's OWN skip
// ratio, plus accurate metadata. The existing severity test only checks the
// repo-level aggregate signal.
func TestStaticSkipDetector_PerFileSeverity(t *testing.T) {
	t.Parallel()
	snap := &models.TestSuiteSnapshot{TestFiles: []models.TestFile{
		{Path: "high.test.ts", Framework: "jest", TestCount: 10, SkipCount: 7}, // 70% → High
		{Path: "low.test.ts", Framework: "jest", TestCount: 10, SkipCount: 1},  // 10% → Low
	}}
	sigs := (&StaticSkipDetector{}).Detect(snap)
	var high, low *models.Signal
	for i := range sigs {
		switch sigs[i].Location.File {
		case "high.test.ts":
			high = &sigs[i]
		case "low.test.ts":
			low = &sigs[i]
		}
	}
	if high == nil || low == nil {
		t.Fatalf("expected per-file signals for both files; got %+v", sigs)
	}
	if high.Severity != models.SeverityHigh {
		t.Errorf("high.test.ts (70%% skipped): want High, got %v", high.Severity)
	}
	if low.Severity != models.SeverityLow {
		t.Errorf("low.test.ts (10%% skipped): want Low, got %v", low.Severity)
	}
	if high.Metadata["skippedCount"] != 7 {
		t.Errorf("skippedCount metadata: want 7, got %v", high.Metadata["skippedCount"])
	}
}

func TestStaticSkipDetector_JSSkipPatterns(t *testing.T) {
	t.Parallel()
	snap := &models.TestSuiteSnapshot{
		TestFiles: []models.TestFile{
			{Path: "tests/auth.test.ts", Framework: "vitest", TestCount: 10, SkipCount: 3},
			{Path: "tests/billing.test.ts", Framework: "vitest", TestCount: 5, SkipCount: 0},
		},
	}

	d := &StaticSkipDetector{}
	signals := d.Detect(snap)

	if len(signals) == 0 {
		t.Fatal("expected signals for 3 skipped tests")
	}

	// Should have repo-level signal + file-level signal for auth.test.ts
	if len(signals) != 2 {
		t.Errorf("expected 2 signals (repo + file), got %d", len(signals))
	}

	// Repo-level signal
	if signals[0].Type != "staticSkippedTest" {
		t.Errorf("expected type staticSkippedTest, got %s", signals[0].Type)
	}
	if signals[0].Location.File != "" {
		t.Error("repo-level signal should not have file location")
	}

	// File-level signal
	if signals[1].Location.File != "tests/auth.test.ts" {
		t.Errorf("expected auth.test.ts, got %s", signals[1].Location.File)
	}
}

func TestStaticSkipDetector_NoSkips(t *testing.T) {
	t.Parallel()
	snap := &models.TestSuiteSnapshot{
		TestFiles: []models.TestFile{
			{Path: "tests/a.test.ts", Framework: "jest", TestCount: 10, SkipCount: 0},
		},
	}

	d := &StaticSkipDetector{}
	signals := d.Detect(snap)

	if len(signals) != 0 {
		t.Errorf("expected 0 signals for no skips, got %d", len(signals))
	}
}

func TestStaticSkipDetector_SeverityThresholds(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		skips   int
		total   int
		wantSev models.SignalSeverity
	}{
		{"low skip rate", 1, 20, models.SeverityLow},
		{"medium skip rate", 5, 20, models.SeverityMedium},
		{"high skip rate", 12, 20, models.SeverityHigh},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			snap := &models.TestSuiteSnapshot{
				TestFiles: []models.TestFile{
					{Path: "tests/a.test.ts", Framework: "jest", TestCount: tt.total, SkipCount: tt.skips},
				},
			}
			d := &StaticSkipDetector{}
			signals := d.Detect(snap)
			if len(signals) == 0 {
				t.Fatal("expected signals")
			}
			if signals[0].Severity != tt.wantSev {
				t.Errorf("expected severity %s, got %s", tt.wantSev, signals[0].Severity)
			}
		})
	}
}

// TestStaticSkipDetector_SplitMechanism_On asserts that flipping
// the static_skipped_test_split mechanism to On observably changes
// the emitted Type: legacy "staticSkippedTest" becomes
// staticSkippedTest-unconditional / -conditional-gate based on
// whether the file has a runtime gate predicate.
func TestStaticSkipDetector_SplitMechanism_On(t *testing.T) {
	dir := t.TempDir()
	// File A: bare skip marker, no gate predicate → unconditional.
	mustWriteFile(t, filepath.Join(dir, "a.test.ts"),
		"test.skip('x', () => {});")
	// File B: skip wrapped by a process.env gate predicate.
	mustWriteFile(t, filepath.Join(dir, "b.test.ts"),
		"if (process.env.CI) { test.skip('y', () => {}); }")

	snap := &models.TestSuiteSnapshot{
		TestFiles: []models.TestFile{
			{Path: "a.test.ts", Framework: "vitest", TestCount: 1, SkipCount: 1},
			{Path: "b.test.ts", Framework: "vitest", TestCount: 1, SkipCount: 1},
		},
	}

	reg, err := mechanisms.Load()
	if err != nil {
		t.Fatalf("load mechanisms: %v", err)
	}
	prev := mechanisms.SetDefault(reg)
	defer mechanisms.SetDefault(prev)

	d := &StaticSkipDetector{RepoRoot: dir}

	// Default state=shadow → legacy "staticSkippedTest" type.
	got := d.Detect(snap)
	if !hasSignalType(got, "staticSkippedTest") {
		t.Fatalf("shadow: expected legacy staticSkippedTest type; got %v", typesOf(got))
	}
	if hasSignalType(got, "staticSkippedTest-unconditional") ||
		hasSignalType(got, "staticSkippedTest-conditional-gate") {
		t.Fatalf("shadow: split types should NOT appear; got %v", typesOf(got))
	}

	// Flip mechanism on → split types appear; legacy disappears from
	// per-file signals.
	if err := reg.ApplyCLIOverrides([]string{"static_skipped_test_split=on"}); err != nil {
		t.Fatalf("override: %v", err)
	}
	got = d.Detect(snap)
	if !hasSignalType(got, "staticSkippedTest-unconditional") {
		t.Errorf("on: expected staticSkippedTest-unconditional for bare-skip file; got %v", typesOf(got))
	}
	if !hasSignalType(got, "staticSkippedTest-conditional-gate") {
		t.Errorf("on: expected staticSkippedTest-conditional-gate for gated-skip file; got %v", typesOf(got))
	}
}

func mustWriteFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func hasSignalType(sigs []models.Signal, t models.SignalType) bool {
	for _, s := range sigs {
		if s.Type == t {
			return true
		}
	}
	return false
}

func typesOf(sigs []models.Signal) []models.SignalType {
	out := make([]models.SignalType, 0, len(sigs))
	for _, s := range sigs {
		out = append(out, s.Type)
	}
	return out
}

func TestStaticSkipDetector_DeterministicOrder(t *testing.T) {
	t.Parallel()
	snap := &models.TestSuiteSnapshot{
		TestFiles: []models.TestFile{
			{Path: "tests/z.test.ts", Framework: "jest", TestCount: 10, SkipCount: 2},
			{Path: "tests/a.test.ts", Framework: "jest", TestCount: 10, SkipCount: 5},
			{Path: "tests/m.test.ts", Framework: "jest", TestCount: 10, SkipCount: 2},
		},
	}

	d := &StaticSkipDetector{}
	s1 := d.Detect(snap)
	s2 := d.Detect(snap)

	if len(s1) != len(s2) {
		t.Fatalf("non-deterministic: %d vs %d", len(s1), len(s2))
	}
	for i := range s1 {
		if s1[i].Location.File != s2[i].Location.File {
			t.Errorf("non-deterministic order at %d: %s vs %s",
				i, s1[i].Location.File, s2[i].Location.File)
		}
	}

	// a.test.ts (50% skip) should come before z/m (20% skip)
	if len(s1) >= 3 && s1[1].Location.File != "tests/a.test.ts" {
		t.Errorf("expected highest skip ratio first, got %s", s1[1].Location.File)
	}
}
