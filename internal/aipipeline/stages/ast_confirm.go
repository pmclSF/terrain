package stages

import (
	"context"
	"fmt"
	"strings"

	"github.com/pmclSF/terrain/internal/aidetect"
	"github.com/pmclSF/terrain/internal/aipipeline"
)

// ASTConfirm is Stage 3: AST-derived call-site confirmation. It wraps
// internal/aidetect's per-language AST detectors and emits structural
// atoms for resolved calls.
//
// Three emission paths:
//
//  1. AST sees a bound call to an LLM SDK — emit "ast.bound_call" with
//     high positive weight. Strongest single positive signal.
//
//  2. AST runs but finds no call site, despite the regex stage having
//     matched an SDK anchor — emit "ast.no_call_despite_regex" with
//     strong negative weight. This is the negative gate that
//     suppresses regex-only candidates with no reachable call site.
//
//  3. AST cannot run (unsupported language, parse failure, file too
//     large) — record an "ast=unavailable" fallback. Composer applies
//     a confidence penalty in Observability and suppresses the finding
//     in Gate.
type ASTConfirm struct {
	// MaxFileBytes is a hard upper bound. Files larger than this skip
	// AST parsing and record the "ast=unavailable" fallback. Default
	// 500 KB — beyond this the AST parse cost outweighs the added signal.
	MaxFileBytes int
}

// NewASTConfirm returns the stage with sensible defaults.
func NewASTConfirm() *ASTConfirm {
	return &ASTConfirm{MaxFileBytes: 500 * 1024}
}

// Name implements pipeline.Stage.
func (s *ASTConfirm) Name() string { return "ast-confirm" }

// Run executes the language-appropriate AST detector and emits atoms.
func (s *ASTConfirm) Run(_ context.Context, c *aipipeline.Candidate) aipipeline.StageResult {
	lang := strings.ToLower(c.Lang)
	supported := lang == "python" || lang == "javascript" || lang == "typescript" ||
		lang == "go" || lang == "java"
	if !supported {
		c.AddFallback("ast=unavailable")
		return aipipeline.StageResult{Continue: true}
	}
	if len(c.Src) == 0 {
		c.AddFallback("ast=unavailable")
		return aipipeline.StageResult{Continue: true}
	}
	if s.MaxFileBytes > 0 && len(c.Src) > s.MaxFileBytes {
		c.AddFallback("ast=unavailable")
		return aipipeline.StageResult{Continue: true}
	}

	// Fast path: skip the tree-sitter parse when the regex stage saw
	// no SDK signal. The AST stage has two jobs — confirm regex-flagged
	// call sites (positive atom) and gate regex-flagged anchors that
	// have no AST-resolvable call (negative atom). Neither job applies
	// when the regex stage was silent, and tree-sitter parsing every
	// source file in a large repo is by far the dominant cost of the
	// pipeline (the large majority of files carry no SDK signal). Skipping
	// the AST parse for those files is what keeps analysis fast on big repos.
	if !hasRegexLexicalAtom(c) {
		return aipipeline.StageResult{Continue: true}
	}

	sites := s.detect(lang, c.Src, c.Path)

	if len(sites) == 0 {
		// Regex flagged but AST sees nothing → strong negative atom,
		// but ONLY when the rule is one the AST detector knows how to
		// verify. The current AST detector covers LLM SDK surfaces
		// (DetectPythonAISurfaces et al. emit AICallSite records for
		// openai/anthropic/langchain/llama_index/huggingface). It does
		// NOT cover ML training (.fit(X_train, ...), Trainer(...), etc.),
		// so firing a negative atom on a training rule wrongly suppresses
		// every training TP. The fix: only fire when the rule's "expected"
		// shape is AST-known.
		if regexFlaggedLLMAnchor(c) && ruleNeedsLLMASTVerify(c.RuleID) {
			c.AddAtom(aipipeline.EvidenceAtom{
				Kind:   aipipeline.EvidenceNegative,
				RuleID: "ast.no_call_despite_regex",
				Source: "ast-confirm",
				Weight: -2.1,
				Span:   aipipeline.Span{Snippet: "AST: no LLM call site"},
			})
		}
		return aipipeline.StageResult{Continue: true}
	}

	// Emit ONE bound-call atom per file even when AST resolved many
	// sites. Multiple call sites in the same file are correlated
	// evidence; compounding them linearly drowns out any negative
	// signal on provider-wrapper files (which can have 10+ internal
	// calls). The Span carries the first site's location plus a
	// count summary so explainability stays precise.
	first := sites[0]
	snippet := first.Method
	if len(sites) > 1 {
		snippet = fmt.Sprintf("%s (and %d more call site%s in this file)",
			first.Method, len(sites)-1, plural(len(sites)-1))
	}
	c.AddAtom(aipipeline.EvidenceAtom{
		Kind:   aipipeline.EvidenceStructural,
		RuleID: "ast.bound_call",
		Source: "ast-confirm:" + first.SDK,
		Weight: +2.0,
		Span: aipipeline.Span{
			Line:    first.Line,
			Snippet: snippet,
		},
	})
	return aipipeline.StageResult{Continue: true}
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

// detect dispatches to the right per-language AST detector.
func (s *ASTConfirm) detect(lang string, src []byte, path string) []aidetect.AICallSite {
	switch lang {
	case "python":
		return aidetect.DetectPythonAISurfaces(src, path)
	case "javascript", "typescript":
		return aidetect.DetectJSAISurfaces(src, path)
	case "go":
		return aidetect.DetectGoAISurfaces(src, path)
	case "java":
		return aidetect.DetectJavaAISurfaces(src, path)
	}
	return nil
}

// hasRegexLexicalAtom reports whether the regex stage emitted ANY
// lexical positive atom (import or call) for the candidate. This is
// the fast-path predicate that lets the AST stage skip its tree-
// sitter parse for files with no SDK signal — see Run().
func hasRegexLexicalAtom(c *aipipeline.Candidate) bool {
	for _, a := range c.Atoms {
		if !strings.HasPrefix(a.Source, "regex-fastscan") {
			continue
		}
		if a.Kind == aipipeline.EvidenceLexical {
			return true
		}
	}
	return false
}

// regexFlaggedLLMAnchor reports whether the regex stage emitted an
// import-anchor atom for an LLM-class SDK. Training-class anchors
// (sklearn, xgboost, lightgbm, catboost, keras, pytorch, transformers)
// are excluded because the AST detector doesn't verify training
// surfaces — firing a "no LLM call" negative atom on a training-rule
// candidate wrongly suppresses every training TP.
func regexFlaggedLLMAnchor(c *aipipeline.Candidate) bool {
	for _, a := range c.Atoms {
		if !strings.HasPrefix(a.Source, "regex-fastscan") {
			continue
		}
		if !strings.HasSuffix(a.RuleID, ".import") {
			continue
		}
		if isLLMAnchorID(a.RuleID) {
			return true
		}
	}
	return false
}

// isLLMAnchorID classifies an atom RuleID as belonging to the LLM
// detector family (vs the ML-training family).
func isLLMAnchorID(ruleID string) bool {
	for _, prefix := range llmAnchorPrefixes {
		if strings.HasPrefix(ruleID, prefix) {
			return true
		}
	}
	return false
}

var llmAnchorPrefixes = []string{
	"regex.openai.",
	"regex.anthropic.",
	"regex.langchain.",
	"regex.llama_index.",
	"regex.openai_compat.",
	"regex.google_genai.",
	"regex.huggingface.",
	"regex.langgraph.",
	"regex.generic_sdk.", // covers LLM-shaped fallback anchors
}

// ruleNeedsLLMASTVerify reports whether the rule under evaluation
// expects AST to find an LLM call. ai.surface.missing_eval and
// related rules want this verification; training rules don't.
func ruleNeedsLLMASTVerify(ruleID string) bool {
	switch ruleID {
	case "ai.surface.missing_eval",
		"ai.prompt_file_missing_eval",
		"ai.uncovered_surface":
		return true
	}
	return false
}
