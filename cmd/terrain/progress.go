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

// isInteractive returns true if stderr is a terminal (TTY).
// Progress output goes to stderr so it doesn't interfere with
// stdout (which carries JSON or report output).
func isInteractive() bool {
	info, err := os.Stderr.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
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
