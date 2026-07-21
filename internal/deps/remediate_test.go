package deps_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pmclSF/terrain/internal/deps"
	"github.com/pmclSF/terrain/internal/findings"
	"github.com/pmclSF/terrain/internal/remediate"
)

func writeManifest(t *testing.T, root, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, "package.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// detectAsFindings runs the real drift-risk detector and converts its
// signals to canonical findings — the same shape the closed-loop validator
// compares before/after.
func detectAsFindings(root string) ([]findings.Finding, error) {
	d := &deps.DriftRiskDetector{Root: root}
	var out []findings.Finding
	for _, s := range d.Detect(nil) {
		out = append(out, findings.FromSignal(s, s.RuleID))
	}
	return out, nil
}

// TestE2E_DepsPinClosesTheLoop proves the trust floor on a general-purpose
// (non-AI) detector with a non-scaffold applier: an all-caret npm manifest
// trips drift-risk; the edit_in_place pin fix rewrites it; re-running the
// real detector clears the finding with no regressions.
func TestE2E_DepsPinClosesTheLoop(t *testing.T) {
	root := t.TempDir()
	writeManifest(t, root, `{
  "name": "app",
  "dependencies": {
    "react": "^18.2.0",
    "lodash": "^4.17.21",
    "axios": "^1.6.0",
    "express": "^4.18.2"
  }
}`)

	before, _ := detectAsFindings(root)
	if len(before) != 1 {
		t.Fatalf("expected 1 drift-risk finding, got %d", len(before))
	}
	target := before[0]

	fix, ok := deps.PinCaretsFix(root, "package.json")
	if !ok {
		t.Fatal("PinCaretsFix should apply to an all-caret manifest")
	}
	if fix.Kind != findings.FixEditInPlace {
		t.Errorf("Fix.Kind = %q, want edit_in_place", fix.Kind)
	}
	target.Suggestions = []findings.Suggestion{{Text: "Pin caret deps", Fix: fix}}

	v, err := remediate.Validate(root, target, before, detectAsFindings)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if !v.Valid {
		t.Errorf("deps pin should close the loop; verdict: %s (new: %d)", v.Note, len(v.NewFindings))
	}

	// Non-destructive: the original caret manifest must be restored.
	got, _ := os.ReadFile(filepath.Join(root, "package.json"))
	if want := "^18.2.0"; !contains(string(got), want) {
		t.Errorf("manifest not restored after validation; missing %q", want)
	}
}

// TestPinCaretsFix_StrictPinsAreJudgeOnly pins the honest boundary: when
// non-caret moving-target deps (bare names) dominate, no deterministic edit
// clears the finding, so PinCaretsFix declines — the remediation is
// judge-only, not falsely claimed as mechanical.
func TestPinCaretsFix_StrictPinsAreJudgeOnly(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	// 1 caret + 3 bare (strict-pin) → residual share after pinning = 3/4,
	// still over threshold. Pinning the one caret cannot clear it.
	writeManifest(t, root, `{
  "name": "app",
  "dependencies": {
    "react": "^18.2.0",
    "lodash": "*",
    "axios": "latest",
    "express": "*"
  }
}`)
	if _, ok := deps.PinCaretsFix(root, "package.json"); ok {
		t.Error("PinCaretsFix should decline when strict-pins dominate (judge-only)")
	}
}

func TestPinCaretsFix_NonNpmDeclines(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "requirements.txt"), []byte("flask\nrequests\ndjango\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, ok := deps.PinCaretsFix(root, "requirements.txt"); ok {
		t.Error("PinCaretsFix should decline non-npm manifests")
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
