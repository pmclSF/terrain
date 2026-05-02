package aidetect

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/pmclSF/terrain/internal/models"
	"github.com/pmclSF/terrain/internal/signals"
)

// PromptVersioningDetector flags prompt-kind surfaces that ship
// without a recognisable version marker. The round-4 plan called for
// detecting "prompt content changed without version bump" via baseline
// comparison; that variant lands when ContentHashes are persisted on
// the snapshot. This 0.2 detector ships the simpler-but-actionable
// static check: any prompt that doesn't declare a version is at risk
// of silent drift the next time it changes.
//
// Recognised version markers:
//   - YAML key `version:` (with any value) at column 0
//   - JSON `"version":` at the top level (heuristic: appears before
//     the first prompt body)
//   - Filename suffix `_vN` / `-vN` / `.vN` where N is one or more
//     digits (e.g. `assistant_v2.yaml`, `prompt-v3.txt`)
//   - Inline `# version: ...` / `// version: ...` comment near the
//     top of the file
type PromptVersioningDetector struct {
	// Root is the absolute path of the repo. Snapshot paths are
	// repo-relative.
	Root string
}

// promptKinds identifies the surface kinds we treat as
// "user-facing prompts that should be versioned". Inline prompts
// detected via Pass 1a (string-literal patterns) tend to be incidental
// — we only require versioning on top-level prompt files.
var versionablePromptKinds = map[models.CodeSurfaceKind]bool{
	models.SurfacePrompt:  true,
	models.SurfaceContext: true,
}

// versionableExtensions is the file-extension allowlist for the
// detector's content-read pass. Prompts in source code (Python /
// JavaScript files) are noisy to flag because they're often inline
// f-strings — we focus on standalone prompt files.
var versionableExtensions = map[string]bool{
	".yaml":     true,
	".yml":      true,
	".json":     true,
	".md":       true,
	".prompt":   true,
	".tmpl":     true,
	".hbs":      true,
	".j2":       true,
	".mustache": true,
	".txt":      true,
}

// filenameVersionPattern matches a version suffix in the filename
// stem: `assistant_v2`, `prompt-v3`, `system.v1`. The suffix N can be
// one or more digits.
var filenameVersionPattern = regexp.MustCompile(`(?:[_\-.]v\d+)$`)

// inlineVersionPattern matches a YAML / config / comment-style
// version declaration anywhere in the first 64 lines of the file.
// Requires a non-empty value matching digits / semver / calver / a
// quoted token.
//
// Pre-0.2.x the pattern only required `version:` followed by anything
// (or nothing). `version: TODO` and `version:` with no value satisfied
// the check, defeating the detector's intent — silent prompt drift.
// Now we require a recognisable version literal.
var inlineVersionPattern = regexp.MustCompile(
	`(?i)(?:^|\s)(?:#|//|\*)?\s*"?version"?\s*[:=]\s*` +
		`(?:` +
		`["']?\d+(?:\.\d+){0,2}["']?` + // 1, 1.2, 1.2.3 (optionally quoted)
		`|["']?v\d+(?:\.\d+){0,2}["']?` + // v1, v1.2
		`|["']?\d{4}-\d{2}-\d{2}["']?` + // calver
		`|"[^"\s][^"]*"` + // quoted non-empty token
		`|'[^'\s][^']*'` +
		`)`,
)

// Detect emits SignalAIPromptVersioning per unversioned prompt surface.
func (d *PromptVersioningDetector) Detect(snap *models.TestSuiteSnapshot) []models.Signal {
	if d == nil || snap == nil {
		return nil
	}

	seen := map[string]bool{}
	var out []models.Signal
	for _, surface := range snap.CodeSurfaces {
		if !versionablePromptKinds[surface.Kind] {
			continue
		}
		if !versionableExtensions[strings.ToLower(filepath.Ext(surface.Path))] {
			continue
		}
		// One signal per unique file even when the file contains
		// multiple prompt surfaces — versioning is a per-file
		// property.
		if seen[surface.Path] {
			continue
		}
		seen[surface.Path] = true

		if filenameLooksVersioned(surface.Path) {
			continue
		}
		abs := filepath.Join(d.Root, surface.Path)
		if fileHasInlineVersion(abs) {
			continue
		}

		out = append(out, models.Signal{
			Type:        signals.SignalAIPromptVersioning,
			Category:    models.CategoryAI,
			Severity:    models.SeverityMedium,
			Confidence:  0.85,
			Location:    models.SignalLocation{File: surface.Path, Symbol: surface.Name},
			Explanation: "Prompt file `" + surface.Path + "` has no recognisable version marker. Future content changes will silently drift; consumers can't detect the change.",
			SuggestedAction: "Add a `version:` field, a `_v<N>` suffix to the filename, or a `# version: ...` comment so downstream consumers can detect content drift.",

			SeverityClauses: []string{"sev-medium-007"},
			Actionability:   models.ActionabilityScheduled,
			LifecycleStages: []models.LifecycleStage{models.StageDesign, models.StageMaintenance},
			AIRelevance:     models.AIRelevanceHigh,
			RuleID:          "TER-AI-101",
			RuleURI:         "docs/rules/ai/prompt-versioning.md",
			DetectorVersion: "0.2.0",
			ConfidenceDetail: &models.ConfidenceDetail{
				Value:        0.85,
				IntervalLow:  0.75,
				IntervalHigh: 0.92,
				Quality:      "heuristic",
				Sources:      []models.EvidenceSource{models.SourceStructuralPattern},
			},
			EvidenceSource:   models.SourceStructuralPattern,
			EvidenceStrength: models.EvidenceModerate,
			Metadata: map[string]any{
				"surfaceId": surface.SurfaceID,
				"kind":      string(surface.Kind),
			},
		})
	}
	return out
}

// filenameLooksVersioned returns true when the filename stem ends in
// `_vN` / `-vN` / `.vN`. Picks up the conventional pattern of pinning
// versions in the filename.
func filenameLooksVersioned(path string) bool {
	stem := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	return filenameVersionPattern.MatchString(stem)
}

// fileHasInlineVersion returns true when the first 64 lines of the
// file contain a version-shaped declaration. Bounds the read so a
// huge prompt file doesn't trigger a full scan; versioning markers
// virtually always appear at the top.
//
// 0.2.0 final-polish: even after `inlineVersionPattern` was tightened
// to require a recognisable version literal, quoted-non-empty branch
// still accepted `"TODO"`, `"tbd"`, `"xxx"`, `"?"`, `"unknown"` etc.
// Reject those placeholder tokens explicitly so a comment like
// `version: "TODO"` doesn't silence the detector.
func fileHasInlineVersion(absPath string) bool {
	f, err := os.Open(absPath)
	if err != nil {
		return false
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	const probeLines = 64
	const maxLine = 1 << 16
	buf := make([]byte, maxLine)
	sc.Buffer(buf, maxLine)

	count := 0
	for sc.Scan() {
		count++
		if count > probeLines {
			break
		}
		text := sc.Text()
		if !inlineVersionPattern.MatchString(text) {
			continue
		}
		if lineLooksLikePlaceholderVersion(text) {
			continue
		}
		return true
	}
	return false
}

// versionPlaceholderPattern catches `version: "TODO"` / `version=TBD`
// / `version: ???` and other obvious placeholders that should NOT
// satisfy the inline-version requirement. The pattern is
// case-insensitive and matches the value side of the assignment only.
var versionPlaceholderPattern = regexp.MustCompile(
	`(?i)(?:^|\s)(?:#|//|\*)?\s*"?version"?\s*[:=]\s*` +
		`["']?(?:TODO|TBD|FIXME|XXX|\?+|unknown|placeholder|none)["']?\s*$`,
)

func lineLooksLikePlaceholderVersion(text string) bool {
	return versionPlaceholderPattern.MatchString(strings.TrimSpace(text))
}
