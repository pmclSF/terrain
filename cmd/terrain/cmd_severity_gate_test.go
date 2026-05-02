package main

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/pmclSF/terrain/internal/analyze"
)

func TestParseSeverityGate(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in     string
		want   severityGate
		errSub string
	}{
		{"", severityGateNone, ""},
		{"critical", severityGateCritical, ""},
		{"CRITICAL", severityGateCritical, ""},
		{"  high  ", severityGateHigh, ""},
		{"medium", severityGateMedium, ""},
		{"low", "", "invalid --fail-on"},
		{"info", "", "invalid --fail-on"},
		{"garbage", "", "invalid --fail-on"},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			t.Parallel()
			got, err := parseSeverityGate(tc.in)
			if tc.errSub != "" {
				if err == nil {
					t.Fatalf("expected error matching %q, got nil", tc.errSub)
				}
				if !strings.Contains(err.Error(), tc.errSub) {
					t.Errorf("error %q does not contain %q", err.Error(), tc.errSub)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("parseSeverityGate(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestSeverityGateBlocked(t *testing.T) {
	t.Parallel()
	type bd = analyze.SignalBreakdown
	cases := []struct {
		name        string
		gate        severityGate
		breakdown   bd
		wantBlocked bool
		wantSubstr  string
	}{
		{"none gate never blocks", severityGateNone, bd{Critical: 5, High: 3}, false, ""},
		{"critical: 0 critical passes", severityGateCritical, bd{High: 99, Medium: 99}, false, ""},
		{"critical: 1 critical blocks (singular)", severityGateCritical, bd{Critical: 1}, true, "1 critical finding"},
		{"critical: 3 critical blocks (plural)", severityGateCritical, bd{Critical: 3}, true, "3 critical findings"},
		{"high: critical+high blocks on critical", severityGateHigh, bd{Critical: 2, High: 0}, true, "2 critical + 0 high"},
		{"high: critical+high blocks on high", severityGateHigh, bd{Critical: 0, High: 5}, true, "0 critical + 5 high"},
		{"high: total count + plural", severityGateHigh, bd{Critical: 2, High: 5}, true, "(7 findings total)"},
		{"high: total count singular", severityGateHigh, bd{High: 1}, true, "(1 finding total)"},
		{"high: medium-only passes", severityGateHigh, bd{Medium: 99, Low: 99}, false, ""},
		{"medium: any of the three blocks", severityGateMedium, bd{Medium: 1}, true, "0 critical + 0 high + 1 medium"},
		{"medium: low-only passes", severityGateMedium, bd{Low: 99}, false, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			blocked, summary := severityGateBlocked(tc.gate, tc.breakdown)
			if blocked != tc.wantBlocked {
				t.Errorf("severityGateBlocked(%q, %+v) blocked = %v, want %v",
					tc.gate, tc.breakdown, blocked, tc.wantBlocked)
			}
			if tc.wantSubstr != "" && !strings.Contains(summary, tc.wantSubstr) {
				t.Errorf("summary %q does not contain %q", summary, tc.wantSubstr)
			}
		})
	}
}

// TestRunAnalyze_GateBlocksOnFixture is an end-to-end exercise of the
// `--fail-on` path that the launch-readiness review flagged as missing.
// It runs `runAnalyze` against the calibration corpus (which we know
// contains medium+ severity findings) and asserts:
//
//  1. The function returns `errSeverityGateBlocked` (so main.go maps to
//     exit code 6).
//  2. The error message contains the expected severity counts.
//  3. The report renders to stdout *before* the error returns — i.e.,
//     stdout is non-empty when the gate fires (the gate decision is the
//     last thing that happens, not the first).
func TestRunAnalyze_GateBlocksOnFixture(t *testing.T) {
	root := fixtureRoot(t)

	stdout, err := captureRun(func() error {
		return runAnalyze(
			root, false, "", false, false,
			"", "", "", "", "", "", "", "",
			defaultSlowThresholdMs, false,
			severityGateMedium, 0,
		)
	})

	// The fixture has medium+ findings — gate should fire.
	if !errors.Is(err, errSeverityGateBlocked) {
		t.Fatalf("expected errSeverityGateBlocked, got %v", err)
	}

	// Error message should be informative (severity counts + label).
	if !strings.Contains(err.Error(), "--fail-on=medium") {
		t.Errorf("error message missing --fail-on label: %v", err)
	}

	// Report renders before the gate check — stdout must be non-empty.
	// Pre-fix, a gate that returns before the report renders would
	// produce empty stdout; the user would only see the gate message
	// without context. This test locks in the "render-then-gate"
	// invariant.
	if len(stdout) == 0 {
		t.Error("stdout is empty — report should render before the gate fires")
	}
	if !strings.Contains(string(stdout), "Terrain") {
		t.Errorf("stdout missing report header; got: %s", string(stdout))
	}
}

// TestRunAnalyze_JSONStdoutPurity verifies that with `--json` enabled
// AND `--fail-on` matching, the JSON snapshot lands on stdout cleanly
// and is parseable as JSON. The gate message goes to the returned
// error (which main.go writes to stderr) so stdout stays a valid JSON
// document. This is the "JSON stdout purity" property the launch-
// readiness review asked for.
func TestRunAnalyze_JSONStdoutPurity(t *testing.T) {
	root := fixtureRoot(t)

	stdout, err := captureRun(func() error {
		return runAnalyze(
			root, true, "", false, false,
			"", "", "", "", "", "", "", "",
			defaultSlowThresholdMs, false,
			severityGateMedium, 0,
		)
	})

	// Gate fired (expected for the fixture).
	if !errors.Is(err, errSeverityGateBlocked) {
		t.Fatalf("expected errSeverityGateBlocked, got %v", err)
	}

	// JSON purity: the entire stdout body must parse as JSON. If the
	// gate message had leaked into stdout, the parse would fail.
	var parsed map[string]any
	if jsonErr := json.Unmarshal(stdout, &parsed); jsonErr != nil {
		t.Errorf("stdout is not valid JSON (gate message leaked into JSON?): %v\nstdout:\n%s", jsonErr, stdout)
	}
}

// TestRunAnalyze_GatePassesWhenSeverityAbsent verifies the inverse:
// `--fail-on critical` against a fixture whose worst severity is
// medium returns nil (no gate block).
func TestRunAnalyze_GatePassesWhenSeverityAbsent(t *testing.T) {
	root := fixtureRoot(t)

	_, err := captureRun(func() error {
		return runAnalyze(
			root, false, "", false, false,
			"", "", "", "", "", "", "", "",
			defaultSlowThresholdMs, false,
			severityGateCritical, 0,
		)
	})

	// The fixture's worst severity is below critical — gate should NOT
	// fire. Any non-nil error here is unexpected (analysis failure or
	// gate misfire).
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}
