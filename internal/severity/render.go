package severity

import (
	"fmt"
	"strings"
)

// RenderMarkdown returns the canonical markdown rendering of the rubric.
// The output is consumed by `make docs-gen` to produce
// docs/severity-rubric.md. Edits to the rubric must go through the Go
// source; the markdown file is regenerated.
//
// Output is deterministic (severity order, then declaration order within
// each severity) and ends with a trailing newline.
func RenderMarkdown() string {
	var b strings.Builder

	b.WriteString("# Terrain severity rubric\n\n")
	b.WriteString("> **Generated from `internal/severity/rubric.go`. Edits go in code, then `make docs-gen`.**\n\n")
	b.WriteString("Every signal Terrain emits assigns a severity (Critical / High / Medium / Low / Info).\n")
	b.WriteString("This rubric is the source of truth for what each level means.\n\n")
	b.WriteString("Detectors cite one or more clause IDs in the `severityClauses` field of every\n")
	b.WriteString("`Signal` they emit (SignalV2, schema 1.1.0+). The IDs are stable forever — once\n")
	b.WriteString("published, a number is never reused. Retired clauses are marked, not removed.\n\n")
	b.WriteString("Severity ≠ actionability. A Critical-severity finding in a deprecated module may\n")
	b.WriteString("still be Advisory; a Medium finding blocking a release may be Immediate. The\n")
	b.WriteString("`actionability` field on Signal handles that axis separately.\n\n")
	b.WriteString("## Clause table\n\n")

	for _, sev := range SeverityOrder() {
		clauses := BySeverity(sev)
		if len(clauses) == 0 {
			continue
		}
		fmt.Fprintf(&b, "### %s\n\n", titleCase(string(sev)))
		for _, c := range clauses {
			fmt.Fprintf(&b, "#### `%s` — %s\n\n", c.ID, c.Title)
			fmt.Fprintf(&b, "%s\n\n", c.Description)

			if len(c.Examples) > 0 {
				b.WriteString("**Applies when:**\n\n")
				for _, ex := range c.Examples {
					fmt.Fprintf(&b, "- %s\n", ex)
				}
				b.WriteString("\n")
			}
			if len(c.CounterExamples) > 0 {
				b.WriteString("**Does not apply when:**\n\n")
				for _, ex := range c.CounterExamples {
					fmt.Fprintf(&b, "- %s\n", ex)
				}
				b.WriteString("\n")
			}
		}
	}

	b.WriteString("## How to cite\n\n")
	b.WriteString("In a detector that emits a `Signal`, set `SeverityClauses` to the IDs that justify\n")
	b.WriteString("the chosen severity:\n\n")
	b.WriteString("```go\n")
	b.WriteString("models.Signal{\n")
	b.WriteString("    Type:            \"weakAssertion\",\n")
	b.WriteString("    Severity:        models.SeverityMedium,\n")
	b.WriteString("    SeverityClauses: []string{\"sev-medium-001\"},\n")
	b.WriteString("    // ... rest of signal\n")
	b.WriteString("}\n")
	b.WriteString("```\n\n")
	b.WriteString("`internal/severity.ValidateClauseIDs` returns the set of unknown IDs from a list,\n")
	b.WriteString("which detectors and tests use to fail loudly on typos.\n\n")
	b.WriteString("## Calibration ladder\n\n")
	b.WriteString("Clauses are heuristic in 0.2 — author-set based on the rule's structure and the\n")
	b.WriteString("examples above. The 0.2 calibration corpus (50 labeled repos) measures per-clause\n")
	b.WriteString("precision/recall and re-anchors borderline severities. Calibrated clauses gain a\n")
	b.WriteString("`Quality: \"calibrated\"` field on the corresponding `ConfidenceDetail`.\n")

	return b.String()
}

// titleCase upper-cases the first letter of s. Avoids pulling in
// strings.Title (deprecated) or x/text/cases for a one-shot.
func titleCase(s string) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	if r[0] >= 'a' && r[0] <= 'z' {
		r[0] -= 32
	}
	return string(r)
}
