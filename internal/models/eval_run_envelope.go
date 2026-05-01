package models

// EvalRunEnvelope is the snapshot-level wrapper around one normalised
// eval-framework result. The detailed EvalRunResult lives in
// internal/airun (so models doesn't depend on adapter implementations);
// the envelope carries enough metadata for the renderer + detectors
// without forcing every consumer to depend on airun.
//
// SignalV2 (0.2). Detectors that need the full case-by-case data load
// the embedded JSON via airun.ParseEvalRunPayload.
type EvalRunEnvelope struct {
	// Framework names the source adapter (e.g. "promptfoo").
	Framework string `json:"framework"`

	// SourcePath is the repo-relative path to the artifact the adapter
	// parsed. Empty when the data was supplied programmatically.
	SourcePath string `json:"sourcePath,omitempty"`

	// RunID is the adapter's identifier for the run.
	RunID string `json:"runId,omitempty"`

	// Aggregates is the run-level summary surfaced directly so reports
	// can render a one-liner without unmarshalling the full payload.
	Aggregates EvalRunAggregates `json:"aggregates"`

	// Payload is the JSON-encoded full EvalRunResult. Detectors that
	// need per-case data unmarshal this via airun.ParseEvalRunPayload.
	// Stored as raw bytes so models stays independent of the airun
	// shape.
	Payload []byte `json:"payload,omitempty"`
}

// EvalRunAggregates mirrors airun.EvalAggregates at the snapshot
// level. Duplicated so that models can render a top-level summary
// without importing airun.
type EvalRunAggregates struct {
	Successes int                  `json:"successes"`
	Failures  int                  `json:"failures"`
	Errors    int                  `json:"errors"`
	Tokens    EvalRunTokenUsage    `json:"tokens,omitempty"`
}

type EvalRunTokenUsage struct {
	Total int     `json:"total,omitempty"`
	Cost  float64 `json:"cost,omitempty"`
}
