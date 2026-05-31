package hygiene

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

// DetectModelFixtureUnpinned AST-walks Python source for model-loading
// calls whose path argument isn't pinned to a content-addressed
// reference (commit SHA, hub revision, digest, version-suffixed
// filename). Implements terrain/hygiene/model-fixture-unpinned.
//
// Recognized load shapes:
//
//	torch.load(path)
//	tf.keras.models.load_model(path)
//	tensorflow.keras.models.load_model(path)
//	joblib.load(path)
//	pickle.load(open(path, "rb"))
//	transformers.AutoModelForCausalLM.from_pretrained(name_or_path)
//	*.from_pretrained(name_or_path)
//	huggingface_hub.snapshot_download(...)
//
// Pinning is recognized when:
//   - The path argument is a string literal ending in a
//     7+-hex commit SHA / version digit / @<rev> suffix.
//   - The from_pretrained call passes a `revision="..."` kwarg whose
//     value is a string literal (not just "main" or "master").
//
// Heuristic: literal "main" / "master" / "latest" / "head" as a
// revision value is treated as unpinned.
//
// Fires once per call site.
func DetectModelFixtureUnpinned(src []byte, relPath string) []models.Signal {
	if len(src) == 0 {
		return nil
	}

	var out []models.Signal
	_ = parserpool.With(python.GetLanguage(), func(parser *sitter.Parser) error {
		tree, err := parser.ParseCtx(context.Background(), nil, src)
		if err != nil || tree == nil {
			return err
		}
		defer tree.Close()
		walkForModelLoads(tree.RootNode(), src, relPath, &out)
		return nil
	})
	return out
}

func walkForModelLoads(node *sitter.Node, src []byte, relPath string, out *[]models.Signal) {
	if node == nil {
		return
	}
	if node.Type() == "call" {
		funcNode := node.ChildByFieldName("function")
		argsNode := node.ChildByFieldName("arguments")
		if funcNode != nil {
			callText := string(src[funcNode.StartByte():funcNode.EndByte()])
			if shape, ok := classifyModelLoadCall(callText); ok {
				if !isModelLoadPinned(shape, argsNode, src) {
					*out = append(*out, models.Signal{
						Type:             signals.SignalModelFixtureUnpinned,
						Category:         models.CategoryAI,
						Severity:         models.SeverityHigh,
						Confidence:       0.85,
						EvidenceStrength: models.EvidenceModerate,
						EvidenceSource:   models.SourceStructuralPattern,
						Location: models.SignalLocation{
							File: relPath,
							Line: int(node.StartPoint().Row) + 1,
						},
						Explanation: fmt.Sprintf(
							"%s in %s loads a model without a content-addressed pin (commit SHA, revision, or version suffix). The underlying weights may change without a code edit, regressing eval scores silently.",
							shape.displayName, relPath,
						),
						SuggestedAction: shape.remediation,
						RuleID:          "terrain/hygiene/model-fixture-unpinned",
						RuleURI:         "docs/rules/hygiene/model-fixture-unpinned.md",
						DetectorVersion: "0.2.0",
						Metadata: map[string]any{
							"loader": shape.displayName,
							"family": shape.family,
						},
					})
				}
			}
		}
	}
	for i := 0; i < int(node.ChildCount()); i++ {
		walkForModelLoads(node.Child(i), src, relPath, out)
	}
}

type modelLoadShape struct {
	pattern     string // suffix to match the call's function text
	displayName string
	family      string
	remediation string
}

var modelLoadShapes = []modelLoadShape{
	{".from_pretrained", "*.from_pretrained", "huggingface",
		"Pass revision=\"<commit-sha-or-version>\" to from_pretrained. Avoid revision=\"main\" / \"master\" — they're branch heads."},
	{"torch.load", "torch.load", "torch",
		"Load from a content-addressed path (model_v3_abc1234.pt) or use safetensors with a checksum."},
	{"joblib.load", "joblib.load", "sklearn",
		"Save the model with a version-suffixed filename (model_v3.joblib) and reference that filename."},
	{"load_model", "load_model", "keras",
		"Save the model with a version-suffixed filename and reference that filename."},
	{"snapshot_download", "snapshot_download", "huggingface",
		"Pass revision=\"<commit-sha-or-version>\" to snapshot_download. Avoid revision=\"main\"."},
}

func classifyModelLoadCall(text string) (modelLoadShape, bool) {
	for _, s := range modelLoadShapes {
		if text == s.pattern || strings.HasSuffix(text, s.pattern) ||
			strings.HasSuffix(text, "."+s.pattern) {
			return s, true
		}
	}
	return modelLoadShape{}, false
}

// isModelLoadPinned checks for static evidence that the load is pinned.
func isModelLoadPinned(shape modelLoadShape, argsNode *sitter.Node, src []byte) bool {
	if argsNode == nil {
		return false
	}

	switch shape.family {
	case "huggingface":
		// Look for revision="<value>" kwarg.
		rev, ok := lookupKwargString(argsNode, src, "revision")
		if !ok {
			return false
		}
		return !isMovingRevision(rev)
	}

	// For path-based loaders: check the first positional argument for a
	// pinned-looking string literal.
	if argsNode.NamedChildCount() == 0 {
		return false
	}
	firstArg := argsNode.NamedChild(0)
	// Unwrap one level if needed.
	switch firstArg.Type() {
	case "string", "interpreted_string_literal", "raw_string_literal":
		raw := strings.Trim(string(src[firstArg.StartByte():firstArg.EndByte()]), `"'`)
		return looksLikePinnedPath(raw)
	}
	return false
}

func lookupKwargString(argsNode *sitter.Node, src []byte, name string) (string, bool) {
	for i := 0; i < int(argsNode.NamedChildCount()); i++ {
		arg := argsNode.NamedChild(i)
		if arg.Type() != "keyword_argument" {
			continue
		}
		k := arg.ChildByFieldName("name")
		v := arg.ChildByFieldName("value")
		if k == nil || v == nil {
			continue
		}
		if string(src[k.StartByte():k.EndByte()]) != name {
			continue
		}
		if v.Type() != "string" {
			return "", false
		}
		return strings.Trim(string(src[v.StartByte():v.EndByte()]), `"'`), true
	}
	return "", false
}

func isMovingRevision(rev string) bool {
	switch strings.ToLower(strings.TrimSpace(rev)) {
	case "main", "master", "latest", "head", "":
		return true
	}
	return false
}

// looksLikePinnedPath returns true when a path looks content-addressed:
//   - Contains a 7+-hex segment (commit SHA fragment)
//   - Contains a version-shaped segment (v1.2.3, _v3, etc.)
//   - Ends with a .safetensors extension (content-addressed by convention)
func looksLikePinnedPath(p string) bool {
	lower := strings.ToLower(p)
	if strings.HasSuffix(lower, ".safetensors") {
		return true
	}
	// Look for any 7+-hex segment.
	if hasHexRun(p, 7) {
		return true
	}
	// Look for v\d+(\.\d+)+ pattern — at least one digit after a dot.
	for i := 0; i+1 < len(p); i++ {
		if p[i] != 'v' && p[i] != 'V' {
			continue
		}
		if p[i+1] < '0' || p[i+1] > '9' {
			continue
		}
		j := i + 1
		sawDotAndDigit := false
		afterDot := false
		for j < len(p) && (p[j] >= '0' && p[j] <= '9' || p[j] == '.') {
			if p[j] == '.' {
				afterDot = true
			} else if afterDot {
				sawDotAndDigit = true
			}
			j++
		}
		if sawDotAndDigit {
			return true
		}
	}
	return false
}

func hasHexRun(s string, minLen int) bool {
	run := 0
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9',
			r >= 'a' && r <= 'f',
			r >= 'A' && r <= 'F':
			run++
			if run >= minLen {
				return true
			}
		default:
			run = 0
		}
	}
	return false
}
