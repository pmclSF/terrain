package convert

import (
	"regexp"
	"strings"
)

var (
	reSelRequireImport      = regexp.MustCompile(`(?m)^const\s*\{[^}\n]*Builder[^}\n]*\}\s*=\s*require\(\s*['"]selenium-webdriver['"]\s*\)\s*;?\s*\n?`)
	reSelESMImport          = regexp.MustCompile(`(?m)^import\s+\{[^}]*\}\s+from\s+['"]selenium-webdriver['"];\s*\n?`)
	reSelJestGlobalsImport  = regexp.MustCompile(`(?m)^const\s*\{[^}\n]*expect[^}\n]*\}\s*=\s*require\(\s*['"]@jest\/globals['"]\s*\)\s*;?\s*\n?`)
	reSelDriverDeclaration  = regexp.MustCompile(`(?m)^\s*(?:let|var)\s+driver\s*;?\s*$\n?`)
	reSelBeforeAllSetup     = regexp.MustCompile(`(?s)\s*beforeAll\s*\(\s*async\s*\(\)\s*=>\s*\{\s*driver\s*=\s*await\s+new\s+Builder\(\).*?\.build\(\)\s*;?\s*\}\s*\)\s*;?\s*\n?`)
	reSelAfterAllTeardown   = regexp.MustCompile(`(?s)\s*afterAll\s*\(\s*async\s*\(\)\s*=>\s*\{\s*await\s+driver\.quit\(\)\s*;?\s*\}\s*\)\s*;?\s*\n?`)
	reSelExpectVisible      = regexp.MustCompile(`expect\s*\(\s*await\s*\(\s*await\s+driver\.findElement\(\s*By\.css\(([^)]+)\)\s*\)\s*\)\.isDisplayed\(\s*\)\s*\)\.toBe\(\s*true\s*\)`)
	reSelExpectHidden       = regexp.MustCompile(`expect\s*\(\s*await\s*\(\s*await\s+driver\.findElement\(\s*By\.css\(([^)]+)\)\s*\)\s*\)\.isDisplayed\(\s*\)\s*\)\.toBe\(\s*false\s*\)`)
	reSelExpectText         = regexp.MustCompile(`expect\s*\(\s*await\s*\(\s*await\s+driver\.findElement\(\s*By\.css\(([^)]+)\)\s*\)\s*\)\.getText\(\s*\)\s*\)\.toBe\(([^)]+)\)`)
	reSelExpectContainText  = regexp.MustCompile(`expect\s*\(\s*await\s*\(\s*await\s+driver\.findElement\(\s*By\.css\(([^)]+)\)\s*\)\s*\)\.getText\(\s*\)\s*\)\.toContain\(([^)]+)\)`)
	reSelExpectValue        = regexp.MustCompile(`expect\s*\(\s*await\s*\(\s*await\s+driver\.findElement\(\s*By\.css\(([^)]+)\)\s*\)\s*\)\.getAttribute\(\s*["']value["']\s*\)\s*\)\.toBe\(([^)]+)\)`)
	reSelExpectChecked      = regexp.MustCompile(`expect\s*\(\s*await\s*\(\s*await\s+driver\.findElement\(\s*By\.css\(([^)]+)\)\s*\)\s*\)\.isSelected\(\s*\)\s*\)\.toBe\(\s*true\s*\)`)
	reSelExpectDisabled     = regexp.MustCompile(`expect\s*\(\s*await\s*\(\s*await\s+driver\.findElement\(\s*By\.css\(([^)]+)\)\s*\)\s*\)\.isEnabled\(\s*\)\s*\)\.toBe\(\s*false\s*\)`)
	reSelExpectEnabled      = regexp.MustCompile(`expect\s*\(\s*await\s*\(\s*await\s+driver\.findElement\(\s*By\.css\(([^)]+)\)\s*\)\s*\)\.isEnabled\(\s*\)\s*\)\.toBe\(\s*true\s*\)`)
	reSelExpectCountZero    = regexp.MustCompile(`expect\s*\(\s*\(\s*await\s+driver\.findElements\(\s*By\.css\(([^)]+)\)\s*\)\s*\)\.length\s*\)\.toBe\(\s*0\s*\)`)
	reSelExpectCount        = regexp.MustCompile(`expect\s*\(\s*\(\s*await\s+driver\.findElements\(\s*By\.css\(([^)]+)\)\s*\)\s*\)\.length\s*\)\.toBe\(([^)]+)\)`)
	reSelExpectAttached     = regexp.MustCompile(`expect\s*\(\s*\(\s*await\s+driver\.findElements\(\s*By\.css\(([^)]+)\)\s*\)\s*\)\.length\s*\)\.toBeGreaterThan\(\s*0\s*\)`)
	reSelExpectCurrentURLIn = regexp.MustCompile(`expect\s*\(\s*await\s+driver\.getCurrentUrl\(\s*\)\s*\)\.toContain\(([^)]+)\)`)
	reSelExpectCurrentURLEq = regexp.MustCompile(`expect\s*\(\s*await\s+driver\.getCurrentUrl\(\s*\)\s*\)\.toBe\(([^)]+)\)`)
	reSelExpectTitle        = regexp.MustCompile(`expect\s*\(\s*await\s+driver\.getTitle\(\s*\)\s*\)\.toBe\(([^)]+)\)`)
	reSelSendKeys           = regexp.MustCompile(`await\s+driver\.findElement\(\s*By\.css\(([^)]+)\)\s*\)\.sendKeys\(([^)]+)\)`)
	reSelClick              = regexp.MustCompile(`await\s+driver\.findElement\(\s*By\.css\(([^)]+)\)\s*\)\.click\(\s*\)`)
	reSelClear              = regexp.MustCompile(`await\s+driver\.findElement\(\s*By\.css\(([^)]+)\)\s*\)\.clear\(\s*\)`)
	reSelGoto               = regexp.MustCompile(`await\s+driver\.get\(([^)]+)\)`)
	reSelRefresh            = regexp.MustCompile(`await\s+driver\.navigate\(\s*\)\.refresh\(\s*\)`)
	reSelBack               = regexp.MustCompile(`await\s+driver\.navigate\(\s*\)\.back\(\s*\)`)
	reSelForward            = regexp.MustCompile(`await\s+driver\.navigate\(\s*\)\.forward\(\s*\)`)
	reSelSleep              = regexp.MustCompile(`await\s+driver\.sleep\((\d+)\)`)
	reSelDeleteCookies      = regexp.MustCompile(`await\s+driver\.manage\(\s*\)\.deleteAllCookies\(\s*\)`)
	reSelLocalStorageClear  = regexp.MustCompile(`await\s+driver\.executeScript\(\s*["']localStorage\.clear\(\)["']\s*\)`)
	reSelXPathClick         = regexp.MustCompile(`await\s+driver\.findElement\(\s*By\.xpath\(([^)]+)\)\s*\)\.click\(\s*\)`)
	reSelFindElementCSS     = regexp.MustCompile(`driver\.findElement\(\s*By\.css\(([^)]+)\)\s*\)`)
	reSelFindElementsCSS    = regexp.MustCompile(`driver\.findElements\(\s*By\.css\(([^)]+)\)\s*\)`)
	reSelUnsupportedLine    = regexp.MustCompile(`\b(?:driver\.wait|driver\.switchTo|driver\.actions|Actions\b|until\.|Key\.|By\.(?:xpath|id|name|linkText|partialLinkText|tagName|className))`)
)

// ConvertSeleniumToPlaywrightSource rewrites the high-confidence Selenium
// browser surface into Go-native Playwright output.
func ConvertSeleniumToPlaywrightSource(source string) (string, error) {
	if strings.TrimSpace(source) == "" {
		return source, nil
	}
	if !strings.Contains(source, "driver.") &&
		!strings.Contains(source, "selenium-webdriver") &&
		!strings.Contains(source, "By.") {
		return ensureTrailingNewline(strings.ReplaceAll(source, "\r\n", "\n")), nil
	}

	result := strings.ReplaceAll(source, "\r\n", "\n")
	astApplied := false
	if astResult, ok := convertSeleniumToPlaywrightSourceAST(result); ok {
		result = astResult
		astApplied = true
	}

	if !astApplied {
		result = rePlaywrightTestImport.ReplaceAllString(result, "")
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
			{reSelExpectVisible, `await expect(page.locator($1)).toBeVisible()`},
			{reSelExpectHidden, `await expect(page.locator($1)).toBeHidden()`},
			{reSelExpectText, `await expect(page.locator($1)).toHaveText($2)`},
			{reSelExpectContainText, `await expect(page.locator($1)).toContainText($2)`},
			{reSelExpectValue, `await expect(page.locator($1)).toHaveValue($2)`},
			{reSelExpectChecked, `await expect(page.locator($1)).toBeChecked()`},
			{reSelExpectDisabled, `await expect(page.locator($1)).toBeDisabled()`},
			{reSelExpectEnabled, `await expect(page.locator($1)).toBeEnabled()`},
			{reSelExpectCountZero, `await expect(page.locator($1)).not.toBeAttached()`},
			{reSelExpectCount, `await expect(page.locator($1)).toHaveCount($2)`},
			{reSelExpectAttached, `await expect(page.locator($1)).toBeAttached()`},
			{reSelExpectCurrentURLIn, `await expect(page).toHaveURL(new RegExp($1))`},
			{reSelExpectCurrentURLEq, `await expect(page).toHaveURL($1)`},
			{reSelExpectTitle, `await expect(page).toHaveTitle($1)`},
		}
		for _, replacement := range assertionReplacements {
			result = replaceCodeRegexString(result, replacement.re, replacement.repl)
		}

		actionReplacements := []struct {
			re   *regexp.Regexp
			repl string
		}{
			{reSelSendKeys, `await page.locator($1).fill($2)`},
			{reSelClick, `await page.locator($1).click()`},
			{reSelClear, `await page.locator($1).clear()`},
			{reSelGoto, `await page.goto($1)`},
			{reSelRefresh, `await page.reload()`},
			{reSelBack, `await page.goBack()`},
			{reSelForward, `await page.goForward()`},
			{reSelSleep, `await page.waitForTimeout($1)`},
			{reSelDeleteCookies, `await page.context().clearCookies()`},
			{reSelLocalStorageClear, `await page.evaluate(() => localStorage.clear())`},
			{reSelXPathClick, "await page.locator(`xpath=$1`).click()"},
		}
		for _, replacement := range actionReplacements {
			result = replaceCodeRegexString(result, replacement.re, replacement.repl)
		}

		result = replaceCodeRegexString(result, reSelFindElementsCSS, `page.locator($1)`)
		result = replaceCodeRegexString(result, reSelFindElementCSS, `page.locator($1)`)

		result = replaceCodeRegexString(result, reDescribeOnly, "${1}test.describe.only(")
		result = replaceCodeRegexString(result, reDescribeSkip, "${1}test.describe.skip(")
		result = replaceCodeRegexString(result, reDescribe, "${1}test.describe(")
		result = replaceCodeRegexString(result, reContext, "${1}test.describe(")
		result = replaceCodeRegexString(result, reItOnly, "${1}test.only(")
		result = replaceCodeRegexString(result, reItSkip, "${1}test.skip(")
		result = replaceCodeRegexString(result, reSpecify, "${1}test(")
		result = replaceCodeRegexString(result, reIt, "${1}test(")
		result = replaceCodeRegexString(result, reBeforeEach, "${1}test.beforeEach(")
		result = replaceCodeRegexString(result, reAfterEach, "${1}test.afterEach(")
		result = replaceCodeRegexString(result, reBefore, "${1}test.beforeAll(")
		result = replaceCodeRegexString(result, reAfter, "${1}test.afterAll(")

		result = replaceCodeRegexString(result, rePlaywrightDescribeCallback, `${1}() => {`)
		result = replaceCodeRegexString(result, rePlaywrightTestEmptyCallback, `${1}async ({ page }) => {`)
		result = replaceCodeRegexString(result, rePlaywrightHookCallback, `test.$1(async ({ page }) => {`)

		result = commentUnsupportedSeleniumLines(result)
	}

	result = cleanupConvertedPlaywrightOutput(result)
	result = prependImportPreservingHeader(result, "import { test, expect } from '@playwright/test';")
	return ensureTrailingNewline(result), nil
}

func commentUnsupportedSeleniumLines(source string) string {
	lines := strings.Split(source, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "//") {
			continue
		}
		if reSelUnsupportedLine.MatchString(line) {
			indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
			lines[i] = indent + "// TERRAIN-TODO: manual Selenium conversion required\n" + indent + "// " + strings.TrimSpace(line)
		}
	}
	return strings.Join(lines, "\n")
}
