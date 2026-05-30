// Package security implements the security/* stable rules.
package security

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

// DetectInsecureDeserialization AST-walks Python source for calls
// into deserialization primitives that execute arbitrary code on
// untrusted input. Implements terrain/security/insecure-deserialization.
//
// Targets:
//
//   - pickle.load / pickle.loads / cPickle.load / cPickle.loads
//   - dill.load / dill.loads
//   - joblib.load (sklearn's preferred save format — wraps pickle)
//   - torch.load (PyTorch checkpoint — uses pickle under the hood
//     unless `weights_only=True` was explicitly passed)
//   - yaml.load without explicit Loader=yaml.SafeLoader
//   - marshal.load / marshal.loads
//
// The detector fires on any matching call. Suppression metadata
// (weights_only=True for torch.load, Loader=SafeLoader for yaml.load)
// is recognized and suppresses the finding when statically determinable.
func DetectInsecureDeserialization(src []byte, relPath string) []models.Signal {
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
		walkForDeserialization(tree.RootNode(), src, relPath, &out)
		return nil
	})
	return out
}

func walkForDeserialization(node *sitter.Node, src []byte, relPath string, out *[]models.Signal) {
	if node == nil {
		return
	}
	if node.Type() == "call" {
		funcNode := node.ChildByFieldName("function")
		argsNode := node.ChildByFieldName("arguments")
		if funcNode != nil {
			callText := string(src[funcNode.StartByte():funcNode.EndByte()])
			if call, ok := classifyDeserializationCall(callText); ok {
				// Check for suppression patterns before emitting.
				if !isExplicitlySafe(call, argsNode, src) {
					*out = append(*out, models.Signal{
						Type:             signals.SignalInsecureDeserialize,
						Category:         models.CategoryAI,
						Severity:         models.SeverityCritical,
						Confidence:       0.95,
						EvidenceStrength: models.EvidenceStrong,
						EvidenceSource:   models.SourceStructuralPattern,
						Location: models.SignalLocation{
							File: relPath,
							Line: int(node.StartPoint().Row) + 1,
						},
						Explanation: fmt.Sprintf(
							"%s in %s deserializes arbitrary code on untrusted input. An adversary who controls the input file can execute arbitrary code.",
							call.displayName, relPath,
						),
						SuggestedAction: call.remediation,
						RuleID:          "terrain/security/insecure-deserialization",
						RuleURI:         "docs/rules/security/insecure-deserialization.md",
						DetectorVersion: "0.2.0",
						Metadata: map[string]any{
							"primitive": call.displayName,
							"family":    call.family,
						},
					})
				}
			}
		}
	}
	for i := 0; i < int(node.ChildCount()); i++ {
		walkForDeserialization(node.Child(i), src, relPath, out)
	}
}

type deserializationCall struct {
	pattern     string
	displayName string
	family      string
	remediation string
}

var deserializationCalls = []deserializationCall{
	{"pickle.load", "pickle.load", "pickle",
		"Replace with a safe format (JSON, msgpack, or safetensors). When pickle is unavoidable, sandbox the deserialization and authenticate the source."},
	{"pickle.loads", "pickle.loads", "pickle",
		"Replace with a safe format (JSON, msgpack, or safetensors). When pickle is unavoidable, sandbox the deserialization and authenticate the source."},
	{"cPickle.load", "cPickle.load", "pickle",
		"Replace with a safe format (JSON, msgpack, or safetensors)."},
	{"dill.load", "dill.load", "pickle",
		"Replace with a safe format (JSON, msgpack, or safetensors)."},
	{"dill.loads", "dill.loads", "pickle",
		"Replace with a safe format (JSON, msgpack, or safetensors)."},
	{"joblib.load", "joblib.load", "pickle",
		"For loading sklearn models from untrusted sources, switch to safetensors / ONNX. Joblib is pickle-backed."},
	{"torch.load", "torch.load", "torch",
		"Pass weights_only=True to torch.load (PyTorch ≥2.0), or load from .safetensors instead."},
	{"yaml.load", "yaml.load", "yaml",
		"Pass Loader=yaml.SafeLoader, or switch to yaml.safe_load()."},
	{"marshal.load", "marshal.load", "marshal",
		"Replace with a safe format. marshal is Python-version-specific and unsafe on untrusted input."},
	{"marshal.loads", "marshal.loads", "marshal",
		"Replace with a safe format."},
}

func classifyDeserializationCall(text string) (deserializationCall, bool) {
	for _, c := range deserializationCalls {
		// Match either `pickle.load` directly or any `.pickle.load` qualified.
		if text == c.pattern || strings.HasSuffix(text, "."+c.pattern) {
			return c, true
		}
	}
	return deserializationCall{}, false
}

// isExplicitlySafe checks for suppression patterns:
//
//	torch.load(path, weights_only=True)            → safe at PyTorch ≥2.0
//	yaml.load(stream, Loader=yaml.SafeLoader)      → safe
//	yaml.load(stream, Loader=SafeLoader)           → safe
//
// Returns true when the call site declares the safe option.
func isExplicitlySafe(call deserializationCall, argsNode *sitter.Node, src []byte) bool {
	if argsNode == nil {
		return false
	}
	switch call.family {
	case "torch":
		return hasKeywordArg(argsNode, src, "weights_only", "True")
	case "yaml":
		return hasYAMLSafeLoader(argsNode, src)
	}
	return false
}

func hasKeywordArg(argsNode *sitter.Node, src []byte, name, value string) bool {
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
		if string(src[k.StartByte():k.EndByte()]) == name &&
			string(src[v.StartByte():v.EndByte()]) == value {
			return true
		}
	}
	return false
}

func hasYAMLSafeLoader(argsNode *sitter.Node, src []byte) bool {
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
		if string(src[k.StartByte():k.EndByte()]) == "Loader" {
			loader := string(src[v.StartByte():v.EndByte()])
			if strings.HasSuffix(loader, "SafeLoader") || strings.HasSuffix(loader, "CSafeLoader") {
				return true
			}
		}
	}
	return false
}
