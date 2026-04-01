package reporting

// ReportOptions controls rendering behavior across all report functions.
type ReportOptions struct {
	Verbose bool
}

// isVerbose extracts the Verbose flag from variadic options.
func isVerbose(opts []ReportOptions) bool {
	return len(opts) > 0 && opts[0].Verbose
}
