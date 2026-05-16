package stages

import (
	"testing"

	"github.com/pmclSF/terrain/internal/aipipeline"
)

// TestCalibrationCoversAllRegexAtoms is a regression test for the
// calibration key-mismatch bug fixed on 2026-05-15. The bug:
// calibration keys ("regex.sklearn.train") didn't match the atom IDs
// the regex stage emits ("regex.sklearn_train.call"), so the hand-
// tuned per-cohort overrides for those atoms were dead code.
//
// This test enumerates every SDK pair the regex stage knows about and
// asserts the default calibration has an explicit weight entry for
// both the .import and .call variants. Without this guard, adding a
// new ctxPair without updating calibration.go would silently fall
// back to the regex stage's switch-statement defaults instead of the
// cohort-aware override path.
func TestCalibrationCoversAllRegexAtoms(t *testing.T) {
	t.Parallel()
	cal := aipipeline.DefaultCalibration()
	for _, pair := range ctxPairs {
		for _, suffix := range []string{".import", ".call"} {
			id := "regex." + pair.name + suffix
			if _, ok := cal.AtomWeight("*", "*", id); !ok {
				t.Errorf("calibration table missing entry for %q — add to internal/aipipeline/calibration.go AtomWeights[\"*\"][\"*\"]", id)
			}
		}
	}
}

// TestCalibrationCoversStructuralAtoms asserts the AST stage's atoms
// have explicit weight entries. Same rationale as above: the AST atom
// IDs are emitted by ast_confirm.go (`ast.bound_call`, etc.) and the
// calibration table needs to know them.
func TestCalibrationCoversStructuralAtoms(t *testing.T) {
	t.Parallel()
	cal := aipipeline.DefaultCalibration()
	required := []string{
		"ast.bound_call",
		"ast.no_call_despite_regex",
		"ast.module_level_call",
		"ast.real_training_call",
	}
	for _, id := range required {
		if _, ok := cal.AtomWeight("*", "*", id); !ok {
			t.Errorf("calibration missing structural atom %q", id)
		}
	}
}

// TestCalibrationCoversPathAtoms asserts every path-prefilter atom is
// in the calibration table.
func TestCalibrationCoversPathAtoms(t *testing.T) {
	t.Parallel()
	cal := aipipeline.DefaultCalibration()
	required := []string{
		"path.examples",
		"path.tests",
		"path.providers",
		"path.llms_subdir_base",
		"path.factory_filename",
		"path.snake_suffix_wrapper",
		"path.exact_name_utility",
		"path.framework_integration",
		"wrapper.class.match",
		"regex.multi_framework",
		"regex.import_without_call",
	}
	for _, id := range required {
		if _, ok := cal.AtomWeight("*", "*", id); !ok {
			t.Errorf("calibration missing path/structural atom %q", id)
		}
	}
}

// TestCalibrationCoversCrossFileAtoms asserts the cross-file stage's
// atoms are registered.
func TestCalibrationCoversCrossFileAtoms(t *testing.T) {
	t.Parallel()
	cal := aipipeline.DefaultCalibration()
	for _, id := range []string{"scope.sibling_has_eval", "scope.package_has_eval"} {
		if _, ok := cal.AtomWeight("*", "*", id); !ok {
			t.Errorf("calibration missing cross-file atom %q", id)
		}
	}
}
