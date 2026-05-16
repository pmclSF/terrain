// Package stages contains the concrete pipeline.Stage implementations
// used by internal/aipipeline.
//
// Each stage is independently testable. Stages emit EvidenceAtoms into
// the candidate's atom slice and return Continue=false to drop the
// candidate from further evaluation.
package stages

import (
	"context"
	"regexp"
	"strings"

	"github.com/pmclSF/terrain/internal/aipipeline"
)

// PathPrefilter is Stage 1: directory / filename gates. It emits
// negative atoms for paths matching known-noisy patterns (examples,
// tests, provider wrappers, factories) and short-circuits on the
// strongest negative signals.
//
// The patterns mirror the empirical "combined-v3" path filter from
// the precision study (2026-05-15), which on the 2,651-row corpus
// lifted precision from 1.96% → 2.72% with zero TP loss.
type PathPrefilter struct {
	// HardDropExamples causes the stage to drop candidates outright
	// when a path matches an examples/tutorials/cookbook directory.
	// Default true — these are almost never actionable for app users.
	HardDropExamples bool

	// HardDropTests likewise drops test files. Default true.
	HardDropTests bool
}

// NewPathPrefilter returns a stage with conservative defaults.
func NewPathPrefilter() *PathPrefilter {
	return &PathPrefilter{
		HardDropExamples: true,
		HardDropTests:    true,
	}
}

// Name implements pipeline.Stage.
func (s *PathPrefilter) Name() string { return "path-prefilter" }

// Run evaluates the candidate's path against the suppression patterns,
// emits atoms, and short-circuits when a hard-drop pattern matches.
func (s *PathPrefilter) Run(_ context.Context, c *aipipeline.Candidate) aipipeline.StageResult {
	p := c.Path
	base := lastPathSegment(p)
	baseNoExt := stripExtension(base)
	lowerPath := strings.ToLower(p)
	fullForExamples := "/" + lowerPath

	if examplesPathRE.MatchString(fullForExamples) {
		c.AddAtom(aipipeline.EvidenceAtom{
			Kind:   aipipeline.EvidenceNegative,
			RuleID: "path.examples",
			Source: "path-prefilter",
			Weight: -3.0,
			Span:   aipipeline.Span{Snippet: p},
		})
		if s.HardDropExamples {
			return aipipeline.StageResult{Continue: false}
		}
	}
	if testsPathRE.MatchString(p) {
		c.AddAtom(aipipeline.EvidenceAtom{
			Kind:   aipipeline.EvidenceNegative,
			RuleID: "path.tests",
			Source: "path-prefilter",
			Weight: -2.5,
			Span:   aipipeline.Span{Snippet: p},
		})
		if s.HardDropTests {
			return aipipeline.StageResult{Continue: false}
		}
	}
	if providerDirsRE.MatchString(p) {
		c.AddAtom(aipipeline.EvidenceAtom{
			Kind:   aipipeline.EvidenceNegative,
			RuleID: "path.providers",
			Source: "path-prefilter",
			Weight: -2.0,
			Span:   aipipeline.Span{Snippet: p},
		})
	}
	if providerFilenameRE.MatchString(base) {
		c.AddAtom(aipipeline.EvidenceAtom{
			Kind:   aipipeline.EvidenceNegative,
			RuleID: "path.providers",
			Source: "path-prefilter",
			Weight: -1.5,
			Span:   aipipeline.Span{Snippet: base},
		})
	}
	if llmsSubdirRE.MatchString(p) {
		c.AddAtom(aipipeline.EvidenceAtom{
			Kind:   aipipeline.EvidenceNegative,
			RuleID: "path.llms_subdir_base",
			Source: "path-prefilter",
			Weight: -2.0,
			Span:   aipipeline.Span{Snippet: p},
		})
	}
	if factoryFilenameRE.MatchString(base) {
		c.AddAtom(aipipeline.EvidenceAtom{
			Kind:   aipipeline.EvidenceNegative,
			RuleID: "path.factory_filename",
			Source: "path-prefilter",
			Weight: -1.5,
			Span:   aipipeline.Span{Snippet: base},
		})
	}
	if snakeSuffixRE.MatchString(baseNoExt) {
		c.AddAtom(aipipeline.EvidenceAtom{
			Kind:   aipipeline.EvidenceNegative,
			RuleID: "path.snake_suffix_wrapper",
			Source: "path-prefilter",
			Weight: -1.5,
			Span:   aipipeline.Span{Snippet: base},
		})
	}
	if exactNameUtility[strings.ToLower(baseNoExt)] {
		c.AddAtom(aipipeline.EvidenceAtom{
			Kind:   aipipeline.EvidenceNegative,
			RuleID: "path.exact_name_utility",
			Source: "path-prefilter",
			Weight: -1.2,
			Span:   aipipeline.Span{Snippet: base},
		})
	}

	return aipipeline.StageResult{Continue: true}
}

func lastPathSegment(p string) string {
	if i := strings.LastIndex(p, "/"); i >= 0 {
		return p[i+1:]
	}
	return p
}

func stripExtension(name string) string {
	for _, ext := range []string{".py", ".ts", ".tsx", ".js", ".mjs"} {
		if strings.HasSuffix(name, ext) {
			return name[:len(name)-len(ext)]
		}
	}
	return name
}

var (
	providerDirsRE     = regexp.MustCompile(`(?i)(^|/)(providers?|adapters?|connectors?|integrations?|readers?|loaders?|writers?)/`)
	providerFilenameRE = regexp.MustCompile(`\w+(?:Provider|Adapter|Client|Wrapper|Backend|Connector)\.(?:py|ts|tsx|js)$`)
	llmsSubdirRE       = regexp.MustCompile(`(?i)/llms/[^/]+/(?:base|llm|provider|client)\.py$`)
	factoryFilenameRE  = regexp.MustCompile(`(?i)(_factory|_parser|_config|_settings|_schema|_model|_setting)\.(?:py|ts)$`)
	snakeSuffixRE      = regexp.MustCompile(`(?i)_(client|provider|adapter|wrapper|llm|chat|model|handler|api|storage|store|loader|engine|executor|router|route|backend|connector|integration|node|state|tool|skill)$`)
	// examplesPathRE matches both bare directories (examples/, demos/) and
	// suffix-style directories (*_demo/, *_demos/, *_examples/), plus the
	// "runnable_scripts" convention used in agent / RAG cookbooks.
	examplesPathRE = regexp.MustCompile(`(?:^|/)(?:[\w\-]*_)?(?:examples?|tutorials?|cookbook|notebooks?|demos?|samples?|runnable_scripts?)/`)
	testsPathRE    = regexp.MustCompile(`(?:^|/)(?:tests?|spec|specs)/|_test\.(?:py|ts)$|/test_[^/]+\.(?:py|ts)$|conftest\.py$`)

	exactNameUtility = map[string]bool{
		"base":       true,
		"types":      true,
		"type":       true,
		"constants":  true,
		"enums":      true,
		"errors":     true,
		"exceptions": true,
		"common":     true,
		"schema":     true,
		"schemas":    true,
		"settings":   true,
	}
)
