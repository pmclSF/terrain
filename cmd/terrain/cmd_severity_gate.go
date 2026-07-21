package main

import (
	"errors"
	"fmt"
	"strings"

	"github.com/pmclSF/terrain/internal/analyze"
	"github.com/pmclSF/terrain/internal/models"
	"github.com/pmclSF/terrain/internal/signals"
)

// errSeverityGateBlocked is the sentinel returned by runAnalyze and
// runPR when `--fail-on` matches at least one finding. main.go uses
// errors.Is to distinguish this from analysis errors and exit with
// `exitSeverityGateBlock` (6) rather than the generic 1.
var errSeverityGateBlocked = errors.New("severity gate blocked")

// prSeverityBreakdown converts a PR's change-scoped findings + AI
// blocking signals into the same SignalBreakdown shape that
// `analyze.SignalSummary` uses, so `severityGateBlocked` works
// uniformly across `terrain analyze --fail-on` and
// `terrain report pr --fail-on`, keeping the gate decision consistent
// across commands by sharing the logic rather than duplicating it.
//
// Counted by case-insensitive severity match. Unknown severities
// are dropped — the renderer is the source of truth for severity
// vocabulary.
func prSeverityBreakdown(severities []string) analyze.SignalBreakdown {
	var b analyze.SignalBreakdown
	for _, sev := range severities {
		switch strings.ToLower(strings.TrimSpace(sev)) {
		case "critical":
			b.Critical++
			b.Total++
		case "high":
			b.High++
			b.Total++
		case "medium":
			b.Medium++
			b.Total++
		case "low":
			b.Low++
			b.Total++
		}
	}
	return b
}

// signalSeverityBreakdown converts raw pipeline signals into the same
// gate summary used by analyze/report-pr. Observability-tier signals
// are intentionally excluded so `terrain test --fail-on` blocks on the
// same release-quality surface as `terrain analyze --fail-on`.
func signalSeverityBreakdown(sigs []models.Signal) analyze.SignalBreakdown {
	severities := make([]string, 0, len(sigs))
	for _, s := range sigs {
		if !signals.IsGateRelevant(s.Type) {
			continue
		}
		severities = append(severities, string(s.Severity))
	}
	return prSeverityBreakdown(severities)
}

// severityGate represents the threshold for `--fail-on`. Findings at
// or above this severity cause the analyze command to exit with
// `exitSeverityGateBlock`. Empty string means "no gate" (the default).
type severityGate string

const (
	severityGateNone     severityGate = ""
	severityGateCritical severityGate = "critical"
	severityGateHigh     severityGate = "high"
	severityGateMedium   severityGate = "medium"
	severityGateLow      severityGate = "low"
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
	case "low":
		return severityGateLow, nil
	default:
		return severityGateNone, fmt.Errorf(
			"invalid --fail-on %q: valid values are 'critical', 'high', 'medium', 'low' (or unset to disable)",
			s,
		)
	}
}

// severityGateBlocked returns (true, summary) when the report contains
// at least one signal at or above the configured threshold. The
// summary is a one-line, human-readable description of which severity
// counts triggered the gate, suitable for printing to stderr before
// exit.
// trustFloorHeldBack returns how many gate-relevant findings at or above the
// --fail-on threshold were held back by the trust floor (present in `raw` but
// not in the trust-floor-filtered `floored` breakdown). Callers surface this so
// a build that passes because of the trust floor is never silent about it.
func trustFloorHeldBack(gate severityGate, raw, floored analyze.SignalBreakdown) int {
	held := countAtOrAbove(gate, raw) - countAtOrAbove(gate, floored)
	if held < 0 {
		return 0
	}
	return held
}

// countAtOrAbove returns how many findings in the breakdown sit at or above the
// gate's severity threshold — i.e. the count that actually fails the merge.
func countAtOrAbove(gate severityGate, b analyze.SignalBreakdown) int {
	switch gate {
	case severityGateCritical:
		return b.Critical
	case severityGateHigh:
		return b.Critical + b.High
	case severityGateMedium:
		return b.Critical + b.High + b.Medium
	case severityGateLow:
		return b.Critical + b.High + b.Medium + b.Low
	}
	return 0
}

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
	case severityGateLow:
		total := summary.Critical + summary.High + summary.Medium + summary.Low
		if total > 0 {
			return true, fmt.Sprintf(
				"%d critical + %d high + %d medium + %d low (%d %s total)",
				summary.Critical, summary.High, summary.Medium, summary.Low, total, plural(total, "finding"),
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
