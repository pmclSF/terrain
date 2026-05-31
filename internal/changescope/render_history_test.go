package changescope

import (
	"bytes"
	"strings"
	"testing"
)

// fakeHistoryStore lets the test name exactly which (ruleID, file)
// pairs should be reported as demote-worthy. Anything not in the
// map returns false — mirrors the empty-store first-run behavior of
// findinghistory.Store.
type fakeHistoryStore struct {
	demote map[string]bool
}

func (f *fakeHistoryStore) ShouldDemote(ruleID, file string) bool {
	if f == nil || f.demote == nil {
		return false
	}
	return f.demote[ruleID+"|"+file]
}

// TestRenderPRSummaryMarkdownWithHistory_DemotesChronicFiring is the
// acceptance test for the demote path. When a (rule, file) pair is
// reported as demote-worthy by the history store, the renderer must
// drop its tier from BLOCK/GATE down to WATCH (observability),
// regardless of what the manifest says about the detector's tier.
//
// This is a renderer-only test — it doesn't exercise the on-disk
// store, only the contract that RenderPRSummaryMarkdownWithHistory
// consults the store when given one and renders the demoted label.
func TestRenderPRSummaryMarkdownWithHistory_DemotesChronicFiring(t *testing.T) {
	t.Parallel()

	pr := &PRAnalysis{
		PostureBand: "moderate",
		NewFindings: []ChangeScopedFinding{
			{
				Type:        "existing_signal",
				SignalType:  "untestedExport",
				Scope:       "direct",
				Path:        "src/agent/prompt.ts",
				Severity:    "high",
				Explanation: "raw detector text",
			},
		},
	}

	// First render: no history store → finding renders at the
	// manifest tier (gate-tier detector + high severity = [GATE]).
	var baseline bytes.Buffer
	RenderPRSummaryMarkdownWithHistory(&baseline, pr, nil)
	baseOut := baseline.String()
	if !strings.Contains(baseOut, "**`src/agent/prompt.ts`** [GATE]") {
		t.Fatalf("baseline (nil store): expected [GATE] for high-severity gate detector; got:\n%s", baseOut)
	}

	// Second render: store reports demote → label flips to [WATCH].
	store := &fakeHistoryStore{
		demote: map[string]bool{
			"untestedExport|src/agent/prompt.ts": true,
		},
	}
	var demoted bytes.Buffer
	RenderPRSummaryMarkdownWithHistory(&demoted, pr, store)
	demotedOut := demoted.String()
	if !strings.Contains(demotedOut, "**`src/agent/prompt.ts`** [WATCH]") {
		t.Errorf("demoted render: expected [WATCH] (observability tier); got:\n%s", demotedOut)
	}
	if strings.Contains(demotedOut, "**`src/agent/prompt.ts`** [GATE]") {
		t.Errorf("demoted render: should NOT contain [GATE] label; got:\n%s", demotedOut)
	}
}

// TestRenderPRSummaryMarkdownWithHistory_NilStoreActsLikeBaseRenderer
// proves that passing a nil store produces output identical to the
// non-history renderer — the demotion path doesn't accidentally affect
// callers that haven't loaded a store yet (or that explicitly opt out).
func TestRenderPRSummaryMarkdownWithHistory_NilStoreActsLikeBaseRenderer(t *testing.T) {
	t.Parallel()

	pr := &PRAnalysis{
		PostureBand: "moderate",
		NewFindings: []ChangeScopedFinding{
			{
				Type:        "existing_signal",
				SignalType:  "untestedExport",
				Scope:       "direct",
				Path:        "src/agent/prompt.ts",
				Severity:    "high",
				Explanation: "raw detector text",
			},
		},
	}

	var withHist, withoutHist bytes.Buffer
	RenderPRSummaryMarkdownWithHistory(&withHist, pr, nil)
	RenderPRSummaryMarkdown(&withoutHist, pr)

	if withHist.String() != withoutHist.String() {
		t.Errorf("nil-store render diverged from base renderer:\n--- with nil hist ---\n%s\n--- base ---\n%s",
			withHist.String(), withoutHist.String())
	}
}
