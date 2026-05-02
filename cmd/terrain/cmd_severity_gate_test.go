package main

import (
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
		{"critical: 1 critical blocks", severityGateCritical, bd{Critical: 1}, true, "1 critical"},
		{"high: critical+high blocks on critical", severityGateHigh, bd{Critical: 2, High: 0}, true, "2 critical + 0 high"},
		{"high: critical+high blocks on high", severityGateHigh, bd{Critical: 0, High: 5}, true, "0 critical + 5 high"},
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
