// Package insights aggregates Terrain's engines into a prioritized health
// report for `terrain insights`. It answers: "What should we fix in our
// test system?" by ranking findings into four categories: optimization
// opportunities, reliability problems, architecture debt, and coverage debt.
package insights

import (
	"fmt"
	"sort"
	"strings"

	"github.com/pmclSF/terrain/internal/depgraph"
	"github.com/pmclSF/terrain/internal/matrix"
	"github.com/pmclSF/terrain/internal/models"
	"github.com/pmclSF/terrain/internal/signals"
	"github.com/pmclSF/terrain/internal/skipstats"
	"github.com/pmclSF/terrain/internal/stability"
)

// Category classifies a finding.
type Category string

const (
	CategoryOptimization     Category = "optimization"
	CategoryReliability      Category = "reliability"
	CategoryArchitectureDebt Category = "architecture_debt"
	CategoryCoverageDebt     Category = "coverage_debt"
)

// maxRecommendations caps the number of recommendations in a report.
const maxRecommendations = 7

// Severity ranks how urgent a finding is.
type Severity string

const (
	SeverityCritical Severity = "critical"
	SeverityHigh     Severity = "high"
	SeverityMedium   Severity = "medium"
	SeverityLow      Severity = "low"
)

// Report is the structured output of `terrain insights`.
type Report struct {
	// Headline is a one-line summary of the most important finding.
	Headline string `json:"headline"`

	// HealthGrade is a letter grade (A–D) summarizing overall health.
	// A = no findings, B = low/medium only, C = high present, D = critical or many high.
	HealthGrade string `json:"healthGrade"`

	// Findings are all detected issues, ranked by priority.
	Findings []Finding `json:"findings"`

	// Recommendations are prioritized actions derived from findings.
	Recommendations []Recommendation `json:"recommendations"`

	// CategorySummary breaks down findings by category.
	CategorySummary map[Category]CategoryBreakdown `json:"categorySummary"`

	// RepoProfile classifies the repository along key dimensions.
	RepoProfile depgraph.RepoProfile `json:"repoProfile"`

	// EdgeCases are structural anomalies that affect analysis confidence.
	EdgeCases []depgraph.EdgeCase `json:"edgeCases,omitempty"`

	// Policy captures the derived policy from edge case analysis.
	Policy depgraph.Policy `json:"policy"`

	// BehaviorRedundancy detects behavior-aware test redundancy.
	BehaviorRedundancy *depgraph.RedundancyResult `json:"behaviorRedundancy,omitempty"`

	// StabilityClusters groups unstable tests by shared root cause.
	StabilityClusters *stability.ClusterResult `json:"stabilityClusters,omitempty"`

	// MatrixCoverage holds device/environment matrix analysis results.
	MatrixCoverage *matrix.MatrixResult `json:"matrixCoverage,omitempty"`

	// DataCompleteness shows which data sources are available.
	DataCompleteness []DataSource `json:"dataCompleteness"`

	// Limitations notes where analysis is incomplete.
	Limitations []string `json:"limitations,omitempty"`
}

// Finding is a single detected issue in the test system.
type Finding struct {
	// Title is a short description of the issue.
	Title string `json:"title"`

	// Description provides detail and context.
	Description string `json:"description"`

	// Category classifies the finding.
	Category Category `json:"category"`

	// Severity ranks urgency.
	Severity Severity `json:"severity"`

	// Priority is the computed rank (1 = most urgent).
	Priority int `json:"priority"`

	// Scope identifies where the issue lives (file, directory, etc.).
	Scope string `json:"scope,omitempty"`

	// Metric is the key number (e.g., "340 redundant tests").
	Metric string `json:"metric,omitempty"`
}

// Recommendation is a prioritized action derived from findings.
type Recommendation struct {
	// Action is what to do.
	Action string `json:"action"`

	// Rationale is why.
	Rationale string `json:"rationale"`

	// Category classifies the recommendation.
	Category Category `json:"category"`

	// Priority is the computed rank (1 = most urgent).
	Priority int `json:"priority"`

	// Impact estimates the benefit (e.g., "reduce CI runtime by ~15%").
	Impact string `json:"impact,omitempty"`

	// TargetFiles lists specific file paths to act on.
	TargetFiles []string `json:"targetFiles,omitempty"`

	// EffortBand is "small" (1-3 files), "medium" (4-10), or "large" (10+).
	EffortBand string `json:"effortBand,omitempty"`

	// Command is a runnable terrain command for drill-down.
	Command string `json:"command,omitempty"`
}

// CategoryBreakdown summarizes findings within a category.
type CategoryBreakdown struct {
	Count         int    `json:"count"`
	CriticalCount int    `json:"criticalCount"`
	HighCount     int    `json:"highCount"`
	TopFinding    string `json:"topFinding,omitempty"`
}

// DataSource describes a data source's availability.
type DataSource struct {
	Name      string `json:"name"`
	Available bool   `json:"available"`
}

// BuildInput contains everything needed to build an insights report.
type BuildInput struct {
	Snapshot  *models.TestSuiteSnapshot
	HasPolicy bool

	// Depgraph results (may be zero-valued if skipped for scale).
	Coverage   depgraph.CoverageResult
	Duplicates depgraph.DuplicateResult
	Fanout     depgraph.FanoutResult
	Profile    depgraph.RepoProfile
	EdgeCases  []depgraph.EdgeCase
	Policy     depgraph.Policy

	// BehaviorRedundancy holds behavior-aware redundancy results, if available.
	BehaviorRedundancy *depgraph.RedundancyResult

	// StabilityClusters holds cluster analysis results, if available.
	StabilityClusters *stability.ClusterResult

	// MatrixCoverage holds device/environment matrix analysis results, if available.
	MatrixCoverage *matrix.MatrixResult

	// DepgraphSkipped indicates depgraph analysis was skipped.
	DepgraphSkipped    bool
	DepgraphSkipReason string
}

// plural returns the singular form when n == 1, otherwise singular +
// "s". Local helper used in finding titles to avoid `n thing(s)`
// notation in user-visible text. The variadic `pluralForm` lets
// callers pass an irregular plural for cases where suffix-"s" is
// wrong (e.g. "scenario has" / "scenarios have", "child" / "children").
func plural(n int, singular string, pluralForm ...string) string {
	if n == 1 {
		return singular
	}
	if len(pluralForm) > 0 {
		return pluralForm[0]
	}
	return singular + "s"
}

// Build constructs an insights Report from analysis results.
//
// nil-safe: a nil input or a non-nil input with a nil Snapshot returns
// an empty Report. The contract is exercised by
// internal/testdata/adversarial_test.go:TestAdversarial_BuildEntryPoints_NilInput.
func Build(input *BuildInput) *Report {
	if input == nil || input.Snapshot == nil {
		return &Report{
			CategorySummary: map[Category]CategoryBreakdown{},
		}
	}
	r := &Report{
		RepoProfile:        input.Profile,
		EdgeCases:          input.EdgeCases,
		Policy:             input.Policy,
		BehaviorRedundancy: input.BehaviorRedundancy,
		StabilityClusters:  input.StabilityClusters,
		MatrixCoverage:     input.MatrixCoverage,
		CategorySummary:    map[Category]CategoryBreakdown{},
		DataCompleteness:   buildDataCompleteness(input),
	}

	// Collect all findings.
	var findings []Finding

	// 1. Duplicate validations (optimization).
	findings = append(findings, duplicateFindings(input)...)

	// 2. High-fanout nodes (architecture debt).
	findings = append(findings, fanoutFindings(input)...)

	// 3. Weak coverage areas (coverage debt).
	findings = append(findings, coverageFindings(input)...)

	// 4. Skip debt (reliability).
	findings = append(findings, skipFindings(input)...)

	// 5. Flaky/stability issues (reliability).
	findings = append(findings, stabilityFindings(input)...)

	// 6. Signal-derived findings.
	findings = append(findings, signalFindings(input)...)

	// 7. Manual coverage overlay findings.
	findings = append(findings, manualCoverageFindings(input)...)

	// 8. Matrix coverage findings.
	findings = append(findings, matrixFindings(input)...)

	// 9. AI scenario duplication.
	findings = append(findings, scenarioDuplicationFindings(input)...)

	// 10. AI surface coverage gaps.
	findings = append(findings, aiCoverageFindings(input)...)

	// 11. "What to test next" — prioritized untested code.
	findings = append(findings, testNextFindings(input)...)

	// 12. AI behavior impact chains — prompt/context downstream gaps.
	findings = append(findings, aiBehaviorChainFindings(input)...)

	// 13. Capability gap detection — missing negative/adversarial scenarios.
	findings = append(findings, capabilityGapFindings(input)...)

	// 14. Depgraph skipped warning.
	if input.DepgraphSkipped {
		findings = append(findings, Finding{
			Title:       "Depgraph analysis skipped",
			Description: input.DepgraphSkipReason,
			Category:    CategoryOptimization,
			Severity:    SeverityMedium,
			Metric:      "limited analysis depth",
		})
	}

	// Deduplicate findings from multiple builders.
	// Key: category + title + scope (two builders can produce the same finding).
	findings = deduplicateInsightFindings(findings)

	// Rank findings by severity, then category priority.
	sort.SliceStable(findings, func(i, j int) bool {
		si := severityOrder(findings[i].Severity)
		sj := severityOrder(findings[j].Severity)
		if si != sj {
			return si > sj
		}
		return categoryOrder(findings[i].Category) < categoryOrder(findings[j].Category)
	})
	for i := range findings {
		findings[i].Priority = i + 1
	}

	r.Findings = findings

	// Build category summary.
	for _, f := range findings {
		cb := r.CategorySummary[f.Category]
		cb.Count++
		if f.Severity == SeverityCritical {
			cb.CriticalCount++
		}
		if f.Severity == SeverityHigh {
			cb.HighCount++
		}
		if cb.TopFinding == "" {
			cb.TopFinding = f.Title
		}
		r.CategorySummary[f.Category] = cb
	}

	// Build recommendations from findings.
	r.Recommendations = buildRecommendations(findings, input)

	// Derive headline and health grade.
	r.Headline = deriveHeadline(r)
	r.HealthGrade = deriveHealthGrade(r)

	// No-tests-detected guard: a snapshot with zero tests AND zero
	// findings is the genuine first-user empty-repo case. The
	// previous behavior returned grade "A" with the headline "Your
	// test suite looks healthy" — dishonest for a repo with no
	// tests. The audit caught this on first-user fresh-repo
	// experience.
	//
	// Conservative trigger: BOTH zero tests AND zero findings.
	// If there are findings (e.g. AI-side signals on a tests-free
	// repo), grading is still meaningful and we leave it alone.
	if len(input.Snapshot.TestFiles) == 0 && len(input.Snapshot.TestCases) == 0 && len(findings) == 0 {
		r.HealthGrade = "—"
		r.Headline = "No tests detected — Terrain has nothing to grade. Add tests with your framework of choice, then re-run."
	}

	// Limitations.
	r.Limitations = buildLimitations(input)

	return r
}

// --- Finding builders ---

func duplicateFindings(input *BuildInput) []Finding {
	var findings []Finding
	dupes := &input.Duplicates

	if dupes.Skipped {
		return findings
	}

	if dupes.DuplicateCount > 0 {
		sev := SeverityMedium
		if dupes.DuplicateCount > 100 {
			sev = SeverityHigh
		}

		f := Finding{
			Title:    fmt.Sprintf("%d redundant tests across %d clusters", dupes.DuplicateCount, len(dupes.Clusters)),
			Category: CategoryOptimization,
			Severity: sev,
			Metric:   fmt.Sprintf("%d duplicates", dupes.DuplicateCount),
		}

		if len(dupes.Clusters) > 0 {
			top := dupes.Clusters[0]
			f.Description = fmt.Sprintf("Largest cluster has %d tests with %.0f%% similarity. Consolidating duplicates reduces CI runtime and maintenance burden.",
				len(top.Tests), top.Similarity*100)
			f.Scope = fmt.Sprintf("%d %s", len(dupes.Clusters), plural(len(dupes.Clusters), "cluster"))
		}

		findings = append(findings, f)
	}

	// Behavior redundancy findings.
	if input.BehaviorRedundancy != nil && len(input.BehaviorRedundancy.Clusters) > 0 {
		br := input.BehaviorRedundancy
		wasteful := 0
		crossFW := 0
		for _, c := range br.Clusters {
			switch c.OverlapKind {
			case depgraph.OverlapWasteful:
				wasteful++
			case depgraph.OverlapCrossFramework:
				crossFW++
			}
		}

		if wasteful > 0 {
			sev := SeverityMedium
			if wasteful > 5 {
				sev = SeverityHigh
			}
			top := br.Clusters[0] // sorted wasteful-first
			findings = append(findings, Finding{
				Title: fmt.Sprintf("%d behavior-redundant test clusters detected", wasteful),
				Description: fmt.Sprintf(
					"Tests exercise identical behavior surfaces without adding coverage. Top cluster: %d tests share %d surfaces. %s",
					len(top.Tests), len(top.SharedSurfaces), top.Rationale),
				Category: CategoryOptimization,
				Severity: sev,
				Metric:   fmt.Sprintf("%d wasteful clusters", wasteful),
			})
		}

		if crossFW > 0 {
			findings = append(findings, Finding{
				Title: fmt.Sprintf("%d cross-framework overlaps found", crossFW),
				Description: "Tests in different frameworks exercise the same behavior surfaces. " +
					"If migrating, remove old-framework tests once migration is validated.",
				Category: CategoryOptimization,
				Severity: SeverityLow,
				Metric:   fmt.Sprintf("%d cross-framework", crossFW),
			})
		}
	}

	return findings
}

func fanoutFindings(input *BuildInput) []Finding {
	var findings []Finding
	fanout := &input.Fanout

	if fanout.Skipped || fanout.FlaggedCount == 0 {
		return findings
	}

	sev := SeverityMedium
	if fanout.FlaggedCount > 5 {
		sev = SeverityHigh
	}

	f := Finding{
		Title:    fmt.Sprintf("%d high-fanout nodes exceed threshold of %d", fanout.FlaggedCount, fanout.Threshold),
		Category: CategoryArchitectureDebt,
		Severity: sev,
		Metric:   fmt.Sprintf("%d nodes flagged", fanout.FlaggedCount),
	}

	if len(fanout.Entries) > 0 && fanout.Entries[0].Flagged {
		top := fanout.Entries[0]
		label := fanoutLabel(top.NodeID, top.Path, top.NodeType)
		f.Description = fmt.Sprintf("Highest: %s with %d transitive dependents. Changes to high-fanout nodes trigger disproportionate test impact.",
			label, top.TransitiveFanout)
		f.Scope = label
	}

	findings = append(findings, f)

	return findings
}

func coverageFindings(input *BuildInput) []Finding {
	var findings []Finding
	cov := &input.Coverage

	lowCount := cov.BandCounts[depgraph.CoverageBandLow]
	if lowCount == 0 || cov.SourceCount == 0 {
		return findings
	}

	pct := 100 * lowCount / cov.SourceCount
	sev := SeverityMedium
	if pct > 50 {
		sev = SeverityHigh
	}
	if pct > 75 {
		sev = SeverityCritical
	}

	f := Finding{
		Title:       fmt.Sprintf("%d source files (%d%%) have low structural coverage", lowCount, pct),
		Description: "Files with no test coverage are blind spots for change-scoped test selection. They will not be protected by `terrain impact`.",
		Category:    CategoryCoverageDebt,
		Severity:    sev,
		Metric:      fmt.Sprintf("%d/%d files uncovered", lowCount, cov.SourceCount),
	}

	// Show top weak areas.
	shown := 0
	var areas []string
	for _, src := range cov.Sources {
		if src.Band == depgraph.CoverageBandLow && shown < 3 {
			areas = append(areas, src.Path)
			shown++
		}
	}
	if len(areas) > 0 {
		f.Scope = areas[0]
	}

	findings = append(findings, f)

	return findings
}

func skipFindings(input *BuildInput) []Finding {
	var findings []Finding
	stats := skipstats.Summarize(input.Snapshot)

	if stats.SkippedTests == 0 {
		return findings
	}

	sev := SeverityLow
	if stats.TestRatio > 0.10 {
		sev = SeverityHigh
	} else if stats.TestRatio > 0.03 {
		sev = SeverityMedium
	}

	findings = append(findings, Finding{
		Title:       fmt.Sprintf("%d skipped tests consuming CI resources", stats.SkippedTests),
		Description: "Skipped tests still occupy CI queue slots and mask coverage gaps. Review whether each skip is still justified or should be removed.",
		Category:    CategoryReliability,
		Severity:    sev,
		Metric:      fmt.Sprintf("%d skipped", stats.SkippedTests),
	})

	return findings
}

func stabilityFindings(input *BuildInput) []Finding {
	var findings []Finding
	snap := input.Snapshot

	flaky := 0
	for _, sig := range snap.Signals {
		if sig.Type == signals.SignalFlakyTest || sig.Type == signals.SignalUnstableSuite {
			flaky++
		}
	}

	if flaky == 0 {
		return findings
	}

	sev := SeverityMedium
	if flaky > 10 {
		sev = SeverityHigh
	}
	if flaky > 50 {
		sev = SeverityCritical
	}

	findings = append(findings, Finding{
		Title:       fmt.Sprintf("%d flaky/unstable test signals detected", flaky),
		Description: "Flaky tests erode developer trust and waste CI cycles on retries. Stabilize or quarantine these tests.",
		Category:    CategoryReliability,
		Severity:    sev,
		Metric:      fmt.Sprintf("%d flaky signals", flaky),
	})

	// Stability cluster findings.
	if input.StabilityClusters != nil && len(input.StabilityClusters.Clusters) > 0 {
		clusters := input.StabilityClusters
		clusterSev := SeverityMedium
		if len(clusters.Clusters) > 3 {
			clusterSev = SeverityHigh
		}

		desc := fmt.Sprintf(
			"%d of %d unstable tests cluster around shared dependencies. Top cause: %s (%s, %d tests).",
			clusters.ClusteredTestCount, clusters.UnstableTestCount,
			clusters.Clusters[0].CauseName, clusters.Clusters[0].CauseKind,
			len(clusters.Clusters[0].Members))

		findings = append(findings, Finding{
			Title:       fmt.Sprintf("%d stability clusters detected — likely shared root causes", len(clusters.Clusters)),
			Description: desc,
			Category:    CategoryReliability,
			Severity:    clusterSev,
			Metric:      fmt.Sprintf("%d clusters, %d clustered tests", len(clusters.Clusters), clusters.ClusteredTestCount),
		})
	}

	return findings
}

func signalFindings(input *BuildInput) []Finding {
	var findings []Finding
	snap := input.Snapshot

	// Count critical and high signals.
	critical := 0
	high := 0
	for _, s := range snap.Signals {
		switch s.Severity {
		case models.SeverityCritical:
			critical++
		case models.SeverityHigh:
			high++
		}
	}

	if critical > 0 {
		findings = append(findings, Finding{
			Title:       fmt.Sprintf("%d critical-severity signals require attention", critical),
			Description: "Critical signals indicate issues that are likely to cause test failures or missed regressions.",
			Category:    CategoryReliability,
			Severity:    SeverityCritical,
			Metric:      fmt.Sprintf("%d critical", critical),
		})
	}

	if high > 10 {
		findings = append(findings, Finding{
			Title:       fmt.Sprintf("%d high-severity signals detected", high),
			Description: "High-severity signals represent significant quality or reliability risks.",
			Category:    CategoryCoverageDebt,
			Severity:    SeverityHigh,
			Metric:      fmt.Sprintf("%d high", high),
		})
	}

	// E2E-only coverage dependence.
	cs := snap.CoverageSummary
	if cs != nil && cs.CoveredOnlyByE2E > 0 {
		findings = append(findings, Finding{
			Title:       fmt.Sprintf("%d code units covered only by e2e tests", cs.CoveredOnlyByE2E),
			Description: "Code covered only by e2e tests has no fast feedback loop. Unit test additions would catch issues earlier.",
			Category:    CategoryCoverageDebt,
			Severity:    SeverityMedium,
			Metric:      fmt.Sprintf("%d e2e-only", cs.CoveredOnlyByE2E),
		})
	}

	return findings
}

func manualCoverageFindings(input *BuildInput) []Finding {
	var findings []Finding
	snap := input.Snapshot
	if snap == nil || len(snap.ManualCoverage) == 0 {
		return findings
	}

	// Count stale artifacts (no LastExecuted date).
	stale := 0
	highCrit := 0
	for _, mc := range snap.ManualCoverage {
		if mc.LastExecuted == "" {
			stale++
		}
		if mc.Criticality == "high" {
			highCrit++
		}
	}

	total := len(snap.ManualCoverage)

	// Staleness finding.
	if stale > 0 && stale >= total/2 {
		sev := SeverityLow
		if highCrit > 0 && stale >= highCrit {
			sev = SeverityMedium
		}
		findings = append(findings, Finding{
			Title:       fmt.Sprintf("%d of %d manual coverage artifacts have no recent execution date", stale, total),
			Description: "Stale manual coverage may provide false confidence. Verify these validation activities are still being performed.",
			Category:    CategoryCoverageDebt,
			Severity:    sev,
			Metric:      fmt.Sprintf("%d stale", stale),
		})
	}

	return findings
}

func matrixFindings(input *BuildInput) []Finding {
	var findings []Finding
	mr := input.MatrixCoverage
	if mr == nil || len(mr.Classes) == 0 {
		return findings
	}

	// Gap finding: uncovered members in classes that have some coverage.
	if len(mr.Gaps) > 0 {
		sev := SeverityLow
		if len(mr.Gaps) > 5 {
			sev = SeverityMedium
		}

		gap := mr.Gaps[0]
		desc := fmt.Sprintf(
			"%d environment/device class members have no test coverage. Example: %s in %s (%s).",
			len(mr.Gaps), gap.MemberName, gap.ClassName, gap.Dimension)

		findings = append(findings, Finding{
			Title:       fmt.Sprintf("%d environment/device coverage gaps detected", len(mr.Gaps)),
			Description: desc,
			Category:    CategoryCoverageDebt,
			Severity:    sev,
			Metric:      fmt.Sprintf("%d gaps across %d classes", len(mr.Gaps), mr.ClassesAnalyzed),
		})
	}

	// Concentration finding: skewed coverage within a class.
	if len(mr.Concentrations) > 0 {
		top := mr.Concentrations[0]
		findings = append(findings, Finding{
			Title: fmt.Sprintf("Device concentration: %.0f%% of %s tests target only %s",
				top.DominantShare*100, top.Dimension, top.DominantName),
			Description: fmt.Sprintf(
				"%s class has %d members but %d/%d are covered. Diversifying coverage reduces platform-specific blind spots.",
				top.ClassName, top.TotalMembers, top.CoveredMembers, top.TotalMembers),
			Category: CategoryCoverageDebt,
			Severity: SeverityLow,
			Metric:   fmt.Sprintf("%.0f%% concentration", top.DominantShare*100),
		})
	}

	return findings
}

func aiCoverageFindings(input *BuildInput) []Finding {
	var findings []Finding
	snap := input.Snapshot
	if snap == nil {
		return findings
	}

	// Count AI surfaces by kind.
	aiSurfaces := 0
	for _, cs := range snap.CodeSurfaces {
		switch cs.Kind {
		case models.SurfacePrompt, models.SurfaceContext, models.SurfaceDataset,
			models.SurfaceToolDef, models.SurfaceRetrieval, models.SurfaceAgent,
			models.SurfaceEvalDef:
			aiSurfaces++
		}
	}
	if aiSurfaces == 0 {
		return findings
	}

	// Count uncovered AI surfaces (not linked to any scenario).
	coveredIDs := map[string]bool{}
	for _, sc := range snap.Scenarios {
		for _, sid := range sc.CoveredSurfaceIDs {
			coveredIDs[sid] = true
		}
	}
	uncovered := 0
	var uncoveredExamples []string
	for _, cs := range snap.CodeSurfaces {
		switch cs.Kind {
		case models.SurfacePrompt, models.SurfaceContext, models.SurfaceDataset,
			models.SurfaceToolDef, models.SurfaceRetrieval, models.SurfaceAgent:
			if !coveredIDs[cs.SurfaceID] {
				uncovered++
				if len(uncoveredExamples) < 3 {
					uncoveredExamples = append(uncoveredExamples, cs.Path)
				}
			}
		}
	}

	if uncovered == 0 {
		return findings
	}

	sev := SeverityMedium
	if uncovered > 5 {
		sev = SeverityHigh
	}

	f := Finding{
		Title: func() string {
			if uncovered == 1 {
				return "1 AI surface has no eval scenario coverage"
			}
			return fmt.Sprintf("%d AI surfaces have no eval scenario coverage", uncovered)
		}(),
		Description: fmt.Sprintf(
			"Changes to uncovered AI surfaces (prompts, contexts, datasets, tool definitions) "+
				"cannot be validated automatically. Add eval scenarios to catch behavioral regressions."),
		Category: CategoryCoverageDebt,
		Severity: sev,
		Scope:    strings.Join(uncoveredExamples, ", "),
		Metric:   fmt.Sprintf("%d/%d uncovered", uncovered, aiSurfaces),
	}
	findings = append(findings, f)

	// If there are scenarios but none cover all surfaces, recommend wiring.
	if len(snap.Scenarios) > 0 && uncovered > 0 {
		wiredCount := 0
		for _, sc := range snap.Scenarios {
			if len(sc.CoveredSurfaceIDs) > 0 {
				wiredCount++
			}
		}
		if wiredCount < len(snap.Scenarios) {
			findings = append(findings, Finding{
				Title: fmt.Sprintf("%d %s no linked code surfaces", len(snap.Scenarios)-wiredCount, plural(len(snap.Scenarios)-wiredCount, "scenario has", "scenarios have")),
				Description: "Scenarios without linked surfaces cannot be selected by impact analysis. " +
					"Wire them via terrain.yaml or ensure eval test files import the surfaces they validate.",
				Category: CategoryArchitectureDebt,
				Severity: SeverityMedium,
				Metric:   fmt.Sprintf("%d/%d unwired", len(snap.Scenarios)-wiredCount, len(snap.Scenarios)),
			})
		}
	}

	return findings
}

func scenarioDuplicationFindings(input *BuildInput) []Finding {
	var findings []Finding
	snap := input.Snapshot
	if snap == nil || len(snap.Scenarios) < 2 {
		return findings
	}

	// Build surface→scenarios index to detect overlap.
	surfaceToScenarios := map[string][]string{}
	for _, sc := range snap.Scenarios {
		for _, sid := range sc.CoveredSurfaceIDs {
			surfaceToScenarios[sid] = append(surfaceToScenarios[sid], sc.ScenarioID)
		}
	}

	// Count scenario pairs that share surfaces.
	type pair struct{ a, b string }
	pairOverlap := map[pair]int{}
	for _, scenarioIDs := range surfaceToScenarios {
		if len(scenarioIDs) < 2 {
			continue
		}
		for i := 0; i < len(scenarioIDs); i++ {
			for j := i + 1; j < len(scenarioIDs); j++ {
				a, b := scenarioIDs[i], scenarioIDs[j]
				if a > b {
					a, b = b, a
				}
				pairOverlap[pair{a, b}]++
			}
		}
	}

	if len(pairOverlap) == 0 {
		return findings
	}

	// Build scenario surface counts for overlap ratio.
	scenarioSurfaceCount := map[string]int{}
	for _, sc := range snap.Scenarios {
		scenarioSurfaceCount[sc.ScenarioID] = len(sc.CoveredSurfaceIDs)
	}

	// Find high-overlap pairs.
	highOverlapPairs := 0
	for p, shared := range pairOverlap {
		minSurfaces := scenarioSurfaceCount[p.a]
		if scenarioSurfaceCount[p.b] < minSurfaces {
			minSurfaces = scenarioSurfaceCount[p.b]
		}
		if minSurfaces > 0 && float64(shared)/float64(minSurfaces) >= 0.5 {
			highOverlapPairs++
		}
	}

	if highOverlapPairs == 0 {
		return findings
	}

	sev := SeverityLow
	if highOverlapPairs > 3 {
		sev = SeverityMedium
	}

	findings = append(findings, Finding{
		Title: fmt.Sprintf("%d AI scenario %s >50%% of covered surfaces", highOverlapPairs, plural(highOverlapPairs, "pair shares", "pairs share")),
		Description: "Overlapping eval scenarios may duplicate validation effort. " +
			"Review whether scenarios can be consolidated or differentiated by coverage target.",
		Category: CategoryOptimization,
		Severity: sev,
		Metric:   fmt.Sprintf("%d overlapping pairs across %d scenarios", highOverlapPairs, len(snap.Scenarios)),
	})

	return findings
}

// --- Feature: "What to test next" ---
//
// Ranks untested code units by risk: high-fanout files with no coverage are
// the highest-priority testing gaps because changes to them affect many tests
// but no test validates them directly.
func testNextFindings(input *BuildInput) []Finding {
	var findings []Finding
	snap := input.Snapshot
	if snap == nil || len(snap.CodeUnits) == 0 {
		return findings
	}

	// Build set of code units that have at least one covering test.
	coveredUnits := map[string]bool{}
	for _, tf := range snap.TestFiles {
		for _, linked := range tf.LinkedCodeUnits {
			coveredUnits[linked] = true
		}
	}

	// Count dependents per source file from the import graph.
	// Each entry in ImportGraph maps a test file to its imports;
	// we reverse it to count how many tests depend on each source.
	fileDependents := map[string]int{}
	for _, imports := range snap.ImportGraph {
		for src := range imports {
			fileDependents[src]++
		}
	}

	// Find untested code units, prioritized by dependent count.
	type candidate struct {
		path       string
		name       string
		dependents int
	}
	var candidates []candidate
	seen := map[string]bool{}
	for _, cu := range snap.CodeUnits {
		unitID := cu.Path + ":" + cu.Name
		if coveredUnits[unitID] || coveredUnits[cu.Name] {
			continue
		}
		if seen[cu.Path] {
			continue
		}
		seen[cu.Path] = true
		candidates = append(candidates, candidate{
			path:       cu.Path,
			name:       cu.Name,
			dependents: fileDependents[cu.Path],
		})
	}

	if len(candidates) == 0 {
		return findings
	}

	// Sort by dependents descending.
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].dependents > candidates[j].dependents
	})

	// Top 5 targets.
	limit := 5
	if len(candidates) < limit {
		limit = len(candidates)
	}
	top := candidates[:limit]
	targetFiles := make([]string, limit)
	for i, c := range top {
		targetFiles[i] = c.path
	}

	sev := SeverityMedium
	if len(candidates) > 10 {
		sev = SeverityHigh
	}

	topDesc := top[0].path
	if top[0].dependents > 0 {
		topDesc = fmt.Sprintf("%s (%d dependents)", top[0].path, top[0].dependents)
	}

	findings = append(findings, Finding{
		Title: fmt.Sprintf("%d untested source %s — start with %s", len(candidates), plural(len(candidates), "file"), topDesc),
		Description: fmt.Sprintf(
			"These source files have exported code units with no covering tests. "+
				"Prioritized by dependency count: files with more dependents create larger blind spots "+
				"for change-scoped test selection."),
		Category: CategoryCoverageDebt,
		Severity: sev,
		Scope:    strings.Join(targetFiles, ", "),
		Metric:   fmt.Sprintf("%d untested files", len(candidates)),
	})

	return findings
}

// --- Feature: AI behavior impact chains ---
//
// Detects when an AI surface (prompt, context, RAG config) feeds into a
// downstream behavior chain that has partial or no eval coverage.
func aiBehaviorChainFindings(input *BuildInput) []Finding {
	var findings []Finding
	snap := input.Snapshot
	if snap == nil || len(snap.CodeSurfaces) == 0 || len(snap.Scenarios) == 0 {
		return findings
	}

	// Build coverage map: surface ID → covered by at least one scenario.
	coveredIDs := map[string]bool{}
	for _, sc := range snap.Scenarios {
		for _, sid := range sc.CoveredSurfaceIDs {
			coveredIDs[sid] = true
		}
	}

	// AI surface kinds that form chains.
	chainKinds := map[models.CodeSurfaceKind]bool{
		models.SurfacePrompt:    true,
		models.SurfaceContext:   true,
		models.SurfaceRetrieval: true,
		models.SurfaceToolDef:   true,
		models.SurfaceAgent:     true,
	}

	// Group surfaces by file to detect chains: if a file has multiple AI
	// surface types, a change to one type may affect behavior of another.
	type fileSurfaces struct {
		kinds    map[models.CodeSurfaceKind]int
		covered  int
		total    int
		surfaces []string
	}
	byFile := map[string]*fileSurfaces{}
	for _, cs := range snap.CodeSurfaces {
		if !chainKinds[cs.Kind] {
			continue
		}
		fs, ok := byFile[cs.Path]
		if !ok {
			fs = &fileSurfaces{kinds: map[models.CodeSurfaceKind]int{}}
			byFile[cs.Path] = fs
		}
		fs.kinds[cs.Kind]++
		fs.total++
		fs.surfaces = append(fs.surfaces, string(cs.Kind)+":"+cs.Name)
		if coveredIDs[cs.SurfaceID] {
			fs.covered++
		}
	}

	// Find files with multiple AI surface types where coverage is partial.
	var partialChains []string
	for path, fs := range byFile {
		if len(fs.kinds) >= 2 && fs.covered < fs.total {
			partialChains = append(partialChains, fmt.Sprintf(
				"%s (%d surface types, %d/%d covered)", path, len(fs.kinds), fs.covered, fs.total))
		}
	}

	if len(partialChains) == 0 {
		return findings
	}

	sort.Strings(partialChains)
	scope := partialChains[0]
	if len(partialChains) > 1 {
		scope = partialChains[0] + fmt.Sprintf(" +%d more", len(partialChains)-1)
	}

	findings = append(findings, Finding{
		Title: func() string {
			n := len(partialChains)
			if n == 1 {
				return "1 file has partially covered AI behavior chains"
			}
			return fmt.Sprintf("%d files have partially covered AI behavior chains", n)
		}(),
		Description: "These files contain multiple AI surface types (e.g., prompt + context, or " +
			"retrieval + tool definition) where some surfaces are tested but others are not. " +
			"A change to the untested surface can alter downstream AI behavior without detection.",
		Category: CategoryCoverageDebt,
		Severity: SeverityHigh,
		Scope:    scope,
		Metric:   fmt.Sprintf("%d partial chains", len(partialChains)),
	})

	return findings
}

// --- Feature: Capability gap detection ---
//
// Detects capabilities that have only positive/accuracy scenarios but no
// negative, adversarial, or safety scenarios. A capability with only
// "does it work?" tests but no "does it fail safely?" tests has a
// validation blind spot.
func capabilityGapFindings(input *BuildInput) []Finding {
	var findings []Finding
	snap := input.Snapshot
	if snap == nil || len(snap.Scenarios) == 0 {
		return findings
	}

	// Group scenarios by capability and classify by category.
	type capInfo struct {
		total      int
		categories map[string]int
	}
	caps := map[string]*capInfo{}
	for _, sc := range snap.Scenarios {
		if sc.Capability == "" {
			continue
		}
		ci, ok := caps[sc.Capability]
		if !ok {
			ci = &capInfo{categories: map[string]int{}}
			caps[sc.Capability] = ci
		}
		ci.total++
		cat := strings.ToLower(sc.Category)
		if cat == "" {
			cat = "general"
		}
		ci.categories[cat]++
	}

	if len(caps) == 0 {
		return findings
	}

	// Negative/safety category keywords.
	negativeCategories := map[string]bool{
		"safety": true, "adversarial": true, "robustness": true,
		"security": true, "edge_case": true, "negative": true,
		"boundary": true, "failure": true, "error_handling": true,
	}

	// Find capabilities with no negative/safety scenarios.
	var gappedCaps []string
	for cap, ci := range caps {
		hasNegative := false
		for cat := range ci.categories {
			if negativeCategories[cat] {
				hasNegative = true
				break
			}
		}
		if !hasNegative && ci.total > 0 {
			cats := make([]string, 0, len(ci.categories))
			for c := range ci.categories {
				cats = append(cats, c)
			}
			sort.Strings(cats)
			gappedCaps = append(gappedCaps, fmt.Sprintf(
				"%s (%d %s: %s)", cap, ci.total, plural(ci.total, "scenario"), strings.Join(cats, ", ")))
		}
	}

	if len(gappedCaps) == 0 {
		return findings
	}

	sort.Strings(gappedCaps)

	findings = append(findings, Finding{
		Title: func() string {
			n := len(gappedCaps)
			if n == 1 {
				return "1 capability has no adversarial or safety scenarios"
			}
			return fmt.Sprintf("%d capabilities have no adversarial or safety scenarios", n)
		}(),
		Description: "These capabilities are validated for correctness (accuracy, quality, regression) " +
			"but have no scenarios testing failure modes, safety boundaries, or adversarial inputs. " +
			"Consider adding scenarios with categories like 'safety', 'adversarial', or 'robustness'.",
		Category: CategoryCoverageDebt,
		Severity: SeverityMedium,
		Scope:    strings.Join(gappedCaps, "; "),
		Metric:   fmt.Sprintf("%d capabilities without negative tests", len(gappedCaps)),
	})

	return findings
}

// --- Recommendation builder ---

func buildRecommendations(findings []Finding, input *BuildInput) []Recommendation {
	var recs []Recommendation

	for _, f := range findings {
		rec := Recommendation{
			Category: f.Category,
		}

		switch {
		case f.Category == CategoryOptimization && strings.Contains(f.Title, "redundant") && len(input.Duplicates.Clusters) > 0:
			rec.Action = fmt.Sprintf("Consolidate %d duplicate test clusters", len(input.Duplicates.Clusters))
			rec.Rationale = "Removing redundant tests reduces CI runtime and maintenance overhead."
			rec.Impact = fmt.Sprintf("~%d fewer tests to maintain", input.Duplicates.DuplicateCount)
			rec.Command = "terrain insights"
			if len(input.Duplicates.Clusters) > 0 {
				cluster := input.Duplicates.Clusters[0]
				for _, t := range cluster.Tests {
					if len(rec.TargetFiles) >= 5 {
						break
					}
					rec.TargetFiles = append(rec.TargetFiles, t)
				}
			}

		case f.Category == CategoryOptimization && strings.Contains(f.Title, "behavior-redundant"):
			rec.Action = f.Title
			rec.Rationale = f.Description
			rec.Impact = "fewer CI cycles spent re-validating identical behavior"
			rec.Command = "terrain insights"

		case f.Category == CategoryOptimization && strings.Contains(f.Title, "scenario pair"):
			rec.Action = "Review overlapping AI eval scenarios"
			rec.Rationale = f.Description
			rec.Impact = "sharper scenario coverage with less duplication"

		case f.Category == CategoryArchitectureDebt && input.Fanout.FlaggedCount > 0:
			rec.Action = "Refactor high-fanout fixtures to reduce blast radius"
			if len(input.Fanout.Entries) > 0 && input.Fanout.Entries[0].Flagged {
				top := input.Fanout.Entries[0]
				label := fanoutLabel(top.NodeID, top.Path, top.NodeType)
				rec.Rationale = fmt.Sprintf("%s fans out to %d dependents — splitting it isolates test impact.", label, top.TransitiveFanout)
			} else {
				rec.Rationale = "High-fanout nodes create fragile dependencies."
			}
			rec.Impact = "narrower test impact per change"
			rec.Command = "terrain debug fanout"
			// Target files from top flagged entries.
			for _, e := range input.Fanout.Entries {
				if !e.Flagged || len(rec.TargetFiles) >= 5 {
					break
				}
				if e.Path != "" {
					rec.TargetFiles = append(rec.TargetFiles, e.Path)
				}
			}

		case f.Category == CategoryCoverageDebt && strings.Contains(f.Title, "coverage gaps"):
			// Matrix gap findings get matrix-specific recommendations.
			rec.Action = f.Title
			rec.Rationale = f.Description
			rec.Impact = "broader device/environment coverage"

		case f.Category == CategoryCoverageDebt && strings.Contains(f.Title, "source files") && input.Coverage.SourceCount > 0:
			lowCount := input.Coverage.BandCounts[depgraph.CoverageBandLow]
			rec.Action = fmt.Sprintf("Add tests for %d uncovered source files", lowCount)
			rec.Rationale = "Coverage gaps mean changes in these files cannot trigger targeted test selection."
			rec.Impact = "improved change-scoped test selection accuracy"
			rec.Command = "terrain analyze --verbose"
			// Target files from lowest-coverage sources. Use Path,
			// not SourceID — SourceID carries the dep-graph node-ID
			// prefix `file:<path>` which leaks into rendered output
			// (the user-visible "files: file:bin/...js" bug).
			for _, src := range input.Coverage.Sources {
				if len(rec.TargetFiles) >= 5 {
					break
				}
				if src.TestCount == 0 {
					rec.TargetFiles = append(rec.TargetFiles, src.Path)
				}
			}

		case f.Category == CategoryReliability && f.Severity == SeverityCritical:
			rec.Action = f.Title
			rec.Rationale = f.Description
			rec.Impact = "reduced risk of missed regressions"
			// Target files from signals matching this finding.
			for _, sig := range input.Snapshot.Signals {
				if len(rec.TargetFiles) >= 5 {
					break
				}
				if sig.Location.File != "" && (sig.Type == signals.SignalFlakyTest || sig.Type == signals.SignalSlowTest) {
					rec.TargetFiles = append(rec.TargetFiles, sig.Location.File)
				}
			}
			if len(rec.TargetFiles) > 0 {
				rec.Command = fmt.Sprintf("terrain show test %s", rec.TargetFiles[0])
			}

		default:
			rec.Action = f.Title
			rec.Rationale = f.Description
		}

		// Derive effort band from target file count.
		rec.EffortBand = effortBand(len(rec.TargetFiles))

		// Deduplicate: skip if we already have a rec with the same action.
		dup := false
		for _, existing := range recs {
			if existing.Action == rec.Action {
				dup = true
				break
			}
		}
		if !dup && rec.Action != "" {
			recs = append(recs, rec)
		}
	}

	// Rank.
	for i := range recs {
		recs[i].Priority = i + 1
	}

	if len(recs) > maxRecommendations {
		recs = recs[:maxRecommendations]
	}

	return recs
}

// effortBand classifies the number of target files into a band.
func effortBand(fileCount int) string {
	switch {
	case fileCount == 0:
		return ""
	case fileCount <= 3:
		return "small"
	case fileCount <= 10:
		return "medium"
	default:
		return "large"
	}
}

// --- Headline and health grade ---

func deriveHeadline(r *Report) string {
	if len(r.Findings) == 0 {
		return "No significant issues detected. Your test system looks healthy."
	}

	top := r.Findings[0]
	return top.Title
}

// Health grade rubric. Each constant names the threshold at which a grade
// flips. Values are uncalibrated heuristics carried forward from 0.1.0;
// docs/health-grade-rubric.md covers what they mean today and what changes
// when the corpus calibration in 0.3 lands. They are extracted as named
// constants so:
//   - the rubric document has stable references to point at
//   - 0.3's recalibration touches a single declaration
//   - tests that exercise grade boundaries don't repeat magic numbers
const (
	healthGradeDHighFindingThreshold   = 3 // > N high findings → D
	healthGradeCMediumFindingThreshold = 3 // > N medium findings → C
)

func deriveHealthGrade(r *Report) string {
	critical := 0
	high := 0
	medium := 0
	for _, f := range r.Findings {
		switch f.Severity {
		case SeverityCritical:
			critical++
		case SeverityHigh:
			high++
		case SeverityMedium:
			medium++
		}
	}

	// Order matters: each clause shadows the next.
	switch {
	case critical > 0:
		return "D" // any Critical → fail
	case high > healthGradeDHighFindingThreshold:
		return "D" // > 3 High → fail
	case high > 0:
		return "C" // 1–3 High → concerning
	case medium > healthGradeCMediumFindingThreshold:
		return "C" // > 3 Medium → concerning
	case medium > 0:
		return "B" // 1–3 Medium → minor issues
	case len(r.Findings) > 0:
		return "B" // any Low/Info finding → minor issues
	default:
		return "A" // clean bill of health
	}
}

// --- Helpers ---

func buildDataCompleteness(input *BuildInput) []DataSource {
	snap := input.Snapshot
	sourceAvailable := len(snap.TestFiles) > 0 || len(snap.CodeUnits) > 0
	return []DataSource{
		{Name: "source", Available: sourceAvailable},
		{Name: "coverage", Available: dsAvailable(snap, "coverage")},
		{Name: "runtime", Available: dsAvailable(snap, "runtime")},
		{Name: "policy", Available: input.HasPolicy || dsAvailable(snap, "policy")},
	}
}

func dsAvailable(snap *models.TestSuiteSnapshot, name string) bool {
	for _, ds := range snap.DataSources {
		if ds.Name == name {
			return ds.Status == models.DataSourceAvailable
		}
	}
	return false
}

func buildLimitations(input *BuildInput) []string {
	var lims []string
	snap := input.Snapshot

	if input.DepgraphSkipped {
		lims = append(lims, input.DepgraphSkipReason)
	}
	if !dsAvailable(snap, "coverage") {
		lims = append(lims, "No coverage data; coverage confidence is structural (import-based) only.")
	}
	if !dsAvailable(snap, "runtime") {
		lims = append(lims, "No runtime data; static skip detection is available, but flaky/slow/dead/unstable signals still require runtime artifacts.")
	}
	if !input.HasPolicy && !dsAvailable(snap, "policy") {
		lims = append(lims, "No policy file found; governance checks skipped.")
	}
	if len(snap.Ownership) == 0 {
		lims = append(lims, "No ownership data; per-team risk attribution unavailable.")
	}

	sort.Strings(lims)
	return lims
}

func severityOrder(s Severity) int {
	switch s {
	case SeverityCritical:
		return 4
	case SeverityHigh:
		return 3
	case SeverityMedium:
		return 2
	case SeverityLow:
		return 1
	default:
		return 0
	}
}

// deduplicateInsightFindings removes findings with the same category + title + scope.
// Sorts by severity descending first so the highest-severity occurrence wins.
func deduplicateInsightFindings(findings []Finding) []Finding {
	// Sort by severity descending so highest severity wins dedup.
	sort.SliceStable(findings, func(i, j int) bool {
		return severityOrder(findings[i].Severity) > severityOrder(findings[j].Severity)
	})
	seen := map[string]bool{}
	var out []Finding
	for _, f := range findings {
		key := string(f.Category) + "|" + f.Title + "|" + f.Scope
		if !seen[key] {
			seen[key] = true
			out = append(out, f)
		}
	}
	return out
}

// fanoutLabel returns a human-readable label for a fanout node.
// Falls back from Path → parsed node ID → node type.
func fanoutLabel(nodeID, path, nodeType string) string {
	if path != "" {
		return path
	}
	parts := strings.SplitN(nodeID, ":", 3)
	if len(parts) >= 3 {
		return parts[2]
	}
	if len(parts) >= 2 {
		return parts[1]
	}
	if nodeType != "" {
		return nodeType
	}
	return nodeID
}

func categoryOrder(c Category) int {
	switch c {
	case CategoryReliability:
		return 1
	case CategoryArchitectureDebt:
		return 2
	case CategoryCoverageDebt:
		return 3
	case CategoryOptimization:
		return 4
	default:
		return 5
	}
}
