package convert

import (
	"regexp"
	"strings"
)

var (
	reMochaBeforeAll          = regexp.MustCompile(`\bbefore\s*\(`)
	reMochaAfterAll           = regexp.MustCompile(`\bafter\s*\(`)
	reMochaExpectDeepEqual    = regexp.MustCompile(`expect\(([^)]+)\)\.to\.deep\.equal\(([^)]+)\)`)
	reMochaExpectEqual        = regexp.MustCompile(`expect\(([^)]+)\)\.to\.equal\(([^)]+)\)`)
	reMochaExpectLength       = regexp.MustCompile(`expect\(([^)]+)\)\.to\.have\.lengthOf\(([^)]+)\)`)
	reMochaExpectTrue         = regexp.MustCompile(`expect\(([^)]+)\)\.to\.be\.true`)
	reMochaExpectFalse        = regexp.MustCompile(`expect\(([^)]+)\)\.to\.be\.false`)
	reMochaExpectContain      = regexp.MustCompile(`expect\(([^)]+)\)\.to\.contain\(([^)]+)\)`)
	reSinonStubMethodCallFake = regexp.MustCompile(`sinon\.stub\(([^,]+),\s*([^)]+)\)\.callsFake\(([^)]+)\)`)
	reSinonStubMethodReturns  = regexp.MustCompile(`sinon\.stub\(([^,]+),\s*([^)]+)\)\.returns\(([^)]+)\)`)
	reSinonStubMethodResolves = regexp.MustCompile(`sinon\.stub\(([^,]+),\s*([^)]+)\)\.resolves\(([^)]+)\)`)
	reSinonStubMethodRejects  = regexp.MustCompile(`sinon\.stub\(([^,]+),\s*([^)]+)\)\.rejects\(([^)]+)\)`)
	reSinonSpy                = regexp.MustCompile(`sinon\.spy\(([^,]+),\s*([^)]+)\)`)
	reSinonStubCallsFake      = regexp.MustCompile(`sinon\.stub\(\)\.callsFake\(([^)]+)\)`)
	reSinonStubReturns        = regexp.MustCompile(`sinon\.stub\(\)\.returns\(([^)]+)\)`)
	reSinonStubResolves       = regexp.MustCompile(`sinon\.stub\(\)\.resolves\(([^)]+)\)`)
	reSinonStubRejects        = regexp.MustCompile(`sinon\.stub\(\)\.rejects\(([^)]+)\)`)
	reSinonStub               = regexp.MustCompile(`sinon\.stub\(\)`)
	reSinonAssertCalledWith   = regexp.MustCompile(`sinon\.assert\.calledWith\(([^,]+),\s*(.+)\)`)
	reSinonAssertCalledOnce   = regexp.MustCompile(`sinon\.assert\.calledOnce\(([^)]+)\)`)
	reSinonAssertCalled       = regexp.MustCompile(`sinon\.assert\.called\(([^)]+)\)`)
	reSinonUseFakeTimers      = regexp.MustCompile(`sinon\.useFakeTimers\(\)`)
	reSinonReset              = regexp.MustCompile(`sinon\.reset\(\)`)
	reSinonRestore            = regexp.MustCompile(`sinon\.restore\(\)`)
	reMochaClockTick          = regexp.MustCompile(`clock\.tick\(([^)]+)\)`)
	reMochaClockRestore       = regexp.MustCompile(`clock\.restore\(\)`)
)

// ConvertMochaToJestSource rewrites the high-confidence Mocha/Chai/Sinon
// surface into Go-native Jest output.
func ConvertMochaToJestSource(source string) (string, error) {
	if strings.TrimSpace(source) == "" {
		return source, nil
	}
	if !strings.Contains(source, "chai") &&
		!strings.Contains(source, "sinon.") &&
		!strings.Contains(source, ".to.equal(") &&
		!strings.Contains(source, ".to.deep.equal(") &&
		!strings.Contains(source, ".to.have.lengthOf(") &&
		!strings.Contains(source, ".to.be.true") &&
		!strings.Contains(source, ".to.be.false") &&
		!strings.Contains(source, ".to.contain(") &&
		!strings.Contains(source, "sinon.assert.") {
		return ensureTrailingNewline(strings.ReplaceAll(source, "\r\n", "\n")), nil
	}

	result := strings.ReplaceAll(source, "\r\n", "\n")
	if astResult, ok := convertMochaToJestSourceAST(result); ok {
		return astResult, nil
	}

	result = stripMochaPreludeImports(result)
	result = replaceCodeRegexString(result, reMochaBeforeAll, "beforeAll(")
	result = replaceCodeRegexString(result, reMochaAfterAll, "afterAll(")

	result = replaceCodeRegexString(result, reSinonStubMethodCallFake, "jest.spyOn($1, $2).mockImplementation($3)")
	result = replaceCodeRegexString(result, reSinonStubMethodReturns, "jest.spyOn($1, $2).mockReturnValue($3)")
	result = replaceCodeRegexString(result, reSinonStubMethodResolves, "jest.spyOn($1, $2).mockResolvedValue($3)")
	result = replaceCodeRegexString(result, reSinonStubMethodRejects, "jest.spyOn($1, $2).mockRejectedValue($3)")
	result = replaceCodeRegexString(result, reSinonSpy, "jest.spyOn($1, $2)")
	result = replaceCodeRegexString(result, reSinonStubCallsFake, "jest.fn().mockImplementation($1)")
	result = replaceCodeRegexString(result, reSinonStubReturns, "jest.fn().mockReturnValue($1)")
	result = replaceCodeRegexString(result, reSinonStubResolves, "jest.fn().mockResolvedValue($1)")
	result = replaceCodeRegexString(result, reSinonStubRejects, "jest.fn().mockRejectedValue($1)")
	result = replaceCodeRegexString(result, reSinonStub, "jest.fn()")
	result = replaceCodeRegexString(result, reSinonAssertCalledWith, "expect($1).toHaveBeenCalledWith($2)")
	result = replaceCodeRegexString(result, reSinonAssertCalledOnce, "expect($1).toHaveBeenCalledTimes(1)")
	result = replaceCodeRegexString(result, reSinonAssertCalled, "expect($1).toHaveBeenCalled()")
	result = replaceCodeRegexString(result, reSinonUseFakeTimers, "jest.useFakeTimers()")
	result = replaceCodeRegexString(result, reSinonReset, "jest.clearAllMocks()")
	result = replaceCodeRegexString(result, reSinonRestore, "jest.restoreAllMocks()")
	result = replaceCodeRegexString(result, reMochaClockTick, "jest.advanceTimersByTime($1)")
	result = replaceCodeRegexString(result, reMochaClockRestore, "jest.useRealTimers()")

	result = replaceCodeRegexString(result, reMochaExpectDeepEqual, "expect($1).toEqual($2)")
	result = replaceCodeRegexString(result, reMochaExpectLength, "expect($1).toHaveLength($2)")
	result = replaceCodeRegexString(result, reMochaExpectTrue, "expect($1).toBe(true)")
	result = replaceCodeRegexString(result, reMochaExpectFalse, "expect($1).toBe(false)")
	result = replaceCodeRegexString(result, reMochaExpectContain, "expect($1).toContain($2)")
	result = replaceCodeRegexString(result, reMochaExpectEqual, "expect($1).toBe($2)")

	result = commentMatchedLines(result, func(line string) bool {
		return strings.Contains(line, "chai.expect(") || strings.Contains(line, "assert.")
	}, "manual Mocha assertion conversion required")
	result = collapseBlankLines(result)
	return ensureTrailingNewline(result), nil
}
