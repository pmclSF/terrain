package airun

import "testing"

// promptfooV3Sample is the v3 nested shape: { results: { results: [...], stats: {...} } }
const promptfooV3Sample = `{
  "evalId": "eval-abc123",
  "createdAt": 1736899200000,
  "results": {
    "version": 3,
    "results": [
      {
        "id": "row-1",
        "description": "happy path",
        "success": true,
        "score": 1.0,
        "latencyMs": 850,
        "provider": "openai:gpt-4-0613",
        "prompt": {"label": "system + user"},
        "response": {
          "output": "ok",
          "tokenUsage": {"prompt": 50, "completion": 30, "total": 80, "cost": 0.0024}
        },
        "namedScores": {"factuality": 0.95, "relevance": 1.0}
      },
      {
        "id": "row-2",
        "description": "edge case",
        "success": false,
        "score": 0.0,
        "latencyMs": 1200,
        "provider": "openai:gpt-4-0613",
        "prompt": {"label": "system + user"},
        "response": {
          "output": "wrong",
          "tokenUsage": {"prompt": 60, "completion": 5, "total": 65, "cost": 0.0019}
        },
        "failureReason": "expected 'paris', got 'wrong'"
      }
    ],
    "stats": {
      "successes": 1,
      "failures": 1,
      "errors": 0,
      "tokenUsage": {"prompt": 110, "completion": 35, "total": 145, "cost": 0.0043}
    }
  }
}`

// promptfooV4Sample flattens results to the top level. Provider may be
// an object instead of a string.
const promptfooV4Sample = `{
  "evalId": "eval-xyz",
  "createdAtISO": "2026-04-30T12:00:00Z",
  "results": [
    {
      "id": "r1",
      "testCase": {"description": "calc"},
      "success": true,
      "score": 1.0,
      "provider": {"id": "anthropic:claude-3-opus-20240229"},
      "response": {"tokenUsage": {"total": 100, "cost": 0.005}}
    },
    {
      "id": "r2",
      "testCase": {"description": "calc edge"},
      "success": false,
      "score": 0.0,
      "provider": {"id": "anthropic:claude-3-opus-20240229"},
      "response": {"tokenUsage": {"total": 80, "cost": 0.004}}
    }
  ],
  "stats": {"successes": 1, "failures": 1, "errors": 0}
}`

func TestParsePromptfoo_V3Nested(t *testing.T) {
	t.Parallel()

	got, err := ParsePromptfooJSON([]byte(promptfooV3Sample))
	if err != nil {
		t.Fatalf("ParsePromptfooJSON: %v", err)
	}
	if got.Framework != "promptfoo" {
		t.Errorf("framework = %q", got.Framework)
	}
	if got.RunID != "eval-abc123" {
		t.Errorf("runId = %q", got.RunID)
	}
	if got.CreatedAt.IsZero() {
		t.Errorf("expected non-zero CreatedAt from epoch-millis field")
	}
	if len(got.Cases) != 2 {
		t.Fatalf("cases = %d, want 2", len(got.Cases))
	}
	if got.Cases[0].Description != "happy path" {
		t.Errorf("cases[0].Description = %q", got.Cases[0].Description)
	}
	if got.Cases[0].Provider != "openai:gpt-4-0613" {
		t.Errorf("cases[0].Provider = %q", got.Cases[0].Provider)
	}
	if got.Cases[0].TokenUsage.Total != 80 || got.Cases[0].TokenUsage.Cost != 0.0024 {
		t.Errorf("cases[0].TokenUsage = %+v", got.Cases[0].TokenUsage)
	}
	if got.Cases[0].NamedScores["factuality"] != 0.95 {
		t.Errorf("cases[0].NamedScores[factuality] = %v", got.Cases[0].NamedScores["factuality"])
	}
	if got.Cases[1].FailureReason == "" {
		t.Errorf("cases[1].FailureReason should be populated")
	}
	if got.Aggregates.Successes != 1 || got.Aggregates.Failures != 1 {
		t.Errorf("aggregates = %+v", got.Aggregates)
	}
	if got.Aggregates.TokenUsage.Total != 145 || got.Aggregates.TokenUsage.Cost != 0.0043 {
		t.Errorf("aggregates.TokenUsage = %+v", got.Aggregates.TokenUsage)
	}
}

func TestParsePromptfoo_V4Flat(t *testing.T) {
	t.Parallel()

	got, err := ParsePromptfooJSON([]byte(promptfooV4Sample))
	if err != nil {
		t.Fatalf("ParsePromptfooJSON: %v", err)
	}
	if got.RunID != "eval-xyz" {
		t.Errorf("runId = %q", got.RunID)
	}
	if got.CreatedAt.IsZero() {
		t.Errorf("expected non-zero CreatedAt from ISO field")
	}
	if len(got.Cases) != 2 {
		t.Fatalf("cases = %d, want 2", len(got.Cases))
	}
	if got.Cases[0].Provider != "anthropic:claude-3-opus-20240229" {
		t.Errorf("provider object form not parsed: %q", got.Cases[0].Provider)
	}
	if got.Cases[0].Description != "calc" {
		t.Errorf("description from testCase.description not picked up: %q", got.Cases[0].Description)
	}
}

func TestParsePromptfoo_DerivesAggregatesWhenMissing(t *testing.T) {
	t.Parallel()

	// stats omitted → aggregates derived from rows.
	const sample = `{
  "evalId": "tiny",
  "results": [
    {"id": "a", "success": true, "response": {"tokenUsage": {"total": 10, "cost": 0.001}}},
    {"id": "b", "success": false, "response": {"tokenUsage": {"total": 20, "cost": 0.002}}}
  ]
}`
	got, err := ParsePromptfooJSON([]byte(sample))
	if err != nil {
		t.Fatalf("ParsePromptfooJSON: %v", err)
	}
	if got.Aggregates.Successes != 1 || got.Aggregates.Failures != 1 {
		t.Errorf("aggregates derived wrong: %+v", got.Aggregates)
	}
	if got.Aggregates.TokenUsage.Total != 30 {
		t.Errorf("token total = %d, want 30", got.Aggregates.TokenUsage.Total)
	}
	if got.Aggregates.TokenUsage.Cost != 0.003 {
		t.Errorf("token cost = %v, want 0.003", got.Aggregates.TokenUsage.Cost)
	}
}

func TestParsePromptfoo_RejectsEmpty(t *testing.T) {
	t.Parallel()

	if _, err := ParsePromptfooJSON(nil); err == nil {
		t.Error("expected empty payload to be rejected")
	}
}

func TestParsePromptfoo_RejectsMalformedShape(t *testing.T) {
	t.Parallel()

	if _, err := ParsePromptfooJSON([]byte(`{"results": "not an array or object"}`)); err == nil {
		t.Error("expected malformed shape to be rejected")
	}
}

func TestEvalAggregates_SuccessRate(t *testing.T) {
	t.Parallel()

	a := EvalAggregates{Successes: 9, Failures: 1, Errors: 0}
	if got := a.SuccessRate(); got != 0.9 {
		t.Errorf("SuccessRate = %v, want 0.9", got)
	}
	if got := (EvalAggregates{}).SuccessRate(); got != 0 {
		t.Errorf("empty SuccessRate = %v, want 0", got)
	}
}

// TestParsePromptfoo_RowDerivedFallback_RoutesErroredRowsToErrors
// locks in the 0.2.0 final-polish fix: when stats are absent (Promptfoo
// v3 small runs, or a raw row dump), the row-derived fallback used to
// classify every non-success row as Failure. Rows where the provider
// crashed (`error: "..."`) should land in Aggregates.Errors instead so
// aiHallucinationRate's `caseIsScoreable` denominator excludes them
// rather than treating them as legitimate evaluation failures.
func TestParsePromptfoo_RowDerivedFallback_RoutesErroredRowsToErrors(t *testing.T) {
	t.Parallel()
	// Nested-results shape with no stats; one success, one assertion
	// failure, one provider crash (error field set).
	body := `{"results":[
		{"id":"a","success":true,"score":1.0,"response":{"tokenUsage":{"total":10}}},
		{"id":"b","success":false,"failureReason":"output mismatch","response":{"tokenUsage":{"total":12}}},
		{"id":"c","success":false,"error":"provider 503 timeout","response":{"tokenUsage":{"total":0}}}
	]}`
	got, err := ParsePromptfooJSON([]byte(body))
	if err != nil {
		t.Fatalf("ParsePromptfooJSON: %v", err)
	}
	if got.Aggregates.Successes != 1 {
		t.Errorf("Successes = %d, want 1", got.Aggregates.Successes)
	}
	if got.Aggregates.Failures != 1 {
		t.Errorf("Failures = %d, want 1 (assertion failure)", got.Aggregates.Failures)
	}
	if got.Aggregates.Errors != 1 {
		t.Errorf("Errors = %d, want 1 (provider crash)", got.Aggregates.Errors)
	}
}

// TestParsePromptfoo_PerCaseCostFallback locks in the 0.2.0 fix where
// per-case cost reads from `r.cost` when `r.response.tokenUsage.cost`
// is absent. Modern Promptfoo writes the same value to both; the
// adapter pre-fix only read response-level, so cost regressions
// silently no-op'd when Promptfoo only populated the top-level field.
func TestParsePromptfoo_PerCaseCostFallback(t *testing.T) {
	t.Parallel()
	body := `{"results":[
		{"id":"a","success":true,"cost":0.0042,"response":{}}
	]}`
	got, err := ParsePromptfooJSON([]byte(body))
	if err != nil {
		t.Fatalf("ParsePromptfooJSON: %v", err)
	}
	if len(got.Cases) != 1 {
		t.Fatalf("expected 1 case, got %d", len(got.Cases))
	}
	if got.Cases[0].TokenUsage.Cost != 0.0042 {
		t.Errorf("per-case cost = %v, want 0.0042 (top-level fallback)", got.Cases[0].TokenUsage.Cost)
	}
}
