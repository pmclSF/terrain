package convert

import (
	"regexp"
	"strings"
)

var (
	rePWToWdioExpectURL          = regexp.MustCompile(`await expect\(page\)\.toHaveURL\(([^)]+)\)`)
	rePWToWdioExpectTitle        = regexp.MustCompile(`await expect\(page\)\.toHaveTitle\(([^)]+)\)`)
	rePWToWdioExpectVisible      = regexp.MustCompile(`await expect\(page\.locator\(([^)]+)\)\)\.toBeVisible\(\)`)
	rePWToWdioExpectHidden       = regexp.MustCompile(`await expect\(page\.locator\(([^)]+)\)\)\.toBeHidden\(\)`)
	rePWToWdioExpectAttached     = regexp.MustCompile(`await expect\(page\.locator\(([^)]+)\)\)\.toBeAttached\(\)`)
	rePWToWdioExpectNotAttached  = regexp.MustCompile(`await expect\(page\.locator\(([^)]+)\)\)\.not\.toBeAttached\(\)`)
	rePWToWdioExpectText         = regexp.MustCompile(`await expect\(page\.locator\(([^)]+)\)\)\.toHaveText\(([^)]+)\)`)
	rePWToWdioExpectContainText  = regexp.MustCompile(`await expect\(page\.locator\(([^)]+)\)\)\.toContainText\(([^)]+)\)`)
	rePWToWdioExpectValue        = regexp.MustCompile(`await expect\(page\.locator\(([^)]+)\)\)\.toHaveValue\(([^)]+)\)`)
	rePWToWdioExpectCount        = regexp.MustCompile(`await expect\(page\.locator\(([^)]+)\)\)\.toHaveCount\(([^)]+)\)`)
	rePWToWdioExpectChecked      = regexp.MustCompile(`await expect\(page\.locator\(([^)]+)\)\)\.toBeChecked\(\)`)
	rePWToWdioExpectEnabled      = regexp.MustCompile(`await expect\(page\.locator\(([^)]+)\)\)\.toBeEnabled\(\)`)
	rePWToWdioExpectDisabled     = regexp.MustCompile(`await expect\(page\.locator\(([^)]+)\)\)\.toBeDisabled\(\)`)
	rePWToWdioExpectAttribute    = regexp.MustCompile(`await expect\(page\.locator\(([^)]+)\)\)\.toHaveAttribute\(([^,]+),\s*([^)]+)\)`)
	rePWToWdioLocatorFill        = regexp.MustCompile(`await page\.locator\(([^)]+)\)\.fill\(([^)]+)\)`)
	rePWToWdioLocatorClick       = regexp.MustCompile(`await page\.locator\(([^)]+)\)\.click\(\)`)
	rePWToWdioLocatorDoubleClick = regexp.MustCompile(`await page\.locator\(([^)]+)\)\.dblclick\(\)`)
	rePWToWdioLocatorHover       = regexp.MustCompile(`await page\.locator\(([^)]+)\)\.hover\(\)`)
	rePWToWdioLocatorText        = regexp.MustCompile(`await page\.locator\(([^)]+)\)\.textContent\(\)`)
	rePWToWdioLocatorVisible     = regexp.MustCompile(`await page\.locator\(([^)]+)\)\.isVisible\(\)`)
	rePWToWdioLocatorWait        = regexp.MustCompile(`await page\.locator\(([^)]+)\)\.waitFor\(\)`)
	rePWToWdioLocatorWaitVisible = regexp.MustCompile(`await page\.locator\(([^)]+)\)\.waitFor\(\{\s*state:\s*['"]visible['"]\s*\}\)`)
	rePWToWdioLocatorClear       = regexp.MustCompile(`await page\.locator\(([^)]+)\)\.clear\(\)`)
	rePWToWdioSelectByLabel      = regexp.MustCompile(`await page\.locator\(([^)]+)\)\.selectOption\(\{\s*label:\s*([^}]+)\}\)`)
	rePWToWdioSelectByValue      = regexp.MustCompile(`await page\.locator\(([^)]+)\)\.selectOption\(([^)]+)\)`)
	rePWToWdioLocatorCheck       = regexp.MustCompile(`await page\.locator\(([^)]+)\)\.check\(\)`)
	rePWToWdioLocatorUncheck     = regexp.MustCompile(`await page\.locator\(([^)]+)\)\.uncheck\(\)`)
	rePWToWdioLocatorStandalone  = regexp.MustCompile(`page\.locator\(([^)]+)\)`)
	rePWToWdioGetByTextClickS    = regexp.MustCompile(`await page\.getByText\('([^'\n]+)'\)\.click\(\)`)
	rePWToWdioGetByTextClickD    = regexp.MustCompile(`await page\.getByText\("([^"\n]+)"\)\.click\(\)`)
	rePWToWdioGetByTextS         = regexp.MustCompile(`page\.getByText\('([^'\n]+)'\)`)
	rePWToWdioGetByTextD         = regexp.MustCompile(`page\.getByText\("([^"\n]+)"\)`)
	rePWToWdioGoto               = regexp.MustCompile(`await page\.goto\(([^)]+)\)`)
	rePWToWdioWaitTimeout        = regexp.MustCompile(`await page\.waitForTimeout\(([^)]+)\)`)
	rePWToWdioEvaluate           = regexp.MustCompile(`await page\.evaluate\(`)
	rePWToWdioTitle              = regexp.MustCompile(`await page\.title\(\)`)
	rePWToWdioURL                = regexp.MustCompile(`await page\.url\(\)`)
	rePWToWdioReload             = regexp.MustCompile(`await page\.reload\(\)`)
	rePWToWdioBack               = regexp.MustCompile(`await page\.goBack\(\)`)
	rePWToWdioForward            = regexp.MustCompile(`await page\.goForward\(\)`)
	rePWToWdioKeyboard           = regexp.MustCompile(`await page\.keyboard\.press\(([^)]+)\)`)
	rePWToWdioContextAddCookies  = regexp.MustCompile(`await (?:page\.context\(\)|context)\.addCookies\(`)
	rePWToWdioContextCookies     = regexp.MustCompile(`await (?:page\.context\(\)|context)\.cookies\(\)`)
	rePWToWdioContextClear       = regexp.MustCompile(`await (?:page\.context\(\)|context)\.clearCookies\(\)`)
	rePWToWdioUnsupportedLine    = regexp.MustCompile(`\b(?:page\.route|page\.waitForRequest|page\.waitForEvent|page\.getByRole|page\.getByTestId|page\.setViewportSize|request\.|download\.|context\.)|page\.context\(`)
)

// ConvertPlaywrightToWdioSource rewrites the high-confidence Playwright
// browser surface into Go-native WebdriverIO output. Unsupported constructs are
// preserved as explicit TODO comments for manual follow-up.
func ConvertPlaywrightToWdioSource(source string) (string, error) {
	if strings.TrimSpace(source) == "" {
		return source, nil
	}
	if !strings.Contains(source, "page.") &&
		!strings.Contains(source, "test.") &&
		!strings.Contains(source, "@playwright/test") {
		return ensureTrailingNewline(strings.ReplaceAll(source, "\r\n", "\n")), nil
	}

	result := strings.ReplaceAll(source, "\r\n", "\n")
	result = rePlaywrightImportRemove.ReplaceAllString(result, "")
	astApplied := false
	if astResult, ok := convertPlaywrightToWdioSourceAST(result); ok {
		result = astResult
		astApplied = true
	}

	if !astApplied {
		assertionReplacements := []struct {
			re   *regexp.Regexp
			repl string
		}{
			{rePWToWdioExpectURL, `await expect(browser).toHaveUrl($1)`},
			{rePWToWdioExpectTitle, `await expect(browser).toHaveTitle($1)`},
			{rePWToWdioExpectVisible, `await expect($$($1)).toBeDisplayed()`},
			{rePWToWdioExpectHidden, `await expect($$($1)).not.toBeDisplayed()`},
			{rePWToWdioExpectAttached, `await expect($$($1)).toExist()`},
			{rePWToWdioExpectNotAttached, `await expect($$($1)).not.toExist()`},
			{rePWToWdioExpectText, `await expect($$($1)).toHaveText($2)`},
			{rePWToWdioExpectContainText, `await expect($$($1)).toHaveTextContaining($2)`},
			{rePWToWdioExpectValue, `await expect($$($1)).toHaveValue($2)`},
			{rePWToWdioExpectCount, `await expect($$$$($1)).toBeElementsArrayOfSize($2)`},
			{rePWToWdioExpectChecked, `await expect($$($1)).toBeSelected()`},
			{rePWToWdioExpectEnabled, `await expect($$($1)).toBeEnabled()`},
			{rePWToWdioExpectDisabled, `await expect($$($1)).toBeDisabled()`},
			{rePWToWdioExpectAttribute, `await expect($$($1)).toHaveAttribute($2, $3)`},
		}
		for _, replacement := range assertionReplacements {
			result = replacement.re.ReplaceAllString(result, replacement.repl)
		}

		actionReplacements := []struct {
			re   *regexp.Regexp
			repl string
		}{
			{rePWToWdioLocatorFill, `await $$($1).setValue($2)`},
			{rePWToWdioLocatorClick, `await $$($1).click()`},
			{rePWToWdioLocatorDoubleClick, `await $$($1).doubleClick()`},
			{rePWToWdioLocatorHover, `await $$($1).moveTo()`},
			{rePWToWdioLocatorText, `await $$($1).getText()`},
			{rePWToWdioLocatorVisible, `await $$($1).isDisplayed()`},
			{rePWToWdioLocatorWaitVisible, `await $$($1).waitForDisplayed()`},
			{rePWToWdioLocatorWait, `await $$($1).waitForDisplayed()`},
			{rePWToWdioLocatorClear, `await $$($1).clearValue()`},
			{rePWToWdioSelectByLabel, `await $$($1).selectByVisibleText($2)`},
			{rePWToWdioSelectByValue, `await $$($1).selectByAttribute('value', $2)`},
			{rePWToWdioLocatorCheck, `await $$($1).click()`},
			{rePWToWdioLocatorUncheck, `await $$($1).click()`},
			{rePWToWdioGoto, `await browser.url($1)`},
			{rePWToWdioWaitTimeout, `await browser.pause($1)`},
			{rePWToWdioTitle, `await browser.getTitle()`},
			{rePWToWdioURL, `await browser.getUrl()`},
			{rePWToWdioReload, `await browser.refresh()`},
			{rePWToWdioBack, `await browser.back()`},
			{rePWToWdioForward, `await browser.forward()`},
			{rePWToWdioKeyboard, `await browser.keys([$1])`},
			{rePWToWdioContextAddCookies, `await browser.setCookies(`},
			{rePWToWdioContextCookies, `await browser.getCookies()`},
			{rePWToWdioContextClear, `await browser.deleteCookies()`},
		}
		for _, replacement := range actionReplacements {
			result = replacement.re.ReplaceAllString(result, replacement.repl)
		}

		result = rePWToWdioEvaluate.ReplaceAllString(result, `await browser.execute(`)
		result = rePWToWdioGetByTextClickS.ReplaceAllString(result, "await $(`*=$1`).click()")
		result = rePWToWdioGetByTextClickD.ReplaceAllString(result, "await $(`*=$1`).click()")
		result = rePWToWdioGetByTextS.ReplaceAllString(result, "$(`*=$1`)")
		result = rePWToWdioGetByTextD.ReplaceAllString(result, "$(`*=$1`)")
		result = rePWToWdioLocatorStandalone.ReplaceAllString(result, `$$($1)`)

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
	}

	result = commentUnsupportedPlaywrightWdioLines(result)
	result = cleanupConvertedWdioOutput(result)
	return ensureTrailingNewline(result), nil
}

func commentUnsupportedPlaywrightWdioLines(source string) string {
	if rows, ok := unsupportedPlaywrightWdioLineRowsAST(source); ok {
		if len(rows) == 0 {
			return source
		}
		return commentSpecificLines(source, rows, "manual Playwright conversion required")
	}

	lines := strings.Split(source, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "///") {
			continue
		}
		if rePWToWdioUnsupportedLine.MatchString(line) {
			indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
			lines[i] = indent + "// TERRAIN-TODO: manual Playwright conversion required\n" + indent + "// " + strings.TrimSpace(line)
		}
	}
	return strings.Join(lines, "\n")
}

func cleanupConvertedWdioOutput(source string) string {
	for strings.Contains(source, "\n\n\n") {
		source = strings.ReplaceAll(source, "\n\n\n", "\n\n")
	}
	return strings.TrimSpace(source) + "\n"
}
