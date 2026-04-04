package convert

import (
	"regexp"
	"strings"
)

var (
	rePWToSelExpectURLRegex  = regexp.MustCompile(`await expect\(page\)\.toHaveURL\((/[^)]+/)\)`)
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

		assertionReplacements := []struct {
			re   *regexp.Regexp
			repl string
		}{
			{rePWToSelExpectURLRegex, `expect(await driver.getCurrentUrl()).toContain($1)`},
			{rePWToSelExpectURL, `expect(await driver.getCurrentUrl()).toBe($1)`},
			{rePWToSelExpectTitle, `expect(await driver.getTitle()).toBe($1)`},
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
			result = replacement.re.ReplaceAllString(result, replacement.repl)
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
			result = replacement.re.ReplaceAllString(result, replacement.repl)
		}

		if rePWToSelCheck.MatchString(result) {
			result = rePWToSelCheck.ReplaceAllString(result, "const checkbox = await driver.findElement(By.css($1));\n    if (!(await checkbox.isSelected())) await checkbox.click()")
		}
		if rePWToSelUncheck.MatchString(result) {
			result = rePWToSelUncheck.ReplaceAllString(result, "const checkbox = await driver.findElement(By.css($1));\n    if (await checkbox.isSelected()) await checkbox.click()")
		}
		if rePWToSelSelectOption.MatchString(result) {
			result = rePWToSelSelectOption.ReplaceAllString(result, "const select = await driver.findElement(By.css($1));\n    await select.findElement(By.css(`option[value=${$2}]`)).click()")
		}

		result = rePWDescribeOnly.ReplaceAllString(result, "describe.only(")
		result = rePWDescribeSkip.ReplaceAllString(result, "describe.skip(")
		result = rePWDescribe.ReplaceAllString(result, "describe(")
		result = rePWTestOnly.ReplaceAllString(result, "it.only(")
		result = rePWTestSkip.ReplaceAllString(result, "it.skip(")
		result = rePWBeforeAll.ReplaceAllString(result, "beforeAll(")
		result = rePWAfterAll.ReplaceAllString(result, "afterAll(")
		result = rePWBeforeEach.ReplaceAllString(result, "beforeEach(")
		result = rePWAfterEach.ReplaceAllString(result, "afterEach(")
		result = rePWTestCall.ReplaceAllString(result, "it($1,")
		result = rePWCallbackArgs.ReplaceAllString(result, "() =>")

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
