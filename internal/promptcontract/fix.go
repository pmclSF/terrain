package promptcontract

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/pmclSF/terrain/internal/findings"
	"github.com/pmclSF/terrain/internal/saferead"
)

// DriftFix produces a CORRECT-SIDE remediation for a prompt-schema-drift
// finding: it corrects the prompt's field reference to the nearest existing
// schema field, but only when there is exactly one confident match (an obvious
// typo). This is the fix producer registered for terrain/ai/prompt-schema-drift.
//
// It DECLINES (returns nil) whenever it cannot be sure — no confident nearest
// field, the attribute access isn't literally on the finding's line (e.g. a
// `.format(**model)` placeholder that lives in a separate template literal), or
// the files can't be read. A declined finding stays judge-only (observability):
// the producer is the claim, the closed-loop validator is the proof, and a fix
// we can't stand behind is worse than none.
func DriftFix(root string, f findings.Finding) *findings.Fix {
	attr := metaString(f.Metadata, "variable")
	object := metaString(f.Metadata, "object")
	schemaRel := metaString(f.Metadata, "schemaPath")
	schemaName := metaString(f.Metadata, "schemaName")
	promptRel := f.PrimaryLoc.Path
	line := f.PrimaryLoc.Line
	// object is required: without knowing which variable the attribute is
	// accessed on, we cannot scope the edit and would risk rewriting a
	// same-named valid field on a different object. Decline rather than corrupt.
	if attr == "" || object == "" || schemaRel == "" || schemaName == "" || promptRel == "" || line <= 0 {
		return nil
	}

	promptSrc, err := saferead.ReadFile(filepath.Join(root, promptRel))
	if err != nil {
		return nil
	}
	lines := strings.Split(string(promptSrc), "\n")
	idx := line - 1
	if idx < 0 || idx >= len(lines) {
		return nil
	}
	// The correction is a surgical edit of `object.attr` on this exact line,
	// bound to the specific object. A bare `.attr` replace would also rewrite a
	// same-named valid field on a DIFFERENT object on the same line — e.g.
	// correcting `u.titel` in `f"{u.titel} {leg.titel}"` must not touch
	// `leg.titel` when `leg` legitimately has that field. If this object's
	// access isn't on the line, we can't locate what to change -> decline.
	accessRe := regexp.MustCompile(`\b` + regexp.QuoteMeta(object) + `\.` + regexp.QuoteMeta(attr) + `\b`)
	if !accessRe.MatchString(lines[idx]) {
		return nil
	}

	schemaSrc, err := saferead.ReadFile(filepath.Join(root, schemaRel))
	if err != nil {
		return nil
	}
	cands := candidateAttrs(extractPython(schemaRel, schemaSrc).schemas, schemaName)
	best, ok := nearestField(attr, cands)
	if !ok {
		return nil // no confident nearest field -> stay judge-only
	}

	lines[idx] = accessRe.ReplaceAllString(lines[idx], object+"."+best)
	return &findings.Fix{
		Kind:    findings.FixEditInPlace,
		Path:    promptRel,
		Content: strings.Join(lines, "\n"),
	}
}

// candidateAttrs returns the valid attribute names of the named schema in a
// parsed file: declared fields, methods/@property, and imperatively-assigned
// self attributes.
func candidateAttrs(schemas []SchemaDef, name string) []string {
	for _, s := range schemas {
		if s.Name != name {
			continue
		}
		out := make([]string, 0, len(s.Fields)+len(s.Methods)+len(s.SelfAttrs))
		for _, fld := range s.Fields {
			out = append(out, fld.Name)
		}
		out = append(out, s.Methods...)
		out = append(out, s.SelfAttrs...)
		return out
	}
	return nil
}

// nearestField returns the single closest candidate to attr when the match is
// confident: a UNIQUE minimum edit distance ≤ 2, on a name long enough that a
// two-edit distance is a typo rather than a different word (len ≥ 4 and the
// edits span less than half the name). Ambiguous or distant matches decline, so
// the producer never guesses a semantically different field.
func nearestField(attr string, cands []string) (string, bool) {
	if len(attr) < 4 {
		return "", false
	}
	best, bestDist, ties := "", 1<<30, 0
	for _, c := range cands {
		if c == attr {
			continue
		}
		d := levenshtein(attr, c)
		switch {
		case d < bestDist:
			best, bestDist, ties = c, d, 1
		case d == bestDist:
			ties++
		}
	}
	if best == "" || ties != 1 {
		return "", false
	}
	if bestDist > 2 || bestDist*2 >= len(attr) {
		return "", false
	}
	return best, true
}

// levenshtein is the standard edit distance (insert/delete/substitute = 1).
func levenshtein(a, b string) int {
	prev := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		cur := make([]int, len(b)+1)
		cur[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			cur[j] = min3(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev = cur
	}
	return prev[len(b)]
}

func min3(a, b, c int) int {
	m := a
	if b < m {
		m = b
	}
	if c < m {
		m = c
	}
	return m
}

func metaString(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}
