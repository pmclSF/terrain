package changescope

import (
	"bytes"
	"strings"
	"testing"
)

func TestDeduplicateFindings_RemovesDuplicates(t *testing.T) {
	t.Parallel()
	findings := []ChangeScopedFinding{
		{Type: "protection_gap", Path: "src/auth.ts", Severity: "high", Explanation: "Function login has no test coverage."},
		{Type: "protection_gap", Path: "src/auth.ts", Severity: "high", Explanation: "Function login has no test coverage."},
		{Type: "protection_gap", Path: "src/auth.ts", Severity: "medium", Explanation: "Different finding."},
		{Type: "existing_signal", Path: "src/auth.ts", Severity: "medium", Explanation: "Weak assertion."},
	}

	deduped := DeduplicateFindings(findings)
	if len(deduped) != 3 {
		t.Errorf("expected 3 unique findings, got %d", len(deduped))
	}
}

func TestDeduplicateFindings_NormalizesExplanation(t *testing.T) {
	t.Parallel()
	findings := []ChangeScopedFinding{
		{Type: "protection_gap", Path: "src/a.ts", Severity: "high", Explanation: "No test coverage."},
		{Type: "protection_gap", Path: "src/a.ts", Severity: "high", Explanation: "No test coverage"},     // missing period
		{Type: "protection_gap", Path: "src/a.ts", Severity: "high", Explanation: "  No test coverage. "}, // extra whitespace
	}

	deduped := DeduplicateFindings(findings)
	if len(deduped) != 1 {
		t.Errorf("expected 1 finding after normalization, got %d", len(deduped))
	}
}

func TestDeduplicateFindings_Empty(t *testing.T) {
	t.Parallel()
	deduped := DeduplicateFindings(nil)
	if len(deduped) != 0 {
		t.Errorf("expected 0, got %d", len(deduped))
	}
}

func TestClassifyFindings_SeparatesNewAndExisting(t *testing.T) {
	t.Parallel()
	findings := []ChangeScopedFinding{
		{Type: "protection_gap", Severity: "high"},
		{Type: "existing_signal", Severity: "medium"},
		{Type: "protection_gap", Severity: "medium"},
		{Type: "existing_signal", Severity: "low"},
	}

	newRisk, existing := ClassifyFindings(findings)
	if len(newRisk) != 2 {
		t.Errorf("expected 2 new risks, got %d", len(newRisk))
	}
	if len(existing) != 2 {
		t.Errorf("expected 2 existing, got %d", len(existing))
	}
}

func TestGroupTestsByPackage(t *testing.T) {
	t.Parallel()
	paths := []string{
		"tests/unit/auth/login.test.ts",
		"tests/unit/auth/session.test.ts",
		"tests/unit/billing/invoice.test.ts",
		"tests/e2e/checkout.test.ts",
	}

	groups := GroupTestsByPackage(paths)
	if len(groups) != 3 {
		t.Fatalf("expected 3 groups, got %d", len(groups))
	}
	// Largest group first.
	if groups[0].Count != 2 {
		t.Errorf("expected largest group to have 2, got %d", groups[0].Count)
	}
}

func TestMergeRecommendation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		band     string
		findings []ChangeScopedFinding
		wantRec  string
	}{
		{"well_protected", nil, "Safe to merge"},
		{"evidence_limited", nil, "Informational only"},
		{"partially_protected", []ChangeScopedFinding{{Severity: "high"}}, "Merge with caution"},
		{"high_risk", []ChangeScopedFinding{{Severity: "medium"}}, "Merge blocked"},
		{"partially_protected", []ChangeScopedFinding{{Severity: "medium"}}, "Merge with caution"},
	}

	for _, tt := range tests {
		rec, _ := MergeRecommendation(tt.band, tt.findings)
		if rec != tt.wantRec {
			t.Errorf("MergeRecommendation(%q, %d findings) = %q, want %q",
				tt.band, len(tt.findings), rec, tt.wantRec)
		}
	}
}

// TestRenderPRSummaryMarkdown_EmptyPRCallout locks the
// pr_change_scoped.V3 audit lift: a clean PR (no findings, no AI
// risk, no protection gaps) renders an "All clear" callout
// before the footer instead of falling through to a thin comment
// that reads as broken.
func TestRenderPRSummaryMarkdown_EmptyPRCallout(t *testing.T) {
	t.Parallel()
	pr := &PRAnalysis{
		PostureBand:        "well_protected",
		ChangedFileCount:   3,
		ChangedSourceCount: 2,
		ChangedTestCount:   1,
		ImpactedUnitCount:  0,
		ProtectionGapCount: 0,
		TotalTestCount:     50,
		// No NewFindings, no AI, no RecommendedTests.
	}
	var buf bytes.Buffer
	RenderPRSummaryMarkdown(&buf, pr)
	output := buf.String()

	if !strings.Contains(output, "All clear") {
		t.Errorf("clean PR should render the All clear callout; got:\n%s", output)
	}
	if !strings.Contains(output, "terrain compare") {
		t.Errorf("All clear callout should suggest `terrain compare`; got:\n%s", output)
	}
}

// TestRenderPRSummaryMarkdown_AllClearOnlyOnEmpty locks the inverse:
// a PR with findings should NOT render the All clear callout.
func TestRenderPRSummaryMarkdown_AllClearOnlyOnEmpty(t *testing.T) {
	t.Parallel()
	pr := &PRAnalysis{
		PostureBand: "weakly_protected",
		NewFindings: []ChangeScopedFinding{
			{Type: "weakAssertion", Scope: "direct", Path: "src/x.ts", Severity: "high", Explanation: "self-comparison"},
		},
	}
	var buf bytes.Buffer
	RenderPRSummaryMarkdown(&buf, pr)

	if strings.Contains(buf.String(), "All clear") {
		t.Errorf("PR with findings should NOT render the All clear callout; got:\n%s", buf.String())
	}
}

// TestBuildConfidenceHistogram_GroupsAndPluralizes locks the
// pr_change_scoped.E3 audit lift: a one-line summary showing how
// the recommended test set distributes by confidence. Stable order
// (first-seen) keeps the output deterministic across runs.
func TestBuildConfidenceHistogram_GroupsAndPluralizes(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   []TestSelection
		want string
	}{
		{
			name: "single",
			in:   []TestSelection{{Path: "a", Confidence: "exact"}},
			want: "**Confidence:** 1 exact (1 test selected)",
		},
		{
			name: "mixed",
			in: []TestSelection{
				{Path: "a", Confidence: "exact"},
				{Path: "b", Confidence: "exact"},
				{Path: "c", Confidence: "inferred"},
				{Path: "d", Confidence: "weak"},
			},
			want: "**Confidence:** 2 exact · 1 inferred · 1 weak (4 tests selected)",
		},
		{
			name: "empty",
			in:   nil,
			want: "",
		},
		{
			name: "missing-confidence",
			in:   []TestSelection{{Path: "a"}},
			want: "**Confidence:** 1 unknown (1 test selected)",
		},
	}
	for _, tc := range cases {
		got := buildConfidenceHistogram(tc.in)
		if got != tc.want {
			t.Errorf("%s: got %q, want %q", tc.name, got, tc.want)
		}
	}
}

// TestRenderPRSummaryMarkdown_DeterministicUnderSourceDateEpoch
// locks the pr_change_scoped.E6 audit lift: byte-identical output
// when SOURCE_DATE_EPOCH varies. The PR comment shouldn't carry any
// timestamp that drifts between runs of the same snapshot — every
// finding has its own timing data inside the snapshot, but the
// comment surface itself is timestamp-free.
func TestRenderPRSummaryMarkdown_DeterministicUnderSourceDateEpoch(t *testing.T) {
	pr := &PRAnalysis{
		PostureBand:        "well_protected",
		ChangedFileCount:   2,
		ChangedSourceCount: 1,
		ChangedTestCount:   1,
		ImpactedUnitCount:  3,
		TotalTestCount:     50,
		NewFindings: []ChangeScopedFinding{
			{Type: "weakAssertion", Scope: "direct", Path: "src/auth.go", Severity: "medium", Explanation: "self-comparison"},
		},
		RecommendedTests: []string{"src/auth_test.go"},
	}

	t.Setenv("SOURCE_DATE_EPOCH", "1700000000")
	var buf1 bytes.Buffer
	RenderPRSummaryMarkdown(&buf1, pr)

	t.Setenv("SOURCE_DATE_EPOCH", "1900000000")
	var buf2 bytes.Buffer
	RenderPRSummaryMarkdown(&buf2, pr)

	if buf1.String() != buf2.String() {
		t.Errorf("PR markdown should be timestamp-independent.\nepoch=1700000000:\n%s\n\nepoch=1900000000:\n%s",
			buf1.String(), buf2.String())
	}
}

func TestRenderPRSummaryMarkdown_Deterministic(t *testing.T) {
	t.Parallel()
	pr := &PRAnalysis{
		PostureBand:        "partially_protected",
		ChangedFileCount:   5,
		ChangedSourceCount: 3,
		ChangedTestCount:   2,
		ImpactedUnitCount:  4,
		ProtectionGapCount: 2,
		TotalTestCount:     100,
		NewFindings: []ChangeScopedFinding{
			{Type: "protection_gap", Scope: "direct", Path: "src/auth.ts", Severity: "high", Explanation: "No coverage"},
			{Type: "protection_gap", Scope: "indirect", Path: "src/utils.ts", Severity: "medium", Explanation: "Indirectly impacted"},
			{Type: "existing_signal", Scope: "direct", Path: "src/db.ts", Severity: "medium", Explanation: "[weakAssertion] Weak"},
		},
		TestSelections: []TestSelection{
			{Path: "tests/auth.test.ts", Confidence: "exact", CoversUnits: []string{"src/auth.ts:login"}},
		},
		RecommendedTests: []string{"tests/auth.test.ts"},
	}

	var buf1, buf2 bytes.Buffer
	RenderPRSummaryMarkdown(&buf1, pr)
	RenderPRSummaryMarkdown(&buf2, pr)

	if buf1.String() != buf2.String() {
		t.Error("markdown render is not deterministic")
	}

	output := buf1.String()
	if !strings.Contains(output, "Merge with caution") {
		t.Error("expected merge recommendation in output")
	}
	if !strings.Contains(output, "Coverage gaps in changed code") {
		t.Error("expected direct risks section")
	}
	if !strings.Contains(output, "Indirectly impacted") {
		t.Error("expected indirect risks section")
	}
	if !strings.Contains(output, "Pre-existing issues") {
		t.Error("expected pre-existing issues section")
	}
	if !strings.Contains(output, "1 of 100") {
		t.Error("expected suite size context in tests row")
	}
}

func TestRenderPRSummaryMarkdown_LargeTestSet_GroupsByPackage(t *testing.T) {
	t.Parallel()

	var selections []TestSelection
	var recommended []string
	for i := 0; i < 25; i++ {
		pkg := "tests/unit/auth"
		if i >= 10 {
			pkg = "tests/unit/billing"
		}
		if i >= 20 {
			pkg = "tests/e2e"
		}
		path := pkg + "/" + strings.Repeat("x", i) + ".test.ts"
		selections = append(selections, TestSelection{Path: path, Confidence: "inferred"})
		recommended = append(recommended, path)
	}

	pr := &PRAnalysis{
		PostureBand:      "partially_protected",
		TestSelections:   selections,
		RecommendedTests: recommended,
	}

	var buf bytes.Buffer
	RenderPRSummaryMarkdown(&buf, pr)
	output := buf.String()

	// Should group by package, not list 25 individual files.
	if strings.Contains(output, "| Confidence |") {
		t.Error("expected grouped table, not individual test table for 25+ tests")
	}
	if !strings.Contains(output, "Package") {
		t.Error("expected Package grouping header")
	}
}

func TestRenderPRSummaryMarkdown_FindingTruncation(t *testing.T) {
	t.Parallel()

	var findings []ChangeScopedFinding
	for i := 0; i < 20; i++ {
		findings = append(findings, ChangeScopedFinding{
			Type:        "protection_gap",
			Path:        "src/file_" + strings.Repeat("a", i) + ".ts",
			Severity:    "medium",
			Explanation: "No coverage",
		})
	}

	pr := &PRAnalysis{
		PostureBand: "weakly_protected",
		NewFindings: findings,
	}

	var buf bytes.Buffer
	RenderPRSummaryMarkdown(&buf, pr)
	output := buf.String()

	// Truncation message — italicized "...and N more (severity counts)"
	// in the new card-style render.
	if !strings.Contains(output, "_...and 10 more") {
		t.Errorf("expected truncation message for >10 findings; got:\n%s", output)
	}
}

func TestClassifyFindingsDetailed_ThreeWay(t *testing.T) {
	t.Parallel()
	findings := []ChangeScopedFinding{
		{Type: "protection_gap", Scope: "direct", Severity: "high", Path: "src/auth.ts"},
		{Type: "protection_gap", Scope: "indirect", Severity: "medium", Path: "src/utils.ts"},
		{Type: "protection_gap", Scope: "direct", Severity: "medium", Path: "src/db.ts"},
		{Type: "existing_signal", Scope: "direct", Severity: "low", Path: "src/auth.ts"},
	}

	direct, indirect, existing := ClassifyFindingsDetailed(findings)
	if len(direct) != 2 {
		t.Errorf("expected 2 direct, got %d", len(direct))
	}
	if len(indirect) != 1 {
		t.Errorf("expected 1 indirect, got %d", len(indirect))
	}
	if len(existing) != 1 {
		t.Errorf("expected 1 existing, got %d", len(existing))
	}
}

func TestRenderPRSummaryMarkdown_SuiteSizeContext(t *testing.T) {
	t.Parallel()
	pr := &PRAnalysis{
		PostureBand:      "well_protected",
		TotalTestCount:   200,
		RecommendedTests: []string{"a.test.ts", "b.test.ts"},
		TestSelections: []TestSelection{
			{Path: "a.test.ts", Confidence: "exact"},
			{Path: "b.test.ts", Confidence: "inferred"},
		},
	}
	var buf bytes.Buffer
	RenderPRSummaryMarkdown(&buf, pr)
	output := buf.String()
	if !strings.Contains(output, "2 of 200") {
		t.Errorf("expected suite size '2 of 200', got:\n%s", output)
	}
	if !strings.Contains(output, "1%% of suite") && !strings.Contains(output, "1% of suite") {
		t.Errorf("expected percentage in output, got:\n%s", output)
	}
}

// TestRenderPRSummaryMarkdown_DirectVsIndirectSections verifies the
// 0.2 layout: directly-changed coverage gaps appear as a top-level
// section ("Coverage gaps in changed code"), indirectly-impacted gaps
// appear inside a collapsed `<details>` block (visual hierarchy:
// direct = scannable on first read, indirect = available on demand).
//
// Pre-fix headings were "New Risks (directly changed)" and
// "Indirectly impacted protection gaps (N)". The new headings prefer
// sentence case and proper pluralization.
func TestRenderPRSummaryMarkdown_DirectVsIndirectSections(t *testing.T) {
	t.Parallel()
	pr := &PRAnalysis{
		PostureBand: "weakly_protected",
		NewFindings: []ChangeScopedFinding{
			{Type: "protection_gap", Scope: "direct", Path: "src/a.ts", Severity: "high", Explanation: "Direct gap"},
			{Type: "protection_gap", Scope: "indirect", Path: "src/b.ts", Severity: "medium", Explanation: "Indirect gap"},
		},
	}
	var buf bytes.Buffer
	RenderPRSummaryMarkdown(&buf, pr)
	output := buf.String()

	// Direct risks heading + card-shape bullet.
	if !strings.Contains(output, "### Coverage gaps in changed code") {
		t.Errorf("expected 'Coverage gaps in changed code' heading; got:\n%s", output)
	}
	if !strings.Contains(output, "**`src/a.ts`** [HIGH] — Direct gap") {
		t.Errorf("expected card-shape direct finding; got:\n%s", output)
	}

	// Indirect risks in collapsed section — singular "gap" for count=1.
	if !strings.Contains(output, "1 indirectly impacted protection gap") {
		t.Errorf("expected indirect section with count; got:\n%s", output)
	}
	if !strings.Contains(output, "**`src/b.ts`** [MED] — Indirect gap") {
		t.Errorf("expected card-shape indirect finding; got:\n%s", output)
	}
}

func TestRenderChangeScopedReport_ShowsTestReduction(t *testing.T) {
	t.Parallel()
	pr := &PRAnalysis{
		PostureBand:      "well_protected",
		TotalTestCount:   50,
		RecommendedTests: []string{"a.test.ts"},
		TestSelections:   []TestSelection{{Path: "a.test.ts", Confidence: "exact"}},
	}
	var buf bytes.Buffer
	RenderChangeScopedReport(&buf, pr)
	output := buf.String()
	if !strings.Contains(output, "1 of 50") {
		t.Errorf("expected '1 of 50' in text report, got:\n%s", output)
	}
}

func TestSummarizeFindingsBySeverity(t *testing.T) {
	t.Parallel()
	findings := []ChangeScopedFinding{
		{Severity: "high"}, {Severity: "high"}, {Severity: "medium"}, {Severity: "low"},
	}
	counts := SummarizeFindingsBySeverity(findings)
	if counts["high"] != 2 || counts["medium"] != 1 || counts["low"] != 1 {
		t.Errorf("unexpected counts: %v", counts)
	}
}

// --- AI PR Section Tests ---

// TestRenderPRSummaryMarkdown_AISection asserts the 0.2 contract for
// the AI Risk Review section in `terrain pr --format markdown` output.
//
// Pre-fix the section dumped one bullet per signal with the detector
// taxonomy (`aiPromptInjectionRisk`) as the headline and no file
// path. After the fix:
//   - bullets are grouped by (file, type), so 12 prompt-injection hits
//     across 4 files become 4 bullets
//   - each bullet leads with `**\`path:line[, line, line]\`**` so the
//     file is the navigation target, not the taxonomy
//   - the bullet text is the plain-language summary from
//     `humanSummary`, not the raw detector explanation
//   - a `→ <action>` line follows with the concrete next step
//   - the section header reads "N new finding(s) introduced by this
//     PR" instead of "Blocking signals (N)"
func TestRenderPRSummaryMarkdown_AISection(t *testing.T) {
	t.Parallel()
	pr := &PRAnalysis{
		PostureBand: "partially_protected",
		AI: &AIValidationSummary{
			ImpactedCapabilities: []string{"refund-explanation", "enterprise-search"},
			SelectedScenarios:    3,
			TotalScenarios:       8,
			Scenarios: []AIScenarioSummary{
				{Name: "refund-accuracy", Capability: "refund-explanation", Reason: "context template changed (policyBlock)"},
				{Name: "search-citations", Capability: "enterprise-search", Reason: "retrieval config changed (chunkConfig)"},
				{Name: "safety-guardrail", Capability: "refund-explanation", Reason: "prompt changed (safetyOverlay)"},
			},
			BlockingSignals: []AISignalSummary{
				{Type: "aiPromptInjectionRisk", Severity: "high", Explanation: "raw detector text",
					File: "src/auth/login.ts", Line: 42},
				{Type: "aiPromptInjectionRisk", Severity: "high", Explanation: "raw detector text",
					File: "src/auth/login.ts", Line: 47},
				{Type: "aiToolWithoutSandbox", Severity: "high", Explanation: "raw detector text",
					File: "src/agent/tools.yaml", Symbol: "delete_user"},
			},
			WarningSignals: []AISignalSummary{
				{Type: "aiNonDeterministicEval", Severity: "medium", Explanation: "raw detector text",
					File: "evals/agent.yaml", Line: 12},
			},
			UncoveredContexts: []string{"customerContext (src/context.ts)"},
		},
	}

	var buf bytes.Buffer
	RenderPRSummaryMarkdown(&buf, pr)
	output := buf.String()

	// Section header.
	if !strings.Contains(output, "### AI Risk Review") {
		t.Error("expected AI Risk Review section")
	}
	// Capabilities + scenario count framing.
	if !strings.Contains(output, "refund-explanation") || !strings.Contains(output, "enterprise-search") {
		t.Error("expected impacted capabilities listed")
	}
	if !strings.Contains(output, "3 of 8 selected") {
		t.Error("expected scenario count '3 of 8 selected'")
	}
	// Blocking section is now framed in terms of new findings on this PR.
	// Properly pluralized: "findings" for >1, "finding" for 1.
	if !strings.Contains(output, "new findings introduced by this PR") {
		t.Errorf("expected new-findings framing in output; got:\n%s", output)
	}
	// Two prompt-injection hits in the same file should collapse to ONE
	// bullet with both line numbers.
	if !strings.Contains(output, "src/auth/login.ts:42, 47") {
		t.Errorf("expected grouped file:line locator `src/auth/login.ts:42, 47`; got:\n%s", output)
	}
	// Plain-language summary, not raw detector text.
	if !strings.Contains(output, "User input flows into a prompt without visible escaping") {
		t.Error("expected plain-language summary for aiPromptInjectionRisk")
	}
	// Concrete action arrow.
	if !strings.Contains(output, "→ Wrap user input through a sanitizer") {
		t.Error("expected actionable next step for aiPromptInjectionRisk")
	}
	// Symbol-keyed locator for the tool finding (no line number).
	if !strings.Contains(output, "src/agent/tools.yaml (delete_user)") {
		t.Errorf("expected symbol-keyed locator for tool finding; got:\n%s", output)
	}
	// Warning signals collapsed under a details block. Singular for 1.
	if !strings.Contains(output, "1 advisory finding") {
		t.Errorf("expected advisory-finding framing for warnings; got:\n%s", output)
	}
	// Raw detector taxonomy should NOT appear as the bold headline.
	// Confirm by looking for the pre-fix shape `[HIGH] **aiPromptInjectionRisk**:`.
	if strings.Contains(output, "**aiPromptInjectionRisk**:") {
		t.Error("raw detector taxonomy leaked into headline; should be plain-language summary")
	}
	// Uncovered contexts unchanged.
	if !strings.Contains(output, "customerContext") {
		t.Error("expected uncovered context")
	}
}

func TestRenderPRSummaryMarkdown_NoAISection(t *testing.T) {
	t.Parallel()
	pr := &PRAnalysis{
		PostureBand: "well_protected",
		// No AI field — traditional-only PR.
	}

	var buf bytes.Buffer
	RenderPRSummaryMarkdown(&buf, pr)
	output := buf.String()

	if strings.Contains(output, "AI Risk Review") {
		t.Error("expected no AI section for traditional PR")
	}
}

func TestRenderPRSummaryMarkdown_MixedTraditionalAndAI(t *testing.T) {
	t.Parallel()
	pr := &PRAnalysis{
		PostureBand:        "partially_protected",
		ChangedFileCount:   5,
		ChangedSourceCount: 3,
		ChangedTestCount:   2,
		ImpactedUnitCount:  4,
		ProtectionGapCount: 1,
		TotalTestCount:     50,
		NewFindings: []ChangeScopedFinding{
			{Type: "protection_gap", Scope: "direct", Path: "src/auth.ts", Severity: "high", Explanation: "No coverage"},
		},
		RecommendedTests: []string{"tests/auth.test.ts"},
		TestSelections:   []TestSelection{{Path: "tests/auth.test.ts", Confidence: "exact"}},
		AI: &AIValidationSummary{
			ImpactedCapabilities: []string{"search"},
			SelectedScenarios:    1,
			TotalScenarios:       3,
			Scenarios: []AIScenarioSummary{
				{Name: "search-quality", Capability: "search", Reason: "retrieval config changed"},
			},
		},
	}

	var buf bytes.Buffer
	RenderPRSummaryMarkdown(&buf, pr)
	output := buf.String()

	// Both traditional and AI sections present (sentence-case headings
	// per the 0.2 layout).
	if !strings.Contains(output, "Coverage gaps in changed code") {
		t.Error("expected traditional Coverage gaps section")
	}
	if !strings.Contains(output, "Recommended tests") {
		t.Error("expected traditional Recommended tests section")
	}
	if !strings.Contains(output, "### AI Risk Review") {
		t.Error("expected AI Risk Review section")
	}
	if !strings.Contains(output, "search-quality") {
		t.Error("expected AI scenario in output")
	}
}

func TestRenderPRSummaryMarkdown_AISection_Deterministic(t *testing.T) {
	t.Parallel()
	pr := &PRAnalysis{
		PostureBand: "well_protected",
		AI: &AIValidationSummary{
			ImpactedCapabilities: []string{"billing", "auth"},
			SelectedScenarios:    2,
			TotalScenarios:       5,
			Scenarios: []AIScenarioSummary{
				{Name: "billing-accuracy", Capability: "billing", Reason: "prompt changed"},
				{Name: "auth-safety", Capability: "auth", Reason: "context changed"},
			},
		},
	}

	var buf1, buf2 bytes.Buffer
	RenderPRSummaryMarkdown(&buf1, pr)
	RenderPRSummaryMarkdown(&buf2, pr)

	if buf1.String() != buf2.String() {
		t.Error("AI section rendering is not deterministic")
	}
}
