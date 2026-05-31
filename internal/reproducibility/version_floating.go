// Package reproducibility implements the reproducibility/* stable
// rules. These flag patterns that introduce non-determinism into a
// test or eval run — version drift in dependencies, missing seeds in
// stochastic code, environment values that aren't pinned.
//
// All detectors in this package operate on artifacts the manifest /
// surface-detection layers (Tier 0/1) already extract; the package
// stays free of file-walking concerns.
package reproducibility

import (
	"fmt"
	"strings"

	"github.com/pmclSF/terrain/internal/manifest"
	"github.com/pmclSF/terrain/internal/models"
	"github.com/pmclSF/terrain/internal/signals"
)

// DetectVersionFloating walks parsed dependency manifests and emits one
// Signal per dependency with a non-exact version pinning. Implements
// the rule terrain/reproducibility/version-floating.
//
// Severity ladder:
//
//   - PinningUnpinned     → high  ("foo" with no spec at all — any
//     version may be resolved at install
//     time)
//   - PinningRange        → medium (^1.2.3, >=1.0,<2.0 — admits patch/
//     minor drift)
//   - PinningGit (non-SHA) → medium (git+https://...@branch — moving
//     reference, not content-addressed)
//   - PinningGit (SHA)    → suppressed (committed SHA is reproducible)
//   - PinningURL          → medium (tarball / wheel URL may move)
//   - PinningPath         → low    (file:// or ./local — reproducible
//     within the repo checkout)
//   - PinningExact        → suppressed
//
// Build / dev dependencies are flagged at one severity step lower
// than runtime; the rule explanation notes the section to make the
// distinction visible.
func DetectVersionFloating(manifests []*manifest.Manifest) []models.Signal {
	var out []models.Signal
	for _, m := range manifests {
		if m == nil {
			continue
		}
		for _, dep := range m.Dependencies {
			sig, ok := buildVersionFloatingSignal(m, dep)
			if !ok {
				continue
			}
			out = append(out, sig)
		}
	}
	return out
}

func buildVersionFloatingSignal(m *manifest.Manifest, dep manifest.Dependency) (models.Signal, bool) {
	severity, classification, ok := classifyDepPinning(dep)
	if !ok {
		return models.Signal{}, false
	}
	// Step down for dev / build / optional sections.
	if dep.Section != manifest.SectionRuntime {
		severity = stepDownSeverity(severity)
	}

	explanation := fmt.Sprintf(
		"Dependency %q in %s has %s. Subsequent installs may resolve to a different version, introducing non-determinism in test and eval runs.",
		dep.Name, m.Path, classification,
	)
	suggestion := fmt.Sprintf(
		"Pin to an exact version (e.g., %q in requirements.txt or a fixed semver in package.json), or commit a lockfile that records the resolved set.",
		dep.Name+"==<version>",
	)

	return models.Signal{
		Type:             signals.SignalVersionFloating,
		Category:         models.CategoryQuality,
		Severity:         severity,
		Confidence:       confidenceForPinning(dep.Pinning),
		EvidenceStrength: models.EvidenceStrong,
		EvidenceSource:   models.SourceStructuralPattern,
		Location: models.SignalLocation{
			File: m.Path,
			Line: dep.Line,
		},
		Explanation:     explanation,
		SuggestedAction: suggestion,
		RuleID:          "terrain/reproducibility/version-floating",
		RuleURI:         "docs/rules/reproducibility/version-floating.md",
		DetectorVersion: "0.2.0",
		Metadata: map[string]any{
			"dependency": dep.Name,
			"spec":       dep.Spec,
			"pinning":    string(dep.Pinning),
			"section":    string(dep.Section),
			"ecosystem":  string(m.Ecosystem),
		},
	}, true
}

func classifyDepPinning(dep manifest.Dependency) (models.SignalSeverity, string, bool) {
	switch dep.Pinning {
	case manifest.PinningUnpinned:
		return models.SeverityHigh, "no version specifier (unpinned)", true
	case manifest.PinningRange:
		return models.SeverityMedium, "a range version specifier (" + dep.Spec + ")", true
	case manifest.PinningGit:
		// Git refs may or may not be reproducible depending on whether
		// the reference is a commit SHA. We treat anything that looks
		// like a 40-hex SHA suffix as pinned; everything else (branches,
		// tags) as moving.
		if hasGitCommitSHA(dep.Spec) {
			return "", "", false
		}
		return models.SeverityMedium, "a moving Git reference (" + dep.Spec + ")", true
	case manifest.PinningURL:
		return models.SeverityMedium, "a direct URL reference (" + dep.Spec + ")", true
	case manifest.PinningPath:
		return models.SeverityLow, "a local-path reference (" + dep.Spec + ")", true
	}
	return "", "", false
}

func confidenceForPinning(p manifest.Pinning) float64 {
	switch p {
	case manifest.PinningUnpinned:
		return 0.99 // the absence of a spec is unambiguous
	case manifest.PinningRange, manifest.PinningURL:
		return 0.95
	case manifest.PinningGit:
		return 0.85
	case manifest.PinningPath:
		return 0.9
	}
	return 0.8
}

func stepDownSeverity(s models.SignalSeverity) models.SignalSeverity {
	switch s {
	case models.SeverityHigh:
		return models.SeverityMedium
	case models.SeverityMedium:
		return models.SeverityLow
	}
	return s
}

// hasGitCommitSHA returns true when spec contains a 40-hex commit SHA
// somewhere (typical pip / npm conventions append `@<sha>` to the URL).
// A 7-or-more-hex run after `@` is accepted to handle short SHAs which
// adopters sometimes use deliberately.
func hasGitCommitSHA(spec string) bool {
	at := strings.LastIndex(spec, "@")
	if at < 0 {
		return false
	}
	candidate := spec[at+1:]
	// Strip trailing `#egg=...` fragment when present.
	if i := strings.Index(candidate, "#"); i >= 0 {
		candidate = candidate[:i]
	}
	if len(candidate) < 7 {
		return false
	}
	for _, r := range candidate {
		switch {
		case r >= '0' && r <= '9',
			r >= 'a' && r <= 'f',
			r >= 'A' && r <= 'F':
		default:
			return false
		}
	}
	return true
}
