package checkruns

import (
	"testing"

	"github.com/pmclSF/terrain/internal/models"
	"github.com/pmclSF/terrain/internal/signals"
)

// fakeHistoryStore is the test double for findinghistory.Store.
type fakeHistoryStore struct {
	demote map[string]bool
}

func (f *fakeHistoryStore) ShouldDemote(ruleID, file string) bool {
	if f == nil || f.demote == nil {
		return false
	}
	return f.demote[ruleID+"|"+file]
}

// TestBuildBundleWithHistory_DemoteRoutesToObservability is the
// integration acceptance test for the cross-surface consistency fix.
// A gate-tier signal that the history store marks ShouldDemote MUST:
//
//   - move out of the gate check (so the required check doesn't fail
//     on a finding the PR comment renders as [WATCH]),
//   - appear in the observability check instead,
//   - not influence the gate conclusion.
//
// Without this, the PR comment and the required check disagree on
// whether the same finding blocks the merge.
func TestBuildBundleWithHistory_DemoteRoutesToObservability(t *testing.T) {
	t.Parallel()

	const ruleID = string(signals.SignalUntestedExport)
	const path = "src/auth/login.ts"

	snap := &models.TestSuiteSnapshot{
		Signals: []models.Signal{
			{
				Type:     models.SignalType(ruleID),
				Severity: models.SeverityHigh,
				Location: models.SignalLocation{File: path},
			},
		},
	}

	// Baseline: no history → finding is gate-tier, gate concludes
	// failure.
	noHist := BuildBundleWithHistory(snap, "deadbeef", nil)
	if noHist.Gate.Conclusion != "failure" {
		t.Errorf("baseline: expected gate conclusion=failure (gate-tier High); got %q", noHist.Gate.Conclusion)
	}

	// With demote: finding routes to observability; gate concludes
	// success (empty).
	demoted := BuildBundleWithHistory(snap, "deadbeef", &fakeHistoryStore{
		demote: map[string]bool{ruleID + "|" + path: true},
	})
	if demoted.Gate.Conclusion != "success" {
		t.Errorf("with demote: gate should conclude success (no remaining gate-tier findings); got %q", demoted.Gate.Conclusion)
	}
	// Observability check must NOT block the merge regardless.
	if demoted.Observability.Conclusion != "neutral" {
		t.Errorf("observability conclusion = %q, want neutral", demoted.Observability.Conclusion)
	}
}

// TestBuildBundleWithHistory_EmptyTypeOrFileIgnoresHistory guards a
// corruption case: signals without a usable (Type, File) cannot
// match a history entry (the store's key requires both). The bundler
// must still route them by manifest tier rather than treating empty
// as a wildcard demote.
func TestBuildBundleWithHistory_EmptyTypeOrFileIgnoresHistory(t *testing.T) {
	t.Parallel()

	store := &fakeHistoryStore{
		demote: map[string]bool{"|": true, "x|": true, "|y": true},
	}

	snap := &models.TestSuiteSnapshot{
		Signals: []models.Signal{
			{Type: "", Severity: models.SeverityHigh, Location: models.SignalLocation{File: "src/x.ts"}},
			{Type: models.SignalType(signals.SignalUntestedExport), Severity: models.SeverityHigh, Location: models.SignalLocation{}},
		},
	}
	bundle := BuildBundleWithHistory(snap, "deadbeef", store)

	// Both signals stay gate-side: history can't have matched a (empty,
	// file) or (rule, empty) entry without the missing half.
	if bundle.Gate.Conclusion != "failure" {
		t.Errorf("expected gate=failure (manifest tier still rules); got %q", bundle.Gate.Conclusion)
	}
}

// TestBuildBundle_NilStoreBehavesLikePriorAPI proves the no-store
// form is the legacy contract. Adopters that haven't yet wired the
// history store should see identical output before vs after this
// PR (no regression in the no-LLM-no-state default flow).
func TestBuildBundle_NilStoreBehavesLikePriorAPI(t *testing.T) {
	t.Parallel()

	snap := &models.TestSuiteSnapshot{
		Signals: []models.Signal{
			{
				Type:     models.SignalType(signals.SignalUntestedExport),
				Severity: models.SeverityHigh,
				Location: models.SignalLocation{File: "src/x.ts"},
			},
		},
	}
	noArg := BuildBundle(snap, "sha")
	nilHist := BuildBundleWithHistory(snap, "sha", nil)

	if noArg.Gate.Conclusion != nilHist.Gate.Conclusion {
		t.Errorf("BuildBundle vs BuildBundleWithHistory(nil): gate conclusion diverged (%q vs %q)",
			noArg.Gate.Conclusion, nilHist.Gate.Conclusion)
	}
	if noArg.Observability.Conclusion != nilHist.Observability.Conclusion {
		t.Errorf("BuildBundle vs BuildBundleWithHistory(nil): observability conclusion diverged (%q vs %q)",
			noArg.Observability.Conclusion, nilHist.Observability.Conclusion)
	}
}
