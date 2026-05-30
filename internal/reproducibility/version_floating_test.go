package reproducibility

import (
	"testing"

	"github.com/pmclSF/terrain/internal/manifest"
	"github.com/pmclSF/terrain/internal/models"
)

func TestDetectVersionFloating_PinningLadder(t *testing.T) {
	t.Parallel()
	m := &manifest.Manifest{
		Path:      "requirements.txt",
		Ecosystem: manifest.EcosystemPython,
		Dependencies: []manifest.Dependency{
			{Name: "exact", Spec: "==1.2.3", Pinning: manifest.PinningExact, Section: manifest.SectionRuntime},
			{Name: "ranged", Spec: ">=1.0,<2.0", Pinning: manifest.PinningRange, Section: manifest.SectionRuntime},
			{Name: "unpinned", Spec: "", Pinning: manifest.PinningUnpinned, Section: manifest.SectionRuntime},
			{Name: "moving-git", Spec: "git+https://github.com/x/y.git@main", Pinning: manifest.PinningGit, Section: manifest.SectionRuntime},
			{Name: "pinned-git", Spec: "git+https://github.com/x/y.git@a1b2c3d4e5f6789abc012def34567890abcdef12", Pinning: manifest.PinningGit, Section: manifest.SectionRuntime},
			{Name: "url", Spec: "https://example.com/foo.tgz", Pinning: manifest.PinningURL, Section: manifest.SectionRuntime},
			{Name: "local", Spec: "file://./foo", Pinning: manifest.PinningPath, Section: manifest.SectionRuntime},
		},
	}

	sigs := DetectVersionFloating([]*manifest.Manifest{m})

	bySource := map[string]models.Signal{}
	for _, s := range sigs {
		bySource[s.Metadata["dependency"].(string)] = s
	}

	// Suppressed.
	if _, ok := bySource["exact"]; ok {
		t.Error("exact pin should not fire")
	}
	if _, ok := bySource["pinned-git"]; ok {
		t.Error("commit-SHA git ref should not fire")
	}

	// Should fire.
	checks := []struct {
		dep      string
		severity models.SignalSeverity
	}{
		{"unpinned", models.SeverityHigh},
		{"ranged", models.SeverityMedium},
		{"moving-git", models.SeverityMedium},
		{"url", models.SeverityMedium},
		{"local", models.SeverityLow},
	}
	for _, c := range checks {
		sig, ok := bySource[c.dep]
		if !ok {
			t.Errorf("missing signal for %q", c.dep)
			continue
		}
		if sig.Severity != c.severity {
			t.Errorf("%s severity = %q, want %q", c.dep, sig.Severity, c.severity)
		}
		if sig.RuleID != "terrain/reproducibility/version-floating" {
			t.Errorf("%s rule ID = %q", c.dep, sig.RuleID)
		}
	}
}

func TestDetectVersionFloating_DevSectionStepsDown(t *testing.T) {
	t.Parallel()
	m := &manifest.Manifest{
		Path:      "package.json",
		Ecosystem: manifest.EcosystemNode,
		Dependencies: []manifest.Dependency{
			{Name: "jest", Spec: "*", Pinning: manifest.PinningUnpinned, Section: manifest.SectionDev},
			{Name: "lodash", Spec: "*", Pinning: manifest.PinningUnpinned, Section: manifest.SectionRuntime},
		},
	}
	sigs := DetectVersionFloating([]*manifest.Manifest{m})

	bySource := map[string]models.Signal{}
	for _, s := range sigs {
		bySource[s.Metadata["dependency"].(string)] = s
	}

	if bySource["lodash"].Severity != models.SeverityHigh {
		t.Errorf("runtime unpinned should be high, got %q", bySource["lodash"].Severity)
	}
	if bySource["jest"].Severity != models.SeverityMedium {
		t.Errorf("dev unpinned should step down to medium, got %q", bySource["jest"].Severity)
	}
}

func TestDetectVersionFloating_EmptyOrNil(t *testing.T) {
	t.Parallel()
	if got := DetectVersionFloating(nil); len(got) != 0 {
		t.Errorf("nil manifests should yield no signals, got %d", len(got))
	}
	if got := DetectVersionFloating([]*manifest.Manifest{nil}); len(got) != 0 {
		t.Errorf("nil entry should be skipped, got %d", len(got))
	}
}

func TestHasGitCommitSHA(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   string
		want bool
	}{
		{"git+https://github.com/x/y.git@a1b2c3d4e5f6789abc012def34567890abcdef12", true},
		{"git+https://github.com/x/y.git@main", false},
		{"git+https://github.com/x/y.git@v1.0.0", false},
		{"git+https://github.com/x/y.git@abc1234", true},
		{"git+https://github.com/x/y.git@abc123", false},         // too short
		{"git+https://github.com/x/y.git@abcdef1#egg=pkg", true}, // strips egg fragment
		{"git+https://github.com/x/y.git@a1b2c3d4e5f6#egg=pkg", true},
		{"no-at-symbol", false},
	}
	for _, c := range cases {
		if got := hasGitCommitSHA(c.in); got != c.want {
			t.Errorf("hasGitCommitSHA(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}
