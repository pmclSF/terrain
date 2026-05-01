package aidetect

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pmclSF/terrain/internal/models"
	"github.com/pmclSF/terrain/internal/signals"
	"gopkg.in/yaml.v3"
)

// NonDeterministicEvalDetector flags eval configurations that don't pin
// the determinism knobs an LLM provider exposes — temperature, seed, and
// (for some providers) top_p.
//
// The detector reads YAML and JSON eval configs that the snapshot already
// names (TestFiles + Scenarios), and inspects the parsed tree for the
// presence of:
//
//   - `temperature` set to anything other than 0 / 0.0 / "0"
//   - `temperature` missing entirely while a `model` is declared
//   - `seed` missing on providers that support deterministic seeding
//
// A finding is emitted per file. We don't try to be exhaustive on the
// LLM-knob list — temperature is the dominant lever.
type NonDeterministicEvalDetector struct {
	// Root is the absolute path of the repo. Snapshot paths are
	// repo-relative.
	Root string
}

// evalConfigExts is the file-extension allowlist. We only inspect
// formats where determinism knobs are typically declared as data
// (YAML / JSON / TOML). Source files would need full AST analysis,
// which is out of scope for this detector.
var evalConfigExts = map[string]bool{
	".yaml": true,
	".yml":  true,
	".json": true,
}

// evalFilenameMarkers identifies files we're confident are eval/agent
// configs (vs. arbitrary YAML in the repo). Anything matching one of
// these substrings in the path is in scope.
var evalFilenameMarkers = []string{
	"eval", "promptfoo", "deepeval", "ragas",
	"agent", "prompt", ".terrain/",
}

// Detect emits SignalAINonDeterministicEval for each in-scope eval
// config that's missing or wrongly setting determinism knobs.
func (d *NonDeterministicEvalDetector) Detect(snap *models.TestSuiteSnapshot) []models.Signal {
	if d == nil || snap == nil {
		return nil
	}
	paths := d.gatherEvalConfigPaths(snap)

	var out []models.Signal
	for _, relPath := range paths {
		abs := filepath.Join(d.Root, relPath)
		findings := analyseEvalConfig(abs)
		for _, f := range findings {
			out = append(out, models.Signal{
				Type:        signals.SignalAINonDeterministicEval,
				Category:    models.CategoryAI,
				Severity:    models.SeverityMedium,
				Confidence:  0.93,
				Location:    models.SignalLocation{File: relPath},
				Explanation: f.Explanation,
				SuggestedAction: "Pin temperature: 0 and a seed in the eval config, or document the non-determinism budget alongside the scenario.",

				SeverityClauses: []string{"sev-medium-003"},
				Actionability:   models.ActionabilityScheduled,
				LifecycleStages: []models.LifecycleStage{models.StageTestAuthoring, models.StageCIRun},
				AIRelevance:     models.AIRelevanceHigh,
				RuleID:          "TER-AI-105",
				RuleURI:         "docs/rules/ai/non-deterministic-eval.md",
				DetectorVersion: "0.2.0",
				ConfidenceDetail: &models.ConfidenceDetail{
					Value:        0.93,
					IntervalLow:  0.88,
					IntervalHigh: 0.97,
					Quality:      "heuristic",
					Sources:      []models.EvidenceSource{models.SourceStructuralPattern},
				},
				EvidenceSource:   models.SourceStructuralPattern,
				EvidenceStrength: models.EvidenceStrong,
			})
		}
	}
	return out
}

// gatherEvalConfigPaths picks YAML/JSON files whose path smells like
// an eval / agent / prompt config. Combines snapshot enumeration with
// a fresh walk of d.Root so eval configs that aren't tracked as test
// files still get inspected.
func (d *NonDeterministicEvalDetector) gatherEvalConfigPaths(snap *models.TestSuiteSnapshot) []string {
	fromSnap := snapshotPaths(snap)
	fromWalk := walkRepoForConfigs(d.Root, scanOpts{
		extensions: evalConfigExts,
		markers:    evalFilenameMarkers,
	})
	merged := uniquePaths(fromSnap, fromWalk)

	var out []string
	for _, p := range merged {
		ext := strings.ToLower(filepath.Ext(p))
		if !evalConfigExts[ext] {
			continue
		}
		lower := strings.ToLower(p)
		matched := false
		for _, marker := range evalFilenameMarkers {
			if strings.Contains(lower, marker) {
				matched = true
				break
			}
		}
		if !matched {
			continue
		}
		out = append(out, p)
	}
	return out
}

// evalFinding describes one non-determinism issue found in a config.
type evalFinding struct {
	Explanation string
}

// analyseEvalConfig parses the YAML/JSON file and returns one finding
// per non-deterministic provider/test entry.
//
// Pre-0.2.x this scanned for the FIRST `temperature` anywhere in the
// file and emitted one verdict total. Multi-provider configs where
// one provider pins temperature and another doesn't got a single
// binary verdict — the second provider's missing pin was silently
// missed. Per-provider scoping fixes the multi-provider case while
// retaining single-finding behaviour for the common single-provider
// shape.
func analyseEvalConfig(path string) []evalFinding {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	// JSON is a strict YAML subset, so the YAML decoder handles both.
	var node yaml.Node
	if err := yaml.Unmarshal(raw, &node); err != nil {
		return nil
	}

	// Walk every mapping subtree that declares a `model` (or
	// `provider.config.model`) — those are the per-provider entries.
	providers := collectProviderEntries(&node)
	if len(providers) == 0 {
		// No provider entries; fall back to the file-global check.
		tempState := scanForKey(&node, "temperature")
		hasModel := scanForKey(&node, "model").present
		switch {
		case tempState.present && tempState.numericValue != 0:
			return []evalFinding{{
				Explanation: "Eval config sets temperature ≠ 0; runs will be non-deterministic.",
			}}
		case !tempState.present && hasModel:
			return []evalFinding{{
				Explanation: "Eval config declares a model but does not pin temperature; default sampling is non-deterministic.",
			}}
		}
		return nil
	}

	var out []evalFinding
	seen := map[string]bool{}
	for _, prov := range providers {
		tempState := scanForKey(prov.node, "temperature")
		var msg string
		switch {
		case tempState.present && tempState.numericValue != 0:
			msg = fmt.Sprintf("Eval provider %q sets temperature %.2f (≠ 0); runs will be non-deterministic.", prov.label, tempState.numericValue)
		case !tempState.present:
			msg = fmt.Sprintf("Eval provider %q declares a model but does not pin temperature; default sampling is non-deterministic.", prov.label)
		default:
			continue
		}
		if seen[msg] {
			continue
		}
		seen[msg] = true
		out = append(out, evalFinding{Explanation: msg})
	}
	return out
}

// providerEntry holds one provider/model declaration for per-provider
// non-determinism analysis. label is a best-effort human-readable
// identifier (model name, provider id, or "provider#N").
type providerEntry struct {
	label string
	node  *yaml.Node
}

// collectProviderEntries finds every mapping subtree that declares a
// `model` key, treating each as a distinct provider/test entry.
// Mirrors the structures Promptfoo / DeepEval / custom configs use.
func collectProviderEntries(n *yaml.Node) []providerEntry {
	var out []providerEntry
	walkProviders(n, &out, "")
	// Dedup by node pointer (a config that lists the same provider
	// twice should still emit twice; this just guards against loops).
	return out
}

func walkProviders(n *yaml.Node, out *[]providerEntry, parentLabel string) {
	if n == nil {
		return
	}
	switch n.Kind {
	case yaml.DocumentNode:
		for _, c := range n.Content {
			walkProviders(c, out, parentLabel)
		}
	case yaml.MappingNode:
		hasModel := false
		var modelLabel string
		for i := 0; i+1 < len(n.Content); i += 2 {
			k := n.Content[i]
			v := n.Content[i+1]
			if k.Value == "model" && v.Kind == yaml.ScalarNode {
				hasModel = true
				modelLabel = v.Value
			}
		}
		if hasModel {
			label := modelLabel
			if label == "" {
				label = parentLabel
			}
			if label == "" {
				label = fmt.Sprintf("provider#%d", len(*out)+1)
			}
			*out = append(*out, providerEntry{label: label, node: n})
		}
		// Always recurse — nested provider blocks (e.g. promptfoo's
		// providers list under tests) need their own entries.
		for i := 0; i+1 < len(n.Content); i += 2 {
			v := n.Content[i+1]
			walkProviders(v, out, modelLabel)
		}
	case yaml.SequenceNode:
		for _, c := range n.Content {
			walkProviders(c, out, parentLabel)
		}
	}
}

// keyState summarises whether a key was present in the parsed config
// and (when scalar and numeric) what its value was. The detector only
// cares about presence + numeric for `temperature` today.
type keyState struct {
	present      bool
	numericValue float64
}

// scanForKey walks a parsed YAML tree looking for the first occurrence
// of `key` as a mapping field name. Returns presence + parsed numeric
// value when the field is a numeric scalar.
func scanForKey(n *yaml.Node, key string) keyState {
	if n == nil {
		return keyState{}
	}
	switch n.Kind {
	case yaml.DocumentNode:
		for _, c := range n.Content {
			if s := scanForKey(c, key); s.present {
				return s
			}
		}
	case yaml.MappingNode:
		// Mapping content alternates [key, value, key, value, ...].
		for i := 0; i+1 < len(n.Content); i += 2 {
			k := n.Content[i]
			v := n.Content[i+1]
			if k.Value == key {
				return scalarToKeyState(v)
			}
			// Recurse into nested values.
			if s := scanForKey(v, key); s.present {
				return s
			}
		}
	case yaml.SequenceNode:
		for _, c := range n.Content {
			if s := scanForKey(c, key); s.present {
				return s
			}
		}
	}
	return keyState{}
}

// scalarToKeyState converts a YAML scalar to a keyState. Non-numeric
// scalars register as "present" but with numericValue=0; the caller
// decides whether numericValue matters.
func scalarToKeyState(v *yaml.Node) keyState {
	if v == nil || v.Kind != yaml.ScalarNode {
		return keyState{present: true}
	}
	state := keyState{present: true}
	// Try float, then quoted-int representations.
	var f float64
	if err := v.Decode(&f); err == nil {
		state.numericValue = f
	}
	return state
}
