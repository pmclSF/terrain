package evaladapter

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
)

// PromptfooAdapter ingests `promptfoo eval --output results.json`
// artifacts. Promptfoo's schema has stabilized at version 3 (the
// header `results.version` field); the adapter accepts that version
// and surfaces a warning-less degraded path for older schemas.
type PromptfooAdapter struct{}

// Name implements Adapter.
func (PromptfooAdapter) Name() Framework { return FrameworkPromptfoo }

// CanIngest implements Adapter. Promptfoo writes results.json or
// <name>.json with a top-level `results` object containing version,
// timestamp, and an inner `results` array.
func (PromptfooAdapter) CanIngest(path string) bool {
	if !strings.HasSuffix(strings.ToLower(path), ".json") {
		return false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	// Promptfoo's outer shape (a tiny prefix is enough to disambiguate
	// against other frameworks' JSONs — we deliberately don't full-parse
	// here).
	var head struct {
		Results struct {
			Version int             `json:"version"`
			Results json.RawMessage `json:"results"`
		} `json:"results"`
	}
	if err := json.Unmarshal(data, &head); err != nil {
		return false
	}
	return head.Results.Version >= 2 && len(head.Results.Results) > 0
}

// Ingest implements Adapter.
func (PromptfooAdapter) Ingest(path string) (*EvalRun, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("promptfoo: read %s: %w", path, err)
	}

	var raw struct {
		EvalID  string `json:"evalId"`
		Results struct {
			Version   int    `json:"version"`
			Timestamp string `json:"timestamp"`
			Results   []struct {
				ID            string         `json:"id"`
				PromptIdx     int            `json:"promptIdx"`
				Success       bool           `json:"success"`
				Score         float64        `json:"score"`
				NamedScores   map[string]float64 `json:"namedScores"`
				TestCase      struct {
					Description string                 `json:"description"`
					Vars        map[string]interface{} `json:"vars"`
					Threshold   float64                `json:"threshold"`
				} `json:"testCase"`
				GradingResult struct {
					Pass    bool    `json:"pass"`
					Score   float64 `json:"score"`
					Reason  string  `json:"reason"`
				} `json:"gradingResult"`
			} `json:"results"`
			Stats struct {
				Successes int `json:"successes"`
				Failures  int `json:"failures"`
			} `json:"stats"`
		} `json:"results"`
	}

	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("promptfoo: parse %s: %w", path, err)
	}

	run := &EvalRun{
		Framework: FrameworkPromptfoo,
		Source:    path,
		Timestamp: raw.Results.Timestamp,
	}

	var scoreSum float64
	var scoreCount int

	for i, r := range raw.Results.Results {
		// Prefer an explicit case ID; fall back to test description; fall
		// back to "promptfoo-case-<index>". Promptfoo doesn't require a
		// unique ID per case but adopters who care about run-to-run diff
		// usually set `description` or `id`.
		id := r.ID
		name := r.TestCase.Description
		if name == "" {
			name = id
		}
		if id == "" {
			id = name
		}
		if id == "" {
			id = fmt.Sprintf("%s-case-%d", filepath.Base(path), i)
			name = id
		}

		// Promptfoo's gradingResult.reason carries the failure explanation
		// when a case fails its assertion. We surface it for failed cases.
		reason := ""
		if !r.Success {
			reason = r.GradingResult.Reason
		}

		run.Cases = append(run.Cases, EvalCaseResult{
			ID:        id,
			Name:      name,
			Success:   r.Success,
			Score:     r.Score,
			Metrics:   r.NamedScores,
			Reason:    reason,
			Threshold: r.TestCase.Threshold,
		})

		if !math.IsNaN(r.Score) {
			scoreSum += r.Score
			scoreCount++
		}
	}

	run.Stats = EvalRunStats{
		Total:     len(raw.Results.Results),
		Successes: raw.Results.Stats.Successes,
		Failures:  raw.Results.Stats.Failures,
	}
	if scoreCount == len(run.Cases) && scoreCount > 0 {
		run.Stats.PrimaryMetric = scoreSum / float64(scoreCount)
	}

	return run, nil
}

// Compile-time check that PromptfooAdapter implements Adapter.
var _ Adapter = PromptfooAdapter{}
