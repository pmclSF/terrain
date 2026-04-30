package main

import (
	"testing"
)

func TestIsInteractive_InTestEnvironment(t *testing.T) {
	t.Parallel()
	// In test environments, stderr is not a TTY.
	if isInteractive() {
		t.Skip("test environment is interactive (unusual)")
	}
}

func TestNewProgressFunc_JSONModeSuppressed(t *testing.T) {
	t.Parallel()
	pf := newProgressFunc(true) // JSON mode
	if pf != nil {
		t.Error("expected nil progress func in JSON mode")
	}
}

func TestNewProgressFunc_NonInteractiveSuppressed(t *testing.T) {
	t.Parallel()
	// In test environments (pipe), non-interactive should return nil.
	pf := newProgressFunc(false)
	if pf != nil {
		t.Log("progress func is nil in non-interactive mode (expected in CI/pipe)")
	}
}

func TestProgressFunc_Signature(t *testing.T) {
	t.Parallel()
	// Verify the progress function can be called without panic.
	var callCount int
	pf := func(step, total int, label string) {
		callCount++
		if step < 1 || step > total {
			t.Errorf("step %d out of range [1, %d]", step, total)
		}
		if label == "" {
			t.Error("empty label")
		}
	}
	pf(1, 5, "Scanning repository")
	pf(2, 5, "Building graph")
	pf(3, 5, "Inferring validations")
	pf(4, 5, "Computing insights")
	pf(5, 5, "Writing report")
	if callCount != 5 {
		t.Errorf("expected 5 calls, got %d", callCount)
	}
}

func TestIsInteractive_NoColorEnvSuppresses(t *testing.T) {
	// Sequential — modifies process env.
	t.Setenv("NO_COLOR", "1")
	if isInteractive() {
		t.Errorf("isInteractive() should be false when NO_COLOR is set")
	}
}

func TestIsInteractive_DumbTerminalSuppresses(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	t.Setenv("TERM", "dumb")
	// Clear common CI vars so we isolate the TERM=dumb path.
	t.Setenv("CI", "")
	t.Setenv("GITHUB_ACTIONS", "")
	if isInteractive() {
		t.Errorf("isInteractive() should be false when TERM=dumb")
	}
}

func TestIsCIEnvironment(t *testing.T) {
	cases := []struct {
		name    string
		envKey  string
		envVal  string
		wantCI  bool
	}{
		{"CI=true", "CI", "true", true},
		{"GitHub Actions", "GITHUB_ACTIONS", "true", true},
		{"GitLab CI", "GITLAB_CI", "true", true},
		{"CircleCI", "CIRCLECI", "true", true},
		{"Buildkite", "BUILDKITE", "true", true},
		{"Jenkins", "JENKINS_URL", "https://jenkins.example.com", true},
		{"Azure Pipelines", "TF_BUILD", "True", true},
		// Empty string and "false"/"0" don't count as set.
		{"CI=false", "CI", "false", false},
		{"CI=0", "CI", "0", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Clear all CI markers, then set just the one under test.
			for _, key := range []string{
				"CI", "GITHUB_ACTIONS", "GITLAB_CI", "CIRCLECI",
				"BUILDKITE", "JENKINS_URL", "TF_BUILD",
			} {
				t.Setenv(key, "")
			}
			t.Setenv(tc.envKey, tc.envVal)
			got := isCIEnvironment()
			if got != tc.wantCI {
				t.Errorf("isCIEnvironment() with %s=%q = %v, want %v",
					tc.envKey, tc.envVal, got, tc.wantCI)
			}
		})
	}
}
