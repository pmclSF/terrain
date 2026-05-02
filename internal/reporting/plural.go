// Package reporting carries the human-output renderers for every CLI
// command. plural is the shared helper for proper noun pluralization
// in user-visible text — replaces the awkward `finding(s)` / `test(s)`
// / `signal(s)` notation that previously appeared throughout the
// rendered output.
package reporting

// Plural returns the singular form when n == 1, otherwise the plural
// form. For regular nouns (most English), pass the base and it'll
// suffix "s":
//
//	Plural(1, "finding")   → "finding"
//	Plural(2, "finding")   → "findings"
//	Plural(0, "finding")   → "findings"
//
// For irregular plurals, the variadic third argument lets callers
// pass an explicit plural:
//
//	Plural(1, "fixture", "fixtures")  → "fixture"
//	Plural(2, "child", "children")    → "children"
//
// 0.2: introduced to standardize phrasing across renderers — `n
// fixture(s)` reads as a tool's escape hatch; `1 fixture` /
// `5 fixtures` reads like a sentence.
func Plural(n int, singular string, plural ...string) string {
	if n == 1 {
		return singular
	}
	if len(plural) > 0 {
		return plural[0]
	}
	return singular + "s"
}
