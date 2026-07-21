// Package astguard rejects source that would make a tree-sitter parse or the
// subsequent node traversal pathologically slow. Terrain parses every source
// file in whatever repository it is pointed at, so a crafted file — thousands
// of unclosed or deeply nested brackets — can force tree-sitter into a large
// error tree whose per-node CGO iteration is superlinear, spinning a scan for
// many seconds per file. Real source nests a few dozen levels at most; a file
// past the threshold is generated, minified past recognition, or hostile, and
// is skipped before any parser sees it.
package astguard

// maxBracketDepth is the deepest legitimate bracket nesting we will parse. Real
// code rarely exceeds a few dozen; the margin here is generous while still
// bounding a file built to be pathological.
const maxBracketDepth = 1000

// LooksPathological reports whether src has bracket nesting deep enough to make
// tree-sitter parsing/traversal a denial-of-service risk. It is a single O(n)
// byte scan — cheaper than the parse it guards — and never allocates. Callers
// skip a file (treat it as yielding no findings) when this returns true.
//
// It ignores brackets inside string/char literals and comments only loosely
// (it does not fully tokenize), which is fine: the goal is to catch runaway
// structural nesting, and a real file never approaches the threshold whether or
// not a handful of bracket characters sit inside strings.
func LooksPathological(src []byte) bool {
	depth := 0
	for _, b := range src {
		switch b {
		case '(', '[', '{':
			depth++
			if depth > maxBracketDepth {
				return true
			}
		case ')', ']', '}':
			if depth > 0 {
				depth--
			}
		}
	}
	return false
}

// LooksPathologicalString is LooksPathological for a string source, without
// allocating a byte copy — some parsers hold source as a string.
func LooksPathologicalString(src string) bool {
	depth := 0
	for i := 0; i < len(src); i++ {
		switch src[i] {
		case '(', '[', '{':
			depth++
			if depth > maxBracketDepth {
				return true
			}
		case ')', ']', '}':
			if depth > 0 {
				depth--
			}
		}
	}
	return false
}
