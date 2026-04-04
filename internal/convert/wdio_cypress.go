package convert

import (
	"regexp"
	"strings"
)

var (
	reWdioImportGlobals = regexp.MustCompile(`(?m)^import\s+\{[^}]*\}\s+from\s+['"]@wdio\/globals['"];\s*\n?`)
	reWdioImportPlain   = regexp.MustCompile(`(?m)^import\s+\{[^}]*\}\s+from\s+['"]webdriverio['"];\s*\n?`)

	reWdioToCyExpectURL         = regexp.MustCompile(`await expect\(browser\)\.toHaveUrl\(([^)]+)\)`)
	reWdioToCyExpectURLContains = regexp.MustCompile(`await expect\(browser\)\.toHaveUrlContaining\(([^)]+)\)`)
	reWdioToCyExpectTitle       = regexp.MustCompile(`await expect\(browser\)\.toHaveTitle\(([^)]+)\)`)
	reWdioToCyDisplayed         = regexp.MustCompile(`await expect\(\$\(([^)]+)\)\)\.toBeDisplayed\(\)`)
	reWdioToCyNotDisplayed      = regexp.MustCompile(`await expect\(\$\(([^)]+)\)\)\.not\.toBeDisplayed\(\)`)
	reWdioToCyExist             = regexp.MustCompile(`await expect\(\$\(([^)]+)\)\)\.toExist\(\)`)
	reWdioToCyNotExist          = regexp.MustCompile(`await expect\(\$\(([^)]+)\)\)\.not\.toExist\(\)`)
	reWdioToCyText              = regexp.MustCompile(`await expect\(\$\(([^)]+)\)\)\.toHaveText\(([^)]+)\)`)
	reWdioToCyContainText       = regexp.MustCompile(`await expect\(\$\(([^)]+)\)\)\.toHaveTextContaining\(([^)]+)\)`)
	reWdioToCyValue             = regexp.MustCompile(`await expect\(\$\(([^)]+)\)\)\.toHaveValue\(([^)]+)\)`)
	reWdioToCyCount             = regexp.MustCompile(`await expect\(\$\$\(([^)]+)\)\)\.toBeElementsArrayOfSize\(([^)]+)\)`)
	reWdioToCySelected          = regexp.MustCompile(`await expect\(\$\(([^)]+)\)\)\.toBeSelected\(\)`)
	reWdioToCyEnabled           = regexp.MustCompile(`await expect\(\$\(([^)]+)\)\)\.toBeEnabled\(\)`)
	reWdioToCyDisabled          = regexp.MustCompile(`await expect\(\$\(([^)]+)\)\)\.toBeDisabled\(\)`)
	reWdioToCyAttribute         = regexp.MustCompile(`await expect\(\$\(([^)]+)\)\)\.toHaveAttribute\(([^,]+),\s*([^)]+)\)`)

	reWdioToCyExactClickS   = regexp.MustCompile(`await \$\('=([^'\n]+)'\)\.click\(\)`)
	reWdioToCyExactClickD   = regexp.MustCompile(`await \$\("=([^"\n]+)"\)\.click\(\)`)
	reWdioToCyPartialClickS = regexp.MustCompile(`await \$\('\*=([^'\n]+)'\)\.click\(\)`)
	reWdioToCyPartialClickD = regexp.MustCompile(`await \$\("\*=([^"\n]+)"\)\.click\(\)`)
	reWdioToCyExactSelS     = regexp.MustCompile(`\$\('=([^'\n]+)'\)`)
	reWdioToCyExactSelD     = regexp.MustCompile(`\$\("=([^"\n]+)"\)`)
	reWdioToCyPartialSelS   = regexp.MustCompile(`\$\('\*=([^'\n]+)'\)`)
	reWdioToCyPartialSelD   = regexp.MustCompile(`\$\("\*=([^"\n]+)"\)`)

	reWdioToCySetValue         = regexp.MustCompile(`await \$\(([^)]+)\)\.setValue\(([^)]+)\)`)
	reWdioToCyClick            = regexp.MustCompile(`await \$\(([^)]+)\)\.click\(\)`)
	reWdioToCyDoubleClick      = regexp.MustCompile(`await \$\(([^)]+)\)\.doubleClick\(\)`)
	reWdioToCyClearValue       = regexp.MustCompile(`await \$\(([^)]+)\)\.clearValue\(\)`)
	reWdioToCyMoveTo           = regexp.MustCompile(`await \$\(([^)]+)\)\.moveTo\(\)`)
	reWdioToCyGetText          = regexp.MustCompile(`await \$\(([^)]+)\)\.getText\(\)`)
	reWdioToCyIsDisplayed      = regexp.MustCompile(`await \$\(([^)]+)\)\.isDisplayed\(\)`)
	reWdioToCyIsExisting       = regexp.MustCompile(`await \$\(([^)]+)\)\.isExisting\(\)`)
	reWdioToCyWaitForDisplayed = regexp.MustCompile(`await \$\(([^)]+)\)\.waitForDisplayed\(\)`)
	reWdioToCyWaitForExist     = regexp.MustCompile(`await \$\(([^)]+)\)\.waitForExist\(\)`)
	reWdioToCySelectText       = regexp.MustCompile(`await \$\(([^)]+)\)\.selectByVisibleText\(([^)]+)\)`)
	reWdioToCySelectValue      = regexp.MustCompile(`await \$\(([^)]+)\)\.selectByAttribute\(['"]value['"],\s*([^)]+)\)`)
	reWdioToCyGetAttribute     = regexp.MustCompile(`await \$\(([^)]+)\)\.getAttribute\(([^)]+)\)`)
	reWdioToCyManySelectors    = regexp.MustCompile(`\$\$\(([^)]+)\)`)
	reWdioToCySingleSelectors  = regexp.MustCompile(`\$\(([^)]+)\)`)

	reWdioToCyVisit         = regexp.MustCompile(`await browser\.url\(([^)]+)\)`)
	reWdioToCyWait          = regexp.MustCompile(`await browser\.pause\(([^)]+)\)`)
	reWdioToCyExecStorage   = regexp.MustCompile(`await browser\.execute\(\(\)\s*=>\s*localStorage\.clear\(\)\)`)
	reWdioToCyExec          = regexp.MustCompile(`await browser\.execute\(([^)]*)\)`)
	reWdioToCyReload        = regexp.MustCompile(`await browser\.refresh\(\)`)
	reWdioToCyBack          = regexp.MustCompile(`await browser\.back\(\)`)
	reWdioToCyForward       = regexp.MustCompile(`await browser\.forward\(\)`)
	reWdioToCyGetTitle      = regexp.MustCompile(`await browser\.getTitle\(\)`)
	reWdioToCyGetURL        = regexp.MustCompile(`await browser\.getUrl\(\)`)
	reWdioToCyKeys          = regexp.MustCompile(`await browser\.keys\(\[([^\]]+)\]\)`)
	reWdioToCyDeleteCookies = regexp.MustCompile(`await browser\.deleteCookies\(\)`)
	reWdioToCyGetCookies    = regexp.MustCompile(`await browser\.getCookies\(\)`)
	reWdioToCyConsoleLog    = regexp.MustCompile(`console\.log\(([^)]+)\)`)
	reWdioToCyUnsupported   = regexp.MustCompile(`\b(?:browser\.mock|browser\.setCookies|browser\.executeAsync|browser\.uploadFile|browser\.)|@wdio/|webdriverio\b`)
)

// ConvertWdioToCypressSource rewrites the high-confidence WebdriverIO browser
// surface into Go-native Cypress output. Unsupported constructs are preserved
// as explicit TODO comments for manual follow-up.
func ConvertWdioToCypressSource(source string) (string, error) {
	if strings.TrimSpace(source) == "" {
		return source, nil
	}
	if !strings.Contains(source, "browser.") &&
		!strings.Contains(source, "$(") &&
		!strings.Contains(source, "$$(") &&
		!strings.Contains(source, "@wdio/") &&
		!strings.Contains(source, "webdriverio") {
		return ensureTrailingNewline(strings.ReplaceAll(source, "\r\n", "\n")), nil
	}

	result := strings.ReplaceAll(source, "\r\n", "\n")
	result = reWdioImportGlobals.ReplaceAllString(result, "")
	result = reWdioImportPlain.ReplaceAllString(result, "")

	assertionReplacements := []struct {
		re   *regexp.Regexp
		repl string
	}{
		{reWdioToCyExpectURL, `cy.url().should('eq', $1)`},
		{reWdioToCyExpectURLContains, `cy.url().should('include', $1)`},
		{reWdioToCyExpectTitle, `cy.title().should('eq', $1)`},
		{reWdioToCyDisplayed, `cy.get($1).should('be.visible')`},
		{reWdioToCyNotDisplayed, `cy.get($1).should('not.be.visible')`},
		{reWdioToCyExist, `cy.get($1).should('exist')`},
		{reWdioToCyNotExist, `cy.get($1).should('not.exist')`},
		{reWdioToCyText, `cy.get($1).should('have.text', $2)`},
		{reWdioToCyContainText, `cy.get($1).should('contain', $2)`},
		{reWdioToCyValue, `cy.get($1).should('have.value', $2)`},
		{reWdioToCyCount, `cy.get($1).should('have.length', $2)`},
		{reWdioToCySelected, `cy.get($1).should('be.checked')`},
		{reWdioToCyEnabled, `cy.get($1).should('be.enabled')`},
		{reWdioToCyDisabled, `cy.get($1).should('be.disabled')`},
		{reWdioToCyAttribute, `cy.get($1).should('have.attr', $2, $3)`},
	}
	for _, replacement := range assertionReplacements {
		result = replacement.re.ReplaceAllString(result, replacement.repl)
	}

	result = reWdioToCyExactClickS.ReplaceAllString(result, "cy.contains('$1').click()")
	result = reWdioToCyExactClickD.ReplaceAllString(result, "cy.contains('$1').click()")
	result = reWdioToCyPartialClickS.ReplaceAllString(result, "cy.contains('$1').click()")
	result = reWdioToCyPartialClickD.ReplaceAllString(result, "cy.contains('$1').click()")

	actionReplacements := []struct {
		re   *regexp.Regexp
		repl string
	}{
		{reWdioToCySetValue, `cy.get($1).clear().type($2)`},
		{reWdioToCyClick, `cy.get($1).click()`},
		{reWdioToCyDoubleClick, `cy.get($1).dblclick()`},
		{reWdioToCyClearValue, `cy.get($1).clear()`},
		{reWdioToCyMoveTo, `cy.get($1).trigger('mouseover')`},
		{reWdioToCyGetText, `cy.get($1).invoke('text')`},
		{reWdioToCyIsDisplayed, `cy.get($1).should('be.visible')`},
		{reWdioToCyIsExisting, `cy.get($1).should('exist')`},
		{reWdioToCyWaitForDisplayed, `cy.get($1).should('be.visible')`},
		{reWdioToCyWaitForExist, `cy.get($1).should('exist')`},
		{reWdioToCySelectText, `cy.get($1).select($2)`},
		{reWdioToCySelectValue, `cy.get($1).select($2)`},
		{reWdioToCyGetAttribute, `cy.get($1).invoke('attr', $2)`},
		{reWdioToCyVisit, `cy.visit($1)`},
		{reWdioToCyWait, `cy.wait($1)`},
		{reWdioToCyExecStorage, `cy.clearLocalStorage()`},
		{reWdioToCyReload, `cy.reload()`},
		{reWdioToCyBack, `cy.go('back')`},
		{reWdioToCyForward, `cy.go('forward')`},
		{reWdioToCyGetTitle, `cy.title()`},
		{reWdioToCyGetURL, `cy.url()`},
		{reWdioToCyKeys, `cy.get('body').type($1)`},
		{reWdioToCyDeleteCookies, `cy.clearCookies()`},
		{reWdioToCyGetCookies, `cy.getCookies()`},
		{reWdioToCyConsoleLog, `cy.log($1)`},
	}
	for _, replacement := range actionReplacements {
		result = replacement.re.ReplaceAllString(result, replacement.repl)
	}

	result = reWdioToCyExec.ReplaceAllString(result, `cy.window().then($1)`)
	result = reWdioToCyExactSelS.ReplaceAllString(result, "cy.contains('$1')")
	result = reWdioToCyExactSelD.ReplaceAllString(result, "cy.contains('$1')")
	result = reWdioToCyPartialSelS.ReplaceAllString(result, "cy.contains('$1')")
	result = reWdioToCyPartialSelD.ReplaceAllString(result, "cy.contains('$1')")
	result = reWdioToCyManySelectors.ReplaceAllString(result, `cy.get($1)`)
	result = reWdioToCySingleSelectors.ReplaceAllString(result, `cy.get($1)`)

	result = commentUnsupportedWdioCypressLines(result)
	result = cleanupConvertedCypressOutput(result)
	return ensureTrailingNewline(result), nil
}

func commentUnsupportedWdioCypressLines(source string) string {
	lines := strings.Split(source, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "///") {
			continue
		}
		if reWdioToCyUnsupported.MatchString(line) {
			indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
			lines[i] = indent + "// TERRAIN-TODO: manual WebdriverIO conversion required\n" + indent + "// " + strings.TrimSpace(line)
		}
	}
	return strings.Join(lines, "\n")
}
