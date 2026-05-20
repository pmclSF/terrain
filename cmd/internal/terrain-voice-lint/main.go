// Command terrain-voice-lint enforces the voice-and-tone rules
// documented in the parity plan's Track 10.7. The lint scans Go
// source files for user-visible string literals and reports any
// that violate the canonical voice rules:
//
//  1. No exclamation marks. The Terrain voice is confident, not
//     jarring; exclamation marks read as either pushy or
//     celebratory in CLI output and have no place in finding text.
//  2. No British spellings. Pick one English; we use American.
//     Surfaced as a release-blocker because mixed spellings make
//     the product feel under-edited.
//
// Scope:
//
//  - User-visible string literals in internal/signals/manifest.go
//    (Description, Remediation, PromotionPlan fields).
//  - User-visible string literals in internal/signals/signal_types.go
//    (typeInfoBySignal map values).
//  - User-visible literals in cmd/terrain/*.go that go to stdout
//    or stderr via fmt.Println / fmt.Fprintf.
//
// Out of scope: doc files (already covered by docs-linkcheck +
// truth-verify); Go comments (developer-facing); test files.
//
// Exit codes:
//
//  0 — clean
//  1 — violations found (per-line offender output)
//  2 — invocation error (cannot read filesystem)
//
// Wired into the release-readiness pipeline as `make voice-lint`.
package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// britishSpellingPattern matches common British spellings that the
// Terrain voice rejects. The list is curated, not exhaustive — we
// surface the high-frequency cases that real adopters will spot
// (colour, behaviour, favour) rather than every variant.
//
// Word boundaries are enforced via \b on both sides so substrings
// like "color" or "favorable" don't false-positive on legitimate
// American spellings that happen to contain a British root.
var britishSpellingPattern = regexp.MustCompile(
	`\b(?:` +
		// -our endings (American: -or)
		`colour|colours|coloured|colouring|behaviour|behaviours|favour|favours|favoured|favouring|honour|honoured|labour|labours|laboured|` +
		// -re endings (American: -er)
		`centre|centres|centred|centring|metre|metres|theatre|theatres|fibre|fibres|` +
		// -ise / -ising / -isation (American: -ize / -izing / -ization)
		`optimise|optimised|optimising|optimisation|optimisations|` +
		`recognise|recognised|recognising|recognition|` +
		`organise|organised|organising|organisation|organisations|` +
		`customise|customised|customising|customisation|` +
		`sanitise|sanitised|sanitising|sanitisation|` +
		`prioritise|prioritised|prioritising|prioritisation|` +
		`analyse|analysed|analysing|` +
		// -ce endings (American: -se for verbs)
		`defence|offence|licence|practise|practised|` +
		// -logue endings (American: -log)
		`catalogue|cataloguing|catalogues|dialogue|dialogues|` +
		// Other
		`grey|aluminium|enrol|enrolled|enrolling|fulfil|fulfilled|fulfilling|travelled|travelling|cancelled|cancelling` +
		`)\b`,
)

// exclamationPattern matches an exclamation mark that follows a
// letter — the prose pattern ("Hello!", "Done!", "Wow!"). Marks
// that follow non-alpha characters (`<!DOCTYPE`, `[!]`, `[!!]`) are
// either markup or visual badges and aren't the jarring prose
// pattern the voice rule targets.
//
// The exclamation rule exists to keep CLI output confident rather
// than chirpy. A `[!]` posture marker conveys severity through its
// bracketed-glyph shape, not through exclamatory tone, and falls
// outside the rule's scope.
var exclamationPattern = regexp.MustCompile(`[A-Za-z]!`)

type violation struct {
	file   string
	line   int
	rule   string // "exclamation" | "british-spelling"
	detail string // the offending text
	value  string // the full string literal
}

func main() {
	flag.Parse()

	roots := flag.Args()
	if len(roots) == 0 {
		// Default scan targets — the user-visible string surfaces
		// that are part of the 0.2.0 release contract.
		roots = []string{
			"internal/signals/manifest.go",
			"internal/signals/signal_types.go",
			"cmd/terrain",
			"internal/reporting",
			"internal/changescope",
		}
	}

	var violations []violation
	for _, root := range roots {
		v, err := scan(root)
		if err != nil {
			fmt.Fprintf(os.Stderr, "voice-lint: scan %q: %v\n", root, err)
			os.Exit(2)
		}
		violations = append(violations, v...)
	}

	violations = filter(violations)

	if len(violations) == 0 {
		fmt.Printf("voice-lint: scanned %d root(s); voice & tone clean.\n", len(roots))
		return
	}

	sort.Slice(violations, func(i, j int) bool {
		if violations[i].file != violations[j].file {
			return violations[i].file < violations[j].file
		}
		return violations[i].line < violations[j].line
	})

	fmt.Fprintf(os.Stderr, "::error::%d voice-and-tone violation(s):\n", len(violations))
	for _, v := range violations {
		fmt.Fprintf(os.Stderr, "  %s:%d  [%s]  %s\n    in: %s\n",
			v.file, v.line, v.rule, v.detail, truncate(v.value, 80))
	}
	os.Exit(1)
}

func scan(root string) ([]violation, error) {
	info, err := os.Stat(root)
	if err != nil {
		return nil, err
	}
	var files []string
	if info.IsDir() {
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			if !strings.HasSuffix(path, ".go") {
				return nil
			}
			if strings.HasSuffix(path, "_test.go") {
				return nil
			}
			files = append(files, path)
			return nil
		})
		if err != nil {
			return nil, err
		}
	} else {
		files = []string{root}
	}

	var out []violation
	for _, f := range files {
		v, err := scanFile(f)
		if err != nil {
			return nil, fmt.Errorf("scan %s: %w", f, err)
		}
		out = append(out, v...)
	}
	return out, nil
}

func scanFile(path string) ([]violation, error) {
	fset := token.NewFileSet()
	src, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	var out []violation
	ast.Inspect(src, func(n ast.Node) bool {
		lit, ok := n.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return true
		}
		val := lit.Value
		// Strip surrounding quotes (raw string vs interpreted both
		// have a paired delimiter character at index 0 and -1).
		if len(val) < 2 {
			return true
		}
		val = val[1 : len(val)-1]

		// Skip empty / single-char literals.
		if len(val) < 2 {
			return true
		}

		pos := fset.Position(lit.Pos())

		// Exclamation rule.
		if exclamationPattern.MatchString(val) {
			// Allow `!` in code-shaped substrings (regex literals,
			// JSON pointer fragments, etc.). Heuristic: if the
			// string looks like a Go source pattern (contains
			// `\\w`, `\\d`, `\\s`, `(?`, etc.), skip.
			if !looksLikeRegex(val) {
				out = append(out, violation{
					file:   path,
					line:   pos.Line,
					rule:   "exclamation",
					detail: "literal contains '!'",
					value:  val,
				})
			}
		}

		// British spelling rule.
		if m := britishSpellingPattern.FindString(strings.ToLower(val)); m != "" {
			out = append(out, violation{
				file:   path,
				line:   pos.Line,
				rule:   "british-spelling",
				detail: m,
				value:  val,
			})
		}

		return true
	})
	return out, nil
}

// looksLikeRegex is a heuristic for Go string literals that hold
// regex patterns rather than user-visible prose. Helpful because
// regex character classes legitimately contain `!` and we don't
// want to flag patterns like `[A-Za-z!]+`.
func looksLikeRegex(s string) bool {
	regexHints := []string{
		`\w`, `\d`, `\s`, `\b`, `(?`, `(?:`, `(?P<`, `[^`,
		`\\w`, `\\d`, `\\s`, `\\b`,
	}
	for _, h := range regexHints {
		if strings.Contains(s, h) {
			return true
		}
	}
	return false
}

// filter strips violations the lint should explicitly ignore.
// Today: nothing in the default scan targets is allow-listed, but
// the hook exists so future false positives can be silenced
// without weakening the regex.
func filter(in []violation) []violation {
	allowList := map[string]bool{
		// (file:line:rule) → allow
	}
	var out []violation
	for _, v := range in {
		key := fmt.Sprintf("%s:%d:%s", v.file, v.line, v.rule)
		if allowList[key] {
			continue
		}
		out = append(out, v)
	}
	return out
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
