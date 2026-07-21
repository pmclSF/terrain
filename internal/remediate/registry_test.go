package remediate

import (
	"testing"

	"github.com/pmclSF/terrain/internal/findings"
)

func ruleFinding(rule, path, text string) findings.Finding {
	f := findings.Finding{
		Version: findings.SchemaVersion, RuleID: rule, Severity: findings.SeverityWarning,
		PrimaryLoc: findings.Location{Path: path}, ShortMessage: "m", DocsURL: "d",
	}
	if text != "" {
		f.Suggestions = []findings.Suggestion{{Text: text}}
	}
	return f
}

// TestAttach_PopulatesFixAndPreservesText: a registered producer attaches a
// structured Fix to the existing text suggestion without dropping the text.
func TestAttach_PopulatesFixAndPreservesText(t *testing.T) {
	t.Parallel()
	reg := NewFixRegistry()
	reg.Register("terrain/deps/drift-risk", func(_ string, f findings.Finding) *findings.Fix {
		return &findings.Fix{Kind: findings.FixEditInPlace, Path: f.PrimaryLoc.Path, Content: "pinned"}
	})

	fs := []findings.Finding{ruleFinding("terrain/deps/drift-risk", "package.json", "Pin your deps")}
	if n := reg.Attach("/repo", fs); n != 1 {
		t.Fatalf("attached %d, want 1", n)
	}
	sg := fs[0].Suggestions[0]
	if sg.Text != "Pin your deps" {
		t.Errorf("text dropped: %q", sg.Text)
	}
	if sg.Fix == nil || sg.Fix.Kind != findings.FixEditInPlace {
		t.Errorf("Fix not attached: %+v", sg.Fix)
	}
}

// TestAttach_DeclinedProducerLeavesJudgeOnly: a producer that returns nil
// (e.g. strict-pin deps) leaves the finding as a text-only, judge-only
// suggestion — no Fix.
func TestAttach_DeclinedProducerLeavesJudgeOnly(t *testing.T) {
	t.Parallel()
	reg := NewFixRegistry()
	reg.Register("terrain/deps/drift-risk", func(string, findings.Finding) *findings.Fix { return nil })

	fs := []findings.Finding{ruleFinding("terrain/deps/drift-risk", "package.json", "Pin your deps")}
	if n := reg.Attach("/repo", fs); n != 0 {
		t.Errorf("attached %d, want 0", n)
	}
	if fs[0].Suggestions[0].Fix != nil {
		t.Error("declined producer must not attach a Fix")
	}
}

// TestAttach_UnregisteredRuleUntouched: findings whose rule has no producer
// pass through unchanged.
func TestAttach_UnregisteredRuleUntouched(t *testing.T) {
	t.Parallel()
	reg := NewFixRegistry()
	fs := []findings.Finding{ruleFinding("terrain/quality/weak-assertion", "x.go", "")}
	if n := reg.Attach("/repo", fs); n != 0 {
		t.Errorf("attached %d, want 0", n)
	}
	if fs[0].Suggestions != nil {
		t.Error("unregistered finding must be untouched")
	}
}

// TestAttach_NoSuggestionCreatesOne: a finding with no prior suggestion gets
// one carrying the Fix.
func TestAttach_NoSuggestionCreatesOne(t *testing.T) {
	t.Parallel()
	reg := NewFixRegistry()
	reg.Register("terrain/deps/drift-risk", func(_ string, f findings.Finding) *findings.Fix {
		return &findings.Fix{Kind: findings.FixEditInPlace, Path: f.PrimaryLoc.Path}
	})
	fs := []findings.Finding{ruleFinding("terrain/deps/drift-risk", "package.json", "")}
	if n := reg.Attach("/repo", fs); n != 1 {
		t.Fatalf("attached %d, want 1", n)
	}
	if len(fs[0].Suggestions) != 1 || fs[0].Suggestions[0].Fix == nil {
		t.Errorf("expected a created suggestion with a Fix; got %+v", fs[0].Suggestions)
	}
}
