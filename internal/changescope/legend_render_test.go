package changescope

import (
	"bytes"
	"strings"
	"testing"
)

// TestRenderPRSummaryMarkdown_SeverityLegendConditional proves the
// BLOCK/GATE/WATCH/NOTE legend renders in the PR comment when (and only
// when) there are findings to label — the "severity = disclosure" design
// rule: quiet on an all-clear comment, explanatory when it matters.
func TestRenderPRSummaryMarkdown_SeverityLegendConditional(t *testing.T) {
	t.Parallel()
	const legendMarker = "`BLOCK` fails the merge"

	// With a finding → the legend renders so a first-time reader can decode
	// the labels without leaving the comment.
	withFinding := &PRAnalysis{
		PostureBand: "moderate",
		NewFindings: []ChangeScopedFinding{{
			Type:        "existing_signal",
			SignalType:  "untestedExport",
			Scope:       "direct",
			Path:        "src/agent/prompt.ts",
			Severity:    "high",
			Explanation: "raw detector text",
		}},
	}
	var withBuf bytes.Buffer
	RenderPRSummaryMarkdown(&withBuf, withFinding)
	if !strings.Contains(withBuf.String(), legendMarker) {
		t.Errorf("expected severity legend when findings exist; got:\n%s", withBuf.String())
	}

	// All-clear (no findings) → the legend is suppressed (quiet by default).
	empty := &PRAnalysis{PostureBand: "strong"}
	var emptyBuf bytes.Buffer
	RenderPRSummaryMarkdown(&emptyBuf, empty)
	if strings.Contains(emptyBuf.String(), legendMarker) {
		t.Errorf("legend should be suppressed on an all-clear comment; got:\n%s", emptyBuf.String())
	}
}

// TestRenderPRSummaryMarkdown_SwissVerdict pins the Swiss verdict line: the PR
// comment leads with render.VerdictLine, counting new risks in changed code
// (directRisk) as the merge-blocking findings.
func TestRenderPRSummaryMarkdown_SwissVerdict(t *testing.T) {
	t.Parallel()
	// A new risk in changed code → "1 finding blocks this merge."
	blocking := &PRAnalysis{
		PostureBand: "moderate",
		NewFindings: []ChangeScopedFinding{{
			Type:        "new_signal",
			Scope:       "direct",
			Path:        "src/x.ts",
			Severity:    "high",
			Explanation: "new risk introduced",
		}},
	}
	var bbuf bytes.Buffer
	RenderPRSummaryMarkdown(&bbuf, blocking)
	if !strings.Contains(bbuf.String(), "1 finding blocks this merge") {
		t.Errorf("blocking PR: expected Swiss verdict line; got:\n%s", bbuf.String())
	}

	// An all-clear PR → "Clear — nothing blocks this merge."
	var cbuf bytes.Buffer
	RenderPRSummaryMarkdown(&cbuf, &PRAnalysis{PostureBand: "strong"})
	if !strings.Contains(cbuf.String(), "Clear — nothing blocks this merge") {
		t.Errorf("clear PR: expected clear verdict; got:\n%s", cbuf.String())
	}
}
