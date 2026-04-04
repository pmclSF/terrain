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
	result = stripMochaPreludeImports(result)
	result = reMochaBeforeAll.ReplaceAllString(result, "beforeAll(")
	result = reMochaAfterAll.ReplaceAllString(result, "afterAll(")

	result = reSinonStubMethodCallFake.ReplaceAllString(result, "jest.spyOn($1, $2).mockImplementation($3)")
	result = reSinonStubMethodReturns.ReplaceAllString(result, "jest.spyOn($1, $2).mockReturnValue($3)")
	result = reSinonStubMethodResolves.ReplaceAllString(result, "jest.spyOn($1, $2).mockResolvedValue($3)")
	result = reSinonStubMethodRejects.ReplaceAllString(result, "jest.spyOn($1, $2).mockRejectedValue($3)")
	result = reSinonSpy.ReplaceAllString(result, "jest.spyOn($1, $2)")
	result = reSinonStubCallsFake.ReplaceAllString(result, "jest.fn().mockImplementation($1)")
	result = reSinonStubReturns.ReplaceAllString(result, "jest.fn().mockReturnValue($1)")
	result = reSinonStubResolves.ReplaceAllString(result, "jest.fn().mockResolvedValue($1)")
	result = reSinonStubRejects.ReplaceAllString(result, "jest.fn().mockRejectedValue($1)")
	result = reSinonStub.ReplaceAllString(result, "jest.fn()")
	result = reSinonAssertCalledWith.ReplaceAllString(result, "expect($1).toHaveBeenCalledWith($2)")
	result = reSinonAssertCalledOnce.ReplaceAllString(result, "expect($1).toHaveBeenCalledTimes(1)")
	result = reSinonAssertCalled.ReplaceAllString(result, "expect($1).toHaveBeenCalled()")
	result = reSinonUseFakeTimers.ReplaceAllString(result, "jest.useFakeTimers()")
	result = reSinonReset.ReplaceAllString(result, "jest.clearAllMocks()")
	result = reSinonRestore.ReplaceAllString(result, "jest.restoreAllMocks()")
	result = reMochaClockTick.ReplaceAllString(result, "jest.advanceTimersByTime($1)")
	result = reMochaClockRestore.ReplaceAllString(result, "jest.useRealTimers()")

	result = reMochaExpectDeepEqual.ReplaceAllString(result, "expect($1).toEqual($2)")
	result = reMochaExpectLength.ReplaceAllString(result, "expect($1).toHaveLength($2)")
	result = reMochaExpectTrue.ReplaceAllString(result, "expect($1).toBe(true)")
	result = reMochaExpectFalse.ReplaceAllString(result, "expect($1).toBe(false)")
	result = reMochaExpectContain.ReplaceAllString(result, "expect($1).toContain($2)")
	result = reMochaExpectEqual.ReplaceAllString(result, "expect($1).toBe($2)")

	result = commentMatchedLines(result, func(line string) bool {
		return strings.Contains(line, "chai.expect(") || strings.Contains(line, "assert.")
	}, "manual Mocha assertion conversion required")
	result = collapseBlankLines(result)
	return ensureTrailingNewline(result), nil
}
