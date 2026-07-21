package aipipeline

import (
	"encoding/json"
	"fmt"
	"os"
)

// Calibration carries per-rule, per-cohort base rates, per-atom weight
// overrides, severity declarations, and posture thresholds.
//
// Weights are hand-tuned log-odds; this table is the calibration
// baseline the composer consumes.
type Calibration struct {
	// BaseRates maps (cohort, rule) → base log-odds. Cohort "unknown"
	// is the fallback when cohort detection didn't fire.
	BaseRates map[string]map[string]float64

	// AtomWeights maps (cohort, rule, atomRuleID) → log-odds. When
	// present, overrides the atom's default weight. Cohort can be "*"
	// to apply across cohorts.
	AtomWeights map[string]map[string]map[string]float64

	// Severities maps rule → declared severity.
	Severities map[string]Severity

	// Preview marks rules that ship but are not in the default rule
	// set; users must opt in via --rule. The findings renderer
	// surfaces a [preview] tag so the calling engineer knows the
	// confidence number is heuristic.
	Preview map[string]bool

	// Thresholds maps (posture, rule) → confidence threshold.
	Thresholds map[Posture]map[string]float64
}

// DefaultCalibration returns the hand-tuned calibration table used at
// pipeline launch.
//
// Convention:
//   - Atom IDs use dotted namespaces (e.g. "regex.openai.import",
//     "regex.langchain.invoke", "ast.bound_call.openai",
//     "ast.no_call_despite_regex", "wrapper.class.match",
//     "path.examples", "path.tests").
//   - Positive atoms support the verdict; negative atoms oppose.
//   - Weights are in log-odds (natural log). +2.0 ≈ 7.4× odds boost.
func DefaultCalibration() *Calibration {
	c := &Calibration{
		// Per-rule overrides for production-context atoms — defined
		// before BaseRates so the literal stays grouped with the
		// related comment. See AtomWeights[*]["ai.train.missing_tracker"]
		// below for how training rule scopes these.
		BaseRates: map[string]map[string]float64{
			"unknown": {
				"ai.surface.missing_eval":     -3.5,
				"ai.train.missing_tracker":    -3.5,
				"ai.prompt_file_missing_eval": -3.0,
				"ai.uncovered_surface":        -3.5,
			},
			"rag-app": {
				"ai.surface.missing_eval":     -2.5,
				"ai.prompt_file_missing_eval": -2.0,
				"ai.uncovered_surface":        -2.5,
			},
			"agent-app": {
				"ai.surface.missing_eval": -2.5,
				"ai.uncovered_surface":    -2.0,
			},
			"ml-pipeline": {
				"ai.train.missing_tracker": -2.0,
			},
			// library-sdk base rate matches unknown. Cohort labels are
			// kept (still useful for emission posture and explain
			// strings) but the base rate is not differentiated.
			"library-sdk": {
				"ai.surface.missing_eval":  -3.5,
				"ai.train.missing_tracker": -3.5,
			},
		},

		AtomWeights: map[string]map[string]map[string]float64{
			"*": {
				"*": {
					// Lexical positives — weights calibrated for the 0.40
					// observability threshold.
					"regex.openai.import":    +0.8,
					"regex.openai.call":      +1.6,
					"regex.anthropic.import": +0.6,
					"regex.anthropic.call":   +1.4,
					// Framework atoms are held below neutral so a lone
					// framework import or call does not, on its own, push
					// a single file over the emission threshold; the
					// intended signal for these cases is sibling-eval
					// evidence rather than the framework reference itself.
					"regex.langchain.import":     -0.8,
					"regex.langchain.call":       -0.3,
					"regex.langgraph.import":     -0.6,
					"regex.langgraph.call":       -0.3,
					"regex.llama_index.import":   -0.5,
					"regex.llama_index.call":     -0.1,
					"regex.huggingface.import":   +0.3,
					"regex.huggingface.call":     +0.8,
					"regex.google_genai.import":  +0.4,
					"regex.google_genai.call":    +1.0,
					"regex.openai_compat.import": +0.4,
					"regex.openai_compat.call":   +1.0,
					"regex.generic_sdk.import":   -0.2,
					"regex.generic_sdk.call":     +0.4,
					// Training-detector atoms — calibration keys match
					// the emitted atom IDs.
					"regex.sklearn_train.import":      +0.4,
					"regex.sklearn_train.call":        +1.2,
					"regex.xgb_lgb_cat_train.import":  +0.4,
					"regex.xgb_lgb_cat_train.call":    +1.4,
					"regex.keras_train.import":        +0.6,
					"regex.keras_train.call":          +0.8,
					"regex.pytorch_train.import":      +0.3,
					"regex.pytorch_train.call":        +1.0,
					"regex.transformers_train.import": +0.4,
					"regex.transformers_train.call":   +1.4,

					// Structural positives — AST confirmed. ast.bound_call
					// carries weight at the 0.40 observability threshold so
					// it lifts SDK-anchored positives over the emit bar.
					"ast.bound_call":         +2.0,
					"ast.module_level_call":  +1.0,
					"ast.real_training_call": +2.0,

					// Topological positives
					"topo.exported_from_package":  +0.4,
					"topo.imported_by_app_module": +0.6,

					// Scope (per-PR)
					"scope.diff_touched_file":      +0.8,
					"scope.diff_touched_line":      +1.4,
					"scope.diff_added_pr_evidence": -1.5, // PR added the missing artifact

					// Cross-file scope — eval present in a sibling or
					// package mate strongly opposes "missing eval".
					"scope.sibling_has_eval": -1.8,
					"scope.package_has_eval": -1.4,

					// Repo-shape
					"shape.is_application": +0.4,
					"shape.is_library":     -0.9,

					// Negative atoms — strong suppression
					"wrapper.class.match":        -2.0,
					"ast.no_call_despite_regex":  -2.1,
					"regex.import_without_call":  -1.6, // regex-only version of the above
					"path.examples":              -3.0,
					"path.tests":                 -2.5,
					"path.providers":             -2.0,
					"path.framework_integration": -2.5,
					"regex.multi_framework":      -2.0,
					"path.snake_suffix_wrapper":  -1.5,
					"path.exact_name_utility":    -1.2,
					"path.llms_subdir_base":      -2.0,
					"path.factory_filename":      -1.5,

					// Production-context atoms — neutral by default
					// (per-rule overrides below give them weight for
					// ai.train.missing_tracker). The fastscan emits
					// these whenever it sees production-ML signals;
					// the surface rule shouldn't be influenced by
					// them, so the universal entry is 0.0.
					"regex.production_ml_sdk":       0.0,
					"regex.scheduling_decorator":    0.0,
					"regex.model_registry_register": 0.0,
				},
				// Per-rule override for ai.train.missing_tracker.
				// Many training-anchored files are tutorials or
				// research scripts that do not need tracking;
				// production-context atoms are the signal that
				// distinguishes production training that should track
				// from early-dev that is expected to skip tracking.
				"ai.train.missing_tracker": {
					"regex.production_ml_sdk":       +1.8,
					"regex.scheduling_decorator":    +1.5,
					"regex.model_registry_register": +1.2,
				},
			},
		},

		Severities: map[string]Severity{
			"ai.surface.missing_eval":     SeverityMedium,
			"ai.train.missing_tracker":    SeverityMedium,
			"ai.prompt_file_missing_eval": SeverityHigh,
			"ai.uncovered_surface":        SeverityMedium,
		},

		// Preview rules are opt-in only (not in the default rule set);
		// their findings carry a [preview] tag so callers know the
		// confidence number is heuristic.
		Preview: map[string]bool{
			// Production-context gating (regex.production_ml_sdk etc.)
			// is the intended signal for ai.train.missing_tracker.
			"ai.train.missing_tracker": true,
		},

		Thresholds: map[Posture]map[string]float64{
			PostureObservability: {
				// Emit at confidence ≥ 0.40 — the Observability floor.
			},
			PostureGate: {
				// Fail at confidence ≥ 0.80.
			},
		},
	}
	return c
}

// LoadCalibration reads a JSON-encoded calibration table from disk.
// Returns DefaultCalibration() when path is empty or the file is
// missing — production deployments should ship a calibration file.
func LoadCalibration(path string) (*Calibration, error) {
	if path == "" {
		return DefaultCalibration(), nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultCalibration(), nil
		}
		return nil, fmt.Errorf("read calibration: %w", err)
	}
	var c Calibration
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("parse calibration: %w", err)
	}
	return &c, nil
}

// BaseRate returns the base log-odds for (cohort, rule). Falls back
// to cohort "unknown" when the requested cohort has no entry, and to
// 0.0 when no entry exists at all.
func (c *Calibration) BaseRate(cohort, rule string) float64 {
	if c == nil {
		return 0.0
	}
	if rates, ok := c.BaseRates[cohort]; ok {
		if r, ok := rates[rule]; ok {
			return r
		}
	}
	if rates, ok := c.BaseRates["unknown"]; ok {
		if r, ok := rates[rule]; ok {
			return r
		}
	}
	return 0.0
}

// AtomWeight returns the calibrated weight for an atom under
// (cohort, rule). Lookup order:
//
//	c.AtomWeights[cohort][rule][atomRuleID]
//	c.AtomWeights[cohort]["*"][atomRuleID]
//	c.AtomWeights["*"][rule][atomRuleID]
//	c.AtomWeights["*"]["*"][atomRuleID]
//
// Returns (weight, true) on hit; (0, false) when no entry exists at
// any layer — caller should fall back to the atom's default weight.
func (c *Calibration) AtomWeight(cohort, rule, atomRuleID string) (float64, bool) {
	if c == nil {
		return 0, false
	}
	keys := []struct{ co, ru string }{
		{cohort, rule},
		{cohort, "*"},
		{"*", rule},
		{"*", "*"},
	}
	for _, k := range keys {
		if byCohort, ok := c.AtomWeights[k.co]; ok {
			if byRule, ok := byCohort[k.ru]; ok {
				if w, ok := byRule[atomRuleID]; ok {
					return w, true
				}
			}
		}
	}
	return 0, false
}

// IsPreview reports whether a rule is marked preview — its calibration
// ships but the empirical floor isn't established yet. Callers should
// render a [preview] tag on findings and exclude preview rules from
// any "default rule set" used in posture=gate CI gates.
func (c *Calibration) IsPreview(rule string) bool {
	if c == nil {
		return false
	}
	return c.Preview[rule]
}

// SeverityFor returns the declared severity for a rule when present.
func (c *Calibration) SeverityFor(rule string) (Severity, bool) {
	if c == nil {
		return "", false
	}
	if s, ok := c.Severities[rule]; ok {
		return s, true
	}
	return "", false
}

// Threshold returns the confidence threshold for (posture, rule).
// Posture-specific rule entry wins; otherwise the posture-wide
// default returned by Composer.ThresholdFor is used by the caller.
func (c *Calibration) Threshold(p Posture, rule string) (float64, bool) {
	if c == nil {
		return 0, false
	}
	if byPosture, ok := c.Thresholds[p]; ok {
		if t, ok := byPosture[rule]; ok {
			return t, true
		}
	}
	return 0, false
}
