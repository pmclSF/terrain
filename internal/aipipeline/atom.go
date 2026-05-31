package aipipeline

import "fmt"

// EvidenceKind classifies the kind of signal an atom represents. The
// kind drives composer behavior (negative kinds reduce score; topological
// kinds carry less weight at the corpus mean than structural kinds).
type EvidenceKind string

const (
	// EvidenceLexical is regex-level co-occurrence or call-site shape
	// detected without parsing. Cheap, broad, occasionally noisy.
	EvidenceLexical EvidenceKind = "lexical"

	// EvidenceStructural is AST-derived: bound calls, class hierarchies,
	// scope-resolved method invocations. Strongest positive signal we
	// have inside a single file.
	EvidenceStructural EvidenceKind = "structural"

	// EvidenceTopological reflects relationships across files: imports,
	// exports, importer counts, depgraph reachability.
	EvidenceTopological EvidenceKind = "topological"

	// EvidenceScope is per-PR / diff-related: whether the file or call
	// site was touched in the current change.
	EvidenceScope EvidenceKind = "scope"

	// EvidenceShape is repo-level signal: cohort, packaging metadata,
	// library-vs-application classification.
	EvidenceShape EvidenceKind = "shape"

	// EvidenceNegative is a suppressing signal that lowers confidence.
	// Examples: regex anchor without a corresponding call, wrapper-class
	// shape that indicates the file relays rather than executes.
	EvidenceNegative EvidenceKind = "negative"
)

// EvidenceAtom is the unit of evidence produced by a stage. Multiple
// atoms compose into a verdict. Weight is the contribution this atom
// makes to the rule's log-odds score; positive supports the verdict,
// negative opposes it.
type EvidenceAtom struct {
	// Kind groups atoms for explanation and composer behavior.
	Kind EvidenceKind

	// RuleID names the specific atom contributor. Stable string; appears
	// in evidence chains shown to users (`ai.callsite.openai.chat`,
	// `wrapper.class.match`, `path.examples`, ...).
	RuleID string

	// Weight is the signed log-odds contribution. Default per-atom
	// weights are declared by the producer; the composer may override
	// from the calibration table at composition time.
	Weight float64

	// Source identifies the stage that produced this atom
	// (`regex:ctx.openai`, `ast:bound.langchain`, `path:examples`).
	Source string

	// Span points back into the source for evidence rendering.
	Span Span
}

// Span identifies a region of a source file. Line and Col are 1-based;
// EndLine and EndCol default to Line and Col when the span is a single
// point. ByteOffset is best-effort and may be zero for some producers.
type Span struct {
	Line    int
	Col     int
	EndLine int
	EndCol  int

	// Snippet is a short single-line excerpt from the source. Truncated
	// at SnippetMaxLen by producers.
	Snippet string
}

const SnippetMaxLen = 200

// String returns a human-readable representation for logs and tests.
func (a EvidenceAtom) String() string {
	return fmt.Sprintf("[%s:%s w=%+.2f L%d %q]",
		a.Kind, a.RuleID, a.Weight, a.Span.Line, a.Span.Snippet)
}
