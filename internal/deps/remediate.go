package deps

import (
	"bytes"
	"path/filepath"
	"regexp"

	"github.com/pmclSF/terrain/internal/findings"
	"github.com/pmclSF/terrain/internal/saferead"
)

// caretInValue strips a leading caret from an npm version spec in JSON value
// position: `"dep": "^1.2.3"` → `"dep": "1.2.3"`. Anchored to a colon +
// quote + caret + a single version token that runs to the closing quote, so
// it only rewrites a value that is EXACTLY one caret-anchored version. A
// compound / OR range (`"^1.2.3 || ^2.0.0"`) is left untouched: stripping only
// the leading caret would leave a remaining alternative floating while
// re-analysis wrongly reads the value as pinned. Everything else in the
// manifest (formatting, other keys) stays byte-identical.
var caretInValue = regexp.MustCompile(`(:\s*")\^(\d[\w.\-+]*")`)

// pinnableDepObjects are the manifest sections whose caret ranges are safe to
// pin: application dependencies the repo itself installs. peerDependencies is
// deliberately excluded — pinning a peer range is a consumer-breaking
// anti-pattern (it over-constrains every downstream installer). engines,
// resolutions/overrides, and the package's own version are not dependency
// specs at all and must never be rewritten (`"engines":{"node":"^18"}` means
// ">=18 <19", not "==18").
var pinnableDepObjects = []string{"dependencies", "devDependencies", "optionalDependencies"}

// stripCaretsInDepObjects rewrites `^X` → `X` version specs only inside the
// pinnable dependency objects, byte-for-byte preserving everything else
// (formatting, key order, peerDependencies, engines, resolutions, version). A
// whole-manifest regex would corrupt those other sections.
func stripCaretsInDepObjects(data []byte) []byte {
	out := data
	for _, key := range pinnableDepObjects {
		out = stripCaretsInObject(out, key)
	}
	return out
}

// stripCaretsInObject applies caretInValue only within the brace-matched value
// object of the given top-level key. Dependency objects hold string values
// only (no nested braces), so a simple brace counter locates the span exactly.
func stripCaretsInObject(data []byte, key string) []byte {
	keyPat := []byte(`"` + key + `"`)
	idx := bytes.Index(data, keyPat)
	if idx < 0 {
		return data
	}
	i := idx + len(keyPat)
	// Advance to the object's opening brace, tolerating only whitespace/colon.
	for i < len(data) && data[i] != '{' {
		switch data[i] {
		case ' ', '\t', '\n', '\r', ':':
			i++
		default:
			return data // value isn't an object (unexpected shape) — leave untouched
		}
	}
	if i >= len(data) {
		return data
	}
	start := i
	depth := 0
	end := -1
	for ; i < len(data); i++ {
		switch data[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				end = i + 1
			}
		}
		if end >= 0 {
			break
		}
	}
	if end < 0 {
		return data
	}
	fixed := caretInValue.ReplaceAll(data[start:end], []byte("$1$2"))
	if bytes.Equal(fixed, data[start:end]) {
		return data
	}
	out := make([]byte, 0, len(data)-(end-start)+len(fixed))
	out = append(out, data[:start]...)
	out = append(out, fixed...)
	out = append(out, data[end:]...)
	return out
}

// PinCaretsFix returns a mechanically-applicable edit_in_place remediation
// for a drift-risk finding on an npm manifest: rewrite package.json pinning
// every caret-range dep (`^1.2.3` → `1.2.3`). The concrete version is
// already present in a caret spec, so this needs no registry or lockfile.
//
// It returns (fix, true) ONLY when stripping carets is SUFFICIENT to clear
// the finding — i.e. the remaining strict-pin issues (bare names, `*`,
// `latest`, loose ranges, which carry no version to pin to) fall below the
// detector's moving-target threshold. Otherwise it returns (nil, false):
// those manifests are judge-only, because no deterministic edit Terrain can
// make would resolve the finding. This is the honest mechanical/judge split
// the trust floor demands.
func PinCaretsFix(root, manifestRel string) (*findings.Fix, bool) {
	if filepath.Base(manifestRel) != "package.json" {
		return nil, false // only npm carets are version-bearing; others judge-only
	}
	abs := filepath.Join(root, manifestRel)
	data, err := saferead.ReadFile(abs)
	if err != nil {
		return nil, false
	}
	stats := analyseNPM(data)
	if stats.TotalDeps == 0 || stats.CaretIssues == 0 {
		return nil, false // nothing to pin mechanically
	}

	// Pin carets only inside the app-dependency objects (never peer/engines/
	// resolutions/version). If that changes nothing — e.g. every caret lives in
	// peerDependencies — there is no safe mechanical fix, so stay judge-only.
	pinned := stripCaretsInDepObjects(data)
	if bytes.Equal(pinned, data) {
		return nil, false
	}
	// Sufficiency, checked by RE-ANALYSING the pinned manifest rather than a
	// residual formula: the fix clears the finding only if the moving-target
	// share the detector would now compute falls below its threshold. This
	// keeps the "is this fix enough?" gate consistent with what the detector
	// sees, and correctly stays judge-only when un-pinnable peer/strict carets
	// keep the manifest over threshold.
	after := analyseNPM(pinned)
	if after.TotalDeps == 0 || float64(after.MovingTargets)/float64(after.TotalDeps) >= movingTargetShare {
		return nil, false
	}
	return &findings.Fix{
		Kind:    findings.FixEditInPlace,
		Path:    manifestRel,
		Content: string(pinned),
	}, true
}
