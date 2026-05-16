package coverage

import (
	"fmt"
	"strings"

	"github.com/pmclSF/terrain/internal/models"
	"github.com/pmclSF/terrain/internal/signals"
)

// DetectNoDataValidation walks Python source bytes for data-pipeline
// files (paths under pipelines/ etl/ dbt/ training/ dags/ flows/)
// that don't import a data-validation library. Implements
// terrain/coverage/no-data-validation.
//
// Recognized validation libraries: Great Expectations, pandera,
// dbt-expectations (via `tests/generic`), soda, pydantic (when used
// on DataFrame rows), data-diff.
//
// Detection is import-presence (substring), not AST: cheap and
// adequate for the structural pattern this rule targets.
type DataValidationInput struct {
	Path    string
	Content []byte
}

// DetectNoDataValidation emits one Signal per pipeline file that has
// no validation-library imports.
func DetectNoDataValidation(files []DataValidationInput) []models.Signal {
	var out []models.Signal
	for _, f := range files {
		if !looksLikePipelinePath(f.Path) {
			continue
		}
		if hasValidationImport(f.Content) {
			continue
		}
		out = append(out, models.Signal{
			Type:             signals.SignalNoDataValidation,
			Category:         models.CategoryQuality,
			Severity:         models.SeverityMedium,
			Confidence:       0.85,
			EvidenceStrength: models.EvidenceModerate,
			EvidenceSource:   models.SourceStructuralPattern,
			Location:         models.SignalLocation{File: f.Path},
			Explanation: fmt.Sprintf(
				"Pipeline file %s has no data-validation library import (Great Expectations, pandera, dbt-expectations, soda). Output drift in schema or distribution can propagate downstream undetected.",
				f.Path,
			),
			SuggestedAction: "Add data validation (GE expectations, pandera schemas, dbt-expectations tests, or soda checks) on pipeline output. Run validation in CI on a fixed sample.",
			RuleID:          "terrain/coverage/no-data-validation",
			RuleURI:         "docs/rules/coverage/no-data-validation.md",
			DetectorVersion: "0.2.0",
		})
	}
	return out
}

func looksLikePipelinePath(path string) bool {
	lower := strings.ToLower(path)
	lower = strings.ReplaceAll(lower, "\\", "/")
	if !strings.HasPrefix(lower, "/") {
		lower = "/" + lower
	}
	for _, m := range pipelinePathMarkers {
		if strings.Contains(lower, m) {
			return true
		}
	}
	return false
}

var pipelinePathMarkers = []string{
	"/pipelines/", "/etl/", "/dbt/", "/training/",
	"/dags/", "/flows/",
}

// validationImportPatterns is the vocabulary of data-validation
// libraries the rule recognizes.
var validationImportPatterns = []string{
	"import great_expectations",
	"from great_expectations",
	"import pandera",
	"from pandera",
	"import soda",
	"from soda",
	"data_diff",
	"dbt-expectations",
	"dbt_expectations",
}

func hasValidationImport(content []byte) bool {
	s := string(content)
	for _, p := range validationImportPatterns {
		if strings.Contains(s, p) {
			return true
		}
	}
	return false
}
