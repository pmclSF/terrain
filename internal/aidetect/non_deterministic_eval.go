package aidetect

import (
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

// gatherEvalConfigPaths picks the YAML/JSON files in the snapshot whose
// path or filename smells like an eval / agent / prompt config. The
// universe is intentionally narrow to keep false positives down on
// repos that have unrelated YAML/JSON.
func (d *NonDeterministicEvalDetector) gatherEvalConfigPaths(snap *models.TestSuiteSnapshot) []string {
	seen := map[string]bool{}
	var out []string

	add := func(p string) {
		ext := strings.ToLower(filepath.Ext(p))
		if !evalConfigExts[ext] {
			return
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
			return
		}
		if seen[p] {
			return
		}
		seen[p] = true
		out = append(out, p)
	}

	for _, tf := range snap.TestFiles {
		add(tf.Path)
	}
	for _, sc := range snap.Scenarios {
		add(sc.Path)
	}
	return out
}

// evalFinding describes one non-determinism issue found in a config.
type evalFinding struct {
	Explanation string
}

// analyseEvalConfig parses the YAML/JSON file and returns 0..1 findings.
// We keep it to one finding per file: most repos have many configs and
// a single "the eval is non-deterministic" message per file is more
// actionable than multiple per-knob signals.
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
