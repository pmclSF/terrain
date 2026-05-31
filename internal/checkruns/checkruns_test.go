package checkruns

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/pmclSF/terrain/internal/models"
	"github.com/pmclSF/terrain/internal/signals"
)

// TestBuildBundle_EmptySnapshot returns clean empty check runs that
// the API still accepts (both pass / neutral).
func TestBuildBundle_EmptySnapshot(t *testing.T) {
	b := BuildBundle(&models.TestSuiteSnapshot{}, "abc123")
	if b.Gate.Name != "terrain (gate)" {
		t.Errorf("gate name: %q", b.Gate.Name)
	}
	if b.Observability.Name != "terrain (observability)" {
		t.Errorf("observability name: %q", b.Observability.Name)
	}
	if b.Gate.Conclusion != "success" {
		t.Errorf("empty gate should be success, got %q", b.Gate.Conclusion)
	}
	if b.Observability.Conclusion != "neutral" {
		t.Errorf("observability is always neutral, got %q", b.Observability.Conclusion)
	}
	if b.Gate.HeadSHA != "abc123" {
		t.Errorf("head_sha not propagated")
	}
}

// TestBuildBundle_NilSnapshot is no-op safe.
func TestBuildBundle_NilSnapshot(t *testing.T) {
	b := BuildBundle(nil, "abc123")
	// Both check runs are zero-value but the function doesn't panic.
	if b.Gate.HeadSHA != "" {
		t.Errorf("nil snapshot should produce zero-value bundle")
	}
}

// TestBuildBundle_GateFinding_FailsConclusion: a Medium-or-above
// gate-tier finding produces conclusion="failure".
func TestBuildBundle_GateFinding_FailsConclusion(t *testing.T) {
	snap := &models.TestSuiteSnapshot{
		Signals: []models.Signal{
			{
				Type:     signals.SignalUntestedExport, // TierGate per manifest
				Severity: models.SeverityHigh,
				Location: models.SignalLocation{File: "src/exported.go", Line: 42},
				Explanation: "exported function has no covering test",
			},
		},
	}
	b := BuildBundle(snap, "head1")
	if b.Gate.Conclusion != "failure" {
		t.Errorf("high gate severity should fail; got %q", b.Gate.Conclusion)
	}
	if len(b.Gate.Output.Annotations) != 1 {
		t.Errorf("expected 1 annotation, got %d", len(b.Gate.Output.Annotations))
	}
	if b.Gate.Output.Annotations[0].AnnotationLevel != "failure" {
		t.Errorf("annotation level for high: %q, want failure", b.Gate.Output.Annotations[0].AnnotationLevel)
	}
}

// TestBuildBundle_ObservabilityFinding_NeutralOnly: observability-tier
// findings never produce a "failure" — even at declared High severity
// (which capSeverity would have demoted earlier in the pipeline).
func TestBuildBundle_ObservabilityFinding_NeutralOnly(t *testing.T) {
	snap := &models.TestSuiteSnapshot{
		Signals: []models.Signal{
			{
				Type:     signals.SignalMockHeavyTest, // TierObservability
				Severity: models.SeverityMedium,
				Location: models.SignalLocation{File: "tests/mocks_test.go", Line: 10},
				Explanation: "test relies on mocks heavily",
			},
		},
	}
	b := BuildBundle(snap, "head1")
	if b.Gate.Conclusion != "success" {
		t.Errorf("no gate findings should pass; got %q", b.Gate.Conclusion)
	}
	if b.Observability.Conclusion != "neutral" {
		t.Errorf("observability conclusion: %q, want neutral", b.Observability.Conclusion)
	}
	if len(b.Observability.Output.Annotations) != 1 {
		t.Errorf("expected 1 observability annotation, got %d", len(b.Observability.Output.Annotations))
	}
	if b.Observability.Output.Annotations[0].AnnotationLevel != "notice" {
		t.Errorf("observability annotation level: %q, want notice", b.Observability.Output.Annotations[0].AnnotationLevel)
	}
}

// TestBuildBundle_PerDetectorCollapse: 5 co-firing instances of the
// same detector produce ONE annotation with a "+4 more" mention in
// the message, not 5 annotations.
func TestBuildBundle_PerDetectorCollapse(t *testing.T) {
	snap := &models.TestSuiteSnapshot{}
	for i := 0; i < 5; i++ {
		snap.Signals = append(snap.Signals, models.Signal{
			Type:     signals.SignalUntestedExport,
			Severity: models.SeverityMedium,
			Location: models.SignalLocation{File: "src/file.go", Line: 10 + i},
			Explanation: "exported function has no covering test",
		})
	}
	b := BuildBundle(snap, "head1")
	if len(b.Gate.Output.Annotations) != 1 {
		t.Errorf("expected 1 collapsed annotation, got %d", len(b.Gate.Output.Annotations))
	}
	if !strings.Contains(b.Gate.Output.Annotations[0].Message, "+4 more") {
		t.Errorf("expected +4 more in collapsed message; got %q", b.Gate.Output.Annotations[0].Message)
	}
	if !strings.Contains(b.Gate.Output.Text, "co-firing in this detector") {
		t.Errorf("text should mention co-firing collapse; got %q", b.Gate.Output.Text)
	}
}

// TestBuildBundle_GateAndObservabilityCoexist: a PR with both kinds
// of findings produces a bundle where each check run only carries its
// respective findings.
func TestBuildBundle_GateAndObservabilityCoexist(t *testing.T) {
	snap := &models.TestSuiteSnapshot{
		Signals: []models.Signal{
			{Type: signals.SignalUntestedExport, Severity: models.SeverityHigh,
				Location: models.SignalLocation{File: "src/a.go", Line: 1}},
			{Type: signals.SignalMockHeavyTest, Severity: models.SeverityMedium,
				Location: models.SignalLocation{File: "tests/b.go", Line: 1}},
		},
	}
	b := BuildBundle(snap, "head1")
	if b.Gate.Conclusion != "failure" {
		t.Errorf("gate should fail (high finding); got %q", b.Gate.Conclusion)
	}
	if len(b.Gate.Output.Annotations) != 1 {
		t.Errorf("gate should have 1 annotation; got %d", len(b.Gate.Output.Annotations))
	}
	if len(b.Observability.Output.Annotations) != 1 {
		t.Errorf("observability should have 1 annotation; got %d", len(b.Observability.Output.Annotations))
	}
	if b.Gate.Output.Annotations[0].Path != "src/a.go" {
		t.Errorf("gate annotation wrong file")
	}
	if b.Observability.Output.Annotations[0].Path != "tests/b.go" {
		t.Errorf("observability annotation wrong file")
	}
}

// TestBuildBundle_JSONMarshalRoundtrip confirms the API JSON shape
// round-trips cleanly and produces parseable output for `gh api`.
func TestBuildBundle_JSONMarshalRoundtrip(t *testing.T) {
	snap := &models.TestSuiteSnapshot{
		Signals: []models.Signal{
			{Type: signals.SignalUntestedExport, Severity: models.SeverityHigh,
				Location: models.SignalLocation{File: "src/a.go", Line: 42}},
		},
	}
	b := BuildBundle(snap, "abc123")
	data, err := json.Marshal(b)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var roundtrip CheckRunsBundle
	if err := json.Unmarshal(data, &roundtrip); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if roundtrip.Gate.HeadSHA != "abc123" {
		t.Errorf("head_sha did not roundtrip")
	}
	if roundtrip.Gate.Name != "terrain (gate)" {
		t.Errorf("name did not roundtrip")
	}
	// Required Checks-API fields present.
	if roundtrip.Gate.Status != "completed" {
		t.Errorf("status: %q", roundtrip.Gate.Status)
	}
	if len(roundtrip.Gate.Output.Annotations) != 1 {
		t.Errorf("annotations didn't roundtrip")
	}
}

// TestSeverityCounts_PRLabels confirms BLOCK / GATE / NOTE
// bucketization matches the uitokens.PRLabel contract.
func TestSeverityCounts_PRLabels(t *testing.T) {
	in := []models.Signal{
		{Type: signals.SignalUntestedExport, Severity: models.SeverityCritical},
		{Type: signals.SignalUntestedExport, Severity: models.SeverityHigh},
		{Type: signals.SignalUntestedExport, Severity: models.SeverityMedium},
		{Type: signals.SignalUntestedExport, Severity: models.SeverityLow},
		{Type: signals.SignalUntestedExport, Severity: models.SeverityInfo},
	}
	counts := severityCounts(in)
	if counts["BLOCK"] != 1 {
		t.Errorf("BLOCK count: %d, want 1", counts["BLOCK"])
	}
	if counts["GATE"] != 2 {
		t.Errorf("GATE count: %d, want 2 (high + medium)", counts["GATE"])
	}
	if counts["NOTE"] != 1 {
		t.Errorf("NOTE count: %d, want 1 (low)", counts["NOTE"])
	}
	// Info-tier drops from PR surface entirely.
	if counts["INFO"] != 0 {
		t.Errorf("info should drop; counts[INFO]=%d", counts["INFO"])
	}
}

// TestAnnotationLevel_Mapping pins severity -> annotation_level.
func TestAnnotationLevel_Mapping(t *testing.T) {
	cases := []struct{ severity, want string }{
		{"critical", "failure"},
		{"high", "failure"},
		{"medium", "warning"},
		{"low", "notice"},
		{"info", "notice"},
		{"unknown", "notice"},
	}
	for _, c := range cases {
		if got := annotationLevel(c.severity); got != c.want {
			t.Errorf("annotationLevel(%q) = %q, want %q", c.severity, got, c.want)
		}
	}
}
