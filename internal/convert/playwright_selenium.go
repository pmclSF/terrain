package convert

import (
	"regexp"
	"strings"
)

var (
	rePWToSelExpectURL       = regexp.MustCompile(`await expect\(page\)\.toHaveURL\(([^)]+)\)`)
	rePWToSelExpectTitle     = regexp.MustCompile(`await expect\(page\)\.toHaveTitle\(([^)]+)\)`)
	rePWToSelExpectVisible   = regexp.MustCompile(`await expect\(page\.locator\(([^)]+)\)\)\.toBeVisible\(\)`)
	rePWToSelExpectHidden    = regexp.MustCompile(`await expect\(page\.locator\(([^)]+)\)\)\.toBeHidden\(\)`)
	rePWToSelExpectAttached  = regexp.MustCompile(`await expect\(page\.locator\(([^)]+)\)\)\.toBeAttached\(\)`)
	rePWToSelExpectNotAttach = regexp.MustCompile(`await expect\(page\.locator\(([^)]+)\)\)\.not\.toBeAttached\(\)`)
	rePWToSelExpectText      = regexp.MustCompile(`await expect\(page\.locator\(([^)]+)\)\)\.toHaveText\(([^)]+)\)`)
	rePWToSelExpectContain   = regexp.MustCompile(`await expect\(page\.locator\(([^)]+)\)\)\.toContainText\(([^)]+)\)`)
	rePWToSelExpectValue     = regexp.MustCompile(`await expect\(page\.locator\(([^)]+)\)\)\.toHaveValue\(([^)]+)\)`)
	rePWToSelExpectChecked   = regexp.MustCompile(`await expect\(page\.locator\(([^)]+)\)\)\.toBeChecked\(\)`)
	rePWToSelExpectDisabled  = regexp.MustCompile(`await expect\(page\.locator\(([^)]+)\)\)\.toBeDisabled\(\)`)
	rePWToSelExpectEnabled   = regexp.MustCompile(`await expect\(page\.locator\(([^)]+)\)\)\.toBeEnabled\(\)`)
	rePWToSelExpectCount     = regexp.MustCompile(`await expect\(page\.locator\(([^)]+)\)\)\.toHaveCount\(([^)]+)\)`)
	rePWToSelFill            = regexp.MustCompile(`await page\.locator\(([^)]+)\)\.fill\(([^)]+)\)`)
	rePWToSelClick           = regexp.MustCompile(`await page\.locator\(([^)]+)\)\.click\(\)`)
	rePWToSelClear           = regexp.MustCompile(`await page\.locator\(([^)]+)\)\.clear\(\)`)
	rePWToSelCheck           = regexp.MustCompile(`await page\.locator\(([^)]+)\)\.check\(\)`)
	rePWToSelUncheck         = regexp.MustCompile(`await page\.locator\(([^)]+)\)\.uncheck\(\)`)
	rePWToSelSelectOption    = regexp.MustCompile(`await page\.locator\(([^)]+)\)\.selectOption\(([^)]+)\)`)
	rePWToSelGetByTextClick  = regexp.MustCompile(`await page\.getByText\(([^)]+)\)\.click\(\)`)
	rePWToSelGoto            = regexp.MustCompile(`await page\.goto\(([^)]+)\)`)
	rePWToSelReload          = regexp.MustCompile(`await page\.reload\(\)`)
	rePWToSelBack            = regexp.MustCompile(`await page\.goBack\(\)`)
	rePWToSelForward         = regexp.MustCompile(`await page\.goForward\(\)`)
	rePWToSelSetViewport     = regexp.MustCompile(`await page\.setViewportSize\(\{\s*width:\s*(\d+),\s*height:\s*(\d+)\s*\}\)`)
	rePWToSelWaitTimeout     = regexp.MustCompile(`await page\.waitForTimeout\((\d+)\)`)
	rePWToSelClearCookies    = regexp.MustCompile(`await (?:page\.context\(\)|context)\.clearCookies\(\)`)
	rePWToSelStorageClear    = regexp.MustCompile(`await page\.evaluate\(\(\)\s*=>\s*localStorage\.clear\(\)\)`)
	rePWToSelUnsupportedLine = regexp.MustCompile(`\b(?:page\.route|page\.waitForRequest|page\.waitForEvent|page\.getByRole|page\.getByTestId|request\.|download\.|context\.)|page\.context\(`)
)

// ConvertPlaywrightToSeleniumSource rewrites the high-confidence Playwright
// browser surface into Go-native Selenium output.
func ConvertPlaywrightToSeleniumSource(source string) (string, error) {
	if strings.TrimSpace(source) == "" {
		return source, nil
	}
	if !strings.Contains(source, "page.") &&
		!strings.Contains(source, "test.") &&
		!strings.Contains(source, "@playwright/test") {
		return ensureTrailingNewline(strings.ReplaceAll(source, "\r\n", "\n")), nil
	}

	result := strings.ReplaceAll(source, "\r\n", "\n")
	astApplied := false
	if astResult, ok := convertPlaywrightToSeleniumSourceAST(result); ok {
		result = astResult
		astApplied = true
	}

	if !astApplied {
		result = rePlaywrightImportRemove.ReplaceAllString(result, "")

		if rePWToSelExpectURL.MatchString(result) {
			result = replaceCodeRegexMatches(result, rePWToSelExpectURL, func(match string, groups []string) string {
				if len(groups) != 1 {
					return match
				}
				return seleniumExpectationAssertion("await driver.getCurrentUrl()", groups[0])
			})
		}
		if rePWToSelExpectTitle.MatchString(result) {
			result = replaceCodeRegexMatches(result, rePWToSelExpectTitle, func(match string, groups []string) string {
				if len(groups) != 1 {
					return match
				}
				return seleniumExpectationAssertion("await driver.getTitle()", groups[0])
			})
		}

		assertionReplacements := []struct {
			re   *regexp.Regexp
			repl string
		}{
			{rePWToSelExpectVisible, `expect(await (await driver.findElement(By.css($1))).isDisplayed()).toBe(true)`},
			{rePWToSelExpectHidden, `expect(await (await driver.findElement(By.css($1))).isDisplayed()).toBe(false)`},
			{rePWToSelExpectAttached, `expect((await driver.findElements(By.css($1))).length).toBeGreaterThan(0)`},
			{rePWToSelExpectNotAttach, `expect((await driver.findElements(By.css($1))).length).toBe(0)`},
			{rePWToSelExpectText, `expect(await (await driver.findElement(By.css($1))).getText()).toBe($2)`},
			{rePWToSelExpectContain, `expect(await (await driver.findElement(By.css($1))).getText()).toContain($2)`},
			{rePWToSelExpectValue, `expect(await (await driver.findElement(By.css($1))).getAttribute("value")).toBe($2)`},
			{rePWToSelExpectChecked, `expect(await (await driver.findElement(By.css($1))).isSelected()).toBe(true)`},
			{rePWToSelExpectDisabled, `expect(await (await driver.findElement(By.css($1))).isEnabled()).toBe(false)`},
			{rePWToSelExpectEnabled, `expect(await (await driver.findElement(By.css($1))).isEnabled()).toBe(true)`},
			{rePWToSelExpectCount, `expect((await driver.findElements(By.css($1))).length).toBe($2)`},
		}
		for _, replacement := range assertionReplacements {
			result = replaceCodeRegexString(result, replacement.re, replacement.repl)
		}

		actionReplacements := []struct {
			re   *regexp.Regexp
			repl string
		}{
			{rePWToSelFill, `await driver.findElement(By.css($1)).sendKeys($2)`},
			{rePWToSelClick, `await driver.findElement(By.css($1)).click()`},
			{rePWToSelClear, `await driver.findElement(By.css($1)).clear()`},
			{rePWToSelGoto, `await driver.get($1)`},
			{rePWToSelReload, `await driver.navigate().refresh()`},
			{rePWToSelBack, `await driver.navigate().back()`},
			{rePWToSelForward, `await driver.navigate().forward()`},
			{rePWToSelSetViewport, `await driver.manage().window().setRect({ width: $1, height: $2 })`},
			{rePWToSelWaitTimeout, `await driver.sleep($1)`},
			{rePWToSelClearCookies, `await driver.manage().deleteAllCookies()`},
			{rePWToSelStorageClear, `await driver.executeScript("localStorage.clear()")`},
			{rePWToSelGetByTextClick, "await driver.findElement(By.xpath(`//*[contains(text(),$1)]`)).click()"},
		}
		for _, replacement := range actionReplacements {
			result = replaceCodeRegexString(result, replacement.re, replacement.repl)
		}

		if rePWToSelCheck.MatchString(result) {
			result = replaceCodeRegexString(result, rePWToSelCheck, "const checkbox = await driver.findElement(By.css($1));\n    if (!(await checkbox.isSelected())) await checkbox.click()")
		}
		if rePWToSelUncheck.MatchString(result) {
			result = replaceCodeRegexString(result, rePWToSelUncheck, "const checkbox = await driver.findElement(By.css($1));\n    if (await checkbox.isSelected()) await checkbox.click()")
		}
		if rePWToSelSelectOption.MatchString(result) {
			result = replaceCodeRegexString(result, rePWToSelSelectOption, "const select = await driver.findElement(By.css($1));\n    await select.findElement(By.css(`option[value=${$2}]`)).click()")
		}

		result = replaceCodeRegexString(result, rePWDescribeOnly, "describe.only(")
		result = replaceCodeRegexString(result, rePWDescribeSkip, "describe.skip(")
		result = replaceCodeRegexString(result, rePWDescribe, "describe(")
		result = replaceCodeRegexString(result, rePWTestOnly, "it.only(")
		result = replaceCodeRegexString(result, rePWTestSkip, "it.skip(")
		result = replaceCodeRegexString(result, rePWBeforeAll, "beforeAll(")
		result = replaceCodeRegexString(result, rePWAfterAll, "afterAll(")
		result = replaceCodeRegexString(result, rePWBeforeEach, "beforeEach(")
		result = replaceCodeRegexString(result, rePWAfterEach, "afterEach(")
		result = replaceCodeRegexString(result, rePWTestCall, "it($1,")
		result = replaceCodeRegexString(result, rePWCallbackArgs, "() =>")

		result = commentUnsupportedPlaywrightSeleniumLines(result)
	}

	result = cleanupConvertedSeleniumOutput(result)
	result = prependImportPreservingHeader(result, seleniumBoilerplate)
	return ensureTrailingNewline(result), nil
}

const seleniumBoilerplate = "const { Builder, By, Key, until } = require('selenium-webdriver');\n\nlet driver;\n\nbeforeAll(async () => {\n  driver = await new Builder().forBrowser('chrome').build();\n});\n\nafterAll(async () => {\n  await driver.quit();\n});"

func commentUnsupportedPlaywrightSeleniumLines(source string) string {
	lines := strings.Split(source, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "//") {
			continue
		}
		if rePWToSelUnsupportedLine.MatchString(line) {
			indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
			lines[i] = indent + "// TERRAIN-TODO: manual Playwright conversion required\n" + indent + "// " + strings.TrimSpace(line)
		}
	}
	return strings.Join(lines, "\n")
}

func cleanupConvertedSeleniumOutput(source string) string {
	for strings.Contains(source, "\n\n\n") {
		source = strings.ReplaceAll(source, "\n\n\n", "\n\n")
	}
	return strings.TrimSpace(source) + "\n"
}

func seleniumExpectationAssertion(actual, expected string) string {
	if isJSRegexLiteral(expected) {
		return "expect(" + actual + ").toMatch(" + expected + ")"
	}
	return "expect(" + actual + ").toBe(" + expected + ")"
}
