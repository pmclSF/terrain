package convert

import (
	"regexp"
	"strings"
)

var (
	reSelCyCheckboxCheck = regexp.MustCompile(`const\s+checkbox\s*=\s*await\s+driver\.findElement\(\s*By\.css\(([^)]+)\)\s*\)\s*;?\s*\n?\s*if\s*\(\s*!\s*\(\s*await\s+checkbox\.isSelected\(\s*\)\s*\)\s*\)\s*await\s+checkbox\.click\(\s*\)`)
	reSelCyCheckboxUnchk = regexp.MustCompile(`const\s+checkbox\s*=\s*await\s+driver\.findElement\(\s*By\.css\(([^)]+)\)\s*\)\s*;?\s*\n?\s*if\s*\(\s*await\s+checkbox\.isSelected\(\s*\)\s*\)\s*await\s+checkbox\.click\(\s*\)`)
	reSelCyUnsupported   = regexp.MustCompile(`\b(?:driver\.wait|driver\.switchTo|driver\.actions|Actions\b|until\.|Key\.|By\.(?:xpath|id|name|linkText|partialLinkText|tagName|className))`)
)

// ConvertSeleniumToCypressSource rewrites the high-confidence Selenium browser
// surface into Go-native Cypress output.
func ConvertSeleniumToCypressSource(source string) (string, error) {
	if strings.TrimSpace(source) == "" {
		return source, nil
	}
	if !strings.Contains(source, "driver.") &&
		!strings.Contains(source, "selenium-webdriver") &&
		!strings.Contains(source, "By.") {
		return ensureTrailingNewline(strings.ReplaceAll(source, "\r\n", "\n")), nil
	}

	result := strings.ReplaceAll(source, "\r\n", "\n")
	result = reSelRequireImport.ReplaceAllString(result, "")
	result = reSelESMImport.ReplaceAllString(result, "")
	result = reSelJestGlobalsImport.ReplaceAllString(result, "")
	result = reSelDriverDeclaration.ReplaceAllString(result, "")
	result = reSelBeforeAllSetup.ReplaceAllString(result, "")
	result = reSelAfterAllTeardown.ReplaceAllString(result, "")

	assertionReplacements := []struct {
		re   *regexp.Regexp
		repl string
	}{
		{reSelExpectVisible, `cy.get($1).should('be.visible')`},
		{reSelExpectHidden, `cy.get($1).should('not.be.visible')`},
		{reSelExpectText, `cy.get($1).should('have.text', $2)`},
		{reSelExpectContainText, `cy.get($1).should('contain', $2)`},
		{reSelExpectValue, `cy.get($1).should('have.value', $2)`},
		{reSelExpectChecked, `cy.get($1).should('be.checked')`},
		{reSelExpectDisabled, `cy.get($1).should('be.disabled')`},
		{reSelExpectEnabled, `cy.get($1).should('be.enabled')`},
		{reSelExpectCountZero, `cy.get($1).should('not.exist')`},
		{reSelExpectCount, `cy.get($1).should('have.length', $2)`},
		{reSelExpectAttached, `cy.get($1).should('exist')`},
		{reSelExpectCurrentURLIn, `cy.url().should('include', $1)`},
		{reSelExpectCurrentURLEq, `cy.url().should('eq', $1)`},
		{reSelExpectTitle, `cy.title().should('eq', $1)`},
	}
	for _, replacement := range assertionReplacements {
		result = replacement.re.ReplaceAllString(result, replacement.repl)
	}

	actionReplacements := []struct {
		re   *regexp.Regexp
		repl string
	}{
		{reSelSendKeys, `cy.get($1).type($2)`},
		{reSelClick, `cy.get($1).click()`},
		{reSelClear, `cy.get($1).clear()`},
		{reSelGoto, `cy.visit($1)`},
		{reSelRefresh, `cy.reload()`},
		{reSelBack, `cy.go('back')`},
		{reSelForward, `cy.go('forward')`},
		{reSelSleep, `cy.wait($1)`},
		{reSelDeleteCookies, `cy.clearCookies()`},
		{reSelLocalStorageClear, `cy.clearLocalStorage()`},
	}
	for _, replacement := range actionReplacements {
		result = replacement.re.ReplaceAllString(result, replacement.repl)
	}

	result = reSelCyCheckboxCheck.ReplaceAllString(result, `cy.get($1).check()`)
	result = reSelCyCheckboxUnchk.ReplaceAllString(result, `cy.get($1).uncheck()`)
	result = reBefore.ReplaceAllString(result, "${1}before(")
	result = reAfter.ReplaceAllString(result, "${1}after(")
	result = commentUnsupportedSeleniumCypressLines(result)
	result = cleanupConvertedCypressOutput(result)
	result = prependCypressReference(result)
	return ensureTrailingNewline(result), nil
}

func commentUnsupportedSeleniumCypressLines(source string) string {
	lines := strings.Split(source, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "///") {
			continue
		}
		if reSelCyUnsupported.MatchString(line) || reSelUnsupportedLine.MatchString(line) {
			indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
			lines[i] = indent + "// TERRAIN-TODO: manual Selenium conversion required\n" + indent + "// " + strings.TrimSpace(line)
		}
	}
	return strings.Join(lines, "\n")
}
