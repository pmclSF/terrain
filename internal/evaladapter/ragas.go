package evaladapter

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
)

// RagasAdapter ingests ragas evaluation output. ragas doesn't ship a
// canonical results-file format — it produces an `EvaluationResult`
// object that adopters typically serialize via
// `result.to_pandas().to_json(orient='records')`. That produces an
// array of records each carrying the input fields (question, answer,
// contexts) and the per-metric scores (faithfulness, answer_relevancy,
// context_precision, context_recall, etc.).
//
// To support both that array-of-records shape and the older object
// shape (when present), the adapter accepts:
//
//  1. A top-level array: [{question, answer, faithfulness, ...}, ...]
//  2. A wrapper object: {"results": [...], "metrics": {...}}
//
// Either form is parsed by attempting both shapes during Ingest.
type RagasAdapter struct{}

// ragasMetricNames is the canonical metric set ragas emits. Used by
// CanIngest to recognize the format (any one of these appearing as a
// key on the first record indicates ragas) and by Ingest to map record
// fields into the Metrics map.
var ragasMetricNames = []string{
	"faithfulness",
	"answer_relevancy",
	"context_precision",
	"context_recall",
	"context_entity_recall",
	"answer_correctness",
	"answer_similarity",
	"harmfulness",
	"maliciousness",
	"coherence",
	"conciseness",
}

// Name implements Adapter.
func (RagasAdapter) Name() Framework { return FrameworkRagas }

// CanIngest implements Adapter. A file is treated as ragas output when
// it parses as JSON, the first record is an object, and that record
// has at least one canonical ragas metric key.
func (RagasAdapter) CanIngest(path string) bool {
	if !strings.HasSuffix(strings.ToLower(path), ".json") {
		return false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	// Probe both shapes.
	if rec, ok := firstRagasRecord(data); ok && hasRagasMetric(rec) {
		return true
	}
	return false
}

// firstRagasRecord returns the first record from either the array-of-
// records shape or the wrapper-object shape.
func firstRagasRecord(data []byte) (map[string]interface{}, bool) {
	// Try array-of-records.
	var arr []map[string]interface{}
	if err := json.Unmarshal(data, &arr); err == nil && len(arr) > 0 {
		return arr[0], true
	}
	// Try wrapper-object.
	var wrap struct {
		Results []map[string]interface{} `json:"results"`
	}
	if err := json.Unmarshal(data, &wrap); err == nil && len(wrap.Results) > 0 {
		return wrap.Results[0], true
	}
	return nil, false
}

// hasRagasMetric returns true when the record carries any canonical
// ragas metric key.
func hasRagasMetric(rec map[string]interface{}) bool {
	for _, m := range ragasMetricNames {
		if _, ok := rec[m]; ok {
			return true
		}
	}
	return false
}

// Ingest implements Adapter.
func (RagasAdapter) Ingest(path string) (*EvalRun, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("ragas: read %s: %w", path, err)
	}

	records, err := loadRagasRecords(data)
	if err != nil {
		return nil, fmt.Errorf("ragas: parse %s: %w", path, err)
	}

	run := &EvalRun{
		Framework: FrameworkRagas,
		Source:    path,
	}

	var primarySum float64
	var primaryCount int

	for i, rec := range records {
		// ragas doesn't record per-case IDs. Synthesize from the
		// question text (truncated, slugified) or fall back to index.
		id := ""
		if q, ok := rec["question"].(string); ok && q != "" {
			id = slugifyRagasQuestion(q, 60)
		}
		if id == "" {
			id = fmt.Sprintf("%s-case-%d", filepath.Base(path), i)
		}

		metrics := map[string]float64{}
		for _, name := range ragasMetricNames {
			if v, ok := numericValue(rec[name]); ok {
				metrics[name] = v
			}
		}

		// Primary metric: faithfulness if present (the most commonly
		// blocked metric in RAG); otherwise the first metric ragas
		// recorded for this case in declaration order.
		var primary float64
		var primaryName string
		if v, ok := metrics["faithfulness"]; ok {
			primary = v
			primaryName = "faithfulness"
		} else {
			for _, name := range ragasMetricNames {
				if v, ok := metrics[name]; ok {
					primary = v
					primaryName = name
					break
				}
			}
		}
		_ = primaryName // reserved for future per-rule annotation

		// ragas doesn't carry per-case pass/fail — that decision is made
		// at the adopter's threshold-evaluation step. We surface
		// Success=true and let the regression rule compare scores.
		// Future: if the record has a `passed` field (some adopter
		// pipelines add one) honor it.
		success := true
		if p, ok := rec["passed"].(bool); ok {
			success = p
		}

		run.Cases = append(run.Cases, EvalCaseResult{
			ID:      id,
			Name:    id,
			Success: success,
			Score:   primary,
			Metrics: metrics,
		})

		if !math.IsNaN(primary) {
			primarySum += primary
			primaryCount++
		}
	}

	run.Stats.Total = len(records)
	for _, c := range run.Cases {
		if c.Success {
			run.Stats.Successes++
		} else {
			run.Stats.Failures++
		}
	}
	if primaryCount == len(run.Cases) && primaryCount > 0 {
		run.Stats.PrimaryMetric = primarySum / float64(primaryCount)
	}

	return run, nil
}

func loadRagasRecords(data []byte) ([]map[string]interface{}, error) {
	var arr []map[string]interface{}
	if err := json.Unmarshal(data, &arr); err == nil {
		return arr, nil
	}
	var wrap struct {
		Results []map[string]interface{} `json:"results"`
	}
	if err := json.Unmarshal(data, &wrap); err == nil {
		return wrap.Results, nil
	}
	return nil, fmt.Errorf("not a recognized ragas results JSON")
}

// numericValue coerces a JSON-decoded value to float64 when it's
// numeric, or returns false. Handles the float64 path that
// encoding/json produces for numbers.
func numericValue(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	}
	return 0, false
}

// slugifyRagasQuestion produces a short, deterministic case ID from
// the question text. Lowercase, alphanumerics and dashes only,
// truncated to maxLen.
func slugifyRagasQuestion(q string, maxLen int) string {
	q = strings.ToLower(strings.TrimSpace(q))
	var b strings.Builder
	prevDash := false
	for _, r := range q {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
			prevDash = false
		case b.Len() == 0:
			// drop leading non-alphanumerics
		case !prevDash:
			b.WriteRune('-')
			prevDash = true
		}
		if b.Len() >= maxLen {
			break
		}
	}
	return strings.TrimRight(b.String(), "-")
}

var _ Adapter = RagasAdapter{}
