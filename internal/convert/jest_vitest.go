package convert

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

var (
	reJestGlobalsImport  = regexp.MustCompile(`(?m)^import\s+\{[^}]*\}\s+from\s+['"]@jest/globals['"];\s*\n?`)
	reJestGlobalsRequire = regexp.MustCompile(`(?m)^(?:const|let|var)\s+\{[^}]*\}\s*=\s*require\(\s*['"]@jest/globals['"]\s*\);\s*\n?`)
	reVitestImport       = regexp.MustCompile(`(?m)^import\s+\{([^}]*)\}\s+from\s+['"]vitest['"];\s*\n?`)
	reJestSetTimeout     = regexp.MustCompile(`\bjest\.setTimeout\s*\(\s*([^)]+?)\s*\)`)
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

// ConvertJestToVitestSource rewrites common Jest test patterns to their
// Vitest equivalents. This first Go-native slice is intentionally scoped to
// the high-confidence, API-compatible surface.
func ConvertJestToVitestSource(source string) (string, error) {
	if strings.TrimSpace(source) == "" {
		return source, nil
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
	result = strings.ReplaceAll(result, "\r\n", "\n")

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

func detectVitestImports(source string) map[string]bool {
	imports := map[string]bool{}

	patterns := map[string]*regexp.Regexp{
		"describe":   regexp.MustCompile(`\bdescribe\b\s*(?:\(|\.)`),
		"it":         regexp.MustCompile(`\bit\b\s*(?:\(|\.)`),
		"test":       regexp.MustCompile(`\btest\b\s*(?:\(|\.)`),
		"expect":     regexp.MustCompile(`\bexpect\b\s*(?:\(|\.)`),
		"beforeEach": regexp.MustCompile(`\bbeforeEach\b\s*\(`),
		"afterEach":  regexp.MustCompile(`\bafterEach\b\s*\(`),
		"beforeAll":  regexp.MustCompile(`\bbeforeAll\b\s*\(`),
		"afterAll":   regexp.MustCompile(`\bafterAll\b\s*\(`),
	}
	for name, pattern := range patterns {
		if pattern.MatchString(source) {
			imports[name] = true
		}
	}
	if regexp.MustCompile(`\bvi\.`).MatchString(source) {
		imports["vi"] = true
	}
	return imports
}

func buildVitestImport(imports map[string]bool) string {
	names := make([]string, 0, len(imports))
	for _, name := range vitestImportOrder {
		if imports[name] {
			names = append(names, name)
		}
	}
	if len(names) == 0 {
		for name := range imports {
			names = append(names, name)
		}
		sort.Strings(names)
	}
	return fmt.Sprintf("import { %s } from 'vitest';", strings.Join(names, ", "))
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
