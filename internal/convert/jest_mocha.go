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
	result = reJestBeforeAll.ReplaceAllString(result, "before(")
	result = reJestAfterAll.ReplaceAllString(result, "after(")

	result = reJestSpyOnReturnValue.ReplaceAllString(result, "sinon.stub($1, $2).returns($3)")
	result = reJestSpyOnResolvedValue.ReplaceAllString(result, "sinon.stub($1, $2).resolves($3)")
	result = reJestSpyOnRejectedValue.ReplaceAllString(result, "sinon.stub($1, $2).rejects($3)")
	result = reJestSpyOnMockImpl.ReplaceAllString(result, "sinon.stub($1, $2).callsFake($3)")
	result = reJestSpyOn.ReplaceAllString(result, "sinon.spy($1, $2)")
	result = reJestFnReturnValue.ReplaceAllString(result, "sinon.stub().returns($1)")
	result = reJestFnResolvedValue.ReplaceAllString(result, "sinon.stub().resolves($1)")
	result = reJestFnRejectedValue.ReplaceAllString(result, "sinon.stub().rejects($1)")
	result = reJestFnImplementation.ReplaceAllString(result, "sinon.stub().callsFake($1)")
	result = reJestFnWithImpl.ReplaceAllString(result, "sinon.stub().callsFake($1)")
	result = reJestFn.ReplaceAllString(result, "sinon.stub()")

	result = reJestCalledWith.ReplaceAllString(result, "expect($1).to.have.been.calledWith($2)")
	result = reJestCalledOnce.ReplaceAllString(result, "expect($1).to.have.been.calledOnce")
	result = reJestCalledTimes.ReplaceAllString(result, "expect($1.callCount).to.equal($2)")
	result = reJestNotCalled.ReplaceAllString(result, "expect($1).to.not.have.been.called")
	result = reJestCalled.ReplaceAllString(result, "expect($1).to.have.been.called")

	result = reJestToStrictEqual.ReplaceAllString(result, "expect($1).to.deep.equal($2)")
	result = reJestToEqual.ReplaceAllString(result, "expect($1).to.deep.equal($2)")
	result = reJestToBeTrue.ReplaceAllString(result, "expect($1).to.be.true")
	result = reJestToBeFalse.ReplaceAllString(result, "expect($1).to.be.false")
	result = reJestToHaveLength.ReplaceAllString(result, "expect($1).to.have.lengthOf($2)")
	result = reJestToContain.ReplaceAllString(result, "expect($1).to.contain($2)")
	result = reJestToBe.ReplaceAllString(result, "expect($1).to.equal($2)")

	result = reJestUseFakeTimers.ReplaceAllString(result, "sinon.useFakeTimers()")
	result = reJestAdvanceTimers.ReplaceAllString(result, "clock.tick($1)")
	result = reJestUseRealTimers.ReplaceAllString(result, "clock.restore()")
	result = reJestClearMocks.ReplaceAllString(result, "sinon.reset()")
	result = reJestRestoreMocks.ReplaceAllString(result, "sinon.restore()")

	result = commentMatchedLines(result, func(line string) bool {
		return reJestMockLine.MatchString(line)
	}, "manual Jest module mock conversion required")
	result = collapseBlankLines(result)
	result = prependMochaPrelude(result, useESM, strings.Contains(result, "sinon.") || strings.Contains(result, "clock."))
	return ensureTrailingNewline(result), nil
}
