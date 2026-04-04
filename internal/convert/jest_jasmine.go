package convert

import (
	"regexp"
	"strings"
)

var (
	reJestSpyOnReturnValueJasmine = regexp.MustCompile(`jest\.spyOn\(([^,]+),\s*([^)]+)\)\.mockReturnValue\(([^)]+)\)`)
	reJestSpyOnMockImplJasmine    = regexp.MustCompile(`jest\.spyOn\(([^,]+),\s*([^)]+)\)\.mockImplementation\(([^)]+)\)`)
	reJestSpyOnJasmine            = regexp.MustCompile(`jest\.spyOn\(([^,]+),\s*([^)]+)\)`)
	reJestFnReturnValueJasmine    = regexp.MustCompile(`jest\.fn\(\)\.mockReturnValue\(([^)]+)\)`)
	reJestFnResolvedValueJasmine  = regexp.MustCompile(`jest\.fn\(\)\.mockResolvedValue\(([^)]+)\)`)
	reJestFnRejectedValueJasmine  = regexp.MustCompile(`jest\.fn\(\)\.mockRejectedValue\(([^)]+)\)`)
	reJestFnImplementationJasm    = regexp.MustCompile(`jest\.fn\(\)\.mockImplementation\(([^)]+)\)`)
	reJestFnWithImplJasmine       = regexp.MustCompile(`jest\.fn\(([^)]+)\)`)
	reJestSetSystemTime           = regexp.MustCompile(`jest\.setSystemTime\(([^)]+)\)`)
)

var jestToJasmineReplacer = strings.NewReplacer(
	"expect.anything()", "jasmine.anything()",
	"expect.objectContaining(", "jasmine.objectContaining(",
	"expect.arrayContaining(", "jasmine.arrayContaining(",
	"expect.stringMatching(", "jasmine.stringMatching(",
	"expect.any(", "jasmine.any(",
)

// ConvertJestToJasmineSource rewrites the high-confidence Jest surface into
// Go-native Jasmine output.
func ConvertJestToJasmineSource(source string) (string, error) {
	if strings.TrimSpace(source) == "" {
		return source, nil
	}
	if !strings.Contains(source, "jest.") &&
		!strings.Contains(source, "@jest/globals") &&
		!strings.Contains(source, ".toHaveBeenCalled") {
		return ensureTrailingNewline(strings.ReplaceAll(source, "\r\n", "\n")), nil
	}

	result := strings.ReplaceAll(source, "\r\n", "\n")
	if astResult, ok := convertJestToJasmineSourceAST(result); ok {
		return astResult, nil
	}

	result = reJestGlobalsImport.ReplaceAllString(result, "")
	result = reJestGlobalsRequire.ReplaceAllString(result, "")
	result = reJestSpyOnReturnValueJasmine.ReplaceAllString(result, "spyOn($1, $2).and.returnValue($3)")
	result = reJestSpyOnMockImplJasmine.ReplaceAllString(result, "spyOn($1, $2).and.callFake($3)")
	result = reJestSpyOnJasmine.ReplaceAllString(result, "spyOn($1, $2)")
	result = reJestFnReturnValueJasmine.ReplaceAllString(result, "jasmine.createSpy().and.returnValue($1)")
	result = reJestFnResolvedValueJasmine.ReplaceAllString(result, "jasmine.createSpy().and.returnValue(Promise.resolve($1))")
	result = reJestFnRejectedValueJasmine.ReplaceAllString(result, "jasmine.createSpy().and.returnValue(Promise.reject($1))")
	result = reJestFnImplementationJasm.ReplaceAllString(result, "jasmine.createSpy().and.callFake($1)")
	result = reJestFnWithImplJasmine.ReplaceAllString(result, "jasmine.createSpy().and.callFake($1)")
	result = reJestFn.ReplaceAllString(result, "jasmine.createSpy()")
	result = reJestUseFakeTimers.ReplaceAllString(result, "jasmine.clock().install()")
	result = reJestUseRealTimers.ReplaceAllString(result, "jasmine.clock().uninstall()")
	result = reJestAdvanceTimers.ReplaceAllString(result, "jasmine.clock().tick($1)")
	result = reJestSetSystemTime.ReplaceAllString(result, "jasmine.clock().mockDate($1)")
	result = jestToJasmineReplacer.Replace(result)
	result = commentMatchedLines(result, func(line string) bool {
		return reJestMockLine.MatchString(line)
	}, "manual Jest module mock conversion required")
	result = collapseBlankLines(result)
	return ensureTrailingNewline(result), nil
}
