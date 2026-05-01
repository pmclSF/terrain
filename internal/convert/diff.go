package convert

import (
	"fmt"
	"strings"
)

// UnifiedDiff produces a unified-diff-shaped rendering of the change
// from old to new. Output looks like `diff -u` output: a `---` / `+++`
// header followed by line markers (` `, `+`, `-`).
//
// The implementation is LCS-based without `@@ -a,b +c,d @@` hunk
// headers — simple and adequate for showing a converter's output diff.
// Files that are byte-identical produce a single "no changes" line so
// callers can render an empty result clearly.
func UnifiedDiff(oldPath, newPath, oldContent, newContent string) string {
	if oldContent == newContent {
		return fmt.Sprintf("--- %s\n+++ %s\n(no changes)\n", oldPath, newPath)
	}

	oldLines := splitLinesPreservingTerminator(oldContent)
	newLines := splitLinesPreservingTerminator(newContent)

	edits := lcsEditScript(oldLines, newLines)

	var b strings.Builder
	fmt.Fprintf(&b, "--- %s\n+++ %s\n", oldPath, newPath)
	for _, e := range edits {
		switch e.kind {
		case editEqual:
			fmt.Fprintf(&b, " %s\n", e.line)
		case editAdd:
			fmt.Fprintf(&b, "+%s\n", e.line)
		case editDel:
			fmt.Fprintf(&b, "-%s\n", e.line)
		}
	}
	return b.String()
}

type editKind int

const (
	editEqual editKind = iota
	editAdd
	editDel
)

type editOp struct {
	kind editKind
	line string
}

// splitLinesPreservingTerminator splits s on '\n' and drops the empty
// trailing element that strings.Split produces when s ends with '\n'.
// Returns nil for an empty input so the LCS loops handle it cleanly.
func splitLinesPreservingTerminator(s string) []string {
	if s == "" {
		return nil
	}
	out := strings.Split(s, "\n")
	if len(out) > 0 && out[len(out)-1] == "" {
		out = out[:len(out)-1]
	}
	return out
}

// lcsEditScript returns the edit-script transforming a into b using a
// standard longest-common-subsequence backtrack. Output is in source
// order: same / add / del per line.
//
// Time + space O(len(a) * len(b)). Test files are typically <1k lines
// so the worst-case is fine.
func lcsEditScript(a, b []string) []editOp {
	la, lb := len(a), len(b)
	lcs := make([][]int, la+1)
	for i := range lcs {
		lcs[i] = make([]int, lb+1)
	}
	for i := 1; i <= la; i++ {
		for j := 1; j <= lb; j++ {
			if a[i-1] == b[j-1] {
				lcs[i][j] = lcs[i-1][j-1] + 1
			} else if lcs[i-1][j] >= lcs[i][j-1] {
				lcs[i][j] = lcs[i-1][j]
			} else {
				lcs[i][j] = lcs[i][j-1]
			}
		}
	}

	// Backtrack from (la, lb) building the edit list in reverse.
	var edits []editOp
	i, j := la, lb
	for i > 0 || j > 0 {
		switch {
		case i > 0 && j > 0 && a[i-1] == b[j-1]:
			edits = append(edits, editOp{kind: editEqual, line: a[i-1]})
			i--
			j--
		case j > 0 && (i == 0 || lcs[i][j-1] >= lcs[i-1][j]):
			edits = append(edits, editOp{kind: editAdd, line: b[j-1]})
			j--
		default:
			edits = append(edits, editOp{kind: editDel, line: a[i-1]})
			i--
		}
	}
	// Reverse in-place.
	for left, right := 0, len(edits)-1; left < right; left, right = left+1, right-1 {
		edits[left], edits[right] = edits[right], edits[left]
	}
	return edits
}
