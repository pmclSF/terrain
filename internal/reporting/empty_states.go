package reporting

import (
	"fmt"
	"io"
	"strings"
)

// EmptyStateKind identifies which empty-state path is being rendered.
// Track 10.6 of the 0.2.0 release plan calls for every list-producing
// command to have a *designed* empty-state path — a clear next-move
// nudge instead of silence — so first-run / clean-repo experiences
// don't read as broken output.
//
// One enum value per distinct empty case keeps the wiring tight: the
// renderer asks "which kind?" and the helper produces a stable,
// designed string. Adding a new kind requires updating one switch in
// RenderEmptyState below; tests in empty_states_test.go lock the
// strings.
type EmptyStateKind int

const (
	// EmptyZeroFindings — analyze / insights / posture ran cleanly
	// and produced zero findings. Most repos will never see this
	// state, but those that do should feel rewarded, not confused.
	EmptyZeroFindings EmptyStateKind = iota

	// EmptyNoAISurfaces — the AI surface inventory pass found no
	// detectable AI surfaces (no models, no prompts, no eval
	// frameworks). The AI Risk Review section should be skipped
	// entirely with a single explanatory line so adopters know
	// it's deliberate, not a bug.
	EmptyNoAISurfaces

	// EmptyNoPolicyFile — `terrain policy check` ran but no
	// `.terrain/policy.yaml` is present. The right next move is
	// pointing at `terrain init`, not silently exiting 0.
	EmptyNoPolicyFile

	// EmptyFirstRun — the binary appears to be running on a
	// repo that has never been analyzed before (no
	// .terrain/snapshots/, no terrain.yaml). A single warm
	// greeting that suggests the next command beats no output.
	EmptyFirstRun

	// EmptyNoImpact — `terrain report impact` ran but the change
	// scope produced zero impacted units (tiny doc change, etc.).
	// The right next move is "merge with confidence", not blank
	// output that reads as "Terrain failed."
	EmptyNoImpact

	// EmptyNoTestSelection — `terrain report select-tests` ran
	// but no tests were selected. Often the right answer (the
	// change has no test impact) but adopters need to see that
	// it's deliberate.
	EmptyNoTestSelection

	// EmptyNoMigrationCandidates — `terrain migrate readiness`
	// found no convertible files. Right when the repo is already
	// on the framework of record; otherwise a possible
	// detection bug.
	EmptyNoMigrationCandidates
)

// EmptyState is the rendered shape of an empty-state path: a one-line
// header (designed, not blank) plus an optional next-move nudge.
//
// We keep the data here rather than emitting strings inline so that
// callers can render to terminal-text, JSON envelopes, or markdown
// without each callsite reinventing the message. JSON consumers
// receive {empty: true, kind: "...", header: "...", nextMove: "..."}.
type EmptyState struct {
	Kind     EmptyStateKind `json:"-"`
	Header   string         `json:"header"`
	NextMove string         `json:"nextMove,omitempty"`
}

// EmptyStateFor returns the canonical EmptyState for a given kind.
// The strings are deliberately short — first sentence is the header,
// next-move nudge is one short imperative. No exclamation marks
// (jarring on terminal); no emojis (out-of-vocabulary in the design
// system); plain English voice consistent with Track 10.7.
func EmptyStateFor(kind EmptyStateKind) EmptyState {
	switch kind {
	case EmptyZeroFindings:
		return EmptyState{
			Kind:     kind,
			Header:   "Nothing to flag — your test system looks healthy.",
			NextMove: "Run `terrain compare` over time to track posture; this clean state is the bar to hold.",
		}
	case EmptyNoAISurfaces:
		return EmptyState{
			Kind:     kind,
			Header:   "No AI surfaces detected in this repo.",
			NextMove: "Skipping AI risk review. Run `terrain ai list` to confirm if you expected AI surfaces.",
		}
	case EmptyNoPolicyFile:
		return EmptyState{
			Kind:     kind,
			Header:   "No policy file found.",
			NextMove: "Run `terrain init` to scaffold `.terrain/policy.yaml`, then re-run policy check.",
		}
	case EmptyFirstRun:
		return EmptyState{
			Kind:     kind,
			Header:   "First time here? Welcome.",
			NextMove: "Try `terrain analyze` to map your test terrain — typical service repos finish in 5–15 seconds.",
		}
	case EmptyNoImpact:
		return EmptyState{
			Kind:     kind,
			Header:   "This change has no impact on the test system.",
			NextMove: "Merge with confidence — no impacted units, no protection gaps introduced. Run `terrain analyze` to confirm overall posture is unchanged.",
		}
	case EmptyNoTestSelection:
		return EmptyState{
			Kind:     kind,
			Header:   "No tests selected for this change.",
			NextMove: "Either the change is purely structural (docs, config) or its impact graph is empty. Re-run with `--explain-selection` to see why.",
		}
	case EmptyNoMigrationCandidates:
		return EmptyState{
			Kind:     kind,
			Header:   "No migration candidates detected.",
			NextMove: "Either the repo is already on the framework of record, or none of the supported source frameworks are in use. Run `terrain migrate list` to see what's supported.",
		}
	default:
		// Unknown kind — return empty so the renderer skips. Keeps
		// the contract: only designed kinds render anything.
		return EmptyState{Kind: kind}
	}
}

// RenderEmptyState writes an empty-state to a terminal-text writer.
// Format is two lines: header, indented next-move (when present).
// Trailing blank line is the caller's responsibility — keeps the
// helper symmetric with renderFindingCard and friends in
// internal/changescope/render.go.
func RenderEmptyState(w io.Writer, kind EmptyStateKind) {
	es := EmptyStateFor(kind)
	if es.Header == "" {
		return
	}
	fmt.Fprintln(w, es.Header)
	if es.NextMove != "" {
		fmt.Fprintln(w, "  → "+es.NextMove)
	}
}

// EmptyStateMarkdown renders an empty-state for inclusion in PR-comment
// markdown output. Uses a blockquote callout for the header (renders
// as a tinted callout on GitHub) plus an italicized next-move line.
// Designed to fit the same visual rhythm as the populated stanzas in
// internal/changescope/render.go.
func EmptyStateMarkdown(kind EmptyStateKind) string {
	es := EmptyStateFor(kind)
	if es.Header == "" {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "> %s\n", es.Header)
	if es.NextMove != "" {
		fmt.Fprintf(&b, "\n*%s*\n", es.NextMove)
	}
	return b.String()
}
