package main

import (
	"errors"
	"fmt"
	"strings"

	"github.com/pmclSF/terrain/internal/analyze"
)

// errSeverityGateBlocked is the sentinel returned by runAnalyze when
// `--fail-on` matches at least one finding. main.go uses errors.Is to
// distinguish this from analysis errors and exit with
// `exitSeverityGateBlock` (6) rather than the generic 1.
var errSeverityGateBlocked = errors.New("severity gate blocked")

// severityGate represents the threshold for `--fail-on`. Findings at
// or above this severity cause the analyze command to exit with
// `exitSeverityGateBlock`. Empty string means "no gate" (the default).
type severityGate string

const (
	severityGateNone     severityGate = ""
	severityGateCritical severityGate = "critical"
	severityGateHigh     severityGate = "high"
	severityGateMedium   severityGate = "medium"
)

// parseSeverityGate accepts the user-supplied flag value and returns a
// validated gate, or an error explaining valid choices. We accept
// canonical lowercase ("critical", "high", "medium") plus an empty
// string for "no gate".
func parseSeverityGate(s string) (severityGate, error) {
	v := strings.ToLower(strings.TrimSpace(s))
	switch v {
	case "":
		return severityGateNone, nil
	case "critical":
		return severityGateCritical, nil
	case "high":
		return severityGateHigh, nil
	case "medium":
		return severityGateMedium, nil
	default:
		return severityGateNone, fmt.Errorf(
			"invalid --fail-on %q: valid values are 'critical', 'high', 'medium' (or unset to disable)",
			s,
		)
	}
}

// severityGateBlocked returns (true, summary) when the report contains
// at least one signal at or above the configured threshold. The
// summary is a one-line, human-readable description of which severity
// counts triggered the gate, suitable for printing to stderr before
// exit.
func severityGateBlocked(gate severityGate, summary analyze.SignalBreakdown) (bool, string) {
	switch gate {
	case severityGateNone:
		return false, ""
	case severityGateCritical:
		if summary.Critical > 0 {
			return true, fmt.Sprintf("%d critical %s", summary.Critical, plural(summary.Critical, "finding"))
		}
	case severityGateHigh:
		total := summary.Critical + summary.High
		if total > 0 {
			return true, fmt.Sprintf(
				"%d critical + %d high (%d %s total)",
				summary.Critical, summary.High, total, plural(total, "finding"),
			)
		}
	case severityGateMedium:
		total := summary.Critical + summary.High + summary.Medium
		if total > 0 {
			return true, fmt.Sprintf(
				"%d critical + %d high + %d medium (%d %s total)",
				summary.Critical, summary.High, summary.Medium, total, plural(total, "finding"),
			)
		}
	}
	return false, ""
}

// plural is a small helper to avoid the awkward `n thing(s)` notation
// in user-visible text. Mirrors the pluralization helper added in the
// 0.2 polish PRs (internal/reporting/plural.go) but kept local here
// so the cmd package doesn't pull in reporting just for one call.
func plural(n int, singular string) string {
	if n == 1 {
		return singular
	}
	return singular + "s"
}
