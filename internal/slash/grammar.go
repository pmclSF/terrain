// Package slash implements Terrain's deterministic slash-command
// grammar for PR-comment interaction. No LLM parsing — every verb is
// a literal string match with explicit positional + keyword args.
//
// The 10-verb canonical set:
//
//	/terrain bench <id>            — run a benchmark suite by ID.
//	/terrain commands              — re-print the pinned command list.
//	/dismiss reason:<text>         — suppress the active finding (must
//	                                  be invoked as a reply to a
//	                                  finding's inline comment).
//	/terrain escalate              — flip the active finding from
//	                                  observability to gate for this
//	                                  PR (one-off).
//	/terrain explain <rule-id>     — long-form explanation of a rule.
//	/terrain expand                — expand a `+N more` collapsed
//	                                  block (replies inline).
//	/terrain refresh               — re-run analyze + replace the PR
//	                                  comment with fresh output.
//	/terrain scaffold accept       — accept the test scaffold the
//	                                  active finding suggested.
//	/terrain show <id>             — render one finding by ID.
//	/terrain why <rule-id>         — short rationale for a rule.
//
// Grammar shape: leading `/`, verb (with optional `terrain` prefix
// for verbs that need to disambiguate from third-party bots), then
// either positional args or `key:value` pairs. Whitespace is the
// only separator. Quoted values support spaces in the reason text.
//
// Unknown verbs produce a ParseError with the closest match
// suggestion (Levenshtein-like) so users see helpful corrections.
package slash

import (
	"fmt"
	"strings"
)

// Verb enumerates the 10 canonical slash commands. Values are the
// canonical literal the user types. The package also accepts an
// optional `/terrain ` prefix on every verb so users can be explicit
// about which bot they're talking to.
type Verb string

const (
	VerbBench         Verb = "bench"
	VerbCommands      Verb = "commands"
	VerbDismiss       Verb = "dismiss"
	VerbEscalate      Verb = "escalate"
	VerbExplain       Verb = "explain"
	VerbExpand        Verb = "expand"
	VerbRefresh       Verb = "refresh"
	VerbScaffold      Verb = "scaffold"
	VerbShow          Verb = "show"
	VerbWhy           Verb = "why"
)

// AllVerbs lists every canonical verb. Used by the grammar parser
// for autocomplete-style "did you mean" hints and by the command-list
// renderer to generate the pinned comment.
var AllVerbs = []Verb{
	VerbBench, VerbCommands, VerbDismiss, VerbEscalate,
	VerbExplain, VerbExpand, VerbRefresh, VerbScaffold,
	VerbShow, VerbWhy,
}

// Command is one parsed slash-command invocation.
type Command struct {
	// Verb is the canonical verb.
	Verb Verb
	// Positional carries any positional args after the verb (e.g.
	// the `<id>` for `/terrain show <id>`). Always normalized — no
	// surrounding whitespace.
	Positional []string
	// Keyword carries `key:value` pairs (e.g. `reason:false-positive`
	// for `/dismiss`). Always present, may be empty.
	Keyword map[string]string
	// Raw is the original input line for logging / audit.
	Raw string
}

// ParseError is the parser's error type. Includes a Suggestion when
// the closest registered verb is within edit distance 2.
type ParseError struct {
	Input      string
	Reason     string
	Suggestion string
}

func (e *ParseError) Error() string {
	if e.Suggestion != "" {
		return fmt.Sprintf("slash: %s (did you mean %q?)", e.Reason, e.Suggestion)
	}
	return fmt.Sprintf("slash: %s", e.Reason)
}

// Parse turns a single PR-comment line into a Command. Lines that
// don't start with `/` return (nil, nil) — they're not slash commands
// and the caller should ignore them. Malformed slash commands return
// (nil, *ParseError).
//
// Accepted shapes:
//
//	/terrain <verb> [positional...] [key:value...]
//	/<verb> [positional...] [key:value...]      (no /terrain prefix)
//	/dismiss reason:<text>                       (alias: /dismiss is
//	                                              special-cased as a
//	                                              top-level verb so
//	                                              users don't have to
//	                                              remember the prefix
//	                                              for the most common
//	                                              command).
//
// Quoted values: `key:"value with spaces"` is supported. Backslash-
// escape inside quoted values for embedded quotes (`\"`).
func Parse(input string) (*Command, error) {
	line := strings.TrimSpace(input)
	if !strings.HasPrefix(line, "/") {
		return nil, nil
	}
	body := strings.TrimPrefix(line, "/")
	if body == "" {
		return nil, &ParseError{Input: input, Reason: "empty slash command"}
	}

	tokens, err := tokenize(body)
	if err != nil {
		return nil, &ParseError{Input: input, Reason: err.Error()}
	}
	if len(tokens) == 0 {
		return nil, &ParseError{Input: input, Reason: "empty slash command"}
	}

	// Determine the verb. If the first token is "terrain", the verb
	// is the second token (canonical `/terrain <verb>` shape). Else
	// the first token is the verb directly (compact shape).
	var verb string
	var rest []string
	if strings.EqualFold(tokens[0], "terrain") {
		if len(tokens) < 2 {
			return nil, &ParseError{Input: input, Reason: "missing verb after /terrain"}
		}
		verb = tokens[1]
		rest = tokens[2:]
	} else {
		verb = tokens[0]
		rest = tokens[1:]
	}

	// Validate verb.
	canonical, ok := canonicalVerb(verb)
	if !ok {
		return nil, &ParseError{
			Input:      input,
			Reason:     fmt.Sprintf("unknown verb %q", verb),
			Suggestion: suggestVerb(verb),
		}
	}

	cmd := &Command{
		Verb:    canonical,
		Keyword: map[string]string{},
		Raw:     line,
	}

	// Partition rest into positional and keyword args.
	for _, tok := range rest {
		if k, v, ok := splitKeyword(tok); ok {
			cmd.Keyword[k] = v
		} else {
			cmd.Positional = append(cmd.Positional, tok)
		}
	}

	if err := validate(cmd); err != nil {
		return nil, err
	}
	return cmd, nil
}

// canonicalVerb returns the Verb constant for a typed string, plus a
// presence bool. Case-insensitive match.
func canonicalVerb(s string) (Verb, bool) {
	s = strings.ToLower(strings.TrimSpace(s))
	for _, v := range AllVerbs {
		if string(v) == s {
			return v, true
		}
	}
	return "", false
}

// splitKeyword pulls a `key:value` token apart. Returns ok=false when
// the token has no colon, starts with one, or the key segment looks
// like part of an identifier (contains `@`, `.`, `/`, `#`). The
// identifier-shape rejection lets positional tokens like finding-IDs
// (`weakAssertion@path:Symbol#hash`) pass through as a single
// positional rather than being mis-split on the colon.
func splitKeyword(tok string) (key, value string, ok bool) {
	idx := strings.Index(tok, ":")
	if idx <= 0 || idx == len(tok)-1 {
		return "", "", false
	}
	k := strings.ToLower(strings.TrimSpace(tok[:idx]))
	if !isSimpleIdent(k) {
		return "", "", false
	}
	v := strings.TrimSpace(tok[idx+1:])
	v = unquote(v)
	if k == "" || v == "" {
		return "", "", false
	}
	return k, v, true
}

// isSimpleIdent returns true when s is an unqualified lowercase
// identifier (letters / digits / underscore, must start with letter).
// Used to distinguish keyword keys (e.g. "reason") from the colon
// that appears inside compound IDs (e.g. "type@path:symbol#hash").
func isSimpleIdent(s string) bool {
	if len(s) == 0 {
		return false
	}
	if !(s[0] >= 'a' && s[0] <= 'z') {
		return false
	}
	for i := 1; i < len(s); i++ {
		c := s[i]
		if !((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '_') {
			return false
		}
	}
	return true
}

// validate enforces per-verb argument requirements.
func validate(c *Command) error {
	switch c.Verb {
	case VerbBench:
		if len(c.Positional) == 0 {
			return &ParseError{Input: c.Raw, Reason: "bench requires a benchmark id (usage: /terrain bench <id>)"}
		}
	case VerbDismiss:
		if c.Keyword["reason"] == "" {
			return &ParseError{Input: c.Raw, Reason: "dismiss requires reason:<text> (usage: /dismiss reason:\"why\")"}
		}
	case VerbExplain, VerbWhy:
		if len(c.Positional) == 0 {
			return &ParseError{Input: c.Raw, Reason: fmt.Sprintf("%s requires a rule-id (usage: /terrain %s <rule-id>)", c.Verb, c.Verb)}
		}
	case VerbShow:
		if len(c.Positional) == 0 {
			return &ParseError{Input: c.Raw, Reason: "show requires an id (usage: /terrain show <finding-id-or-rule-id>)"}
		}
	case VerbScaffold:
		// /terrain scaffold accept — requires the literal "accept" subverb.
		if len(c.Positional) == 0 || c.Positional[0] != "accept" {
			return &ParseError{Input: c.Raw, Reason: "scaffold requires the 'accept' subverb (usage: /terrain scaffold accept)"}
		}
	}
	return nil
}

// suggestVerb returns the closest canonical verb to s within edit
// distance 2, or "" when no good match exists.
func suggestVerb(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	best := ""
	bestDist := 3
	for _, v := range AllVerbs {
		d := editDistance(s, string(v))
		if d < bestDist {
			bestDist = d
			best = string(v)
		}
	}
	return best
}

// editDistance computes the Levenshtein edit distance between a and b.
// Used by suggestVerb; small inputs only.
func editDistance(a, b string) int {
	ar := []rune(a)
	br := []rune(b)
	la, lb := len(ar), len(br)
	if la == 0 {
		return lb
	}
	if lb == 0 {
		return la
	}
	prev := make([]int, lb+1)
	curr := make([]int, lb+1)
	for j := 0; j <= lb; j++ {
		prev[j] = j
	}
	for i := 1; i <= la; i++ {
		curr[0] = i
		for j := 1; j <= lb; j++ {
			cost := 1
			if ar[i-1] == br[j-1] {
				cost = 0
			}
			curr[j] = min3(prev[j]+1, curr[j-1]+1, prev[j-1]+cost)
		}
		prev, curr = curr, prev
	}
	return prev[lb]
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

func unquote(s string) string {
	if len(s) < 2 {
		return s
	}
	if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
		return s[1 : len(s)-1]
	}
	return s
}
