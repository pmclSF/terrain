package evaladapter

import (
	"encoding/json"
	"fmt"
	"math"
	"path/filepath"
	"strings"

	"github.com/pmclSF/terrain/internal/saferead"
)

// DeepevalAdapter ingests deepeval test-run JSON output, the artifact
// produced by `deepeval test run ... --output <file>.json` and the
// per-test JSON emitted by `deepeval.evaluate(...)`.
//
// deepeval's shape carries multiple named metrics per case
// (metricsMetadata), each with its own score, threshold, reason, and
// pass/fail. EvalCaseResult.Score is the case's overall pass-as-1.0,
// EvalCaseResult.Metrics holds metric_name → score, and Threshold
// holds the first metric's threshold (deepeval's convention is to
// score against the worst metric; a single threshold is a meaningful
// summary).
type DeepevalAdapter struct{}

// Name implements Adapter.
func (DeepevalAdapter) Name() Framework { return FrameworkDeepeval }

// CanIngest implements Adapter. deepeval's JSON has a top-level
// `testCases` array with objects carrying `input`, `actualOutput`,
// and `metricsMetadata`. The trio is distinctive enough to
// disambiguate from promptfoo / ragas / GE without false matches.
func (DeepevalAdapter) CanIngest(path string) bool {
	if !strings.HasSuffix(strings.ToLower(path), ".json") {
		return false
	}
	data, err := saferead.ReadFile(path)
	if err != nil {
		return false
	}
	var head struct {
		TestCases []struct {
			Input           string          `json:"input"`
			ActualOutput    string          `json:"actualOutput"`
			MetricsMetadata json.RawMessage `json:"metricsMetadata"`
		} `json:"testCases"`
	}
	if err := json.Unmarshal(data, &head); err != nil {
		return false
	}
	if len(head.TestCases) == 0 {
		return false
	}
	// The metricsMetadata field is the deepeval distinguishing marker —
	// neither promptfoo nor ragas use that key.
	return len(head.TestCases[0].MetricsMetadata) > 0
}

// Ingest implements Adapter.
func (DeepevalAdapter) Ingest(path string) (*EvalRun, error) {
	data, err := saferead.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("deepeval: read %s: %w", path, err)
	}

	var raw struct {
		TestFile  string `json:"testFile"`
		TestCases []struct {
			Name            string `json:"name"`
			Input           string `json:"input"`
			ActualOutput    string `json:"actualOutput"`
			Success         bool   `json:"success"`
			MetricsMetadata []struct {
				Metric    string  `json:"metric"`
				Score     float64 `json:"score"`
				Threshold float64 `json:"threshold"`
				Reason    string  `json:"reason"`
				Success   bool    `json:"success"`
			} `json:"metricsMetadata"`
		} `json:"testCases"`
	}

	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("deepeval: parse %s: %w", path, err)
	}

	run := &EvalRun{
		Framework: FrameworkDeepeval,
		Source:    path,
	}

	var primarySum float64
	var primaryCount int

	for i, tc := range raw.TestCases {
		id := tc.Name
		if id == "" {
			id = fmt.Sprintf("%s-case-%d", filepath.Base(path), i)
		}

		metrics := map[string]float64{}
		var primaryScore float64
		var primaryThreshold float64
		var failureReasons []string
		havePrimary := false

		for j, m := range tc.MetricsMetadata {
			metrics[m.Metric] = m.Score
			if !havePrimary {
				primaryScore = m.Score
				primaryThreshold = m.Threshold
				havePrimary = true
			}
			if !m.Success && m.Reason != "" {
				if j == 0 || len(failureReasons) < 3 {
					failureReasons = append(failureReasons, m.Metric+": "+m.Reason)
				}
			}
		}

		// If deepeval didn't record a top-level success, derive it from
		// whether every metric succeeded.
		success := tc.Success
		if len(tc.MetricsMetadata) > 0 && !tc.Success {
			allOK := true
			for _, m := range tc.MetricsMetadata {
				if !m.Success {
					allOK = false
					break
				}
			}
			if allOK {
				success = true
			}
		}

		reason := ""
		if !success {
			reason = strings.Join(failureReasons, "; ")
		}

		run.Cases = append(run.Cases, EvalCaseResult{
			ID:        id,
			Name:      id,
			Success:   success,
			Score:     primaryScore,
			Metrics:   metrics,
			Reason:    reason,
			Threshold: primaryThreshold,
		})

		if havePrimary && !math.IsNaN(primaryScore) {
			primarySum += primaryScore
			primaryCount++
		}
	}

	run.Stats.Total = len(raw.TestCases)
	for _, c := range run.Cases {
		if c.Success {
			run.Stats.Successes++
		} else {
			run.Stats.Failures++
		}
	}
	if primaryCount == len(run.Cases) && primaryCount > 0 {
		run.Stats.PrimaryMetric = primarySum / float64(primaryCount)
		run.Stats.HasPrimaryMetric = true
	}

	return run, nil
}

var _ Adapter = DeepevalAdapter{}
