package main

import (
	"bytes"
	"io"
	"os"
	"strings"
	"sync"
	"testing"
)

var captureRunMu sync.Mutex

// TestCLISmoke_ReportingCommands runs every reporting command against the
// fixture repo and verifies no errors or panics. These are smoke tests
// that catch regressions (broken imports, nil panics, serialization
// failures) without the maintenance burden of golden files.
//
// Not parallel: commands write to os.Stdout which requires sequential capture.
func TestCLISmoke_ReportingCommands(t *testing.T) {
	root := fixtureRoot(t)

	tests := []struct {
		name string
		run  func() error
	}{
		{"summary", func() error { return runSummary(root, true, false) }},
		{"posture", func() error { return runPosture(root, true, false) }},
		{"metrics", func() error { return runMetrics(root, true, false) }},
		{"portfolio", func() error { return runPortfolio(root, true, false) }},
		{"focus", func() error { return runFocus(root, true, false) }},
		{"migration", func() error { return runMigration("readiness", root, true, "", "") }},
		{"benchmark", func() error { return runExportBenchmark(root) }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := captureRun(tt.run)
			if err != nil {
				t.Errorf("%s returned error: %v", tt.name, err)
			}
			if len(out) == 0 {
				t.Errorf("%s produced no output", tt.name)
			}
		})
	}
}

// TestCLISmoke_ExplainCommand verifies the explain command runs without error.
func TestCLISmoke_ExplainCommand(t *testing.T) {
	root := fixtureRoot(t)

	out, err := captureRun(func() error {
		return runExplain("selection", root, "HEAD~1", true, false)
	})
	if err != nil {
		t.Errorf("explain selection failed: %v", err)
	}
	if len(out) == 0 {
		t.Error("explain selection produced no output")
	}
}

// TestCLISmoke_PolicyCheckCommand verifies policy check runs without error.
func TestCLISmoke_PolicyCheckCommand(t *testing.T) {
	root := fixtureRoot(t)

	out, _ := captureRun(func() error {
		exitCode := runPolicyCheck(root, true, "", "", "", 0)
		if exitCode != 0 && exitCode != 2 {
			t.Errorf("policy check exit code = %d, want 0 or 2", exitCode)
		}
		return nil
	})
	_ = out // Policy check may produce empty output when no policy is configured.
}

// TestCLISmoke_DepgraphCommand verifies the debug depgraph command works.
func TestCLISmoke_DepgraphCommand(t *testing.T) {
	root := fixtureRoot(t)

	out, err := captureRun(func() error {
		return runDepgraph(root, true, "stats", "")
	})
	if err != nil {
		t.Errorf("depgraph stats failed: %v", err)
	}
	if len(out) == 0 {
		t.Error("depgraph stats produced no output")
	}
}

// TestCLISmoke_ShowCommand verifies show doesn't panic on unknown IDs.
func TestCLISmoke_ShowCommand(t *testing.T) {
	root := fixtureRoot(t)

	// Non-existent ID should not panic. It may return an error or "not found".
	_, _ = captureRun(func() error {
		return runShow("test", "nonexistent-id", root, true)
	})
}

// TestCLISmoke_SelectTestsCommand verifies select-tests runs.
func TestCLISmoke_SelectTestsCommand(t *testing.T) {
	root := fixtureRoot(t)

	out, err := captureRun(func() error {
		return runSelectTests(root, "HEAD~1", true)
	})
	if err != nil {
		t.Errorf("select-tests failed: %v", err)
	}
	if len(out) == 0 {
		t.Error("select-tests produced no output")
	}
}

// TestCLISmoke_PRCommand verifies the PR command runs.
func TestCLISmoke_PRCommand(t *testing.T) {
	root := fixtureRoot(t)

	out, err := captureRun(func() error {
		return runPR(root, "HEAD~1", true, "")
	})
	if err != nil {
		t.Errorf("pr failed: %v", err)
	}
	if len(out) == 0 {
		t.Error("pr produced no output")
	}
}

// TestCLISmoke_AIBaselineCompare verifies the compare subcommand runs
// without panicking. It may error if no baseline exists (expected).
func TestCLISmoke_AIBaselineCompare(t *testing.T) {
	root := fixtureRoot(t)

	_, _ = captureRun(func() error {
		// This will likely return "no baseline found" error — that's fine.
		// We're testing that the command doesn't panic.
		return runAIBaselineCompare(root, true)
	})
	// No assertion on error — "no baseline found" is a valid outcome.
}

// captureRun redirects os.Stdout, runs fn, and returns captured output.
// Must NOT be used concurrently — os.Stdout is global.
//
// The reader goroutine drains the pipe concurrently with fn(). Without this,
// any output larger than the OS pipe buffer (~4 KB on Windows) deadlocks the
// writer while we wait for fn() to return before reading. Linux/macOS pipes
// are large enough to mask the bug for small JSON outputs; Windows hangs
// reliably on commands like `posture --json`.
func captureRun(fn func() error) ([]byte, error) {
	captureRunMu.Lock()
	defer captureRunMu.Unlock()

	old := os.Stdout
	r, w, pipeErr := os.Pipe()
	if pipeErr != nil {
		return nil, pipeErr
	}
	os.Stdout = w

	var buf bytes.Buffer
	done := make(chan struct{})
	go func() {
		io.Copy(&buf, r)
		close(done)
	}()

	fnErr := fn()

	w.Close()
	os.Stdout = old
	<-done
	r.Close()

	return buf.Bytes(), fnErr
}

// runCaptured serializes stdout-affecting commands even when the caller only
// cares about the returned error or side effects on disk.
func runCaptured(fn func() error) error {
	_, err := captureRun(fn)
	return err
}

// captureStderr is the stderr counterpart of captureRun. Some commands
// route help / usage output to stderr (per long-standing CLI
// convention so that `cmd > out` doesn't hide the usage on error), so
// tests asserting on usage text need to read from stderr rather than
// stdout. Same single-shot semantics as captureRun: not safe for
// concurrent use.
func captureStderr(fn func() error) (string, error) {
	captureRunMu.Lock()
	defer captureRunMu.Unlock()

	old := os.Stderr
	r, w, pipeErr := os.Pipe()
	if pipeErr != nil {
		return "", pipeErr
	}
	os.Stderr = w

	var buf bytes.Buffer
	done := make(chan struct{})
	go func() {
		io.Copy(&buf, r)
		close(done)
	}()

	fnErr := fn()

	w.Close()
	os.Stderr = old
	<-done
	r.Close()

	return buf.String(), fnErr
}

// contains is a thin wrapper around strings.Contains kept for test
// readability; reads better than `strings.Contains(out, x)` in dense
// assertion blocks.
func contains(haystack, needle string) bool {
	return strings.Contains(haystack, needle)
}
