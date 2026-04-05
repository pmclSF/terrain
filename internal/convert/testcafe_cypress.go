package convert

import (
	"regexp"
	"strings"
)

var (
	reTcCySelectorAssign     = regexp.MustCompile(`(?m)^(\s*)(const|let|var)\s+([A-Za-z_$][\w$]*)\s*=\s*Selector\(([^)]+)\)\s*;?\s*$`)
	reTcCySelectorAssignNth  = regexp.MustCompile(`(?m)^(\s*)(const|let|var)\s+([A-Za-z_$][\w$]*)\s*=\s*Selector\(([^)]+)\)\.nth\(([^)]+)\)\s*;?\s*$`)
	reTcCySelectorAssignFind = regexp.MustCompile(`(?m)^(\s*)(const|let|var)\s+([A-Za-z_$][\w$]*)\s*=\s*Selector\(([^)]+)\)\.find\(([^)]+)\)\s*;?\s*$`)
	reTcCySelectorAssignText = regexp.MustCompile(`(?m)^(\s*)(const|let|var)\s+([A-Za-z_$][\w$]*)\s*=\s*Selector\(([^)]+)\)\.withText\(([^)]+)\)\s*;?\s*$`)
	reTcCyTestCallback       = regexp.MustCompile(`\btest\(([^,]+),\s*async\s+t\s*=>\s*\{`)
	reTcCyExpectExistsOk     = regexp.MustCompile(`await\s+t\.expect\(Selector\(([^)]+)\)\.exists\)\.ok\(\)`)
	reTcCyExpectExistsNotOk  = regexp.MustCompile(`await\s+t\.expect\(Selector\(([^)]+)\)\.exists\)\.notOk\(\)`)
	reTcCyExpectVisibleOk    = regexp.MustCompile(`await\s+t\.expect\(Selector\(([^)]+)\)\.visible\)\.ok\(\)`)
	reTcCyExpectVisibleNotOk = regexp.MustCompile(`await\s+t\.expect\(Selector\(([^)]+)\)\.visible\)\.notOk\(\)`)
	reTcCyExpectCountEq      = regexp.MustCompile(`await\s+t\.expect\(Selector\(([^)]+)\)\.count\)\.eql\(([^)]+)\)`)
	reTcCyExpectTextEq       = regexp.MustCompile(`await\s+t\.expect\(Selector\(([^)]+)\)\.innerText\)\.eql\(([^)]+)\)`)
	reTcCyExpectTextIn       = regexp.MustCompile(`await\s+t\.expect\(Selector\(([^)]+)\)\.innerText\)\.contains\(([^)]+)\)`)
	reTcCyExpectValueEq      = regexp.MustCompile(`await\s+t\.expect\(Selector\(([^)]+)\)\.value\)\.eql\(([^)]+)\)`)
	reTcCyClickSelectorText  = regexp.MustCompile(`await\s+t\.click\(Selector\(([^)]+)\)\.withText\(([^)]+)\)\)`)
	reTcCyClickSelectorNth   = regexp.MustCompile(`await\s+t\.click\(Selector\(([^)]+)\)\.nth\(([^)]+)\)\)`)
	reTcCyClickSelectorFind  = regexp.MustCompile(`await\s+t\.click\(Selector\(([^)]+)\)\.find\(([^)]+)\)\)`)
	reTcCyClickSelector      = regexp.MustCompile(`await\s+t\.click\(Selector\(([^)]+)\)\)`)
	reTcCyTypeSelector       = regexp.MustCompile(`await\s+t\.typeText\(Selector\(([^)]+)\),\s*([^)]+)\)`)
	reTcCyClick              = regexp.MustCompile(`await\s+t\.click\(([^)]+)\)`)
	reTcCyTypeText           = regexp.MustCompile(`await\s+t\.typeText\(([^,]+),\s*([^)]+)\)`)
	reTcCyDoubleClick        = regexp.MustCompile(`await\s+t\.doubleClick\(([^)]+)\)`)
	reTcCyHover              = regexp.MustCompile(`await\s+t\.hover\(([^)]+)\)`)
	reTcCyNavigate           = regexp.MustCompile(`await\s+t\.navigateTo\(([^)]+)\)`)
	reTcCyWait               = regexp.MustCompile(`await\s+t\.wait\(([^)]+)\)`)
	reTcCyScreenshot         = regexp.MustCompile(`await\s+t\.takeScreenshot\(\)`)
	reTcCySelectorWithText   = regexp.MustCompile(`Selector\(([^)]+)\)\.withText\(([^)]+)\)`)
	reTcCySelectorFind       = regexp.MustCompile(`Selector\(([^)]+)\)\.find\(([^)]+)\)`)
	reTcCySelectorNth        = regexp.MustCompile(`Selector\(([^)]+)\)\.nth\(([^)]+)\)`)
	reTcCySelectorStandalone = regexp.MustCompile(`Selector\(([^)]+)\)`)
)

// ConvertTestCafeToCypressSource rewrites the high-confidence TestCafe browser
// surface into Go-native Cypress output.
func ConvertTestCafeToCypressSource(source string) (string, error) {
	if strings.TrimSpace(source) == "" {
		return source, nil
	}
	if !strings.Contains(source, "testcafe") &&
		!strings.Contains(source, "fixture") &&
		!strings.Contains(source, "await t.") &&
		!strings.Contains(source, "Selector(") {
		return ensureTrailingNewline(strings.ReplaceAll(source, "\r\n", "\n")), nil
	}

	result := strings.ReplaceAll(source, "\r\n", "\n")
	suiteName, pageURL := "", ""
	astApplied := false
	if astResult, astSuiteName, astPageURL, ok := convertTestCafeToCypressSourceAST(result); ok {
		result = astResult
		suiteName = astSuiteName
		pageURL = astPageURL
		astApplied = true
	}

	if !astApplied {
		result = reTcImport.ReplaceAllString(result, "")
		result, suiteName, pageURL = extractTestCafeFixture(result)
		result = reTcCySelectorAssignText.ReplaceAllString(result, `${1}${2} ${3} = ${4};`)
		result = reTcCySelectorAssignFind.ReplaceAllString(result, `${1}${2} ${3} = ${4};`)
		result = reTcCySelectorAssignNth.ReplaceAllString(result, `${1}${2} ${3} = ${4};`)
		result = reTcCySelectorAssign.ReplaceAllString(result, `${1}${2} ${3} = ${4};`)

		assertionReplacements := []struct {
			re   *regexp.Regexp
			repl string
		}{
			{reTcCyExpectExistsOk, `cy.get($1).should('exist')`},
			{reTcCyExpectExistsNotOk, `cy.get($1).should('not.exist')`},
			{reTcCyExpectVisibleOk, `cy.get($1).should('be.visible')`},
			{reTcCyExpectVisibleNotOk, `cy.get($1).should('not.be.visible')`},
			{reTcCyExpectCountEq, `cy.get($1).should('have.length', $2)`},
			{reTcCyExpectTextEq, `cy.get($1).should('have.text', $2)`},
			{reTcCyExpectTextIn, `cy.get($1).should('contain', $2)`},
			{reTcCyExpectValueEq, `cy.get($1).should('have.value', $2)`},
		}
		for _, replacement := range assertionReplacements {
			result = replacement.re.ReplaceAllString(result, replacement.repl)
		}

		actionReplacements := []struct {
			re   *regexp.Regexp
			repl string
		}{
			{reTcCyClickSelectorText, `cy.contains($1, $2).click()`},
			{reTcCyClickSelectorNth, `cy.get($1).eq($2).click()`},
			{reTcCyClickSelectorFind, `cy.get($1).find($2).click()`},
			{reTcCyClickSelector, `cy.get($1).click()`},
			{reTcCyTypeSelector, `cy.get($1).type($2)`},
			{reTcCyDoubleClick, `cy.get($1).dblclick()`},
			{reTcCyHover, `cy.get($1).trigger('mouseover')`},
			{reTcCyNavigate, `cy.visit($1)`},
			{reTcCyWait, `cy.wait($1)`},
			{reTcCyScreenshot, `cy.screenshot()`},
			{reTcCyClick, `cy.get($1).click()`},
			{reTcCyTypeText, `cy.get($1).type($2)`},
		}
		for _, replacement := range actionReplacements {
			result = replacement.re.ReplaceAllString(result, replacement.repl)
		}

		result = reTcCySelectorWithText.ReplaceAllString(result, `cy.contains($1, $2)`)
		result = reTcCySelectorFind.ReplaceAllString(result, `cy.get($1).find($2)`)
		result = reTcCySelectorNth.ReplaceAllString(result, `cy.get($1).eq($2)`)
		result = reTcCySelectorStandalone.ReplaceAllString(result, `cy.get($1)`)
		result = reTcCyTestCallback.ReplaceAllString(result, `it($1, () => {`)

		result = commentUnsupportedTestCafeCypressLines(result)
	}
	result = cleanupConvertedCypressOutput(result)
	result = wrapTestCafeCypressSuite(result, suiteName, pageURL)
	return ensureTrailingNewline(result), nil
}

func wrapTestCafeCypressSuite(body, suiteName, pageURL string) string {
	body = strings.TrimSpace(body)
	if suiteName == "" {
		return prependCypressReference(body)
	}

	lines := []string{`describe(` + quoteSingle(suiteName) + `, () => {`}
	if pageURL != "" {
		lines = append(lines,
			"  beforeEach(() => {",
			"    cy.visit("+quoteSingle(pageURL)+")",
			"  });",
			"",
		)
	}
	if body != "" {
		lines = append(lines, body)
	}
	lines = append(lines, "});")
	return prependCypressReference(strings.Join(lines, "\n"))
}

func commentUnsupportedTestCafeCypressLines(source string) string {
	lines := strings.Split(source, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "///") {
			continue
		}
		if reTcUnsupportedLine.MatchString(line) || strings.Contains(line, "await t.") || strings.Contains(line, "Selector(") {
			indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
			lines[i] = indent + "// TERRAIN-TODO: manual TestCafe conversion required\n" + indent + "// " + strings.TrimSpace(line)
		}
	}
	return strings.Join(lines, "\n")
}
