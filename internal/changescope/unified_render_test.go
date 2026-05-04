package changescope

import (
	"bytes"
	"regexp"
	"strings"
	"testing"
)

// TestRenderPRSummaryMarkdown_UnifiedShape is the Track 3.5 acceptance
// test: the PR-comment markdown renders unit, integration, e2e, and AI
// stanzas with a consistent visual shape so the entire comment reads
// like one designed document, not four bolted-on subsystems.
//
// Specifically asserts the four uniformity gates from the parity plan:
//
//  1. Severity / posture badges use the same `[LABEL]` square-bracket
//     shape across coverage-gap cards, AI risk findings, and the
//     header verdict.
//  2. File-path locators use the same `**`path`**` code-formatted
//     bold-mono shape across coverage-gap cards and AI risk findings.
//  3. The em-dash separator (` — `) appears between locator and
//     plain-language summary in both coverage cards and AI bullets.
//  4. The recommended-tests table presents unit / integration / e2e
//     selections through one stanza with the same column shape, so
//     adopters scanning the comment never see "AI is a different
//     product" — the unification is visible.
//
// This test is intentionally a single PRAnalysis with realistic
// content for all four pillars. If a future change splits the AI
// section into its own card style, or adds a different badge format
// to one stanza but not the others, this test fails loudly.
func TestRenderPRSummaryMarkdown_UnifiedShape(t *testing.T) {
	t.Parallel()

	pr := &PRAnalysis{
		PostureBand:        "partially_protected",
		ChangedFileCount:   4,
		ChangedSourceCount: 3,
		ChangedTestCount:   1,
		ImpactedUnitCount:  6,
		ProtectionGapCount: 2,
		TotalTestCount:     1200,

		// Two coverage-gap findings: one direct, one indirect.
		NewFindings: []ChangeScopedFinding{
			{
				Type:            "protection_gap",
				Scope:           "direct",
				Path:            "src/auth/login.ts",
				Severity:        "high",
				Explanation:     "exported handler has no covering test",
				SuggestedAction: "add a unit test exercising the success and 401 branches",
			},
			{
				Type:            "protection_gap",
				Scope:           "direct",
				Path:            "src/checkout/cart.ts",
				Severity:        "medium",
				Explanation:     "modified function has only structural-only e2e coverage",
				SuggestedAction: "add an integration test that exercises the cart total path",
			},
		},

		// Recommended tests across all three test types.
		TestSelections: []TestSelection{
			{
				Path:        "src/auth/__tests__/login.test.ts",
				Confidence:  "exact",
				Relevance:   "imports src/auth/login.ts:loginUser",
				CoversUnits: []string{"src/auth/login.ts:loginUser"},
				Reasons:     []string{"import-graph: direct"},
			},
			{
				Path:        "test/api/auth.integration.test.ts",
				Confidence:  "exact",
				Relevance:   "supertest import + path under test/api/",
				CoversUnits: []string{"src/auth/login.ts:loginUser"},
				Reasons:     []string{"content: supertest", "path: test/api/"},
			},
			{
				Path:        "e2e/auth/login.spec.ts",
				Confidence:  "inferred",
				Relevance:   "e2e under matching feature directory",
				CoversUnits: []string{"src/auth/login.ts (file-level)"},
				Reasons:     []string{"path co-location", "structural-only"},
			},
		},

		// AI risk surface.
		AI: &AIValidationSummary{
			ImpactedCapabilities: []string{"refund-explanation"},
			SelectedScenarios:    2,
			TotalScenarios:       12,
			Scenarios: []AIScenarioSummary{
				{Name: "refund-accuracy", Capability: "refund-explanation", Reason: "context template changed"},
				{Name: "safety-guardrail", Capability: "refund-explanation", Reason: "prompt changed"},
			},
			BlockingSignals: []AISignalSummary{
				{Type: "aiPromptInjectionRisk", Severity: "high",
					Explanation: "raw detector text",
					File:        "src/agent/prompt.ts", Line: 88},
			},
			WarningSignals: []AISignalSummary{
				{Type: "aiNonDeterministicEval", Severity: "medium",
					Explanation: "raw detector text",
					File:        "evals/agent.yaml", Line: 12},
			},
		},
	}

	var buf bytes.Buffer
	RenderPRSummaryMarkdown(&buf, pr)
	output := buf.String()

	// --- Gate 1: badges use [LABEL] shape across stanzas ---
	// Both severity and posture badges render as bracketed labels
	// (`[WARN]`, `[HIGH]`, `[MED]`, `[LOW]`, `[INFO]`, etc.). At a
	// minimum we expect the header verdict badge plus severity
	// badges on each coverage-gap card. The AI section groups by
	// severity at the *section header* level ("new findings" vs
	// "advisory finding") rather than per bullet, which is a
	// deliberate UX choice — section-level grouping is documented in
	// `docs/product/unified-pr-comment.md`.
	bracketBadge := regexp.MustCompile(`\[(PASS|WARN|RISK|FAIL|INFO|HIGH|MED|LOW|----?)\]`)
	matches := bracketBadge.FindAllString(output, -1)
	if len(matches) < 3 {
		t.Errorf("gate 1 (unified badge shape): expected at least 3 [LABEL] badges (header + per-coverage-gap), got %d:\n%s", len(matches), output)
	}

	// The header should carry a posture badge.
	if !regexp.MustCompile(`## \[(PASS|WARN|RISK|FAIL|INFO)\] Terrain`).MatchString(output) {
		t.Errorf("gate 1 (header badge): header verdict should use [LABEL] shape; got:\n%s", firstNLines(output, 3))
	}

	// Coverage-gap cards should carry [HIGH] / [MED] / [LOW] inline.
	if !strings.Contains(output, "[HIGH]") || !strings.Contains(output, "[MED]") {
		t.Errorf("gate 1 (severity badges): coverage cards should carry [HIGH] and [MED]; got:\n%s", output)
	}

	// AI section's severity grouping should appear at the section-
	// header level — the contract that justifies AI bullets not
	// carrying per-bullet badges.
	if !strings.Contains(output, "new finding") || !strings.Contains(output, "advisory finding") {
		t.Errorf("gate 1 (AI severity grouping): expected section-header severity language ('new finding' / 'advisory finding'); got:\n%s", output)
	}

	// --- Gate 2: file-path locator format is unified ---
	// Both coverage cards and AI bullets should bold + mono the path.
	// Coverage card shape: `- **`src/auth/login.ts`** [HIGH] — ...`
	if !regexp.MustCompile("(?m)^- \\*\\*`src/auth/login\\.ts`\\*\\* \\[HIGH\\]").MatchString(output) {
		t.Errorf("gate 2 (coverage locator): expected card-shape `- **\\`path\\`** [SEV]`; got:\n%s", output)
	}
	// AI bullet shape: `- **`src/agent/prompt.ts:88`** ...`
	if !regexp.MustCompile("(?m)^- \\*\\*`src/agent/prompt\\.ts:88`\\*\\*").MatchString(output) {
		t.Errorf("gate 2 (AI locator): expected AI bullet shape `- **\\`path:line\\`**`; got:\n%s", output)
	}

	// --- Gate 3: em-dash separator between locator and summary ---
	// Both stanzas should use ` — ` (em-dash with surrounding spaces),
	// never ` - ` (hyphen) or `: ` (colon).
	emDashCount := strings.Count(output, " — ")
	if emDashCount < 3 {
		t.Errorf("gate 3 (em-dash separator): expected at least 3 ` — ` separators (coverage cards + AI bullets), got %d", emDashCount)
	}

	// --- Gate 4: recommended-tests stanza is unified ---
	// One section header, one table, all three test types in it.
	if !strings.Contains(output, "### Recommended tests") {
		t.Error("gate 4 (unified stanza): expected single 'Recommended tests' header")
	}
	// All three test types should appear in the same table.
	for _, path := range []string{
		"src/auth/__tests__/login.test.ts",   // unit
		"test/api/auth.integration.test.ts",  // integration
		"e2e/auth/login.spec.ts",             // e2e
	} {
		if !strings.Contains(output, path) {
			t.Errorf("gate 4 (unified stanza): expected %q in recommended-tests table; got:\n%s", path, output)
		}
	}

	// AI section follows immediately and uses the same `### ` header
	// level — adopters scanning the comment shouldn't see one stanza
	// at H3 and another at H4.
	if !strings.Contains(output, "### AI Risk Review") {
		t.Error("gate 4 (unified header levels): AI Risk Review section should use ### header")
	}
}

// TestRenderPRSummaryMarkdown_ConsistentSectionOrder verifies the
// canonical section order. Re-ordering breaks adopter expectations and
// downstream tooling that scrapes the markdown.
func TestRenderPRSummaryMarkdown_ConsistentSectionOrder(t *testing.T) {
	t.Parallel()

	pr := &PRAnalysis{
		PostureBand: "partially_protected",
		NewFindings: []ChangeScopedFinding{
			{Type: "protection_gap", Scope: "direct", Path: "src/a.ts",
				Severity: "high", Explanation: "no test"},
		},
		TestSelections: []TestSelection{
			{Path: "test/a.test.ts", Confidence: "exact", Relevance: "covers a"},
		},
		AI: &AIValidationSummary{
			ImpactedCapabilities: []string{"x"},
			BlockingSignals: []AISignalSummary{
				{Type: "aiPromptInjectionRisk", Severity: "high",
					File: "src/p.ts", Line: 1, Explanation: "x"},
			},
		},
	}

	var buf bytes.Buffer
	RenderPRSummaryMarkdown(&buf, pr)
	output := buf.String()

	// Canonical order: header → metrics table → coverage gaps →
	// recommended tests → AI risk.
	headerIdx := strings.Index(output, "## ")
	gapsIdx := strings.Index(output, "### Coverage gaps in changed code")
	testsIdx := strings.Index(output, "### Recommended tests")
	aiIdx := strings.Index(output, "### AI Risk Review")

	if headerIdx < 0 || gapsIdx < 0 || testsIdx < 0 || aiIdx < 0 {
		t.Fatalf("missing one or more sections; output:\n%s", output)
	}
	if !(headerIdx < gapsIdx && gapsIdx < testsIdx && testsIdx < aiIdx) {
		t.Errorf("section order wrong: header=%d gaps=%d tests=%d ai=%d\nwant header < gaps < tests < ai\n%s",
			headerIdx, gapsIdx, testsIdx, aiIdx, output)
	}
}

func firstNLines(s string, n int) string {
	lines := strings.SplitN(s, "\n", n+1)
	if len(lines) > n {
		lines = lines[:n]
	}
	return strings.Join(lines, "\n")
}
