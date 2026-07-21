package main

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/pmclSF/terrain/internal/aipipeline"
)

// TestRenderFindingsJSON_RedactSource pins the contract of redact_source on
// the path that does the real work (AI-findings JSON): EVERY evidence
// atom's source snippet is blanked, the finding and positional data
// survive, and the output stays valid JSON. It parses the output (not
// substring-matches it) and uses MULTIPLE atoms so a "blank only the first
// atom" or "emit malformed JSON" regression cannot escape.
func TestRenderFindingsJSON_RedactSource(t *testing.T) {
	const snippet1 = "client.chat.completions.create({apiKey: SECRET})"
	const snippet2 = "db.query(`SELECT * WHERE token=${USERTOKEN}`)"
	findings := []aipipeline.Finding{{
		Path:   "src/handler.ts",
		RuleID: "ai.surface.missing_eval",
		Atoms: []aipipeline.EvidenceAtom{
			{Kind: aipipeline.EvidenceStructural, RuleID: "ast.openai.call", Span: aipipeline.Span{Line: 4, Snippet: snippet1}},
			{Kind: aipipeline.EvidenceLexical, RuleID: "regex.db.call", Span: aipipeline.Span{Line: 9, Snippet: snippet2}},
		},
	}}

	type atom struct {
		Span string `json:"span"`
		Line int    `json:"line"`
	}
	type finding struct {
		Rule     string `json:"rule"`
		Evidence []atom `json:"evidence"`
	}
	parse := func(t *testing.T, b []byte) []finding {
		t.Helper()
		var out []finding
		if err := json.Unmarshal(b, &out); err != nil {
			t.Fatalf("output must be valid JSON: %v\n%s", err, b)
		}
		return out
	}

	// Baseline (not redacting): both snippets present in the parsed output —
	// guards against a vacuous "it's gone" pass below.
	var plain bytes.Buffer
	if err := renderFindingsJSON(&plain, findings, false); err != nil {
		t.Fatalf("render (plain): %v", err)
	}
	pf := parse(t, plain.Bytes())
	if len(pf) != 1 || len(pf[0].Evidence) != 2 {
		t.Fatalf("expected 1 finding with 2 atoms; got %+v", pf)
	}
	if pf[0].Evidence[0].Span != snippet1 || pf[0].Evidence[1].Span != snippet2 {
		t.Fatalf("baseline: both snippets must be present; got %+v", pf[0].Evidence)
	}

	// Redacted: EVERY atom's span blanked; rule + every line preserved.
	var redacted bytes.Buffer
	if err := renderFindingsJSON(&redacted, findings, true); err != nil {
		t.Fatalf("render (redacted): %v", err)
	}
	rf := parse(t, redacted.Bytes())
	if len(rf) != 1 || len(rf[0].Evidence) != 2 {
		t.Fatalf("redaction must keep the finding + all atoms (only blank spans); got %+v", rf)
	}
	if rf[0].Rule != "ai.surface.missing_eval" {
		t.Errorf("redaction must preserve the finding rule; got %q", rf[0].Rule)
	}
	for i, a := range rf[0].Evidence {
		if a.Span != "" {
			t.Errorf("redaction must blank EVERY atom's span; atom %d kept %q", i, a.Span)
		}
	}
	if rf[0].Evidence[0].Line != 4 || rf[0].Evidence[1].Line != 9 {
		t.Errorf("redaction must preserve positional data; got lines %d,%d", rf[0].Evidence[0].Line, rf[0].Evidence[1].Line)
	}
	// Defense in depth: no secret material survives anywhere in the bytes.
	if bytes.Contains(redacted.Bytes(), []byte("SECRET")) || bytes.Contains(redacted.Bytes(), []byte("USERTOKEN")) {
		t.Errorf("redacted output still contains secret material:\n%s", redacted.Bytes())
	}
}
