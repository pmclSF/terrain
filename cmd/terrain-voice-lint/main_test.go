package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestExclamationPattern locks the prose-vs-symbol exclamation
// distinction. The lint should flag word-final exclamations
// (jarring prose tone) but not bracketed visual badges or HTML
// markup that legitimately use `!`.
func TestExclamationPattern(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input string
		match bool
	}{
		// Prose exclamations — should match.
		{"Done!", true},
		{"All set!", true},
		{"Found 3 issues!", true},
		{"Hello there!", true},

		// Visual badges / markup — should NOT match.
		{"[!]", false},
		{"[!!]", false},
		{"<!DOCTYPE html>", false},
		{"!important", false}, // CSS-like prefix, also non-prose
		{"#!/bin/bash", false},
	}
	for _, tc := range tests {
		got := exclamationPattern.MatchString(tc.input)
		if got != tc.match {
			t.Errorf("exclamationPattern(%q) = %v, want %v", tc.input, got, tc.match)
		}
	}
}

// TestBritishSpellingPattern locks the curated British-spelling
// list. Each entry is a word that adopters using American English
// would flag as inconsistent; the curated list keeps false-positive
// risk low (we don't catch every variant — we catch the ones that
// matter).
func TestBritishSpellingPattern(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input string
		match bool
	}{
		// British spellings — should match.
		{"the colour of the badge", true},
		{"behaviour was unexpected", true},
		{"central optimisation", true},
		{"prioritise the test set", true},
		{"recognise the framework", true},
		{"in our favour", true},
		{"defence against drift", true},

		// American spellings — should NOT match.
		{"the color of the badge", false},
		{"behavior was unexpected", false},
		{"central optimization", false},
		{"prioritize the test set", false},
		{"recognize the framework", false},
		{"in our favor", false},
		{"defense against drift", false},

		// Edge cases — partial-word matches that shouldn't fire.
		{"colorful output", false}, // "color" not "colour"
		{"factored design", false}, // "factor" not "favour"
	}
	for _, tc := range tests {
		got := britishSpellingPattern.MatchString(strings.ToLower(tc.input))
		if got != tc.match {
			t.Errorf("britishSpellingPattern(%q) = %v, want %v", tc.input, got, tc.match)
		}
	}
}

// TestLooksLikeRegex covers the regex-pattern guard: string
// literals that contain regex syntax should not trigger the
// exclamation rule even if they contain `!`. Without this, any
// regex that includes a character class with `!` would false-
// positive.
func TestLooksLikeRegex(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input string
		want  bool
	}{
		{`\w+`, true},
		{`\d{2,4}`, true},
		{`[A-Z]+`, false}, // not regex-shaped enough
		{`[^abc]`, true},
		{`(?P<name>\w+)`, true},
		{`hello world`, false},
	}
	for _, tc := range tests {
		got := looksLikeRegex(tc.input)
		if got != tc.want {
			t.Errorf("looksLikeRegex(%q) = %v, want %v", tc.input, got, tc.want)
		}
	}
}

// TestScanFile_DetectsViolations exercises the full scan pipeline
// against a synthetic source file with known violations.
func TestScanFile_DetectsViolations(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	src := `package main

const (
	Greeting   = "Welcome!"
	Behaviour  = "behaviour is undefined"
	Cleanly    = "no issues here"
)
`
	path := filepath.Join(dir, "fixture.go")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := scanFile(path)
	if err != nil {
		t.Fatalf("scanFile: %v", err)
	}

	if len(got) != 2 {
		t.Errorf("expected 2 violations (Greeting + Behaviour), got %d:\n%+v", len(got), got)
	}

	rules := map[string]bool{}
	for _, v := range got {
		rules[v.rule] = true
	}
	if !rules["exclamation"] || !rules["british-spelling"] {
		t.Errorf("expected both rule kinds in output; got %v", rules)
	}
}

// TestScanFile_SkipsTestFiles is the implicit contract: tests can
// use any prose they want without tripping the lint.
func TestScanFile_SkipsTestFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	src := `package main
const Bad = "Hello!"
`
	path := filepath.Join(dir, "x_test.go")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := scan(dir)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("test files should be skipped; got %d violations:\n%+v", len(got), got)
	}
}
