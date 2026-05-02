package airun

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

// ParsePromptfooJSON parses a Promptfoo `--output result.json` payload
// and returns a normalised EvalRunResult.
//
// Promptfoo's JSON format has shifted across major versions (v3 / v4
// most commonly seen in the wild). This adapter handles both shapes:
//
//   v3 (current): top-level { evalId, results: { results: [...], stats: {...} } }
//   v4+ (newer):  top-level { evalId, results: [...], stats: {...} }
//
// Anything we can't recognise is returned as an error rather than
// silently producing an empty result; the calibration corpus catches
// adapter regressions explicitly.
func ParsePromptfooJSON(data []byte) (*EvalRunResult, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("empty payload")
	}

	var raw promptfooEnvelope
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse promptfoo payload: %w", err)
	}

	out := &EvalRunResult{
		Framework: "promptfoo",
		RunID:     raw.EvalID,
	}
	if raw.CreatedAt > 0 {
		// Promptfoo writes createdAt as a unix-millis number.
		out.CreatedAt = time.UnixMilli(raw.CreatedAt).UTC()
	} else if raw.CreatedAtISO != "" {
		if t, err := time.Parse(time.RFC3339, raw.CreatedAtISO); err == nil {
			out.CreatedAt = t.UTC()
		}
	}

	// Pick the results envelope. v3 nests under `results.results`;
	// v4+ flattens to a top-level `results` array. We accept either.
	var rows []promptfooResult
	var stats promptfooStats
	switch {
	case raw.Results.IsArray():
		rows = raw.Results.Array
		stats = raw.Stats
	case raw.Results.IsNested():
		rows = raw.Results.Nested.Results
		stats = raw.Results.Nested.Stats
		// Some v3 dumps put stats only at the inner level; if the
		// outer one is empty fall back to the inner.
		if stats == (promptfooStats{}) {
			stats = raw.Stats
		}
	default:
		return nil, fmt.Errorf("promptfoo payload has no results array (neither top-level nor nested)")
	}

	out.Cases = make([]EvalCase, 0, len(rows))
	for _, row := range rows {
		out.Cases = append(out.Cases, normalisePromptfooRow(row))
	}

	out.Aggregates = EvalAggregates{
		Successes: stats.Successes,
		Failures:  stats.Failures,
		Errors:    stats.Errors,
		TokenUsage: TokenUsage{
			Prompt:     stats.TokenUsage.Prompt,
			Completion: stats.TokenUsage.Completion,
			Total:      stats.TokenUsage.Total,
			Cost:       stats.TokenUsage.Cost,
		},
	}
	// If stats.Successes etc. are zero but rows were present, derive
	// the aggregates from the rows. Promptfoo v3 dumps occasionally
	// omit stats entirely on small runs.
	//
	// Pre-0.2.x final-polish, this loop classified every non-success
	// row as Failure, including rows where the provider crashed
	// (`error: "..."`). That polluted aiHallucinationRate's denominator
	// (which excludes Errors but counts Failures). Now we route
	// errored rows into Aggregates.Errors via promptfooRowErrored.
	if out.Aggregates.CaseCount() == 0 && len(out.Cases) > 0 {
		for i, c := range out.Cases {
			switch {
			case c.Success:
				out.Aggregates.Successes++
			case promptfooRowErrored(rows[i]):
				out.Aggregates.Errors++
			default:
				out.Aggregates.Failures++
			}
			out.Aggregates.TokenUsage.Total += c.TokenUsage.Total
			out.Aggregates.TokenUsage.Prompt += c.TokenUsage.Prompt
			out.Aggregates.TokenUsage.Completion += c.TokenUsage.Completion
			out.Aggregates.TokenUsage.Cost += c.TokenUsage.Cost
		}
	}

	return out, nil
}

// LoadPromptfooFile is a thin convenience wrapper that reads the file
// at path and delegates to ParsePromptfooJSON.
func LoadPromptfooFile(path string) (*EvalRunResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return ParsePromptfooJSON(data)
}

// normalisePromptfooRow converts one Promptfoo result row to an EvalCase.
func normalisePromptfooRow(r promptfooResult) EvalCase {
	// Prefer the response-level cost (long-standing Promptfoo shape).
	// When zero, fall back to the top-level `cost` field that modern
	// Promptfoo emits — both fields can carry per-case cost depending
	// on the version. Without this fallback, aiCostRegression saw zero
	// per-case cost and silently no-op'd on cost regressions even when
	// the aggregate was non-zero.
	cost := r.Response.TokenUsage.Cost
	if cost == 0 {
		cost = r.Cost
	}
	// FailureReason carries assertion-failure detail; an `error` string
	// at the row level indicates a runtime/provider crash that should
	// be surfaced as a separate "this case errored" axis (see
	// errorRow() below). We keep both, with FailureReason preferring
	// the assertion message when present.
	failureReason := strings.TrimSpace(r.FailureReason)
	if failureReason == "" && r.Error != "" {
		failureReason = strings.TrimSpace(r.Error)
	}
	c := EvalCase{
		CaseID:        r.ID,
		Description:   firstNonEmpty(r.TestCase.Description, r.Description),
		Provider:      flattenProvider(r),
		PromptLabel:   r.Prompt.Label,
		Success:       r.Success,
		Score:         r.Score,
		LatencyMs:     r.LatencyMs,
		FailureReason: failureReason,
		TokenUsage: TokenUsage{
			Prompt:     r.Response.TokenUsage.Prompt,
			Completion: r.Response.TokenUsage.Completion,
			Total:      r.Response.TokenUsage.Total,
			Cost:       cost,
		},
	}
	if len(r.NamedScores) > 0 {
		c.NamedScores = make(map[string]float64, len(r.NamedScores))
		for k, v := range r.NamedScores {
			c.NamedScores[k] = v
		}
	}
	return c
}

// promptfooRowErrored reports whether a row represents a runtime
// failure (provider crash, schema-parse error, network) as opposed to
// an assertion failure. Used by the row-derived stats fallback so
// errored cases land in Aggregates.Errors instead of Aggregates.Failures.
func promptfooRowErrored(r promptfooResult) bool {
	return strings.TrimSpace(r.Error) != ""
}

// flattenProvider resolves the provider identifier across Promptfoo's
// shapes. It can appear as a top-level string, a {id} object, or
// inside the prompt block as `provider`.
func flattenProvider(r promptfooResult) string {
	if r.Provider.String != "" {
		return r.Provider.String
	}
	if r.Provider.Object.ID != "" {
		return r.Provider.Object.ID
	}
	if r.Prompt.Provider != "" {
		return r.Prompt.Provider
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// ── Promptfoo wire shapes (subset we consume) ──────────────────────

type promptfooEnvelope struct {
	EvalID       string                  `json:"evalId,omitempty"`
	CreatedAt    int64                   `json:"createdAt,omitempty"`
	CreatedAtISO string                  `json:"createdAtISO,omitempty"`
	Results      promptfooResultsAdapter `json:"results"`
	Stats        promptfooStats          `json:"stats,omitempty"`
}

// promptfooResultsAdapter handles the v3 vs v4 shape difference for
// the `results` field. v4+ is an array; v3 is `{ results: [...], stats: {...} }`.
type promptfooResultsAdapter struct {
	Array  []promptfooResult
	Nested *promptfooResultsNested
}

func (a promptfooResultsAdapter) IsArray() bool  { return a.Array != nil }
func (a promptfooResultsAdapter) IsNested() bool { return a.Nested != nil }

func (a *promptfooResultsAdapter) UnmarshalJSON(data []byte) error {
	// Try array first.
	var asArray []promptfooResult
	if err := json.Unmarshal(data, &asArray); err == nil {
		a.Array = asArray
		return nil
	}
	// Then try nested object.
	var nested promptfooResultsNested
	if err := json.Unmarshal(data, &nested); err == nil {
		a.Nested = &nested
		return nil
	}
	return fmt.Errorf("promptfoo `results` field is neither an array nor a nested object")
}

type promptfooResultsNested struct {
	Results []promptfooResult `json:"results"`
	Stats   promptfooStats    `json:"stats"`
}

type promptfooResult struct {
	ID            string                   `json:"id,omitempty"`
	Description   string                   `json:"description,omitempty"`
	Success       bool                     `json:"success"`
	Score         float64                  `json:"score,omitempty"`
	LatencyMs     int                      `json:"latencyMs,omitempty"`
	NamedScores   map[string]float64       `json:"namedScores,omitempty"`
	Provider      promptfooProviderAdapter `json:"provider,omitempty"`
	Prompt        promptfooPrompt          `json:"prompt,omitempty"`
	Response      promptfooResponse        `json:"response,omitempty"`
	TestCase      promptfooTestCase        `json:"testCase,omitempty"`
	FailureReason string                   `json:"failureReason,omitempty"`
	// Error captures provider/runtime errors (Promptfoo v4+ writes a
	// per-row `error` string when the provider crashed, the assertion
	// engine errored, or any non-assertion failure occurred). Pre-0.2.x
	// final-polish, Promptfoo's `stats.errors` aggregate was wired into
	// EvalAggregates.Errors, but the row-derived fallback (used when
	// stats are absent) lumped errored rows into Failures — polluting
	// aiHallucinationRate's `caseIsScoreable` denominator.
	Error string `json:"error,omitempty"`
	// Cost is Promptfoo's top-level per-case cost (parallel to
	// `r.Response.TokenUsage.Cost`). Modern Promptfoo emits cost both
	// places; reading both lets aiCostRegression see per-case cost
	// when the response-level field is empty.
	Cost float64 `json:"cost,omitempty"`
}

// promptfooProviderAdapter accepts both `"provider": "openai:gpt-4"`
// and `"provider": {"id": "openai:gpt-4", ...}`.
type promptfooProviderAdapter struct {
	String string
	Object promptfooProviderObject
}

func (a *promptfooProviderAdapter) UnmarshalJSON(data []byte) error {
	if len(data) == 0 || string(data) == "null" {
		return nil
	}
	if data[0] == '"' {
		return json.Unmarshal(data, &a.String)
	}
	return json.Unmarshal(data, &a.Object)
}

type promptfooProviderObject struct {
	ID string `json:"id"`
}

type promptfooPrompt struct {
	Raw      string `json:"raw,omitempty"`
	Label    string `json:"label,omitempty"`
	Provider string `json:"provider,omitempty"`
}

type promptfooResponse struct {
	Output     string                 `json:"output,omitempty"`
	TokenUsage promptfooTokenUsage    `json:"tokenUsage,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

type promptfooTestCase struct {
	Description string `json:"description,omitempty"`
}

type promptfooStats struct {
	Successes  int                 `json:"successes,omitempty"`
	Failures   int                 `json:"failures,omitempty"`
	Errors     int                 `json:"errors,omitempty"`
	TokenUsage promptfooTokenUsage `json:"tokenUsage,omitempty"`
}

type promptfooTokenUsage struct {
	Prompt     int     `json:"prompt,omitempty"`
	Completion int     `json:"completion,omitempty"`
	Total      int     `json:"total,omitempty"`
	Cost       float64 `json:"cost,omitempty"`
}
