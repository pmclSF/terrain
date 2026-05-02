package airun

import "time"

// EvalRunResult is Terrain's normalized representation of one execution
// of an eval framework (Promptfoo, DeepEval, Ragas, ...). Each adapter
// parses its framework's native output into this shape; downstream
// detectors and reports consume EvalRunResult without caring which
// framework produced it.
//
// The 0.2 milestone calls for adapters under internal/airun/ that
// populate this struct. The 6 still-planned AI detectors
// (aiCostRegression, aiHallucinationRate, aiRetrievalRegression in
// particular) will consume EvalRunResult against a baseline to detect
// regressions.
type EvalRunResult struct {
	// Framework is the source adapter ("promptfoo" / "deepeval" / "ragas"
	// / "custom"). Lowercased canonical form.
	Framework string `json:"framework"`

	// RunID is the framework's identifier for this run when present.
	// Empty when the framework didn't supply one.
	RunID string `json:"runId,omitempty"`

	// CreatedAt is when the eval run was produced. Zero value when the
	// framework didn't expose a timestamp.
	CreatedAt time.Time `json:"createdAt,omitempty"`

	// Cases is one entry per (test, prompt, provider) combination. A
	// Promptfoo run with 50 tests × 2 providers produces 100 entries.
	Cases []EvalCase `json:"cases,omitempty"`

	// Aggregates summarizes the run. Populated either from the
	// framework's own summary fields or computed by the adapter.
	Aggregates EvalAggregates `json:"aggregates"`
}

// EvalCase is one (test, prompt, provider) result row.
type EvalCase struct {
	// CaseID is a stable identifier within the run, matching whatever
	// the framework used (e.g. promptfoo's `id` field). Empty when not
	// supplied; downstream code must treat positional ordering as a
	// fallback identifier.
	CaseID string `json:"caseId,omitempty"`

	// Description is the human-readable label (e.g. promptfoo
	// `testCase.description`).
	Description string `json:"description,omitempty"`

	// Provider is the framework-specific provider identifier
	// (e.g. "openai:gpt-4-0613"). Used by aiModelDeprecationRisk
	// follow-ups and by the report renderer.
	Provider string `json:"provider,omitempty"`

	// PromptLabel is the prompt's user-facing label when supplied.
	// Empty when the framework only attached prompt content.
	PromptLabel string `json:"promptLabel,omitempty"`

	// Success indicates whether the case passed. Scoring varies by
	// framework — Promptfoo treats Success differently from Score.
	Success bool `json:"success"`

	// Score is the framework's per-case score in [0.0, 1.0] when
	// available. Adapters that produce a single yes/no result map
	// that to {0.0, 1.0}.
	Score float64 `json:"score"`

	// LatencyMs is the wall-clock latency of the case in milliseconds.
	// Zero when the framework didn't record one.
	LatencyMs int `json:"latencyMs,omitempty"`

	// TokenUsage is the per-case token + cost data when present.
	TokenUsage TokenUsage `json:"tokenUsage,omitempty"`

	// NamedScores carries framework-specific scoring axes
	// (e.g. Promptfoo's `namedScores` field, Ragas's
	// retrieval_score / faithfulness / answer_relevancy). Adapters
	// may pass these through verbatim; the cost/hallucination/retrieval
	// detectors look for specific keys.
	NamedScores map[string]float64 `json:"namedScores,omitempty"`

	// FailureReason is the framework's diagnostic string when the
	// case failed, useful for the report renderer.
	FailureReason string `json:"failureReason,omitempty"`
}

// TokenUsage tracks LLM token + cost per case or aggregated.
type TokenUsage struct {
	Prompt     int     `json:"prompt,omitempty"`
	Completion int     `json:"completion,omitempty"`
	Total      int     `json:"total,omitempty"`
	Cost       float64 `json:"cost,omitempty"`
}

// EvalAggregates summarizes an eval run.
type EvalAggregates struct {
	// Successes / Failures / Errors mirror Promptfoo's three-bucket
	// stats. A "failure" is an assertion fail; an "error" is a runtime
	// problem (provider rejection, network timeout) that prevents
	// scoring at all.
	Successes int `json:"successes"`
	Failures  int `json:"failures"`
	Errors    int `json:"errors"`

	// TokenUsage is the run-level total across all cases.
	TokenUsage TokenUsage `json:"tokenUsage,omitempty"`
}

// CaseCount returns the total number of cases recorded.
func (a EvalAggregates) CaseCount() int {
	return a.Successes + a.Failures + a.Errors
}

// SuccessRate returns Successes / CaseCount, or 0 when there are no
// cases. Used by the regression detectors and by the report renderer.
func (a EvalAggregates) SuccessRate() float64 {
	total := a.CaseCount()
	if total == 0 {
		return 0
	}
	return float64(a.Successes) / float64(total)
}
