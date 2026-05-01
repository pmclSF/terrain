package models

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestSignalV2_RoundTrip exercises every SignalV2 field through marshal +
// unmarshal so accidental tag changes get caught.
func TestSignalV2_RoundTrip(t *testing.T) {
	t.Parallel()

	original := Signal{
		Type:        "weakAssertion",
		Category:    CategoryQuality,
		Severity:    SeverityMedium,
		Confidence:  0.84,
		Location:    SignalLocation{File: "src/auth/login.test.js", Line: 42},
		Explanation: "uses toBeTruthy where toEqual would be more specific",

		SeverityClauses: []string{"sev-clause-005", "sev-clause-018"},
		ConfidenceDetail: &ConfidenceDetail{
			Value:        0.84,
			IntervalLow:  0.78,
			IntervalHigh: 0.89,
			Quality:      "calibrated",
			Sources:      []EvidenceSource{SourceAST, SourceCoverage},
		},
		Actionability:   ActionabilityScheduled,
		LifecycleStages: []LifecycleStage{StageTestAuthoring, StageMaintenance},
		AIRelevance:     AIRelevanceNone,
		RuleID:          "TER-QUALITY-005",
		RuleURI:         "docs/rules/quality/weak-assertion.md",
		DetectorVersion: "v0.2.0",
		RelatedSignals: []SignalReference{
			{Type: "untestedExport", Relationship: "corroborates"},
		},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded Signal
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.Type != original.Type {
		t.Errorf("type: got %q want %q", decoded.Type, original.Type)
	}
	if len(decoded.SeverityClauses) != 2 {
		t.Errorf("severityClauses: got %d want 2", len(decoded.SeverityClauses))
	}
	if decoded.ConfidenceDetail == nil {
		t.Fatal("confidenceDetail dropped during round-trip")
	}
	if decoded.ConfidenceDetail.Quality != "calibrated" {
		t.Errorf("confidenceDetail.quality: got %q", decoded.ConfidenceDetail.Quality)
	}
	if decoded.Actionability != ActionabilityScheduled {
		t.Errorf("actionability: got %q", decoded.Actionability)
	}
	if len(decoded.LifecycleStages) != 2 {
		t.Errorf("lifecycleStages: got %d want 2", len(decoded.LifecycleStages))
	}
	if decoded.AIRelevance != AIRelevanceNone {
		t.Errorf("aiRelevance: got %q", decoded.AIRelevance)
	}
	if decoded.RuleID != "TER-QUALITY-005" {
		t.Errorf("ruleId: got %q", decoded.RuleID)
	}
	if decoded.DetectorVersion != "v0.2.0" {
		t.Errorf("detectorVersion: got %q", decoded.DetectorVersion)
	}
	if len(decoded.RelatedSignals) != 1 || decoded.RelatedSignals[0].Type != "untestedExport" {
		t.Errorf("relatedSignals: got %+v", decoded.RelatedSignals)
	}
}

// TestSignalV2_OmitsEmptyV2Fields makes sure a v1-shaped Signal serialises
// without any of the new field names appearing in the JSON, so downstream
// consumers don't see noise from omittable defaults.
func TestSignalV2_OmitsEmptyV2Fields(t *testing.T) {
	t.Parallel()

	v1 := Signal{
		Type:        "flakyTest",
		Category:    CategoryHealth,
		Severity:    SeverityHigh,
		Confidence:  0.9,
		Location:    SignalLocation{File: "test/login.test.ts"},
		Explanation: "intermittent failure",
	}
	data, err := json.Marshal(v1)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(data)
	for _, key := range []string{
		"severityClauses", "confidenceDetail", "actionability",
		"lifecycleStages", "aiRelevance", "ruleId", "ruleUri",
		"detectorVersion", "relatedSignals",
	} {
		if strings.Contains(s, "\""+key+"\"") {
			t.Errorf("v1-shaped Signal leaked v2 field %q in JSON: %s", key, s)
		}
	}
}

// TestSignalV2_ForwardCompat_V1ReaderReadsV2 demonstrates the migration
// shim contract: a v1 reader (one that doesn't know the new fields)
// successfully decodes a v2 payload, ignoring unknown fields.
func TestSignalV2_ForwardCompat_V1ReaderReadsV2(t *testing.T) {
	t.Parallel()

	// "v1-shaped" decoder: only the fields v1 knew about.
	type v1Signal struct {
		Type            SignalType       `json:"type"`
		Category        SignalCategory   `json:"category"`
		Severity        SignalSeverity   `json:"severity"`
		Confidence      float64          `json:"confidence,omitempty"`
		EvidenceSource  EvidenceSource   `json:"evidenceSource,omitempty"`
		Location        SignalLocation   `json:"location"`
		Explanation     string           `json:"explanation"`
		SuggestedAction string           `json:"suggestedAction,omitempty"`
		Metadata        map[string]any   `json:"metadata,omitempty"`
		_               EvidenceStrength // referenced for the import only
	}

	v2 := Signal{
		Type:            "weakAssertion",
		Category:        CategoryQuality,
		Severity:        SeverityMedium,
		Confidence:      0.84,
		Location:        SignalLocation{File: "src/auth.test.js"},
		Explanation:     "fixed",
		SeverityClauses: []string{"sev-clause-005"},
		ConfidenceDetail: &ConfidenceDetail{
			Value: 0.84, IntervalLow: 0.78, IntervalHigh: 0.89,
		},
		RuleID: "TER-QUALITY-005",
	}
	payload, err := json.Marshal(v2)
	if err != nil {
		t.Fatalf("marshal v2: %v", err)
	}

	var v1Decoded v1Signal
	if err := json.Unmarshal(payload, &v1Decoded); err != nil {
		t.Fatalf("v1 reader rejected v2 payload: %v", err)
	}
	if v1Decoded.Confidence != 0.84 {
		t.Errorf("v1 reader saw confidence %v, want 0.84", v1Decoded.Confidence)
	}
}

// TestSignalV2_BackwardCompat_V2ReaderReadsV1 confirms that adding the new
// fields didn't break decoding of historical (pre-0.2) signals.
func TestSignalV2_BackwardCompat_V2ReaderReadsV1(t *testing.T) {
	t.Parallel()

	v1Payload := []byte(`{
		"type": "skippedTest",
		"category": "health",
		"severity": "low",
		"confidence": 0.95,
		"location": {"file": "test/auth.test.js"},
		"explanation": "test.skip without ticket",
		"metadata": {"ticket": ""}
	}`)

	var sig Signal
	if err := json.Unmarshal(v1Payload, &sig); err != nil {
		t.Fatalf("v2 reader rejected v1 payload: %v", err)
	}
	if sig.Type != "skippedTest" || sig.Severity != SeverityLow {
		t.Errorf("v1 payload mis-decoded: %+v", sig)
	}
	// All v2 fields should be at zero values.
	if sig.ConfidenceDetail != nil {
		t.Errorf("expected nil confidenceDetail on v1 payload, got %+v", sig.ConfidenceDetail)
	}
	if sig.Actionability != "" || sig.AIRelevance != "" || sig.RuleID != "" {
		t.Errorf("expected v2 fields empty on v1 payload, got actionability=%q ai=%q rule=%q",
			sig.Actionability, sig.AIRelevance, sig.RuleID)
	}
}
