package injection

import (
	"sort"
	"strings"
)

// Match describes one (pattern, evidence) pair where a prompt
// template body exhibits a vulnerable shape.
type Match struct {
	Pattern Pattern
	// Marker is the substring from the pattern's VulnerableMarkers
	// that appeared in the template body. Useful for explaining why
	// the pattern matched.
	Marker string
}

// DetectMatches returns the injection patterns that match a prompt
// template body. A pattern matches when any of its VulnerableMarkers
// is a substring of the body.
//
// Returns at most one Match per pattern (the first marker that hits),
// sorted by pattern ID for deterministic output.
func DetectMatches(promptBody string) []Match {
	if promptBody == "" {
		return nil
	}
	var matches []Match
	for _, p := range Library() {
		for _, marker := range p.VulnerableMarkers {
			if strings.Contains(promptBody, marker) {
				matches = append(matches, Match{Pattern: p, Marker: marker})
				break
			}
		}
	}
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].Pattern.ID < matches[j].Pattern.ID
	})
	return matches
}
