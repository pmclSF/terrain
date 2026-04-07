package reporting

import (
	"fmt"
	"io"
)

// ReportOptions controls rendering behavior across all report functions.
type ReportOptions struct {
	Verbose bool
}

// isVerbose extracts the Verbose flag from variadic options.
func isVerbose(opts []ReportOptions) bool {
	return len(opts) > 0 && opts[0].Verbose
}

// reportHelpers returns line and blank writer functions bound to w.
func reportHelpers(w io.Writer) (func(string, ...any), func()) {
	line := func(format string, args ...any) {
		fmt.Fprintf(w, format+"\n", args...)
	}
	blank := func() { fmt.Fprintln(w) }
	return line, blank
}
