// Package checkruns produces the structured JSON for two GitHub
// Checks-API check runs:
//
//   - `terrain (gate)`         — required check; fails when an
//     undismissed gate-tier finding fires.
//   - `terrain (observability)` — informational; always neutral
//     conclusion.
//
// Terrain itself does NOT post to GitHub. Adopters' CI workflows
// shell out to `gh api` (or actions/github-script) to post each
// JSON to the Checks API. This keeps Terrain's no-network-calls
// contract intact: the binary writes JSON; the workflow handles
// auth and HTTP.
//
// The contract: terrain (gate) is required and fails on any
// undismissed gate-tier finding; terrain (observability) is
// informational only. Observability findings live in one collapsed
// <details> footer in the gate check's top-level summary.
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
//   - Within the gate check, the per-detector visibility floor
//     applies: one headline annotation per detector that fired,
//     co-fires collapse to "+N more" in the message.
//   - The observability check carries every observability-tier
//     finding (no per-detector collapse) in a `<details>` footer
//     section in the Text. Annotations are present for each so they
//     still render inline on the diff, just without a required-check
//     conclusion.
func BuildBundle(snap *models.TestSuiteSnapshot, headSHA string) CheckRunsBundle {
	return BuildBundleWithHistory(snap, headSHA, nil)
}

// HistoryStore is the minimal interface the check-runs bundler
// needs from internal/findinghistory.Store. Declared locally so the
// checkruns package doesn't import findinghistory directly — callers
// in cmd/terrain pass the concrete store through.
type HistoryStore interface {
	ShouldDemote(ruleID, file string) bool
}

// BuildBundleWithHistory is the history-aware form of BuildBundle.
// When `hist` is non-nil, gate-tier findings whose (rule_id, file)
// pair the store reports as ShouldDemote are routed to the
// observability check instead of the gate check — matching the
// PR-comment renderer's behavior so the two surfaces don't disagree.
//
// Without this routing, a chronically-firing finding that the PR
// comment demotes to [WATCH] would still appear in the gate check
// and fail the required CI check. That contradiction is the symptom
// this function exists to prevent.
func BuildBundleWithHistory(snap *models.TestSuiteSnapshot, headSHA string, hist HistoryStore) CheckRunsBundle {
	// Default gate threshold is Medium, preserving the historical behavior for
	// callers that don't specify one.
	return BuildBundleAt(snap, headSHA, hist, models.SeverityMedium)
}

// BuildBundleAt is the threshold-aware form of BuildBundleWithHistory: a
// gate-tier finding at or above blockAt fails the required gate check. Callers
// thread the same --fail-on the CLI gate uses, so the required GitHub check and
// the CLI exit code agree on the merge verdict. An empty or unrecognized
// blockAt means the gate check never fails (no threshold configured).
func BuildBundleAt(snap *models.TestSuiteSnapshot, headSHA string, hist HistoryStore, blockAt models.SignalSeverity) CheckRunsBundle {
	return BuildBundleAtWithGate(snap, headSHA, hist, blockAt, nil)
}

// BuildBundleAtWithGate is BuildBundleAt plus the trust-floor predicate: a
// gate-tier signal for which blockable returns false is demoted to the
// observability check (it still surfaces, but cannot fail the required gate),
// matching `terrain analyze --fail-on` under the default trust floor. Pass nil
// for blockable to gate on tier + severity alone (trust floor off).
func BuildBundleAtWithGate(snap *models.TestSuiteSnapshot, headSHA string, hist HistoryStore, blockAt models.SignalSeverity, blockable func(models.Signal) bool) CheckRunsBundle {
	if snap == nil {
		return CheckRunsBundle{}
	}
	gateSignals, obsSignals := splitByTierWithHistory(snap.Signals, hist, blockable)

	// The gate set is what survived demotion — every signal here is
	// gate-tier per the manifest AND not demoted by history. Its
	// effective tier is "gate".
	gateOut := buildGateOutput(gateSignals, blockAt)
	// The obs set is the union of manifest-observability signals AND
	// demoted gate signals. We need to know per-signal which is which
	// so the per-detector block renders the right [WATCH] vs [BLOCK]
	// label — matching the PR-comment renderer's behavior.
	obsOut := buildObservabilityOutputWithHistory(obsSignals, hist)

	gateConclusion := "success"
	if hasBlockingFinding(gateSignals, blockAt) {
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
	return splitByTierWithHistory(in, nil, nil)
}

// splitByTierWithHistory is the history-aware partition. When `hist`
// is non-nil, any gate-tier signal whose (Type, Location.File) pair
// the store reports as ShouldDemote is moved to the observability
// set. Empty Type or empty File pairs cannot match a history entry,
// so they always follow the manifest tier.
func splitByTierWithHistory(in []models.Signal, hist HistoryStore, blockable func(models.Signal) bool) (gate, obs []models.Signal) {
	for _, s := range in {
		if signals.IsObservabilityTier(s.Type) {
			obs = append(obs, s)
			continue
		}
		if hist != nil && s.Type != "" && s.Location.File != "" &&
			hist.ShouldDemote(string(s.Type), s.Location.File) {
			obs = append(obs, s)
			continue
		}
		// Trust-floor demotion: a gate-tier signal whose remediation is not
		// closed-loop validated cannot fail the required check — it routes to the
		// observability set, matching what `terrain analyze --fail-on` does. When
		// blockable is nil (trust floor off) every gate-tier signal stays gating.
		if blockable != nil && !blockable(s) {
			obs = append(obs, s)
			continue
		}
		gate = append(gate, s)
	}
	return gate, obs
}

// hasBlockingFinding returns true if any gate-set signal is at or above the
// blockAt severity — the configured --fail-on threshold, so the required check
// matches the CLI verdict. An empty or unrecognized blockAt (rank 0) configures
// no threshold and the gate check never fails.
func hasBlockingFinding(gate []models.Signal, blockAt models.SignalSeverity) bool {
	min := severityRank(blockAt)
	if min == 0 {
		return false
	}
	for _, s := range gate {
		if severityRank(s.Severity) >= min {
			return true
		}
	}
	return false
}

// isAtOrAbove reports whether sev meets the blockAt threshold. An empty
// or unrecognized blockAt (rank 0) configures no threshold, so nothing
// blocks — mirroring hasBlockingFinding.
func isAtOrAbove(sev, blockAt models.SignalSeverity) bool {
	min := severityRank(blockAt)
	if min == 0 {
		return false
	}
	return severityRank(sev) >= min
}

// countAtOrAbove counts signals at or above the blockAt threshold.
func countAtOrAbove(in []models.Signal, blockAt models.SignalSeverity) int {
	n := 0
	for _, s := range in {
		if isAtOrAbove(s.Severity, blockAt) {
			n++
		}
	}
	return n
}

// buildGateOutput renders the gate check's title, summary, text, and
// annotations. blockAt is the configured --fail-on threshold: findings
// below it are surfaced but cannot fail the check, so the title and
// annotation severities are clamped to that decision.
func buildGateOutput(gate []models.Signal, blockAt models.SignalSeverity) Output {
	if len(gate) == 0 {
		return Output{
			Title:   "No gate-tier findings",
			Summary: "Terrain found no blocking issues in this PR.",
		}
	}

	groups := groupByDetector(gate)

	// Title: reflect the gate decision so the headline reconciles with
	// the check conclusion. Count only findings at/above the block
	// threshold; when none block, say so explicitly.
	blocking := countAtOrAbove(gate, blockAt)
	var title string
	if blocking == 0 {
		title = fmt.Sprintf("No blocking findings (%d below threshold)", len(gate))
	} else {
		title = fmt.Sprintf("%d blocking gate %s", blocking, pluralize(blocking, "finding", "findings"))
	}

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
	// Filter empty-path signals (repo-level findings without a file
	// location can't render inline on the diff) and cap at 50 per the
	// GitHub Checks API limit per request.
	var ann []Annotation
	for _, g := range groups {
		if len(g.signals) == 0 {
			continue
		}
		head := g.signals[0]
		if head.Location.File == "" {
			continue
		}
		ann = append(ann, signalToAnnotation(head, len(g.signals)-1, blockAt))
	}
	ann = capAnnotations(ann)

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
	return buildObservabilityOutputWithHistory(obs, nil)
}

// buildObservabilityOutputWithHistory is the history-aware form. When
// the partition step moved a manifest-gate-tier finding into the obs
// set (because the store reports ShouldDemote), this renderer needs
// to label that finding as observability — otherwise the obs check's
// markdown body would show [GATE]/[BLOCK] for a finding the PR
// comment renders as [WATCH], breaking cross-surface consistency.
func buildObservabilityOutputWithHistory(obs []models.Signal, hist HistoryStore) Output {
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
		text.WriteString(renderDetectorBlockWithHistory(g, hist))
		text.WriteString("\n")
	}
	text.WriteString("</details>\n")

	// Annotations: every observability finding gets a notice-level
	// callout so the inline diff still shows them. Filter empty-path
	// signals + cap at the 50-per-request GitHub Checks API limit.
	var ann []Annotation
	for _, s := range obs {
		if s.Location.File == "" {
			continue
		}
		ann = append(ann, signalToAnnotationNotice(s))
	}
	ann = capAnnotations(ann)

	return Output{
		Title:       title,
		Summary:     summary,
		Text:        text.String(),
		Annotations: ann,
	}
}

// capAnnotations enforces GitHub's Checks API limit of 50 annotations
// per check-run-update request. Returning a request with more than 50
// annotations triggers HTTP 422 Validation Failed on the annotations
// field. Adopters with > 50 findings see the first 50 inline plus the
// full list in the check's `Text` body — better than the whole upload
// failing.
func capAnnotations(ann []Annotation) []Annotation {
	const maxPerRequest = 50
	if len(ann) <= maxPerRequest {
		return ann
	}
	return ann[:maxPerRequest]
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
	return renderDetectorBlockWithHistory(g, nil)
}

// renderDetectorBlockWithHistory mirrors renderDetectorBlock but
// downgrades the displayed tier to observability when the history
// store reports the head signal's (Type, File) as ShouldDemote.
// This keeps the obs check's markdown body label-consistent with
// the PR-comment renderer.
func renderDetectorBlockWithHistory(g detectorGroup, hist HistoryStore) string {
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

	tier := tierFor(head.Type)
	if hist != nil && head.Type != "" && head.Location.File != "" &&
		hist.ShouldDemote(string(head.Type), head.Location.File) {
		tier = "observability"
	}
	label := uitokens.BracketedPRLabel(tier, string(head.Severity))
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
//
// blockAt is the configured --fail-on threshold. A finding below it
// cannot fail the check, so its annotation level is capped at
// "warning" rather than "failure" — keeping the inline diff
// consistent with the "success" conclusion.
func signalToAnnotation(s models.Signal, extraCount int, blockAt models.SignalSeverity) Annotation {
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
	level := annotationLevel(string(s.Severity))
	// A finding below the block threshold concludes "success"; a
	// "failure"-level annotation would contradict that, so cap it.
	if level == "failure" && !isAtOrAbove(s.Severity, blockAt) {
		level = "warning"
	}
	return Annotation{
		Path:            s.Location.File,
		StartLine:       line,
		EndLine:         line,
		AnnotationLevel: level,
		Title:           string(s.Type),
		Message:         msg,
	}
}

func signalToAnnotationNotice(s models.Signal) Annotation {
	a := signalToAnnotation(s, 0, "")
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
// labels per the PR-comment label scheme.
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
