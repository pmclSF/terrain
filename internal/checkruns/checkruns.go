// Package checkruns produces the structured JSON for two GitHub
// Checks-API check runs:
//
//   - `terrain (gate)`         — required check; fails when an
//                                 undismissed gate-tier finding fires.
//   - `terrain (observability)` — informational; always neutral
//                                 conclusion.
//
// Terrain itself does NOT post to GitHub. Adopters' CI workflows
// shell out to `gh api` (or actions/github-script) to post each
// JSON to the Checks API. This keeps Terrain's no-network-calls
// contract intact: the binary writes JSON; the workflow handles
// auth and HTTP.
//
// Spec § P5.3 — "Two GitHub check runs: terrain (gate) (required,
// fails on undismissed gate finding) and terrain (observability)
// (informational only). Observability findings live in one
// collapsed <details> footer in the gate check's top-level summary."
package checkruns

import (
	"fmt"
	"sort"
	"strings"

	"github.com/pmclSF/terrain/internal/models"
	"github.com/pmclSF/terrain/internal/prtemplates"
	"github.com/pmclSF/terrain/internal/signals"
	"github.com/pmclSF/terrain/internal/uitokens"
)

// CheckRun mirrors the GitHub Checks-API "create check run" request
// shape (https://docs.github.com/en/rest/checks/runs).
//
// Only the fields Terrain actually populates are exposed; an adopter's
// workflow can extend the marshaled JSON with additional fields
// (started_at, completed_at, details_url, etc.) before posting if
// needed.
type CheckRun struct {
	// Name is what GitHub shows in the PR's "Checks" tab. Stable
	// across runs so PRs see a single check rather than a new one
	// per re-run.
	Name string `json:"name"`
	// HeadSHA is the commit being checked. Required by the API.
	HeadSHA string `json:"head_sha"`
	// Status is one of "queued", "in_progress", "completed".
	// Terrain emits "completed" — the binary already ran.
	Status string `json:"status"`
	// Conclusion is set when Status="completed". One of "action_required",
	// "cancelled", "failure", "neutral", "success", "skipped",
	// "stale", "timed_out". Terrain emits "failure" / "success" /
	// "neutral".
	Conclusion string `json:"conclusion,omitempty"`
	// Output carries the human-readable surface: title, summary
	// (markdown), text (full body markdown), and per-line annotations.
	Output Output `json:"output"`
}

// Output is the human-readable surface of a check run.
type Output struct {
	// Title is the one-line headline (max ~100 chars per the API).
	Title string `json:"title"`
	// Summary is the short summary in markdown (max ~65k chars).
	// Renders in the check-run pane before "Show more".
	Summary string `json:"summary"`
	// Text is the full body in markdown (max ~65k chars). Renders
	// after "Show more" — the place for long detail.
	Text string `json:"text,omitempty"`
	// Annotations are per-line callouts shown inline on the diff.
	// Each annotation has a path, line range, severity level, and
	// short message. The GitHub API accepts up to 50 per request;
	// callers exceeding 50 should batch follow-up requests with the
	// same check-run name.
	Annotations []Annotation `json:"annotations,omitempty"`
}

// Annotation is one per-line callout.
type Annotation struct {
	// Path is the repo-relative file path.
	Path string `json:"path"`
	// StartLine and EndLine are 1-indexed.
	StartLine int `json:"start_line"`
	EndLine   int `json:"end_line"`
	// AnnotationLevel is "notice", "warning", or "failure".
	AnnotationLevel string `json:"annotation_level"`
	// Title is the short headline (max ~256 chars).
	Title string `json:"title,omitempty"`
	// Message is the body (max ~64k chars).
	Message string `json:"message"`
	// RawDetails is optional extended detail rendered in a
	// collapsible section.
	RawDetails string `json:"raw_details,omitempty"`
}

// CheckRunsBundle is the package's headline output type: both check
// runs serialized side-by-side. Adopter workflows can write the
// whole bundle to disk and split during posting, or post each half
// separately.
type CheckRunsBundle struct {
	Gate          CheckRun `json:"gate_check"`
	Observability CheckRun `json:"observability_check"`
}

// BuildBundle constructs both check runs from a snapshot. The
// `headSHA` and `repoRoot` come from the calling workflow.
//
// Splitting rules:
//   - A signal's Tier (gate vs observability) determines which check
//     run it lands in. Tier is read from the manifest entry; signals
//     without a manifest entry default to gate-tier (legacy CI gate
//     relevance).
//   - Within the gate check, the per-detector visibility floor (P5.4)
//     applies: one headline annotation per detector that fired,
//     co-fires collapse to "+N more" in the message.
//   - The observability check carries every observability-tier
//     finding (no per-detector collapse) in a `<details>` footer
//     section in the Text. Annotations are present for each so they
//     still render inline on the diff, just without a required-check
//     conclusion.
func BuildBundle(snap *models.TestSuiteSnapshot, headSHA string) CheckRunsBundle {
	if snap == nil {
		return CheckRunsBundle{}
	}
	gateSignals, obsSignals := splitByTier(snap.Signals)

	gateOut := buildGateOutput(gateSignals)
	obsOut := buildObservabilityOutput(obsSignals)

	gateConclusion := "success"
	if hasBlockingFinding(gateSignals) {
		gateConclusion = "failure"
	}

	return CheckRunsBundle{
		Gate: CheckRun{
			Name:       "terrain (gate)",
			HeadSHA:    headSHA,
			Status:     "completed",
			Conclusion: gateConclusion,
			Output:     gateOut,
		},
		Observability: CheckRun{
			Name:       "terrain (observability)",
			HeadSHA:    headSHA,
			Status:     "completed",
			Conclusion: "neutral", // never blocks merge
			Output:     obsOut,
		},
	}
}

// splitByTier partitions signals into gate vs observability based on
// the manifest Tier. Signals with no manifest entry are treated as
// gate-relevant (legacy behavior; runtime/ingestion-derived signals
// keep the previous "treat as gate" semantics).
func splitByTier(in []models.Signal) (gate, obs []models.Signal) {
	for _, s := range in {
		if signals.IsObservabilityTier(s.Type) {
			obs = append(obs, s)
		} else {
			gate = append(gate, s)
		}
	}
	return gate, obs
}

// hasBlockingFinding returns true if any signal in the gate set has
// a severity at or above Medium (the "would fail --fail-on=medium or
// stricter" threshold). Info-tier signals don't block; Low/Medium/
// High/Critical do.
func hasBlockingFinding(gate []models.Signal) bool {
	for _, s := range gate {
		sev := strings.ToLower(string(s.Severity))
		switch sev {
		case "critical", "high", "medium":
			return true
		}
	}
	return false
}

// buildGateOutput renders the gate check's title, summary, text, and
// annotations.
func buildGateOutput(gate []models.Signal) Output {
	if len(gate) == 0 {
		return Output{
			Title:   "No gate-tier findings",
			Summary: "Terrain found no blocking issues in this PR.",
		}
	}

	groups := groupByDetector(gate)

	// Title: short headline with finding count.
	title := fmt.Sprintf("%d gate %s", len(gate), pluralize(len(gate), "finding", "findings"))

	// Summary: severity-bucketed counts.
	var summary strings.Builder
	counts := severityCounts(gate)
	summary.WriteString("Gate-tier findings by label:\n")
	for _, label := range []string{"BLOCK", "GATE", "NOTE"} {
		n := counts[label]
		if n > 0 {
			summary.WriteString(fmt.Sprintf("- **%s**: %d\n", label, n))
		}
	}

	// Text: per-detector headline cards with co-fire collapse.
	var text strings.Builder
	for _, g := range groups {
		text.WriteString(renderDetectorBlock(g))
		text.WriteString("\n")
	}

	// Annotations: one per detector group (the headline finding).
	var ann []Annotation
	for _, g := range groups {
		if len(g.signals) == 0 {
			continue
		}
		head := g.signals[0]
		ann = append(ann, signalToAnnotation(head, len(g.signals)-1))
	}

	return Output{
		Title:       title,
		Summary:     summary.String(),
		Text:        text.String(),
		Annotations: ann,
	}
}

// buildObservabilityOutput renders the observability check. Always
// neutral conclusion; every finding renders in a collapsed footer
// section in the Text, plus one notice-level annotation per finding
// so they still appear inline on the diff.
func buildObservabilityOutput(obs []models.Signal) Output {
	if len(obs) == 0 {
		return Output{
			Title:   "No observability findings",
			Summary: "Terrain found no informational signals in this PR.",
		}
	}

	title := fmt.Sprintf("%d observability %s", len(obs), pluralize(len(obs), "finding", "findings"))
	summary := fmt.Sprintf("Terrain emitted %d informational %s. These don't block merge; see the details section for the full list.",
		len(obs), pluralize(len(obs), "finding", "findings"))

	// Text: collapsed footer section listing every finding.
	var text strings.Builder
	text.WriteString("<details><summary><b>")
	text.WriteString(fmt.Sprintf("%d observability %s", len(obs), pluralize(len(obs), "finding", "findings")))
	text.WriteString("</b></summary>\n\n")

	groups := groupByDetector(obs)
	for _, g := range groups {
		text.WriteString(renderDetectorBlock(g))
		text.WriteString("\n")
	}
	text.WriteString("</details>\n")

	// Annotations: every observability finding gets a notice-level
	// callout so the inline diff still shows them.
	var ann []Annotation
	for _, s := range obs {
		ann = append(ann, signalToAnnotationNotice(s))
	}

	return Output{
		Title:       title,
		Summary:     summary,
		Text:        text.String(),
		Annotations: ann,
	}
}

// detectorGroup is a per-detector cluster used by the gate render.
type detectorGroup struct {
	SignalType string
	signals    []models.Signal // severity-sorted desc
}

func groupByDetector(in []models.Signal) []detectorGroup {
	idx := map[string]*detectorGroup{}
	var keys []string
	for _, s := range in {
		key := string(s.Type)
		g, ok := idx[key]
		if !ok {
			g = &detectorGroup{SignalType: key}
			idx[key] = g
			keys = append(keys, key)
		}
		g.signals = append(g.signals, s)
	}
	for _, g := range idx {
		sort.SliceStable(g.signals, func(i, j int) bool {
			return severityRank(g.signals[i].Severity) > severityRank(g.signals[j].Severity)
		})
	}
	out := make([]detectorGroup, 0, len(keys))
	for _, k := range keys {
		out = append(out, *idx[k])
	}
	sort.SliceStable(out, func(i, j int) bool {
		// Highest-severity-of-group first; tie-break by detector key.
		si := severityRank(out[i].signals[0].Severity)
		sj := severityRank(out[j].signals[0].Severity)
		if si != sj {
			return si > sj
		}
		return out[i].SignalType < out[j].SignalType
	})
	return out
}

// renderDetectorBlock emits one detector's section: title from
// prtemplates, headline card, "+N more" collapse if applicable.
func renderDetectorBlock(g detectorGroup) string {
	if len(g.signals) == 0 {
		return ""
	}
	head := g.signals[0]
	var b strings.Builder

	// Title (from template if registered, else SignalType verbatim).
	title := g.SignalType
	summary := ""
	action := ""
	if reg, err := prtemplates.Default(); err == nil && reg != nil {
		if tpl, ok := reg.Get(g.SignalType); ok {
			title = tpl.Title
			summary = tpl.Summary
			action = tpl.Action
		}
	}

	label := uitokens.BracketedPRLabel(tierFor(head.Type), string(head.Severity))
	loc := head.Location.File
	if head.Location.Line > 0 {
		loc = fmt.Sprintf("%s:%d", loc, head.Location.Line)
	}

	b.WriteString(fmt.Sprintf("### %s %s\n\n", label, title))
	b.WriteString(fmt.Sprintf("**`%s`** — %s\n", loc, strings.TrimSpace(summary)))
	if action != "" {
		b.WriteString(fmt.Sprintf("\n→ %s\n", action))
	}
	if rest := len(g.signals) - 1; rest > 0 {
		b.WriteString(fmt.Sprintf("\n_+%d more co-firing in this detector. Run `terrain explain %s` for the full list._\n",
			rest, g.SignalType))
	}
	return b.String()
}

// signalToAnnotation maps a Signal to a Checks-API annotation.
// `extraCount` is the count of additional co-firing signals; folded
// into the annotation message so the inline diff annotation
// acknowledges the +N collapse the Text section shows.
func signalToAnnotation(s models.Signal, extraCount int) Annotation {
	line := s.Location.Line
	if line == 0 {
		line = 1 // Checks API requires start_line ≥ 1
	}
	msg := s.Explanation
	if msg == "" {
		msg = string(s.Type)
	}
	if extraCount > 0 {
		msg += fmt.Sprintf("\n\n(+%d more co-firing in this detector)", extraCount)
	}
	return Annotation{
		Path:            s.Location.File,
		StartLine:       line,
		EndLine:         line,
		AnnotationLevel: annotationLevel(string(s.Severity)),
		Title:           string(s.Type),
		Message:         msg,
	}
}

func signalToAnnotationNotice(s models.Signal) Annotation {
	a := signalToAnnotation(s, 0)
	a.AnnotationLevel = "notice" // observability findings are never warning/failure
	return a
}

func annotationLevel(severity string) string {
	switch strings.ToLower(severity) {
	case "critical", "high":
		return "failure"
	case "medium":
		return "warning"
	}
	return "notice"
}

func severityRank(sev models.SignalSeverity) int {
	switch strings.ToLower(string(sev)) {
	case "critical":
		return 5
	case "high":
		return 4
	case "medium":
		return 3
	case "low":
		return 2
	case "info":
		return 1
	}
	return 0
}

// severityCounts buckets the gate signals into BLOCK/GATE/NOTE
// labels per the PR-comment label scheme (P5.5).
func severityCounts(in []models.Signal) map[string]int {
	out := map[string]int{}
	for _, s := range in {
		label := uitokens.PRLabel(tierFor(s.Type), string(s.Severity))
		if label == "" {
			continue
		}
		out[label]++
	}
	return out
}

// tierFor returns "gate" or "observability" for a SignalType by
// consulting the manifest. Default "gate" for unknown types.
func tierFor(t models.SignalType) string {
	if signals.IsObservabilityTier(t) {
		return "observability"
	}
	return "gate"
}

func pluralize(n int, singular, plural string) string {
	if n == 1 {
		return singular
	}
	return plural
}
