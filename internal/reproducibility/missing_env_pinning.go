package reproducibility

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

// DetectMissingEnvPinning AST-walks Python source for environment-
// variable reads in eval / inference paths that don't supply a default
// value. Without a default, an unset env var either raises at runtime
// or silently changes behavior across CI / local / production
// environments. Implements terrain/reproducibility/missing-env-pinning.
//
// Recognized read shapes:
//
//   os.environ["MODEL"]                    → flagged (raises if unset)
//   os.environ.get("MODEL")                → flagged (returns None silently)
//   os.environ.get("MODEL", "default")     → suppressed (has fallback)
//   os.getenv("MODEL")                     → flagged
//   os.getenv("MODEL", "default")          → suppressed
//
// Fires only in eval/inference paths to avoid noise on application
// config code where unset envs are intentionally configurable.
func DetectMissingEnvPinning(src []byte, relPath string) []models.Signal {
	if len(src) == 0 || !looksLikeStochasticFile(relPath) {
		return nil
	}

	var out []models.Signal
	_ = parserpool.With(python.GetLanguage(), func(parser *sitter.Parser) error {
		tree, err := parser.ParseCtx(context.Background(), nil, src)
		if err != nil || tree == nil {
			return err
		}
		defer tree.Close()
		walkForEnvReads(tree.RootNode(), src, relPath, &out)
		return nil
	})
	return out
}

func walkForEnvReads(node *sitter.Node, src []byte, relPath string, out *[]models.Signal) {
	if node == nil {
		return
	}

	switch node.Type() {
	case "subscript":
		// os.environ["KEY"] — flagged regardless (no default possible).
		obj := node.ChildByFieldName("value")
		if obj != nil {
			objText := string(src[obj.StartByte():obj.EndByte()])
			if objText == "os.environ" || objText == "environ" {
				key := envSubscriptKey(node, src)
				if key != "" {
					*out = append(*out, buildMissingEnvSignal(key, "os.environ[]", relPath, int(node.StartPoint().Row)+1))
				}
			}
		}

	case "call":
		// os.environ.get(...) / os.getenv(...) — suppressed when default
		// argument is a string literal.
		funcNode := node.ChildByFieldName("function")
		argsNode := node.ChildByFieldName("arguments")
		if funcNode != nil {
			fnText := string(src[funcNode.StartByte():funcNode.EndByte()])
			if isEnvGetCall(fnText) {
				if !envCallHasDefault(argsNode, src) {
					key := firstStringArg(argsNode, src)
					*out = append(*out, buildMissingEnvSignal(key, fnText, relPath, int(node.StartPoint().Row)+1))
				}
			}
		}
	}

	for i := 0; i < int(node.ChildCount()); i++ {
		walkForEnvReads(node.Child(i), src, relPath, out)
	}
}

func isEnvGetCall(fnText string) bool {
	return fnText == "os.environ.get" || fnText == "environ.get" ||
		fnText == "os.getenv" || fnText == "getenv"
}

func envSubscriptKey(node *sitter.Node, src []byte) string {
	// subscript has named children: value, subscript.
	// In tree-sitter Python the index is named "subscript".
	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		if child.Type() == "string" || child.Type() == "interpreted_string_literal" {
			return strings.Trim(string(src[child.StartByte():child.EndByte()]), `"'`)
		}
	}
	return ""
}

func envCallHasDefault(argsNode *sitter.Node, src []byte) bool {
	if argsNode == nil {
		return false
	}
	// More than one positional argument means a default is supplied.
	positional := 0
	for i := 0; i < int(argsNode.NamedChildCount()); i++ {
		arg := argsNode.NamedChild(i)
		if arg.Type() == "keyword_argument" {
			// Check for default="..." kwarg.
			name := arg.ChildByFieldName("name")
			if name != nil && string(src[name.StartByte():name.EndByte()]) == "default" {
				return true
			}
			continue
		}
		positional++
	}
	return positional >= 2
}

func firstStringArg(argsNode *sitter.Node, src []byte) string {
	if argsNode == nil {
		return ""
	}
	for i := 0; i < int(argsNode.NamedChildCount()); i++ {
		arg := argsNode.NamedChild(i)
		if arg.Type() == "string" || arg.Type() == "interpreted_string_literal" {
			return strings.Trim(string(src[arg.StartByte():arg.EndByte()]), `"'`)
		}
	}
	return ""
}

func buildMissingEnvSignal(key, access, relPath string, line int) models.Signal {
	return models.Signal{
		Type:             signals.SignalMissingEnvPinning,
		Category:         models.CategoryQuality,
		Severity:         models.SeverityMedium,
		Confidence:       0.85,
		EvidenceStrength: models.EvidenceModerate,
		EvidenceSource:   models.SourceStructuralPattern,
		Location: models.SignalLocation{
			File: relPath,
			Line: line,
		},
		Explanation: fmt.Sprintf(
			"Environment variable %q is read via %s in %s without a default value. The same eval / inference code produces different behavior depending on which environment runs it.",
			key, access, relPath,
		),
		SuggestedAction: fmt.Sprintf(
			"Supply a default — os.environ.get(%q, \"<pinned-value>\") — or declare the variable as required at the top of the file and fail fast with a clear error message.",
			key,
		),
		RuleID:          "terrain/reproducibility/missing-env-pinning",
		RuleURI:         "docs/rules/reproducibility/missing-env-pinning.md",
		DetectorVersion: "0.2.0",
		Metadata: map[string]any{
			"envVar": key,
			"access": access,
		},
	}
}
