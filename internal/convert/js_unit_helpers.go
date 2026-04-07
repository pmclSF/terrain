package convert

import (
	"regexp"
	"strings"
)

var (
	reJSImportLine         = regexp.MustCompile(`(?m)^\s*import\b`)
	reChaiExpectImport     = regexp.MustCompile(`(?m)^import\s+\{[^}]*\bexpect\b[^}]*\}\s+from\s+['"]chai['"];\s*\n?`)
	reChaiExpectRequire    = regexp.MustCompile(`(?m)^(?:const|let|var)\s+\{[^}]*\bexpect\b[^}]*\}\s*=\s*require\(\s*['"]chai['"]\s*\);\s*\n?`)
	reSinonDefaultImport   = regexp.MustCompile(`(?m)^import\s+sinon\s+from\s+['"]sinon['"];\s*\n?`)
	reSinonNamespaceImport = regexp.MustCompile(`(?m)^import\s+\*\s+as\s+sinon\s+from\s+['"]sinon['"];\s*\n?`)
	reSinonRequire         = regexp.MustCompile(`(?m)^(?:const|let|var)\s+sinon\s*=\s*require\(\s*['"]sinon['"]\s*\);\s*\n?`)
	reJestMockLine         = regexp.MustCompile(`\bjest\.mock\s*\(`)
	reJasmineAddMatchers   = regexp.MustCompile(`\bjasmine\.addMatchers\s*\(`)
)

func usesESModuleSyntax(source string) bool {
	return reJSImportLine.MatchString(source)
}

func stripMochaPreludeImports(source string) string {
	result := reChaiExpectImport.ReplaceAllString(source, "")
	result = reChaiExpectRequire.ReplaceAllString(result, "")
	result = reSinonDefaultImport.ReplaceAllString(result, "")
	result = reSinonNamespaceImport.ReplaceAllString(result, "")
	result = reSinonRequire.ReplaceAllString(result, "")
	return result
}

func prependMochaPrelude(source string, useESM bool, needSinon bool) string {
	lines := make([]string, 0, 2)
	if useESM {
		lines = append(lines, "import { expect } from 'chai';")
		if needSinon {
			lines = append(lines, "import sinon from 'sinon';")
		}
	} else {
		lines = append(lines, "const { expect } = require('chai');")
		if needSinon {
			lines = append(lines, "const sinon = require('sinon');")
		}
	}
	return prependImportPreservingHeader(source, strings.Join(lines, "\n"))
}

func commentMatchedLines(source string, match func(string) bool, todo string) string {
	lines := strings.Split(source, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "///") {
			continue
		}
		if !match(line) {
			continue
		}
		indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
		lines[i] = indent + "// TERRAIN-TODO: " + todo + "\n" + indent + "// " + strings.TrimSpace(line)
	}
	return strings.Join(lines, "\n")
}
