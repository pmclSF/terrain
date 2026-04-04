package convert

import (
	"regexp"
	"strings"
)

var (
	rePlaywrightImportRemove = regexp.MustCompile(`(?m)^import\s+\{[^}]*\}\s+from\s+['"]@playwright/test['"];\s*\n?`)
	reCypressReferenceLine   = regexp.MustCompile(`(?m)^///\s*<reference\s+types="cypress"\s*/>\s*\n?`)

	rePWExpectVisible      = regexp.MustCompile(`await expect\(page\.locator\(([^)]+)\)\)\.toBeVisible\(\)`)
	rePWExpectHidden       = regexp.MustCompile(`await expect\(page\.locator\(([^)]+)\)\)\.toBeHidden\(\)`)
	rePWExpectAttached     = regexp.MustCompile(`await expect\(page\.locator\(([^)]+)\)\)\.toBeAttached\(\)`)
	rePWExpectNotAttached  = regexp.MustCompile(`await expect\(page\.locator\(([^)]+)\)\)\.not\.toBeAttached\(\)`)
	rePWExpectText         = regexp.MustCompile(`await expect\(page\.locator\(([^)]+)\)\)\.toHaveText\(([^)]+)\)`)
	rePWExpectContainText  = regexp.MustCompile(`await expect\(page\.locator\(([^)]+)\)\)\.toContainText\(([^)]+)\)`)
	rePWExpectValue        = regexp.MustCompile(`await expect\(page\.locator\(([^)]+)\)\)\.toHaveValue\(([^)]+)\)`)
	rePWExpectClass        = regexp.MustCompile(`await expect\(page\.locator\(([^)]+)\)\)\.toHaveClass\(([^)]+)\)`)
	rePWExpectChecked      = regexp.MustCompile(`await expect\(page\.locator\(([^)]+)\)\)\.toBeChecked\(\)`)
	rePWExpectDisabled     = regexp.MustCompile(`await expect\(page\.locator\(([^)]+)\)\)\.toBeDisabled\(\)`)
	rePWExpectEnabled      = regexp.MustCompile(`await expect\(page\.locator\(([^)]+)\)\)\.toBeEnabled\(\)`)
	rePWExpectCount        = regexp.MustCompile(`await expect\(page\.locator\(([^)]+)\)\)\.toHaveCount\(([^)]+)\)`)
	rePWExpectPageURLRegex = regexp.MustCompile(`await expect\(page\)\.toHaveURL\((/[^)]+/)\)`)
	rePWExpectPageURL      = regexp.MustCompile(`await expect\(page\)\.toHaveURL\(([^)]+)\)`)
	rePWExpectTitle        = regexp.MustCompile(`await expect\(page\)\.toHaveTitle\(([^)]+)\)`)

	rePWGenericVisible     = regexp.MustCompile(`await expect\((.+?)\)\.toBeVisible\(\)`)
	rePWGenericHidden      = regexp.MustCompile(`await expect\((.+?)\)\.not\.toBeVisible\(\)`)
	rePWGenericAttached    = regexp.MustCompile(`await expect\((.+?)\)\.toBeAttached\(\)`)
	rePWGenericText        = regexp.MustCompile(`await expect\((.+?)\)\.toHaveText\(([^)]+)\)`)
	rePWGenericContainText = regexp.MustCompile(`await expect\((.+?)\)\.toContainText\(([^)]+)\)`)
	rePWGenericCount       = regexp.MustCompile(`await expect\((.+?)\)\.toHaveCount\(([^)]+)\)`)

	rePWLocatorClick   = regexp.MustCompile(`await page\.locator\(([^)]+)\)\.click\(\)`)
	rePWLocatorDouble  = regexp.MustCompile(`await page\.locator\(([^)]+)\)\.dblclick\(\)`)
	rePWLocatorFill    = regexp.MustCompile(`await page\.locator\(([^)]+)\)\.fill\(([^)]+)\)`)
	rePWLocatorClear   = regexp.MustCompile(`await page\.locator\(([^)]+)\)\.clear\(\)`)
	rePWLocatorCheck   = regexp.MustCompile(`await page\.locator\(([^)]+)\)\.check\(\)`)
	rePWLocatorUncheck = regexp.MustCompile(`await page\.locator\(([^)]+)\)\.uncheck\(\)`)
	rePWLocatorSelect  = regexp.MustCompile(`await page\.locator\(([^)]+)\)\.selectOption\(([^)]+)\)`)
	rePWLocatorFocus   = regexp.MustCompile(`await page\.locator\(([^)]+)\)\.focus\(\)`)
	rePWLocatorBlur    = regexp.MustCompile(`await page\.locator\(([^)]+)\)\.blur\(\)`)
	rePWLocatorScroll  = regexp.MustCompile(`await page\.locator\(([^)]+)\)\.scrollIntoViewIfNeeded\(\)`)
	rePWLocatorHover   = regexp.MustCompile(`await page\.locator\(([^)]+)\)\.hover\(\)`)
	rePWGetByTextClick = regexp.MustCompile(`await page\.getByText\(([^)]+)\)\.click\(\)`)
	rePWGetByRoleClick = regexp.MustCompile(`await page\.getByRole\(\s*(['"][^'"]+['"])\s*,\s*\{\s*name:\s*([^}]+)\}\s*\)\.click\(\)`)
	rePWGoto           = regexp.MustCompile(`await page\.goto\(([^)]+)\)`)
	rePWReload         = regexp.MustCompile(`await page\.reload\(\)`)
	rePWGoBack         = regexp.MustCompile(`await page\.goBack\(\)`)
	rePWGoForward      = regexp.MustCompile(`await page\.goForward\(\)`)
	rePWWaitTimeout    = regexp.MustCompile(`await page\.waitForTimeout\((\d+)\)`)
	rePWSetViewport    = regexp.MustCompile(`await page\.setViewportSize\(\{\s*width:\s*(\d+),\s*height:\s*(\d+)\s*\}\)`)
	rePWScreenshotPath = regexp.MustCompile(`await page\.screenshot\(\{\s*path:\s*([^}]+)\s*\}\)`)
	rePWScreenshot     = regexp.MustCompile(`await page\.screenshot\(\)`)

	rePWLocatorStandalone   = regexp.MustCompile(`page\.locator\(([^)]+)\)`)
	rePWGetByTextStandalone = regexp.MustCompile(`page\.getByText\(([^)]+)\)`)
	rePWGetByRoleNamed      = regexp.MustCompile(`page\.getByRole\(\s*(['"][^'"]+['"])\s*,\s*\{\s*name:\s*([^}]+)\}\s*\)`)
	rePWNestedLocator       = regexp.MustCompile(`\.locator\(([^)]+)\)`)
	rePWNth                 = regexp.MustCompile(`\.nth\(([^)]+)\)`)

	rePWDescribeOnly = regexp.MustCompile(`test\.describe\.only\(`)
	rePWDescribeSkip = regexp.MustCompile(`test\.describe\.skip\(`)
	rePWDescribe     = regexp.MustCompile(`test\.describe\(`)
	rePWTestOnly     = regexp.MustCompile(`test\.only\(`)
	rePWTestSkip     = regexp.MustCompile(`test\.skip\(`)
	rePWBeforeAll    = regexp.MustCompile(`test\.beforeAll\(`)
	rePWAfterAll     = regexp.MustCompile(`test\.afterAll\(`)
	rePWBeforeEach   = regexp.MustCompile(`test\.beforeEach\(`)
	rePWAfterEach    = regexp.MustCompile(`test\.afterEach\(`)
	rePWTestCall     = regexp.MustCompile(`\btest\(('[^']*'|"[^"]*"|` + "`[^`]*`" + `)\s*,`)
	rePWCallbackArgs = regexp.MustCompile(`\(\s*\{\s*page\s*(?:,\s*request\s*)?\}\s*\)\s*=>`)

	rePWUnsupportedLine = regexp.MustCompile(`\b(page\.route|page\.waitForRequest|page\.waitForEvent|page\.getByTestId|page\.getByRole\(|request\.|download\.|route\.|context\.)`)
)

// ConvertPlaywrightToCypressSource rewrites the high-confidence Playwright
// browser surface into Go-native Cypress output. Unsupported constructs are
// preserved as explicit TODO comments for manual follow-up.
func ConvertPlaywrightToCypressSource(source string) (string, error) {
	if strings.TrimSpace(source) == "" {
		return source, nil
	}
	if !strings.Contains(source, "page.") && !strings.Contains(source, "test.") && !strings.Contains(source, "@playwright/test") {
		return ensureTrailingNewline(strings.ReplaceAll(source, "\r\n", "\n")), nil
	}

	result := strings.ReplaceAll(source, "\r\n", "\n")
	result = rePlaywrightImportRemove.ReplaceAllString(result, "")
	result = reCypressReferenceLine.ReplaceAllString(result, "")

	assertionReplacements := []struct {
		re   *regexp.Regexp
		repl string
	}{
		{rePWExpectVisible, `cy.get($1).should('be.visible')`},
		{rePWExpectHidden, `cy.get($1).should('not.be.visible')`},
		{rePWExpectAttached, `cy.get($1).should('exist')`},
		{rePWExpectNotAttached, `cy.get($1).should('not.exist')`},
		{rePWExpectText, `cy.get($1).should('have.text', $2)`},
		{rePWExpectContainText, `cy.get($1).should('contain', $2)`},
		{rePWExpectValue, `cy.get($1).should('have.value', $2)`},
		{rePWExpectClass, `cy.get($1).should('have.class', $2)`},
		{rePWExpectChecked, `cy.get($1).should('be.checked')`},
		{rePWExpectDisabled, `cy.get($1).should('be.disabled')`},
		{rePWExpectEnabled, `cy.get($1).should('be.enabled')`},
		{rePWExpectCount, `cy.get($1).should('have.length', $2)`},
		{rePWExpectPageURLRegex, `cy.url().should('match', $1)`},
		{rePWExpectPageURL, `cy.url().should('include', $1)`},
		{rePWExpectTitle, `cy.title().should('eq', $1)`},
	}
	for _, replacement := range assertionReplacements {
		result = replacement.re.ReplaceAllString(result, replacement.repl)
	}

	genericAssertionReplacements := []struct {
		re   *regexp.Regexp
		repl string
	}{
		{rePWGenericVisible, `$1.should('be.visible')`},
		{rePWGenericHidden, `$1.should('not.be.visible')`},
		{rePWGenericAttached, `$1.should('exist')`},
		{rePWGenericText, `$1.should('have.text', $2)`},
		{rePWGenericContainText, `$1.should('contain', $2)`},
		{rePWGenericCount, `$1.should('have.length', $2)`},
	}
	for _, replacement := range genericAssertionReplacements {
		result = replacement.re.ReplaceAllString(result, replacement.repl)
	}

	actionReplacements := []struct {
		re   *regexp.Regexp
		repl string
	}{
		{rePWLocatorClick, `cy.get($1).click()`},
		{rePWLocatorDouble, `cy.get($1).dblclick()`},
		{rePWLocatorFill, `cy.get($1).type($2)`},
		{rePWLocatorClear, `cy.get($1).clear()`},
		{rePWLocatorCheck, `cy.get($1).check()`},
		{rePWLocatorUncheck, `cy.get($1).uncheck()`},
		{rePWLocatorSelect, `cy.get($1).select($2)`},
		{rePWLocatorFocus, `cy.get($1).focus()`},
		{rePWLocatorBlur, `cy.get($1).blur()`},
		{rePWLocatorScroll, `cy.get($1).scrollIntoView()`},
		{rePWLocatorHover, `cy.get($1).trigger('mouseover')`},
		{rePWGetByTextClick, `cy.contains($1).click()`},
		{rePWGetByRoleClick, `cy.contains('[role=' + $1 + ']', $2).click()`},
		{rePWGoto, `cy.visit($1)`},
		{rePWReload, `cy.reload()`},
		{rePWGoBack, `cy.go('back')`},
		{rePWGoForward, `cy.go('forward')`},
		{rePWWaitTimeout, `cy.wait($1)`},
		{rePWSetViewport, `cy.viewport($1, $2)`},
		{rePWScreenshotPath, `cy.screenshot($1)`},
		{rePWScreenshot, `cy.screenshot()`},
	}
	for _, replacement := range actionReplacements {
		result = replacement.re.ReplaceAllString(result, replacement.repl)
	}

	result = rePWDescribeOnly.ReplaceAllString(result, "describe.only(")
	result = rePWDescribeSkip.ReplaceAllString(result, "describe.skip(")
	result = rePWDescribe.ReplaceAllString(result, "describe(")
	result = rePWTestOnly.ReplaceAllString(result, "it.only(")
	result = rePWTestSkip.ReplaceAllString(result, "it.skip(")
	result = rePWBeforeAll.ReplaceAllString(result, "before(")
	result = rePWAfterAll.ReplaceAllString(result, "after(")
	result = rePWBeforeEach.ReplaceAllString(result, "beforeEach(")
	result = rePWAfterEach.ReplaceAllString(result, "afterEach(")
	result = rePWTestCall.ReplaceAllString(result, "it($1,")
	result = rePWCallbackArgs.ReplaceAllString(result, "() =>")

	result = rePWGetByRoleNamed.ReplaceAllString(result, `cy.contains('[role=' + $1 + ']', $2)`)
	result = rePWGetByTextStandalone.ReplaceAllString(result, `cy.contains($1)`)
	result = rePWLocatorStandalone.ReplaceAllString(result, `cy.get($1)`)
	result = rePWNestedLocator.ReplaceAllString(result, `.find($1)`)
	result = rePWNth.ReplaceAllString(result, `.eq($1)`)

	result = commentUnsupportedPlaywrightLines(result)
	result = cleanupConvertedCypressOutput(result)
	result = prependCypressReference(result)
	return ensureTrailingNewline(result), nil
}

func commentUnsupportedPlaywrightLines(source string) string {
	lines := strings.Split(source, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "///") {
			continue
		}
		if rePWUnsupportedLine.MatchString(line) {
			indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
			lines[i] = indent + "// TERRAIN-TODO: manual Playwright conversion required\n" + indent + "// " + strings.TrimSpace(line)
		}
	}
	return strings.Join(lines, "\n")
}

func prependCypressReference(source string) string {
	header, body := splitLeadingHeader(source)
	body = strings.TrimLeft(body, "\n")
	ref := "/// <reference types=\"cypress\" />"
	if body == "" {
		return header + ref + "\n"
	}
	if header == "" {
		return ref + "\n\n" + body
	}
	return header + ref + "\n\n" + body
}

func cleanupConvertedCypressOutput(source string) string {
	source = strings.ReplaceAll(source, "async () =>", "() =>")
	source = strings.ReplaceAll(source, "async() =>", "() =>")
	for strings.Contains(source, "\n\n\n") {
		source = strings.ReplaceAll(source, "\n\n\n", "\n\n")
	}
	return strings.TrimSpace(source) + "\n"
}
