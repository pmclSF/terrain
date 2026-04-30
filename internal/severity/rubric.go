// Package severity defines the canonical severity rubric. Every Severity
// assigned to a Signal cites one or more clauses from this rubric via
// Signal.SeverityClauses. The rubric is the source of truth: the
// human-readable doc at docs/severity-rubric.md is regenerated from it.
//
// Clause IDs follow the format `sev-<severity>-<3-digit-number>`, e.g.
// `sev-critical-001`. Numbers are stable once published — never reuse a
// retired number, just append.
//
// To add a clause:
//  1. Append a Clause to clauses below.
//  2. Run `make docs-gen` so docs/severity-rubric.md tracks.
//  3. CI's `make docs-verify` will fail otherwise.
package severity

import (
	"sort"
	"strings"

	"github.com/pmclSF/terrain/internal/models"
)

// Clause is a single justification entry in the rubric. Detectors reference
// the ID via Signal.SeverityClauses to explain why the chosen Severity
// applies. The same finding may cite multiple clauses; the renderer joins
// them when explaining a signal.
type Clause struct {
	// ID is the stable identifier (e.g. "sev-critical-001"). Matches the
	// regex `^sev-(critical|high|medium|low|info)-[0-9]{3}$`.
	ID string

	// Severity is the level this clause justifies.
	Severity models.SignalSeverity

	// Title is a short human-readable summary used as the section heading
	// in the generated doc.
	Title string

	// Description is the precise statement of when the clause applies.
	// One sentence, plain prose, no examples (those go in Examples).
	Description string

	// Examples lists 1-3 concrete situations where this clause fits.
	Examples []string

	// CounterExamples lists situations that look like this clause but
	// don't actually qualify. Optional; omit for clauses where no common
	// confusion exists.
	CounterExamples []string
}

// clauses is the canonical list. Order is: highest severity first, then
// chronological within a severity. Don't reorder existing entries —
// readers cite by ID, not position.
var clauses = []Clause{
	// ── Critical ───────────────────────────────────────────────────
	{
		ID:          "sev-critical-001",
		Severity:    models.SeverityCritical,
		Title:       "Secret leak with production reach",
		Description: "Code, fixture, or eval config contains a credential that grants production access (API key, signing key, DB DSN with creds, OAuth client secret).",
		Examples: []string{
			"OPENAI_API_KEY=sk-... committed to a YAML eval file",
			"hardcoded AWS access key in a test fixture under tests/",
			"`postgres://user:password@prod-host:5432/db` in a pytest conftest",
		},
		CounterExamples: []string{
			"placeholder strings like \"sk-fake-key\" or \"password123\"",
			"keys clearly scoped to a sandbox / staging / mock service",
		},
	},
	{
		ID:          "sev-critical-002",
		Severity:    models.SeverityCritical,
		Title:       "Destructive AI tool without approval gate",
		Description: "An LLM agent or tool definition can perform an irreversible operation (delete, drop, exec) without an explicit approval gate, sandbox, or dry-run mode.",
		Examples: []string{
			"agent definition includes a `run_shell` tool with no allowlist",
			"`tools/delete_user.py` registered as an MCP tool with no confirmation",
		},
	},
	{
		ID:          "sev-critical-003",
		Severity:    models.SeverityCritical,
		Title:       "CI gate disabled in main",
		Description: "A required pre-merge gate (lint, type-check, test suite) has been silently disabled in the configuration on the default branch.",
		Examples: []string{
			"`continue-on-error: true` added to the only test job",
			"`if: false` block around the entire suite invocation",
		},
		CounterExamples: []string{
			"a single flaky test marked .skip with a tracking ticket",
			"non-blocking informational job (e.g. coverage upload)",
		},
	},

	// ── High ───────────────────────────────────────────────────────
	{
		ID:          "sev-high-001",
		Severity:    models.SeverityHigh,
		Title:       "Weak coverage on changed surface",
		Description: "A symbol or path that just changed has no test coverage AND no nearby test files; releases ship blind.",
		Examples: []string{
			"new exported function added in src/auth/ with no test under test/auth/",
			"file modified in this diff has zero LinkedCodeUnits matches",
		},
	},
	{
		ID:          "sev-high-002",
		Severity:    models.SeverityHigh,
		Title:       "Flaky test failing >10% in last 50 runs",
		Description: "Test fails intermittently at a rate that signals a real reliability issue, not transient noise.",
		Examples: []string{
			"5+ failures over 50 most-recent CI runs of the same test",
			"the test has a documented .retry() or @flaky decorator",
		},
		CounterExamples: []string{
			"single observed failure with no historical context",
			"test failed once in a release-blocking pipeline that was reverted",
		},
	},
	{
		ID:          "sev-high-003",
		Severity:    models.SeverityHigh,
		Title:       "Prompt-injection-shaped concatenation",
		Description: "User-controlled input is concatenated into a prompt without escaping, system-prompt boundaries, or structured input boundaries.",
		Examples: []string{
			"f\"You are an assistant. The user said: {user_input}\"",
			"`prompt += request.body.message` with no validation",
		},
	},
	{
		ID:          "sev-high-004",
		Severity:    models.SeverityHigh,
		Title:       "Missing safety eval on agent surface",
		Description: "An LLM agent or autonomous workflow has no eval scenario covering the documented safety category (jailbreak, harm, leak).",
		Examples: []string{
			"agent.yaml references `tools.execute_code` with no eval covering misuse",
			"deployed prompt has no scenario tagged `category: safety`",
		},
	},

	// ── Medium ─────────────────────────────────────────────────────
	{
		ID:          "sev-medium-001",
		Severity:    models.SeverityMedium,
		Title:       "Weak assertion (semantically loose)",
		Description: "Test uses an assertion shape that passes for many incorrect values (`toBeTruthy`, `assert response`, `assertNotNull`) where a precise match is feasible.",
		Examples: []string{
			"`expect(result).toBeTruthy()` checking a string value",
			"`assertNotNull(user)` instead of `assertEquals(\"alice\", user.name)`",
		},
	},
	{
		ID:          "sev-medium-002",
		Severity:    models.SeverityMedium,
		Title:       "Mock-heavy test (>3 mocks)",
		Description: "Test relies on more than three mocks, creating a tight coupling to implementation that breaks under refactoring.",
		Examples: []string{
			"a unit test that mocks DB, cache, queue, and HTTP client",
		},
	},
	{
		ID:          "sev-medium-003",
		Severity:    models.SeverityMedium,
		Title:       "Non-deterministic eval configuration",
		Description: "An LLM eval runs without temperature pinned to 0 or a deterministic seed, so re-runs produce noisy comparisons.",
		Examples: []string{
			"promptfoo config with no `temperature: 0` or `seed:`",
			"eval scenario uses a model variant with stochastic decoding by default",
		},
	},
	{
		ID:          "sev-medium-004",
		Severity:    models.SeverityMedium,
		Title:       "Duplicate test cluster",
		Description: "Two or more tests share ≥0.60 similarity on test name and assertions, indicating likely copy-paste reduction opportunity.",
		Examples: []string{
			"three tests named `test_login_*` differing only in inputs",
		},
		CounterExamples: []string{
			"intentional parametrize / table-driven cases with shared scaffold",
		},
	},
	{
		ID:          "sev-medium-005",
		Severity:    models.SeverityMedium,
		Title:       "Floating model tag",
		Description: "An LLM call references a model name that resolves to whatever the provider currently maps it to (e.g. `gpt-4`), so behaviour silently drifts.",
		Examples: []string{
			"`model: \"claude-3-opus\"` without a version date suffix",
			"`gpt-4` instead of `gpt-4-0613`",
		},
	},

	// ── Low ────────────────────────────────────────────────────────
	{
		ID:          "sev-low-001",
		Severity:    models.SeverityLow,
		Title:       "Skipped test without ticket reference",
		Description: "A `.skip` / `@pytest.mark.skip` / `@Disabled` annotation has no comment or annotation linking to a tracking ticket.",
		Examples: []string{
			"`it.skip(\"flaky\")` with no follow-up ticket",
		},
	},
	{
		ID:          "sev-low-002",
		Severity:    models.SeverityLow,
		Title:       "Deprecated test pattern in legacy area",
		Description: "Older test idiom (sinon, enzyme, JUnit 4 Hamcrest) used in code outside the active migration scope; correct but inconsistent.",
	},
	{
		ID:          "sev-low-003",
		Severity:    models.SeverityLow,
		Title:       "Slow test (>5s)",
		Description: "Single test runtime exceeds 5 seconds without a documented justification (integration test, container startup).",
		CounterExamples: []string{
			"test annotated as @slow / @integration with policy exemption",
		},
	},

	// ── Info ───────────────────────────────────────────────────────
	{
		ID:          "sev-info-001",
		Severity:    models.SeverityInfo,
		Title:       "Untested export, low blast radius",
		Description: "Exported symbol has no direct test, but is internal-only or has zero callers in the repo's import graph.",
	},
	{
		ID:          "sev-info-002",
		Severity:    models.SeverityInfo,
		Title:       "Non-canonical assertion style",
		Description: "Assertion style differs from the project's prevailing convention (e.g. `expect.toBe` in a project that uses `assert.equal`).",
	},
}

// All returns the rubric in the canonical order.
func All() []Clause {
	out := make([]Clause, len(clauses))
	copy(out, clauses)
	return out
}

// ByID returns the clause with the given ID, and a boolean indicating
// whether the ID was found.
func ByID(id string) (Clause, bool) {
	for _, c := range clauses {
		if c.ID == id {
			return c, true
		}
	}
	return Clause{}, false
}

// BySeverity returns every clause that justifies the given severity, in
// canonical order.
func BySeverity(sev models.SignalSeverity) []Clause {
	var out []Clause
	for _, c := range clauses {
		if c.Severity == sev {
			out = append(out, c)
		}
	}
	return out
}

// SeverityOrder returns severities highest-to-lowest for table rendering.
func SeverityOrder() []models.SignalSeverity {
	return []models.SignalSeverity{
		models.SeverityCritical,
		models.SeverityHigh,
		models.SeverityMedium,
		models.SeverityLow,
		models.SeverityInfo,
	}
}

// ValidateClauseIDs checks that every ID referenced exists in the rubric.
// Used by detectors to fail loudly when a code constant cites an unknown
// clause; also used by the manifest cross-check below.
func ValidateClauseIDs(ids []string) []string {
	var missing []string
	for _, id := range ids {
		if _, ok := ByID(id); !ok {
			missing = append(missing, id)
		}
	}
	sort.Strings(missing)
	return missing
}

// FormatClauseList returns a comma-separated list of clause IDs suitable
// for an explanation footer in CLI output. Empty list → empty string.
func FormatClauseList(ids []string) string {
	if len(ids) == 0 {
		return ""
	}
	return strings.Join(ids, ", ")
}
