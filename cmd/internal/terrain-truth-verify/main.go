// Command terrain-truth-verify enforces the contract between
// authored documentation and the canonical signal manifest.
//
// Track 9.7 of the parity-gated 0.2.0 release plan calls for this
// gate: drift between what the README / feature-status doc /
// CHANGELOG promise and what the engine actually ships is the
// failure mode adopters notice when they evaluate the binary
// against the marketing claim. `make truth-verify` catches it
// before the release does.
//
// Scope today (0.2):
//
//  1. Every signal name appearing in docs/release/feature-status.md
//     under the "Detectors / signal types" sections must reference
//     a real entry in the canonical signal manifest. A signal name
//     in the doc that doesn't exist in code is a broken promise.
//
//  2. Every stable manifest entry should be acknowledged in the
//     feature-status doc OR be marked as appearing only in
//     docs/signals/manifest.json (the auto-generated full inventory).
//     The doc explicitly says it's a "curated view"; this check
//     surfaces "stable signals that aren't even in the curated
//     view" — a different drift shape from the missing-from-code one.
//
// Out of scope today (0.3+):
//
//  - README command list ⊆ dispatcher: requires the Track 9.6
//    registry refactor; without it, parsing main.go for the truth
//    is brittle.
//  - CHANGELOG promotion-claim cross-check: useful but lower
//    priority; the manifest already drives the per-signal status,
//    so any "promoted to stable" claim that's wrong is already
//    visible in `make docs-verify`.
//  - CI matrix ⊆ compatibility tier doc: useful but distinct
//    failure mode; lives in workflow YAML rather than markdown.
//
// Exit codes:
//
//  0 — every documented signal resolves; no orphan stable signals
//  1 — one or more drifts (output names every offender)
//  2 — invocation error (missing files, parse failures)
//
// Wired into the release-readiness pipeline via `make truth-verify`.
package main

import (
	"flag"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/pmclSF/terrain/internal/models"
	"github.com/pmclSF/terrain/internal/signals"
)

// signalRefPattern matches a backtick-delimited camelCase signal
// reference. Two constraints disambiguate signal names from CLI
// verbs and placeholder words:
//
//  1. Lower-case letter start (Terrain signal types are lower-camel)
//  2. At least one upper-case letter somewhere in the name (true
//     camelCase) — excludes single English words like `report`,
//     `eval`, `policy` that appear in code spans throughout the doc
//     but aren't signal types.
//
// This is heuristic — a future signal named `foo` (all lowercase)
// would slip through — but every signal in the manifest today is
// camelCase with at least one uppercase letter mid-word, and the
// drift gate is more useful than a perfect-recall pattern that
// drowns in false positives.
var signalRefPattern = regexp.MustCompile(`\x60([a-z][a-z0-9]*[A-Z][A-Za-z0-9]*)\x60`)

// detectorSectionPattern marks the start of the "Detectors /
// signal types" portion of the doc. We scan from this anchor to
// EOF — every signal-name reference after the anchor is in scope.
// References before the anchor (workflow / CLI tables) are not
// signal-name references and would false-positive on terms like
// `analyze` or `init`.
const detectorSectionAnchor = "## Detectors / signal types"

func main() {
	docPath := flag.String("doc", "docs/release/feature-status.md",
		"feature-status doc to verify")
	checkOrphans := flag.Bool("check-orphans", true,
		"also report stable manifest signals missing from the doc")
	flag.Parse()

	doc, err := os.ReadFile(*docPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "truth-verify: read doc %q: %v\n", *docPath, err)
		os.Exit(2)
	}

	docSignals := extractDocSignalNames(string(doc))
	manifest := signals.Manifest()

	manifestByType := map[models.SignalType]signals.ManifestEntry{}
	for _, e := range manifest {
		manifestByType[e.Type] = e
	}

	var brokenRefs []string  // doc names a signal that doesn't exist
	var orphanStable []string // stable signal not mentioned in doc

	for name := range docSignals {
		if _, ok := manifestByType[models.SignalType(name)]; !ok {
			brokenRefs = append(brokenRefs, name)
		}
	}

	if *checkOrphans {
		for _, e := range manifest {
			if e.Status != signals.StatusStable {
				continue
			}
			// Skip engine self-diagnostic signals — adopters don't
			// need them in the curated doc; they're documented
			// inline alongside the panic-recovery / budget /
			// missing-input mechanisms.
			if isEngineDiagnostic(e.Type) {
				continue
			}
			if !docSignals[string(e.Type)] {
				orphanStable = append(orphanStable, string(e.Type))
			}
		}
	}

	rc := 0

	if len(brokenRefs) > 0 {
		sort.Strings(brokenRefs)
		fmt.Fprintf(os.Stderr, "::error::%d broken signal reference(s) in %s:\n",
			len(brokenRefs), *docPath)
		fmt.Fprintln(os.Stderr,
			"  these names appear in the doc but have no entry in internal/signals/manifest.go.")
		fmt.Fprintln(os.Stderr,
			"  either remove the reference, fix the typo, or add the manifest entry.")
		for _, name := range brokenRefs {
			fmt.Fprintf(os.Stderr, "  - %s\n", name)
		}
		rc = 1
	}

	if len(orphanStable) > 0 {
		sort.Strings(orphanStable)
		fmt.Fprintf(os.Stderr,
			"::warning::%d stable signal(s) in the manifest are not surfaced in %s:\n",
			len(orphanStable), *docPath)
		fmt.Fprintln(os.Stderr,
			"  the curated table should mention these explicitly OR the doc should")
		fmt.Fprintln(os.Stderr,
			"  add a sentence pointing readers at docs/signals/manifest.json for the full list.")
		for _, name := range orphanStable {
			fmt.Fprintf(os.Stderr, "  - %s\n", name)
		}
		// Orphans are advisory by default — they don't block CI.
		// Pass --strict-orphans on the command line via the
		// Makefile target if you want orphans to fail the build.
		if strictOrphans() {
			rc = 1
		}
	}

	if rc == 0 {
		fmt.Printf("truth-verify: %s — every documented signal resolves; %d signals reviewed.\n",
			*docPath, len(docSignals))
	}
	os.Exit(rc)
}

// plannedSectionAnchor marks the start of the "Planned" subsection
// of the doc. References inside the planned section name signals
// that explicitly *don't* have a code-side implementation today;
// flagging them would invert the signal — we'd be telling the doc
// to stop being honest about future capabilities.
const plannedSectionAnchor = "### Planned"

// extractDocSignalNames pulls every backtick-delimited camelCase
// token from the doc between the detector-section anchor and the
// planned subsection. Names before the anchor are excluded — they
// false-positive on CLI verbs. Names after the planned anchor are
// excluded — those are intentionally future references.
func extractDocSignalNames(doc string) map[string]bool {
	start := strings.Index(doc, detectorSectionAnchor)
	if start < 0 {
		// No anchor — empty doc or unfamiliar shape; nothing to check.
		return nil
	}
	body := doc[start:]

	if end := strings.Index(body, plannedSectionAnchor); end >= 0 {
		body = body[:end]
	}

	out := map[string]bool{}
	matches := signalRefPattern.FindAllStringSubmatch(body, -1)
	for _, m := range matches {
		out[m[1]] = true
	}
	return out
}

// isEngineDiagnostic returns true for the meta-signals emitted by
// the pipeline itself rather than by registered detectors. These
// don't appear in the curated feature-status table; their behavior
// is documented alongside the panic-recovery / budget /
// missing-input mechanisms.
func isEngineDiagnostic(t models.SignalType) bool {
	switch t {
	case "detectorPanic", "detectorBudgetExceeded", "detectorMissingInput",
		"suppressionExpired":
		return true
	}
	return false
}

// strictOrphans reports whether --strict-orphans was passed.
// flag.Lookup is used rather than declaring the flag inline so the
// usage section reads cleanly; this is the only opt-in escalation.
func strictOrphans() bool {
	for _, a := range os.Args[1:] {
		if a == "--strict-orphans" || a == "-strict-orphans" {
			return true
		}
	}
	return false
}
