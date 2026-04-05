package convert

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
)

var (
	reJestGlobalsImport  = regexp.MustCompile(`(?m)^import\s+\{[^}]*\}\s+from\s+['"]@jest/globals['"];\s*\n?`)
	reJestGlobalsRequire = regexp.MustCompile(`(?m)^(?:const|let|var)\s+\{[^}]*\}\s*=\s*require\(\s*['"]@jest/globals['"]\s*\);\s*\n?`)
	reVitestImport       = regexp.MustCompile(`(?m)^import\s+\{([^}]*)\}\s+from\s+['"]vitest['"];\s*\n?`)
	reJestSetTimeout     = regexp.MustCompile(`\bjest\.setTimeout\s*\(\s*([^)]+?)\s*\)`)
	reVitestUsageVI      = regexp.MustCompile(`\bvi\.`)
)

var jestToVitestReplacer = strings.NewReplacer(
	"jest.fn", "vi.fn",
	"jest.spyOn", "vi.spyOn",
	"jest.mock", "vi.mock",
	"jest.doMock", "vi.doMock",
	"jest.unmock", "vi.unmock",
	"jest.clearAllMocks", "vi.clearAllMocks",
	"jest.resetAllMocks", "vi.resetAllMocks",
	"jest.restoreAllMocks", "vi.restoreAllMocks",
	"jest.useFakeTimers", "vi.useFakeTimers",
	"jest.useRealTimers", "vi.useRealTimers",
	"jest.advanceTimersByTime", "vi.advanceTimersByTime",
	"jest.advanceTimersToNextTimer", "vi.advanceTimersToNextTimer",
	"jest.runAllTimers", "vi.runAllTimers",
	"jest.runOnlyPendingTimers", "vi.runOnlyPendingTimers",
	"jest.clearAllTimers", "vi.clearAllTimers",
	"jest.resetModules", "vi.resetModules",
	"jest.isMockFunction", "vi.isMockFunction",
	"jest.setSystemTime", "vi.setSystemTime",
)

var vitestImportOrder = []string{
	"describe",
	"it",
	"test",
	"expect",
	"beforeEach",
	"afterEach",
	"beforeAll",
	"afterAll",
	"vi",
}

var jestToVitestMembers = map[string]string{
	"fn":                       "fn",
	"spyOn":                    "spyOn",
	"mock":                     "mock",
	"doMock":                   "doMock",
	"unmock":                   "unmock",
	"clearAllMocks":            "clearAllMocks",
	"resetAllMocks":            "resetAllMocks",
	"restoreAllMocks":          "restoreAllMocks",
	"useFakeTimers":            "useFakeTimers",
	"useRealTimers":            "useRealTimers",
	"advanceTimersByTime":      "advanceTimersByTime",
	"advanceTimersToNextTimer": "advanceTimersToNextTimer",
	"runAllTimers":             "runAllTimers",
	"runOnlyPendingTimers":     "runOnlyPendingTimers",
	"clearAllTimers":           "clearAllTimers",
	"resetModules":             "resetModules",
	"isMockFunction":           "isMockFunction",
	"setSystemTime":            "setSystemTime",
}

var vitestGlobalNames = map[string]bool{
	"describe":   true,
	"it":         true,
	"test":       true,
	"expect":     true,
	"beforeEach": true,
	"afterEach":  true,
	"beforeAll":  true,
	"afterAll":   true,
}

var vitestUsagePatterns = map[string]*regexp.Regexp{
	"describe":   regexp.MustCompile(`\bdescribe\b\s*(?:\(|\.)`),
	"it":         regexp.MustCompile(`\bit\b\s*(?:\(|\.)`),
	"test":       regexp.MustCompile(`\btest\b\s*(?:\(|\.)`),
	"expect":     regexp.MustCompile(`\bexpect\b\s*(?:\(|\.)`),
	"beforeEach": regexp.MustCompile(`\bbeforeEach\b\s*\(`),
	"afterEach":  regexp.MustCompile(`\bafterEach\b\s*\(`),
	"beforeAll":  regexp.MustCompile(`\bbeforeAll\b\s*\(`),
	"afterAll":   regexp.MustCompile(`\bafterAll\b\s*\(`),
}

// ConvertJestToVitestSource rewrites common Jest test patterns to their
// Vitest equivalents. This first Go-native slice is intentionally scoped to
// the high-confidence, API-compatible surface.
func ConvertJestToVitestSource(source string) (string, error) {
	if strings.TrimSpace(source) == "" {
		return source, nil
	}

	source = strings.ReplaceAll(source, "\r\n", "\n")
	if result, ok := convertJestToVitestSourceAST(source); ok {
		return result, nil
	}

	result := source
	var existingVitestNames []string
	result = reVitestImport.ReplaceAllStringFunc(result, func(match string) string {
		submatches := reVitestImport.FindStringSubmatch(match)
		if len(submatches) > 1 {
			for _, name := range strings.Split(submatches[1], ",") {
				name = strings.TrimSpace(name)
				if name != "" {
					existingVitestNames = append(existingVitestNames, name)
				}
			}
		}
		return ""
	})

	result = reJestGlobalsImport.ReplaceAllString(result, "")
	result = reJestGlobalsRequire.ReplaceAllString(result, "")
	result = reJestSetTimeout.ReplaceAllString(result, "vi.setConfig({ testTimeout: $1 })")
	result = jestToVitestReplacer.Replace(result)

	imports := detectVitestImports(result)
	for _, name := range existingVitestNames {
		imports[name] = true
	}

	if len(imports) == 0 {
		return ensureTrailingNewline(result), nil
	}

	importLine := buildVitestImport(imports)
	result = prependImportPreservingHeader(result, importLine)
	result = collapseBlankLines(result)
	return ensureTrailingNewline(result), nil
}

func convertJestToVitestSourceAST(source string) (string, bool) {
	tree, ok := parseJSSyntaxTree(source)
	if !ok {
		return "", false
	}
	defer tree.Close()

	imports := map[string]bool{}
	existingVitestNames := map[string]bool{}
	edits := make([]textEdit, 0, 8)

	walkJSNodes(tree.tree.RootNode(), func(node *sitter.Node) bool {
		switch node.Type() {
		case "import_statement":
			module := jsNodeText(node, tree.src)
			switch {
			case strings.Contains(module, "'vitest'") || strings.Contains(module, "\"vitest\""):
				for _, name := range extractNamedImports(module) {
					existingVitestNames[name] = true
				}
				edits = append(edits, textEdit{
					start: int(node.StartByte()),
					end:   int(node.EndByte()),
				})
				return false
			case strings.Contains(module, "'@jest/globals'") || strings.Contains(module, "\"@jest/globals\""):
				edits = append(edits, textEdit{
					start: int(node.StartByte()),
					end:   int(node.EndByte()),
				})
				return false
			}
		case "lexical_declaration", "variable_declaration":
			text := jsNodeText(node, tree.src)
			if strings.Contains(text, "require('@jest/globals')") || strings.Contains(text, "require(\"@jest/globals\")") {
				edits = append(edits, textEdit{
					start: int(node.StartByte()),
					end:   int(node.EndByte()),
				})
				return false
			}
		case "call_expression":
			callee := jsCalleeNode(node)
			if callee != nil {
				base := jsBaseIdentifier(callee, tree.src)
				if vitestGlobalNames[base] {
					imports[base] = true
				}
				if callee.Type() == "member_expression" {
					object := jsMemberObject(callee)
					property := jsNodeText(jsMemberProperty(callee), tree.src)
					if jsNodeText(object, tree.src) == "jest" && property == "setTimeout" {
						args := jsArgumentsNode(node)
						if args != nil && args.NamedChildCount() > 0 {
							imports["vi"] = true
							edits = append(edits, textEdit{
								start:       int(node.StartByte()),
								end:         int(node.EndByte()),
								replacement: "vi.setConfig({ testTimeout: " + jsNodeText(args.NamedChild(0), tree.src) + " })",
							})
							return false
						}
					}
				}
			}
		case "member_expression":
			base := jsBaseIdentifier(node, tree.src)
			if vitestGlobalNames[base] {
				imports[base] = true
			}

			object := jsMemberObject(node)
			property := jsNodeText(jsMemberProperty(node), tree.src)
			if jsNodeText(object, tree.src) != "jest" {
				return true
			}
			if property == "setTimeout" {
				return true
			}
			mapped, ok := jestToVitestMembers[property]
			if !ok {
				return true
			}
			imports["vi"] = true
			edits = append(edits, textEdit{
				start:       int(node.StartByte()),
				end:         int(node.EndByte()),
				replacement: "vi." + mapped,
			})
			return false
		}
		return true
	})

	result := applyTextEdits(source, edits)
	for name := range existingVitestNames {
		imports[name] = true
	}
	if len(imports) == 0 {
		return ensureTrailingNewline(collapseBlankLines(result)), true
	}

	importLine := buildVitestImport(imports)
	result = prependImportPreservingHeader(result, importLine)
	result = collapseBlankLines(result)
	return ensureTrailingNewline(result), true
}

func detectVitestImports(source string) map[string]bool {
	imports := map[string]bool{}

	for name, pattern := range vitestUsagePatterns {
		if pattern.MatchString(source) {
			imports[name] = true
		}
	}
	if reVitestUsageVI.MatchString(source) {
		imports["vi"] = true
	}
	return imports
}

func buildVitestImport(imports map[string]bool) string {
	names := make([]string, 0, len(imports))
	seen := map[string]bool{}
	for _, name := range vitestImportOrder {
		if imports[name] {
			names = append(names, name)
			seen[name] = true
		}
	}
	extra := make([]string, 0, len(imports))
	for name := range imports {
		if seen[name] {
			continue
		}
		extra = append(extra, name)
	}
	sort.Strings(extra)
	names = append(names, extra...)
	return fmt.Sprintf("import { %s } from 'vitest';", strings.Join(names, ", "))
}

func extractNamedImports(importLine string) []string {
	start := strings.Index(importLine, "{")
	end := strings.Index(importLine, "}")
	if start < 0 || end <= start {
		return nil
	}
	raw := strings.Split(importLine[start+1:end], ",")
	names := make([]string, 0, len(raw))
	for _, name := range raw {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		names = append(names, name)
	}
	return names
}

func prependImportPreservingHeader(source, importLine string) string {
	header, body := splitLeadingHeader(source)
	body = strings.TrimLeft(body, "\n")
	if body == "" {
		return header + importLine + "\n"
	}
	if header == "" {
		return importLine + "\n\n" + body
	}
	return header + importLine + "\n\n" + body
}

func splitLeadingHeader(source string) (string, string) {
	i := 0
	if strings.HasPrefix(source, "#!") {
		if end := strings.IndexByte(source, '\n'); end >= 0 {
			i = end + 1
		} else {
			return source + "\n", ""
		}
	}

	for i < len(source) {
		start := i
		for i < len(source) && (source[i] == ' ' || source[i] == '\t' || source[i] == '\n' || source[i] == '\r') {
			i++
		}
		switch {
		case strings.HasPrefix(source[i:], "//"):
			if end := strings.IndexByte(source[i:], '\n'); end >= 0 {
				i += end + 1
				continue
			}
			return source, ""
		case strings.HasPrefix(source[i:], "/*"):
			if end := strings.Index(source[i+2:], "*/"); end >= 0 {
				i += end + 4
				if i < len(source) && source[i] == '\n' {
					i++
				}
				continue
			}
			return source, ""
		default:
			return source[:start], source[start:]
		}
	}

	return source[:i], source[i:]
}

func collapseBlankLines(source string) string {
	for strings.Contains(source, "\n\n\n") {
		source = strings.ReplaceAll(source, "\n\n\n", "\n\n")
	}
	return source
}

func ensureTrailingNewline(source string) string {
	if source == "" || strings.HasSuffix(source, "\n") {
		return source
	}
	return source + "\n"
}
