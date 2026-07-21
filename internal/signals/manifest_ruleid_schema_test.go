package signals

import (
	"regexp"
	"testing"
)

// ruleIDSchemaPattern mirrors the rule_id pattern published in
// schemas/finding.v1.json. Every manifest RuleID is emitted verbatim into
// .terrain/findings.json, so any ID that fails this pattern makes a strict
// JSON-Schema consumer reject the whole artifact. This test keeps the Go
// manifest and the published schema from drifting apart.
var ruleIDSchemaPattern = regexp.MustCompile(`^terrain/[a-z][a-z-]*/[a-z0-9-]+$`)

func TestManifestRuleIDsMatchPublishedSchema(t *testing.T) {
	t.Parallel()
	for _, e := range Manifest() {
		if e.RuleID == "" {
			continue // not every entry documents a rule id
		}
		if !ruleIDSchemaPattern.MatchString(e.RuleID) {
			t.Errorf("RuleID %q does not match the finding.v1.json rule_id pattern %s — a strict schema validator would reject findings.json", e.RuleID, ruleIDSchemaPattern)
		}
	}
}
