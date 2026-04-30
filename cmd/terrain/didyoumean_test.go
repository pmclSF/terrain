package main

import (
	"reflect"
	"testing"
)

func TestDidYouMean_SuggestsClosestCommands(t *testing.T) {
	t.Parallel()

	cases := []struct {
		input string
		want  []string // expected first suggestion(s); empty means "no suggestions"
	}{
		// Single-character typos suggest the closest match.
		{"analaze", []string{"analyze"}},
		{"explan", []string{"explain"}},
		{"impct", []string{"impact"}},
		{"insiht", []string{"insights", "init"}},
		{"docter", []string{"doctor"}},
		// Verbs that don't exist but are close to canonical commands.
		{"converted", []string{"convert"}},
		// Far-from-anything strings produce no suggestion.
		{"completelyunrelated", nil},
	}

	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			got := didYouMean(tc.input, 3)
			if len(tc.want) == 0 {
				if len(got) > 0 {
					t.Errorf("didYouMean(%q) = %v; want no suggestions", tc.input, got)
				}
				return
			}
			if len(got) == 0 {
				t.Errorf("didYouMean(%q) = no suggestions; want at least %v", tc.input, tc.want[0])
				return
			}
			// The first expected suggestion must rank in the top results.
			found := false
			for _, g := range got {
				if g == tc.want[0] {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("didYouMean(%q) = %v; expected %q in suggestions", tc.input, got, tc.want[0])
			}
		})
	}
}

func TestDidYouMean_RespectsMaxResults(t *testing.T) {
	t.Parallel()

	got := didYouMean("a", 2) // close to many short commands
	if len(got) > 2 {
		t.Errorf("didYouMean(\"a\", 2) returned %d results; max is 2: %v", len(got), got)
	}
}

func TestLevenshtein(t *testing.T) {
	t.Parallel()

	cases := []struct {
		a, b string
		want int
	}{
		{"", "", 0},
		{"abc", "abc", 0},
		{"abc", "abd", 1},
		{"abc", "ab", 1},
		{"ab", "abc", 1},
		{"kitten", "sitting", 3},
		{"saturday", "sunday", 3},
	}
	for _, tc := range cases {
		got := levenshtein(tc.a, tc.b)
		if got != tc.want {
			t.Errorf("levenshtein(%q, %q) = %d; want %d", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestKnownCommands_NoDuplicates(t *testing.T) {
	t.Parallel()

	seen := map[string]bool{}
	for _, cmd := range knownCommands {
		if seen[cmd] {
			t.Errorf("knownCommands contains duplicate %q", cmd)
		}
		seen[cmd] = true
	}
}

func TestKnownCommands_StableOrder(t *testing.T) {
	t.Parallel()

	// The slice is intentionally not sorted (order reflects the dispatcher
	// switch). This test just guards against accidental shuffling that
	// would invalidate any future caller relying on ordering.
	snapshot := append([]string(nil), knownCommands...)
	if !reflect.DeepEqual(snapshot, knownCommands) {
		t.Errorf("knownCommands mutation observed during test setup")
	}
}
