package convert

import (
	"context"
	"fmt"
	"os"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/java"
	"github.com/smacker/go-tree-sitter/python"
)

// ValidateSyntax checks whether converted output is parseable for the target language.
func ValidateSyntax(path, language, source string) error {
	if strings.TrimSpace(source) == "" {
		return nil
	}

	switch strings.ToLower(strings.TrimSpace(language)) {
	case "javascript", "typescript":
		tree, ok := parseJSSyntaxTree(source)
		if ok {
			tree.Close()
			return nil
		}
		return syntaxValidationError(path, language, nil)
	case "python":
		return validateTreeSitterSyntax(path, language, source, python.GetLanguage())
	case "java":
		return validateTreeSitterSyntax(path, language, source, java.GetLanguage())
	default:
		return nil
	}
}

// ValidateExecutionResult checks the syntax of converted output returned from Execute.
func ValidateExecutionResult(result ExecutionResult, language string) error {
	if result.Mode == "stdout" {
		path := result.Source
		if len(result.Files) > 0 && strings.TrimSpace(result.Files[0].SourcePath) != "" {
			path = result.Files[0].SourcePath
		}
		return ValidateSyntax(path, language, result.StdoutContent)
	}

	for _, file := range result.Files {
		if strings.TrimSpace(file.OutputPath) == "" {
			continue
		}
		content, err := os.ReadFile(file.OutputPath)
		if err != nil {
			return fmt.Errorf("read converted output for validation: %w", err)
		}
		if err := ValidateSyntax(file.OutputPath, language, string(content)); err != nil {
			return err
		}
	}
	return nil
}

// CleanupExecutionOutputs removes written conversion outputs after a failed validation step.
func CleanupExecutionOutputs(result ExecutionResult) error {
	if result.Mode == "stdout" {
		return nil
	}

	seen := make(map[string]bool, len(result.Files))
	for _, file := range result.Files {
		if strings.TrimSpace(file.OutputPath) == "" || seen[file.OutputPath] {
			continue
		}
		seen[file.OutputPath] = true
		if err := os.Remove(file.OutputPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove invalid converted output %s: %w", file.OutputPath, err)
		}
	}
	return nil
}

func validateTreeSitterSyntax(path, language, source string, lang *sitter.Language) error {
	parser := sitter.NewParser()
	defer parser.Close()
	parser.SetLanguage(lang)

	tree, err := parser.ParseCtx(context.Background(), nil, []byte(source))
	if err != nil || tree == nil {
		return syntaxValidationError(path, language, nil)
	}
	defer tree.Close()

	root := tree.RootNode()
	if root == nil || !root.HasError() {
		return nil
	}
	return syntaxValidationError(path, language, firstSyntaxErrorNode(root))
}

func firstSyntaxErrorNode(node *sitter.Node) *sitter.Node {
	if node == nil || !node.HasError() {
		return nil
	}
	if node.IsError() {
		return node
	}
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil || !child.HasError() {
			continue
		}
		if errNode := firstSyntaxErrorNode(child); errNode != nil {
			return errNode
		}
	}
	return node
}

func syntaxValidationError(path, language string, node *sitter.Node) error {
	target := strings.TrimSpace(path)
	if target == "" {
		target = "converted output"
	}
	if node == nil {
		return fmt.Errorf("syntax validation failed for %s (%s)", target, language)
	}

	point := node.StartPoint()
	return fmt.Errorf(
		"syntax validation failed for %s (%s) near line %d, column %d",
		target,
		language,
		int(point.Row)+1,
		int(point.Column)+1,
	)
}
