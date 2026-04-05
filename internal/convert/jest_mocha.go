package convert

import (
	"regexp"
	"strings"
)

var (
	reJestBeforeAll          = regexp.MustCompile(`\bbeforeAll\s*\(`)
	reJestAfterAll           = regexp.MustCompile(`\bafterAll\s*\(`)
	reJestSpyOnReturnValue   = regexp.MustCompile(`jest\.spyOn\(([^,]+),\s*([^)]+)\)\.mockReturnValue\(([^)]+)\)`)
	reJestSpyOnResolvedValue = regexp.MustCompile(`jest\.spyOn\(([^,]+),\s*([^)]+)\)\.mockResolvedValue\(([^)]+)\)`)
	reJestSpyOnRejectedValue = regexp.MustCompile(`jest\.spyOn\(([^,]+),\s*([^)]+)\)\.mockRejectedValue\(([^)]+)\)`)
	reJestSpyOnMockImpl      = regexp.MustCompile(`jest\.spyOn\(([^,]+),\s*([^)]+)\)\.mockImplementation\(([^)]+)\)`)
	reJestSpyOn              = regexp.MustCompile(`jest\.spyOn\(([^,]+),\s*([^)]+)\)`)
	reJestFnReturnValue      = regexp.MustCompile(`jest\.fn\(\)\.mockReturnValue\(([^)]+)\)`)
	reJestFnResolvedValue    = regexp.MustCompile(`jest\.fn\(\)\.mockResolvedValue\(([^)]+)\)`)
	reJestFnRejectedValue    = regexp.MustCompile(`jest\.fn\(\)\.mockRejectedValue\(([^)]+)\)`)
	reJestFnImplementation   = regexp.MustCompile(`jest\.fn\(\)\.mockImplementation\(([^)]+)\)`)
	reJestFnWithImpl         = regexp.MustCompile(`jest\.fn\(([^)]+)\)`)
	reJestFn                 = regexp.MustCompile(`jest\.fn\(\)`)
	reJestCalledWith         = regexp.MustCompile(`expect\(([^)]+)\)\.toHaveBeenCalledWith\((.+)\)`)
	reJestCalledOnce         = regexp.MustCompile(`expect\(([^)]+)\)\.toHaveBeenCalledTimes\(\s*1\s*\)`)
	reJestCalledTimes        = regexp.MustCompile(`expect\(([^)]+)\)\.toHaveBeenCalledTimes\(([^)]+)\)`)
	reJestNotCalled          = regexp.MustCompile(`expect\(([^)]+)\)\.not\.toHaveBeenCalled\(\)`)
	reJestCalled             = regexp.MustCompile(`expect\(([^)]+)\)\.toHaveBeenCalled\(\)`)
	reJestToStrictEqual      = regexp.MustCompile(`expect\(([^)]+)\)\.toStrictEqual\(([^)]+)\)`)
	reJestToEqual            = regexp.MustCompile(`expect\(([^)]+)\)\.toEqual\(([^)]+)\)`)
	reJestToBeTrue           = regexp.MustCompile(`expect\(([^)]+)\)\.toBe\(\s*true\s*\)`)
	reJestToBeFalse          = regexp.MustCompile(`expect\(([^)]+)\)\.toBe\(\s*false\s*\)`)
	reJestToBe               = regexp.MustCompile(`expect\(([^)]+)\)\.toBe\(([^)]+)\)`)
	reJestToHaveLength       = regexp.MustCompile(`expect\(([^)]+)\)\.toHaveLength\(([^)]+)\)`)
	reJestToContain          = regexp.MustCompile(`expect\(([^)]+)\)\.toContain\(([^)]+)\)`)
	reJestUseFakeTimers      = regexp.MustCompile(`jest\.useFakeTimers\(\)`)
	reJestUseRealTimers      = regexp.MustCompile(`jest\.useRealTimers\(\)`)
	reJestAdvanceTimers      = regexp.MustCompile(`jest\.advanceTimersByTime\(([^)]+)\)`)
	reJestClearMocks         = regexp.MustCompile(`jest\.(?:clearAllMocks|resetAllMocks)\(\)`)
	reJestRestoreMocks       = regexp.MustCompile(`jest\.restoreAllMocks\(\)`)
)

// ConvertJestToMochaSource rewrites the high-confidence Jest surface into
// Go-native Mocha/Chai/Sinon output.
func ConvertJestToMochaSource(source string) (string, error) {
	if strings.TrimSpace(source) == "" {
		return source, nil
	}
	if !strings.Contains(source, "jest.") &&
		!strings.Contains(source, "@jest/globals") &&
		!strings.Contains(source, ".toHaveBeenCalled") &&
		!strings.Contains(source, ".toEqual(") &&
		!strings.Contains(source, ".toStrictEqual(") &&
		!strings.Contains(source, ".toBe(") &&
		!strings.Contains(source, ".toHaveLength(") &&
		!strings.Contains(source, ".toContain(") &&
		!strings.Contains(source, "beforeAll(") &&
		!strings.Contains(source, "afterAll(") {
		return ensureTrailingNewline(strings.ReplaceAll(source, "\r\n", "\n")), nil
	}

	useESM := usesESModuleSyntax(source)
	result := strings.ReplaceAll(source, "\r\n", "\n")
	if astResult, ok := convertJestToMochaSourceAST(result); ok {
		result = astResult
		result = prependMochaPrelude(result, useESM, strings.Contains(result, "sinon.") || strings.Contains(result, "clock."))
		return ensureTrailingNewline(result), nil
	}

	result = reJestGlobalsImport.ReplaceAllString(result, "")
	result = reJestGlobalsRequire.ReplaceAllString(result, "")
	result = stripMochaPreludeImports(result)
	result = replaceCodeRegexString(result, reJestBeforeAll, "before(")
	result = replaceCodeRegexString(result, reJestAfterAll, "after(")

	result = replaceCodeRegexString(result, reJestSpyOnReturnValue, "sinon.stub($1, $2).returns($3)")
	result = replaceCodeRegexString(result, reJestSpyOnResolvedValue, "sinon.stub($1, $2).resolves($3)")
	result = replaceCodeRegexString(result, reJestSpyOnRejectedValue, "sinon.stub($1, $2).rejects($3)")
	result = replaceCodeRegexString(result, reJestSpyOnMockImpl, "sinon.stub($1, $2).callsFake($3)")
	result = replaceCodeRegexString(result, reJestSpyOn, "sinon.spy($1, $2)")
	result = replaceCodeRegexString(result, reJestFnReturnValue, "sinon.stub().returns($1)")
	result = replaceCodeRegexString(result, reJestFnResolvedValue, "sinon.stub().resolves($1)")
	result = replaceCodeRegexString(result, reJestFnRejectedValue, "sinon.stub().rejects($1)")
	result = replaceCodeRegexString(result, reJestFnImplementation, "sinon.stub().callsFake($1)")
	result = replaceCodeRegexString(result, reJestFnWithImpl, "sinon.stub().callsFake($1)")
	result = replaceCodeRegexString(result, reJestFn, "sinon.stub()")

	result = replaceCodeRegexString(result, reJestCalledWith, "expect($1).to.have.been.calledWith($2)")
	result = replaceCodeRegexString(result, reJestCalledOnce, "expect($1).to.have.been.calledOnce")
	result = replaceCodeRegexString(result, reJestCalledTimes, "expect($1.callCount).to.equal($2)")
	result = replaceCodeRegexString(result, reJestNotCalled, "expect($1).to.not.have.been.called")
	result = replaceCodeRegexString(result, reJestCalled, "expect($1).to.have.been.called")

	result = replaceCodeRegexString(result, reJestToStrictEqual, "expect($1).to.deep.equal($2)")
	result = replaceCodeRegexString(result, reJestToEqual, "expect($1).to.deep.equal($2)")
	result = replaceCodeRegexString(result, reJestToBeTrue, "expect($1).to.be.true")
	result = replaceCodeRegexString(result, reJestToBeFalse, "expect($1).to.be.false")
	result = replaceCodeRegexString(result, reJestToHaveLength, "expect($1).to.have.lengthOf($2)")
	result = replaceCodeRegexString(result, reJestToContain, "expect($1).to.contain($2)")
	result = replaceCodeRegexString(result, reJestToBe, "expect($1).to.equal($2)")

	result = replaceCodeRegexString(result, reJestUseFakeTimers, "sinon.useFakeTimers()")
	result = replaceCodeRegexString(result, reJestAdvanceTimers, "clock.tick($1)")
	result = replaceCodeRegexString(result, reJestUseRealTimers, "clock.restore()")
	result = replaceCodeRegexString(result, reJestClearMocks, "sinon.reset()")
	result = replaceCodeRegexString(result, reJestRestoreMocks, "sinon.restore()")

	result = commentMatchedLines(result, func(line string) bool {
		return reJestMockLine.MatchString(line)
	}, "manual Jest module mock conversion required")
	result = collapseBlankLines(result)
	result = prependMochaPrelude(result, useESM, strings.Contains(result, "sinon.") || strings.Contains(result, "clock."))
	return ensureTrailingNewline(result), nil
}
