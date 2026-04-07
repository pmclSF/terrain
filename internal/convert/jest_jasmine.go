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
	result = replaceCodeRegexString(result, reJestSpyOnReturnValueJasmine, "spyOn($1, $2).and.returnValue($3)")
	result = replaceCodeRegexString(result, reJestSpyOnMockImplJasmine, "spyOn($1, $2).and.callFake($3)")
	result = replaceCodeRegexString(result, reJestSpyOnJasmine, "spyOn($1, $2)")
	result = replaceCodeRegexString(result, reJestFnReturnValueJasmine, "jasmine.createSpy().and.returnValue($1)")
	result = replaceCodeRegexString(result, reJestFnResolvedValueJasmine, "jasmine.createSpy().and.returnValue(Promise.resolve($1))")
	result = replaceCodeRegexString(result, reJestFnRejectedValueJasmine, "jasmine.createSpy().and.returnValue(Promise.reject($1))")
	result = replaceCodeRegexString(result, reJestFnImplementationJasm, "jasmine.createSpy().and.callFake($1)")
	result = replaceCodeRegexString(result, reJestFnWithImplJasmine, "jasmine.createSpy().and.callFake($1)")
	result = replaceCodeRegexString(result, reJestFn, "jasmine.createSpy()")
	result = replaceCodeRegexString(result, reJestUseFakeTimers, "jasmine.clock().install()")
	result = replaceCodeRegexString(result, reJestUseRealTimers, "jasmine.clock().uninstall()")
	result = replaceCodeRegexString(result, reJestAdvanceTimers, "jasmine.clock().tick($1)")
	result = replaceCodeRegexString(result, reJestSetSystemTime, "jasmine.clock().mockDate($1)")
	result = jestToJasmineReplacer.Replace(result)
	result = commentMatchedLines(result, func(line string) bool {
		return reJestMockLine.MatchString(line)
	}, "manual Jest module mock conversion required")
	result = collapseBlankLines(result)
	return ensureTrailingNewline(result), nil
}
