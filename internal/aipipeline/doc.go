// Package aipipeline composes regex, AST, repo-shape, and per-PR-diff
// signals into typed evidence atoms and turns those atoms into verdicts
// via weighted log-odds.
//
// # Architecture
//
// Each stage in the pipeline emits zero or more EvidenceAtoms. An atom
// carries a kind (Lexical / Structural / Topological / Scope / Negative),
// a stable rule ID, a signed log-odds weight, and a span pointing back
// into the source file. The Composer sums the per-atom weights together
// with a per-rule/per-cohort base rate, applies a sigmoid, and produces
// a final confidence score for the verdict.
//
// The atom shape lets new signal sources slot in without changing the
// rest of the system. Regex, AST, repo-shape, cross-file, and diff-scope
// are peers, not a ladder; each contributes evidence and the composer
// decides what to do with it.
//
// # Stages
//
// Pipeline stages, in canonical order:
//
//	Stage 0  RepoShape       — cohort + library/application + manifests (cached)
//	Stage 1  PathPrefilter   — directory and filename gates
//	Stage 2  RegexFastscan   — context-window regex atoms (regex-v2 port)
//	Stage 3  ASTConfirm      — AST-derived call-site atoms via internal/aidetect
//	Stage 4  CrossFileContext — exports + importer count (deferred; not yet wired)
//	Stage 5  ChangeScope     — diff-touched / diff-adjacent atoms (per-PR mode)
//	Stage 6  VerdictCompose  — weighted log-odds with per-cohort calibration
//
// A stage may only narrow the candidate set, never broaden it. Each
// stage is independently testable.
//
// # Posture
//
// Same engine drives Observability and Gate tiers; only the confidence
// threshold differs. AST-unavailable findings are emitted with a marker
// in Observability (and degrade with a small confidence penalty) but are
// suppressed in Gate.
//
// # Calibration
//
// Per-rule, per-cohort weights live in calibration.go. Initial weights
// are heuristic; subsequent revisions fit from the labeled corpus at
// tier-4/handlabel/. The composer is the only consumer of the
// calibration table; stages just emit atoms with their declared default
// weight.
package aipipeline
