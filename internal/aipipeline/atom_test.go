package aipipeline

import (
	"strings"
	"testing"
)

func TestEvidenceAtomString(t *testing.T) {
	t.Parallel()
	a := EvidenceAtom{
		Kind:   EvidenceLexical,
		RuleID: "regex.openai.call",
		Weight: +1.4,
		Source: "regex-fastscan",
		Span:   Span{Line: 42, Snippet: "openai.chat.completions.create(..."},
	}
	got := a.String()
	for _, want := range []string{"lexical", "regex.openai.call", "+1.40", "L42"} {
		if !strings.Contains(got, want) {
			t.Errorf("atom.String() = %q; missing %q", got, want)
		}
	}
}
