package aidetect

import "testing"

// TestSetCustomEvalDirs verifies that ai.scenarios_dir augments — never
// replaces — the built-in eval-directory recognition.
func TestSetCustomEvalDirs(t *testing.T) {
	defer SetCustomEvalDirs(nil) // reset package global for other tests

	// Built-in dirs are recognized with no custom config.
	if !IsEvalTestPath("evals/safety.yaml") {
		t.Error("built-in evals/ dir not recognized")
	}
	// A non-standard dir is not an eval path until configured.
	if IsEvalTestPath("quality/checks/test.yaml") {
		t.Error("unconfigured custom dir should not be an eval path")
	}

	SetCustomEvalDirs([]string{"quality/checks"})
	for _, p := range []string{
		"quality/checks/test.yaml",
		"repo/quality/checks/x.py",
		"QUALITY/CHECKS/y", // case-insensitive
	} {
		if !IsEvalTestPath(p) {
			t.Errorf("configured scenarios_dir not recognized for %q", p)
		}
	}
	// Built-ins still work after configuring a custom dir (augment, not replace).
	if !IsEvalTestPath("benchmarks/b.json") {
		t.Error("built-in dir stopped working after custom config")
	}
	// An unrelated path is still not an eval path.
	if IsEvalTestPath("src/main.go") {
		t.Error("unrelated path wrongly classified as eval")
	}

	// Normalization of the dir SPEC itself (caps + surrounding slashes) —
	// distinct from IsEvalTestPath lowercasing the path. A non-normalizing
	// SetCustomEvalDirs would store "/Quality/Checks/" and never match.
	SetCustomEvalDirs([]string{"/Quality/Checks/"})
	if !IsEvalTestPath("quality/checks/test.yaml") {
		t.Error("dir spec '/Quality/Checks/' should normalize and match quality/checks/...")
	}
}
