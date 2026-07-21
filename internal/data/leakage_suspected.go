package data

import (
	"context"
	"fmt"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/python"

	"github.com/pmclSF/terrain/internal/models"
	"github.com/pmclSF/terrain/internal/parserpool"
	"github.com/pmclSF/terrain/internal/signals"
)

// DetectLeakageSuspected AST-walks Python training source for code
// patterns that indicate train/test contamination at the source level.
// Implements terrain/data/leakage-suspected.
//
// Patterns recognized at 0.2.0:
//
//   - .fit() called on a variable that was assigned the full dataset
//     after train_test_split (e.g., model.fit(X, y) after
//     X_train, X_test = train_test_split(...) — uses full X by mistake)
//   - .transform() / .fit_transform() applied to test data using a
//     scaler / encoder fit on full data (preprocessing leakage)
//   - Time-series train/test split that doesn't use TimeSeriesSplit
//     (random split on temporal data — temporal leakage)
//   - Feature engineering computed on the full dataset before split
//
// Detection is heuristic — a true leakage check requires runtime
// data flow analysis. The rule fires on structural shapes that are
// strongly correlated with leakage in practice.
//
// Companion rule: terrain/data/missing-train-test-split fires when
// no split exists at all; this rule fires when a split exists but is
// used incorrectly.
func DetectLeakageSuspected(src []byte, relPath string) []models.Signal {
	if len(src) == 0 || !looksLikeTrainingFile(relPath) {
		return nil
	}

	var out []models.Signal
	_ = parserpool.With(python.GetLanguage(), func(parser *sitter.Parser) error {
		tree, err := parser.ParseCtx(context.Background(), nil, src)
		if err != nil || tree == nil {
			return err
		}
		defer tree.Close()
		analyzeLeakageFile(tree.RootNode(), src, relPath, &out)
		return nil
	})
	return out
}

func analyzeLeakageFile(root *sitter.Node, src []byte, relPath string, out *[]models.Signal) {
	text := string(src)

	// Heuristic 1: random split on time-series data.
	if hasTimeSeriesIndicator(text) && usesRandomSplitOnly(text) {
		*out = append(*out, buildLeakageSignal(
			"temporal-leakage",
			"random train/test split on time-series data — temporal leakage",
			"Use sklearn.model_selection.TimeSeriesSplit or a manual time-based cutoff. Random shuffling on temporal data leaks future information into the training set.",
			relPath, 0,
		))
	}

	// Heuristic 2: feature engineering applied before split.
	if hasFeatureEngineeringBeforeSplit(root, src) {
		*out = append(*out, buildLeakageSignal(
			"preprocessing-leakage",
			"feature engineering / scaling applied before train/test split",
			"Compute scalers / encoders on the train split only, then apply to test. fit_transform on the full dataset before splitting leaks test-set statistics into training.",
			relPath, 0,
		))
	}
}

func hasTimeSeriesIndicator(text string) bool {
	for _, m := range timeSeriesMarkers {
		if strings.Contains(text, m) {
			return true
		}
	}
	return false
}

var timeSeriesMarkers = []string{
	"pd.to_datetime",
	"DatetimeIndex",
	"time_series",
	"timestamp",
	"datetime_col",
	"forecasting",
	"lstm", "LSTM",
	".resample(",
	"freq=",
}

func usesRandomSplitOnly(text string) bool {
	return strings.Contains(text, "train_test_split(") &&
		!strings.Contains(text, "TimeSeriesSplit(") &&
		!strings.Contains(text, "shuffle=False")
}

// hasFeatureEngineeringBeforeSplit walks for the pattern:
//
//	scaler.fit_transform(X)
//	X_train, X_test, ... = train_test_split(X, ...)
//
// where the fit_transform is on the full X before splitting.
func hasFeatureEngineeringBeforeSplit(root *sitter.Node, src []byte) bool {
	// Track line numbers: fit_transform first, then train_test_split.
	var firstFitTransform int = -1
	var firstSplit int = -1

	var visit func(n *sitter.Node)
	visit = func(n *sitter.Node) {
		if n == nil {
			return
		}
		if n.Type() == "call" {
			fn := n.ChildByFieldName("function")
			if fn != nil {
				text := string(src[fn.StartByte():fn.EndByte()])
				line := int(n.StartPoint().Row)
				switch {
				case strings.HasSuffix(text, ".fit_transform") || strings.HasSuffix(text, ".fit"):
					// Only count scaler / encoder / vectorizer fits, not model fits.
					if isPreprocessingFit(text) && firstFitTransform == -1 {
						firstFitTransform = line
					}
				case text == "train_test_split" || strings.HasSuffix(text, ".train_test_split"):
					if firstSplit == -1 {
						firstSplit = line
					}
				}
			}
		}
		for i := 0; i < int(n.ChildCount()); i++ {
			visit(n.Child(i))
		}
	}
	visit(root)

	return firstFitTransform >= 0 && firstSplit >= 0 && firstFitTransform < firstSplit
}

func isPreprocessingFit(callText string) bool {
	// Match common preprocessing class names.
	for _, m := range preprocessingMarkers {
		if strings.Contains(callText, m) {
			return true
		}
	}
	// Heuristic: lowercase variable names typically used for scalers /
	// encoders / vectorizers.
	for _, prefix := range []string{"scaler.", "encoder.", "vectorizer.", "imputer.", "transformer."} {
		if strings.HasPrefix(callText, prefix) {
			return true
		}
	}
	return false
}

var preprocessingMarkers = []string{
	"StandardScaler", "MinMaxScaler", "RobustScaler",
	"OneHotEncoder", "OrdinalEncoder", "LabelEncoder",
	"TfidfVectorizer", "CountVectorizer", "HashingVectorizer",
	"SimpleImputer", "KNNImputer",
	"PowerTransformer", "QuantileTransformer",
}

func buildLeakageSignal(kind, explanation, suggestedAction, relPath string, line int) models.Signal {
	return models.Signal{
		Type:             signals.SignalDataLeakageSuspected,
		Category:         models.CategoryAI,
		Severity:         models.SeverityHigh,
		Confidence:       0.75,
		EvidenceStrength: models.EvidenceModerate,
		EvidenceSource:   models.SourceStructuralPattern,
		Location: models.SignalLocation{
			File:   relPath,
			Symbol: kind, // distinguishes the FindingID anchor per leakage kind
			Line:   line,
		},
		Explanation: fmt.Sprintf(
			"Data leakage suspected in %s: %s. Reported metrics may overstate generalization.",
			relPath, explanation,
		),
		SuggestedAction: suggestedAction,
		RuleID:          "terrain/data/leakage-suspected",
		RuleURI:         "docs/rules/data/leakage-suspected.md",
		DetectorVersion: "0.2.0",
		Metadata: map[string]any{
			"leakageKind": kind,
		},
	}
}
