package evaladapter

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/pmclSF/terrain/internal/saferead"
)

// GreatExpectationsAdapter ingests Great Expectations Validation Result
// JSON. GE's output schema is well-defined and stable across versions
// 0.15+ — a top-level object with `results` (per-expectation array),
// `success`, and `statistics` (run-level totals).
//
// Each expectation maps to one EvalCaseResult: the expectation type
// (e.g., expect_column_values_to_be_in_set) becomes the case name,
// success is the boolean pass/fail, and the expectation's specific
// failure metadata becomes the Reason. Score is 1.0 for pass and 0.0
// for fail — GE doesn't produce scalar metric scores at the
// expectation level, so the regression rule treats GE runs as
// pass-rate-driven rather than score-delta-driven.
type GreatExpectationsAdapter struct{}

// Name implements Adapter.
func (GreatExpectationsAdapter) Name() Framework { return FrameworkGreatExpectations }

// CanIngest implements Adapter. GE's Validation Result has top-level
// `meta.great_expectations_version` (when present) or the distinctive
// `evaluated_expectations` key in `statistics`. We probe for both to
// support older + newer artifacts.
func (GreatExpectationsAdapter) CanIngest(path string) bool {
	if !strings.HasSuffix(strings.ToLower(path), ".json") {
		return false
	}
	data, err := saferead.ReadFile(path)
	if err != nil {
		return false
	}
	var head struct {
		Statistics struct {
			EvaluatedExpectations *int `json:"evaluated_expectations"`
		} `json:"statistics"`
		Meta struct {
			GreatExpectationsVersion string `json:"great_expectations_version"`
		} `json:"meta"`
		Results []json.RawMessage `json:"results"`
	}
	if err := json.Unmarshal(data, &head); err != nil {
		return false
	}
	if head.Meta.GreatExpectationsVersion != "" {
		return true
	}
	if head.Statistics.EvaluatedExpectations != nil {
		return true
	}
	// Last-resort signal: results[0] should have expectation_config.
	if len(head.Results) > 0 {
		var probe struct {
			ExpectationConfig struct {
				ExpectationType string `json:"expectation_type"`
			} `json:"expectation_config"`
		}
		if err := json.Unmarshal(head.Results[0], &probe); err == nil {
			return probe.ExpectationConfig.ExpectationType != ""
		}
	}
	return false
}

// Ingest implements Adapter.
func (GreatExpectationsAdapter) Ingest(path string) (*EvalRun, error) {
	data, err := saferead.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("ge: read %s: %w", path, err)
	}

	var raw struct {
		Success bool `json:"success"`
		Meta    struct {
			GreatExpectationsVersion string `json:"great_expectations_version"`
			RunID                    struct {
				RunTime string `json:"run_time"`
			} `json:"run_id"`
		} `json:"meta"`
		Statistics struct {
			EvaluatedExpectations    int     `json:"evaluated_expectations"`
			SuccessfulExpectations   int     `json:"successful_expectations"`
			UnsuccessfulExpectations int     `json:"unsuccessful_expectations"`
			SuccessPercent           float64 `json:"success_percent"`
		} `json:"statistics"`
		Results []struct {
			Success           bool `json:"success"`
			ExpectationConfig struct {
				ExpectationType string                 `json:"expectation_type"`
				Kwargs          map[string]interface{} `json:"kwargs"`
				Meta            map[string]interface{} `json:"meta"`
			} `json:"expectation_config"`
			Result struct {
				ObservedValue         interface{}   `json:"observed_value"`
				UnexpectedCount       int           `json:"unexpected_count"`
				UnexpectedPercent     float64       `json:"unexpected_percent"`
				PartialUnexpectedList []interface{} `json:"partial_unexpected_list"`
			} `json:"result"`
			ExceptionInfo struct {
				RaisedException  bool   `json:"raised_exception"`
				ExceptionMessage string `json:"exception_message"`
			} `json:"exception_info"`
		} `json:"results"`
	}

	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("ge: parse %s: %w", path, err)
	}

	run := &EvalRun{
		Framework: FrameworkGreatExpectations,
		Source:    path,
		Timestamp: raw.Meta.RunID.RunTime,
	}

	for i, r := range raw.Results {
		// Build a stable ID from expectation_type + the column / table
		// it targets when those are in kwargs.
		col, _ := r.ExpectationConfig.Kwargs["column"].(string)
		id := r.ExpectationConfig.ExpectationType
		if col != "" {
			id = id + ":" + col
		}
		if id == "" {
			id = fmt.Sprintf("%s-case-%d", filepath.Base(path), i)
		}

		// Score = 1.0 for pass, 0.0 for fail. GE doesn't produce a
		// scalar score per expectation, but the binary score lets the
		// regression rule compute pass-rate deltas with the same
		// machinery as other adapters.
		score := 0.0
		if r.Success {
			score = 1.0
		}

		reason := ""
		if !r.Success {
			parts := []string{}
			if r.ExceptionInfo.RaisedException && r.ExceptionInfo.ExceptionMessage != "" {
				parts = append(parts, "exception: "+r.ExceptionInfo.ExceptionMessage)
			}
			if r.Result.UnexpectedCount > 0 {
				parts = append(parts, fmt.Sprintf("unexpected_count=%d unexpected_pct=%.2f",
					r.Result.UnexpectedCount, r.Result.UnexpectedPercent))
			}
			reason = strings.Join(parts, "; ")
		}

		run.Cases = append(run.Cases, EvalCaseResult{
			ID:      id,
			Name:    id,
			Success: r.Success,
			Score:   score,
			Reason:  reason,
		})
	}

	run.Stats = EvalRunStats{
		Total:     raw.Statistics.EvaluatedExpectations,
		Successes: raw.Statistics.SuccessfulExpectations,
		Failures:  raw.Statistics.UnsuccessfulExpectations,
	}
	if run.Stats.Total == 0 && len(run.Cases) > 0 {
		// Older GE versions don't always populate statistics — recompute.
		run.Stats.Total = len(run.Cases)
		for _, c := range run.Cases {
			if c.Success {
				run.Stats.Successes++
			} else {
				run.Stats.Failures++
			}
		}
	}
	if run.Stats.Total > 0 {
		// PrimaryMetric here is the pass rate (0–1), since GE has no
		// scalar score. This matches the rule layer's expectation that
		// PrimaryMetric is a comparable scalar per run.
		run.Stats.PrimaryMetric = float64(run.Stats.Successes) / float64(run.Stats.Total)
		run.Stats.HasPrimaryMetric = true
	}

	return run, nil
}

var _ Adapter = GreatExpectationsAdapter{}
