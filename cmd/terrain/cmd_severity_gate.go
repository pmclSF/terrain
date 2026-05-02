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
			return true, fmt.Sprintf("%d critical finding(s)", summary.Critical)
		}
	case severityGateHigh:
		if summary.Critical > 0 || summary.High > 0 {
			return true, fmt.Sprintf(
				"%d critical + %d high finding(s)",
				summary.Critical, summary.High,
			)
		}
	case severityGateMedium:
		if summary.Critical > 0 || summary.High > 0 || summary.Medium > 0 {
			return true, fmt.Sprintf(
				"%d critical + %d high + %d medium finding(s)",
				summary.Critical, summary.High, summary.Medium,
			)
		}
	}
	return false, ""
}
