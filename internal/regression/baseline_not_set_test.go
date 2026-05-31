package regression

import (
	"testing"

	"github.com/pmclSF/terrain/internal/evaladapter"
)

func TestDetectBaselineNotSet_FiresWithoutBaseline(t *testing.T) {
	t.Parallel()
	current := &evaladapter.EvalRun{
		Source: "current.json",
		Cases:  []evaladapter.EvalCaseResult{{ID: "x", Score: 0.9}},
	}
	sigs := DetectBaselineNotSet(nil, current)
	if len(sigs) != 1 {
		t.Fatalf("expected 1 signal, got %d", len(sigs))
	}
}

func TestDetectBaselineNotSet_SuppressedWithBaseline(t *testing.T) {
	t.Parallel()
	baseline := &evaladapter.EvalRun{
		Cases: []evaladapter.EvalCaseResult{{ID: "x", Score: 0.9}},
	}
	current := &evaladapter.EvalRun{
		Cases: []evaladapter.EvalCaseResult{{ID: "x", Score: 0.9}},
	}
	sigs := DetectBaselineNotSet(baseline, current)
	if len(sigs) != 0 {
		t.Errorf("baseline present should suppress, got %+v", sigs)
	}
}

func TestDetectBaselineNotSet_NoCurrentNoSignal(t *testing.T) {
	t.Parallel()
	if got := DetectBaselineNotSet(nil, nil); len(got) != 0 {
		t.Errorf("nil current = no signal")
	}
	if got := DetectBaselineNotSet(nil, &evaladapter.EvalRun{}); len(got) != 0 {
		t.Errorf("empty current = no signal")
	}
}

func TestDetectPassRateDrop_Fires(t *testing.T) {
	t.Parallel()
	baseline := &evaladapter.EvalRun{Stats: evaladapter.EvalRunStats{Total: 10, Successes: 10}}
	current := &evaladapter.EvalRun{Stats: evaladapter.EvalRunStats{Total: 10, Successes: 7}}
	sigs := DetectPassRateDrop(baseline, current, DefaultPassRateDropConfig())
	if len(sigs) != 1 {
		t.Fatalf("expected 1 signal, got %d", len(sigs))
	}
	if sigs[0].Metadata["delta"].(float64) < 0.29 {
		t.Errorf("delta = %v, want ~0.3", sigs[0].Metadata["delta"])
	}
}

func TestDetectPassRateDrop_SmallDropSuppressed(t *testing.T) {
	t.Parallel()
	baseline := &evaladapter.EvalRun{Stats: evaladapter.EvalRunStats{Total: 100, Successes: 100}}
	current := &evaladapter.EvalRun{Stats: evaladapter.EvalRunStats{Total: 100, Successes: 98}}
	sigs := DetectPassRateDrop(baseline, current, DefaultPassRateDropConfig())
	if len(sigs) != 0 {
		t.Errorf("0.02 drop should be suppressed, got %+v", sigs)
	}
}

func TestDetectSnapshotMismatch_DivergedReason(t *testing.T) {
	t.Parallel()
	baseline := &evaladapter.EvalRun{
		Cases: []evaladapter.EvalCaseResult{
			{ID: "x", Name: "x", Reason: "refused successfully"},
		},
	}
	current := &evaladapter.EvalRun{
		Cases: []evaladapter.EvalCaseResult{
			{ID: "x", Name: "x", Reason: "responded instead of refusing"},
		},
	}
	sigs := DetectSnapshotMismatch(baseline, current)
	if len(sigs) != 1 {
		t.Fatalf("expected 1 mismatch signal, got %d", len(sigs))
	}
}

func TestDetectSnapshotMismatch_SameReasonSuppressed(t *testing.T) {
	t.Parallel()
	baseline := &evaladapter.EvalRun{
		Cases: []evaladapter.EvalCaseResult{{ID: "x", Reason: "same"}},
	}
	current := &evaladapter.EvalRun{
		Cases: []evaladapter.EvalCaseResult{{ID: "x", Reason: "same"}},
	}
	sigs := DetectSnapshotMismatch(baseline, current)
	if len(sigs) != 0 {
		t.Errorf("unchanged reason should not fire, got %+v", sigs)
	}
}

func TestDetectSnapshotMismatch_BothEmpty(t *testing.T) {
	t.Parallel()
	baseline := &evaladapter.EvalRun{
		Cases: []evaladapter.EvalCaseResult{{ID: "x", Reason: ""}},
	}
	current := &evaladapter.EvalRun{
		Cases: []evaladapter.EvalCaseResult{{ID: "x", Reason: ""}},
	}
	if got := DetectSnapshotMismatch(baseline, current); len(got) != 0 {
		t.Errorf("both empty should not fire, got %+v", got)
	}
}
