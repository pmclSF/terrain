package convert

import (
	"regexp"
	"strings"
)

var (
	reJasmineCreateSpyReturnValue = regexp.MustCompile(`jasmine\.createSpy(?:\([^)]*\))?\.and\.returnValue\(([^)]+)\)`)
	reJasmineCreateSpyCallFake    = regexp.MustCompile(`jasmine\.createSpy(?:\([^)]*\))?\.and\.callFake\(([^)]+)\)`)
	reJasmineCreateSpy            = regexp.MustCompile(`jasmine\.createSpy(?:\([^)]*\))?`)
	reJasmineSpyOnReturnValue     = regexp.MustCompile(`spyOn\(([^,]+),\s*([^)]+)\)\.and\.returnValue\(([^)]+)\)`)
	reJasmineSpyOnCallFake        = regexp.MustCompile(`spyOn\(([^,]+),\s*([^)]+)\)\.and\.callFake\(([^)]+)\)`)
	reJasmineSpyOnCallThrough     = regexp.MustCompile(`spyOn\(([^,]+),\s*([^)]+)\)\.and\.callThrough\(\)`)
	reJasmineSpyOn                = regexp.MustCompile(`\bspyOn\(([^,]+),\s*([^)]+)\)`)
	reJasmineClockInstall         = regexp.MustCompile(`jasmine\.clock\(\)\.install\(\)`)
	reJasmineClockUninstall       = regexp.MustCompile(`jasmine\.clock\(\)\.uninstall\(\)`)
	reJasmineClockTick            = regexp.MustCompile(`jasmine\.clock\(\)\.tick\(([^)]+)\)`)
	reJasmineClockMockDate        = regexp.MustCompile(`jasmine\.clock\(\)\.mockDate\(([^)]+)\)`)
)

var jasmineToJestReplacer = strings.NewReplacer(
	"jasmine.anything()", "expect.anything()",
	"jasmine.objectContaining(", "expect.objectContaining(",
	"jasmine.arrayContaining(", "expect.arrayContaining(",
	"jasmine.stringMatching(", "expect.stringMatching(",
	"jasmine.any(", "expect.any(",
)

// ConvertJasmineToJestSource rewrites the high-confidence Jasmine surface into
// Go-native Jest output.
func ConvertJasmineToJestSource(source string) (string, error) {
	if strings.TrimSpace(source) == "" {
		return source, nil
	}
	if !strings.Contains(source, "jasmine.") &&
		!strings.Contains(source, "spyOn(") {
		return ensureTrailingNewline(strings.ReplaceAll(source, "\r\n", "\n")), nil
	}

	result := strings.ReplaceAll(source, "\r\n", "\n")
	result = reJasmineCreateSpyObj.ReplaceAllStringFunc(result, convertJasmineCreateSpyObj)
	result = reJasmineCreateSpyReturnValue.ReplaceAllString(result, "jest.fn().mockReturnValue($1)")
	result = reJasmineCreateSpyCallFake.ReplaceAllString(result, "jest.fn().mockImplementation($1)")
	result = reJasmineCreateSpy.ReplaceAllString(result, "jest.fn()")
	result = reJasmineSpyOnReturnValue.ReplaceAllString(result, "jest.spyOn($1, $2).mockReturnValue($3)")
	result = reJasmineSpyOnCallFake.ReplaceAllString(result, "jest.spyOn($1, $2).mockImplementation($3)")
	result = reJasmineSpyOnCallThrough.ReplaceAllString(result, "jest.spyOn($1, $2)")
	result = reJasmineSpyOn.ReplaceAllString(result, "jest.spyOn($1, $2)")
	result = reJasmineClockInstall.ReplaceAllString(result, "jest.useFakeTimers()")
	result = reJasmineClockUninstall.ReplaceAllString(result, "jest.useRealTimers()")
	result = reJasmineClockTick.ReplaceAllString(result, "jest.advanceTimersByTime($1)")
	result = reJasmineClockMockDate.ReplaceAllString(result, "jest.setSystemTime($1)")
	result = jasmineToJestReplacer.Replace(result)
	result = commentMatchedLines(result, func(line string) bool {
		return reJasmineAddMatchers.MatchString(line)
	}, "manual Jasmine matcher migration required")
	result = collapseBlankLines(result)
	return ensureTrailingNewline(result), nil
}

var reJasmineCreateSpyObj = regexp.MustCompile(`jasmine\.createSpyObj\(\s*['"][^'"]*['"]\s*,\s*\[([^\]]*)\]\s*\)`)

func convertJasmineCreateSpyObj(match string) string {
	parts := reJasmineCreateSpyObj.FindStringSubmatch(match)
	if len(parts) < 2 {
		return match
	}
	methods := strings.Split(parts[1], ",")
	items := make([]string, 0, len(methods))
	for _, method := range methods {
		method = strings.TrimSpace(method)
		method = strings.Trim(method, `"'`)
		if method == "" {
			continue
		}
		items = append(items, method+": jest.fn()")
	}
	if len(items) == 0 {
		return "{}"
	}
	return "{ " + strings.Join(items, ", ") + " }"
}
