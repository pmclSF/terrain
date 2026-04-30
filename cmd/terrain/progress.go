package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/pmclSF/terrain/internal/engine"
	"github.com/pmclSF/terrain/internal/logging"
)

// isInteractive returns true if stderr is a terminal (TTY) AND the user
// hasn't explicitly opted out of styled output via NO_COLOR or a dumb TERM.
//
// Progress output goes to stderr so it doesn't interfere with stdout (which
// carries JSON or report output). It also uses ANSI escape sequences (\r
// for line-overwrite at minimum, colors in some modes), which means
// respecting NO_COLOR semantics: if the user has asked for plain output, we
// emit no progress at all rather than dumping carriage returns into log
// files. NO_COLOR is the de-facto standard (https://no-color.org).
//
// Common CI environments are also non-interactive: GitHub Actions, GitLab
// CI, CircleCI, Buildkite, and Jenkins all set CI=true (or similar) and
// pipe stderr to a log buffer. Progress overwrite makes those logs
// unreadable, so we suppress it whenever CI markers are present.
func isInteractive() bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	switch strings.ToLower(os.Getenv("TERM")) {
	case "dumb", "unknown":
		return false
	}
	if isCIEnvironment() {
		return false
	}
	info, err := os.Stderr.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

// isCIEnvironment detects whether the process is running under a known CI
// system. Detection covers the most common providers; an unknown CI system
// can still opt in by setting CI=true (the lowest-common-denominator
// environment variable that essentially every CI sets).
func isCIEnvironment() bool {
	for _, key := range []string{
		"CI",
		"GITHUB_ACTIONS",
		"GITLAB_CI",
		"CIRCLECI",
		"BUILDKITE",
		"JENKINS_URL",
		"TF_BUILD", // Azure Pipelines
	} {
		if v := os.Getenv(key); v != "" && v != "false" && v != "0" {
			return true
		}
	}
	return false
}

// newProgressFunc returns a ProgressFunc appropriate for the current output
// mode. Returns nil if progress should be suppressed (JSON mode, non-TTY,
// or --log-level quiet).
//
// In interactive mode (TTY), progress is rendered as step-based lines:
//
//	[1/5] Scanning repository
//	[2/5] Building graph
//	...
//
// Each step overwrites the previous line using \r for a clean UX.
// In non-interactive mode (pipe/redirect), no progress is emitted
// to keep output parseable.
func newProgressFunc(jsonOutput bool) engine.ProgressFunc {
	if jsonOutput || !isInteractive() {
		return nil
	}
	// In debug mode, emit progress as structured log lines instead of
	// carriage-return overwrite (avoids garbled output with other log lines).
	if logging.L().Handler().Enabled(context.Background(), slog.LevelDebug) {
		return func(step, total int, label string) {
			logging.L().Debug("pipeline progress", "step", step, "total", total, "label", label)
		}
	}
	return func(step, total int, label string) {
		// Pad with spaces to clear any leftover characters from longer
		// previous labels when overwriting with \r.
		const clearWidth = 60
		line := fmt.Sprintf("[%d/%d] %s", step, total, label)
		if len(line) < clearWidth {
			line += strings.Repeat(" ", clearWidth-len(line))
		}
		if step < total {
			fmt.Fprintf(os.Stderr, "\r%s", line)
		} else {
			fmt.Fprintf(os.Stderr, "\r%s\n", line)
		}
	}
}
