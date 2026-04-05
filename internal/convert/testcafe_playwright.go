package convert

import (
	"regexp"
	"strings"
)

var (
	reTcImport             = regexp.MustCompile(`(?m)^import\s+\{[^}]*\}\s+from\s+['"]testcafe['"];\s*\n?`)
	reTcFixturePage        = regexp.MustCompile("(?s)fixture\\s*`([^`]*)`\\s*\\.page\\s*`([^`]*)`\\s*;?\\s*")
	reTcFixtureOnly        = regexp.MustCompile("(?s)fixture\\s*`([^`]*)`\\s*;?\\s*")
	reTcTestCallback       = regexp.MustCompile(`\btest\(([^,]+),\s*async\s+t\s*=>\s*\{`)
	reTcSelectorAssign     = regexp.MustCompile(`(?m)^(\s*)(const|let|var)\s+([A-Za-z_$][\w$]*)\s*=\s*Selector\(([^)]+)\)\s*;?\s*$`)
	reTcSelectorAssignNth  = regexp.MustCompile(`(?m)^(\s*)(const|let|var)\s+([A-Za-z_$][\w$]*)\s*=\s*Selector\(([^)]+)\)\.nth\(([^)]+)\)\s*;?\s*$`)
	reTcSelectorAssignFind = regexp.MustCompile(`(?m)^(\s*)(const|let|var)\s+([A-Za-z_$][\w$]*)\s*=\s*Selector\(([^)]+)\)\.find\(([^)]+)\)\s*;?\s*$`)
	reTcSelectorAssignText = regexp.MustCompile(`(?m)^(\s*)(const|let|var)\s+([A-Za-z_$][\w$]*)\s*=\s*Selector\(([^)]+)\)\.withText\(([^)]+)\)\s*;?\s*$`)
	reTcExpectExistsOk     = regexp.MustCompile(`await\s+t\.expect\(Selector\(([^)]+)\)\.exists\)\.ok\(\)`)
	reTcExpectExistsNotOk  = regexp.MustCompile(`await\s+t\.expect\(Selector\(([^)]+)\)\.exists\)\.notOk\(\)`)
	reTcExpectVisibleOk    = regexp.MustCompile(`await\s+t\.expect\(Selector\(([^)]+)\)\.visible\)\.ok\(\)`)
	reTcExpectVisibleNotOk = regexp.MustCompile(`await\s+t\.expect\(Selector\(([^)]+)\)\.visible\)\.notOk\(\)`)
	reTcExpectCountEq      = regexp.MustCompile(`await\s+t\.expect\(Selector\(([^)]+)\)\.count\)\.eql\(([^)]+)\)`)
	reTcExpectInnerTextEq  = regexp.MustCompile(`await\s+t\.expect\(Selector\(([^)]+)\)\.innerText\)\.eql\(([^)]+)\)`)
	reTcExpectInnerTextIn  = regexp.MustCompile(`await\s+t\.expect\(Selector\(([^)]+)\)\.innerText\)\.contains\(([^)]+)\)`)
	reTcExpectValueEq      = regexp.MustCompile(`await\s+t\.expect\(Selector\(([^)]+)\)\.value\)\.eql\(([^)]+)\)`)
	reTcClickSelectorText  = regexp.MustCompile(`await\s+t\.click\(Selector\(([^)]+)\)\.withText\(([^)]+)\)\)`)
	reTcClickSelectorNth   = regexp.MustCompile(`await\s+t\.click\(Selector\(([^)]+)\)\.nth\(([^)]+)\)\)`)
	reTcClickSelectorFind  = regexp.MustCompile(`await\s+t\.click\(Selector\(([^)]+)\)\.find\(([^)]+)\)\)`)
	reTcClickSelector      = regexp.MustCompile(`await\s+t\.click\(Selector\(([^)]+)\)\)`)
	reTcTypeTextSelector   = regexp.MustCompile(`await\s+t\.typeText\(Selector\(([^)]+)\),\s*([^)]+)\)`)
	reTcClick              = regexp.MustCompile(`await\s+t\.click\(([^)]+)\)`)
	reTcTypeText           = regexp.MustCompile(`await\s+t\.typeText\(([^,]+),\s*([^)]+)\)`)
	reTcDoubleClick        = regexp.MustCompile(`await\s+t\.doubleClick\(([^)]+)\)`)
	reTcHover              = regexp.MustCompile(`await\s+t\.hover\(([^)]+)\)`)
	reTcNavigate           = regexp.MustCompile(`await\s+t\.navigateTo\(([^)]+)\)`)
	reTcWait               = regexp.MustCompile(`await\s+t\.wait\(([^)]+)\)`)
	reTcTakeScreenshot     = regexp.MustCompile(`await\s+t\.takeScreenshot\(\)`)
	reTcResizeWindow       = regexp.MustCompile(`await\s+t\.resizeWindow\(([^,]+),\s*([^)]+)\)`)
	reTcSetFilesToUpload   = regexp.MustCompile(`await\s+t\.setFilesToUpload\(([^,]+),\s*([^)]+)\)`)
	reTcSelectorNth        = regexp.MustCompile(`Selector\(([^)]+)\)\.nth\(([^)]+)\)`)
	reTcSelectorFind       = regexp.MustCompile(`Selector\(([^)]+)\)\.find\(([^)]+)\)`)
	reTcSelectorWithText   = regexp.MustCompile(`Selector\(([^)]+)\)\.withText\(([^)]+)\)`)
	reTcSelectorStandalone = regexp.MustCompile(`Selector\(([^)]+)\)`)
	reTcPWExpectLocatorVar = regexp.MustCompile(`await expect\(page\.locator\(([A-Za-z_$][\w$]*)\)\)\.`)
	reTcPWActionLocatorVar = regexp.MustCompile(`await page\.locator\(([A-Za-z_$][\w$]*)\)\.`)
	reTcUnsupportedLine    = regexp.MustCompile(`\b(?:Role\(|RequestMock\(|ClientFunction\(|RequestLogger\(|RequestHook\(|t\.useRole|t\.switchToIframe|t\.switchToMainWindow|t\.pressKey|t\.rightClick|t\.eval\()`)
)

// ConvertTestCafeToPlaywrightSource rewrites the high-confidence TestCafe
// browser surface into Go-native Playwright output.
func ConvertTestCafeToPlaywrightSource(source string) (string, error) {
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
	result = rePlaywrightTestImport.ReplaceAllString(result, "")
	suiteName, pageURL := "", ""
	astApplied := false
	if astResult, astSuiteName, astPageURL, ok := convertTestCafeToPlaywrightSourceAST(result); ok {
		result = astResult
		suiteName = astSuiteName
		pageURL = astPageURL
		astApplied = true
	}

	if !astApplied {
		result = reTcImport.ReplaceAllString(result, "")
		result, suiteName, pageURL = extractTestCafeFixture(result)
		result = replaceCodeRegexString(result, reTcSelectorAssignText, `${1}${2} ${3} = page.locator(${4}).filter({ hasText: ${5} });`)
		result = replaceCodeRegexString(result, reTcSelectorAssignFind, `${1}${2} ${3} = page.locator(${4}).locator(${5});`)
		result = replaceCodeRegexString(result, reTcSelectorAssignNth, `${1}${2} ${3} = page.locator(${4}).nth(${5});`)
		result = replaceCodeRegexString(result, reTcSelectorAssign, `${1}${2} ${3} = page.locator(${4});`)

		assertionReplacements := []struct {
			re   *regexp.Regexp
			repl string
		}{
			{reTcExpectExistsOk, `await expect(page.locator($1)).toBeAttached()`},
			{reTcExpectExistsNotOk, `await expect(page.locator($1)).not.toBeAttached()`},
			{reTcExpectVisibleOk, `await expect(page.locator($1)).toBeVisible()`},
			{reTcExpectVisibleNotOk, `await expect(page.locator($1)).toBeHidden()`},
			{reTcExpectCountEq, `await expect(page.locator($1)).toHaveCount($2)`},
			{reTcExpectInnerTextEq, `await expect(page.locator($1)).toHaveText($2)`},
			{reTcExpectInnerTextIn, `await expect(page.locator($1)).toContainText($2)`},
			{reTcExpectValueEq, `await expect(page.locator($1)).toHaveValue($2)`},
		}
		for _, replacement := range assertionReplacements {
			result = replaceCodeRegexString(result, replacement.re, replacement.repl)
		}

		actionReplacements := []struct {
			re   *regexp.Regexp
			repl string
		}{
			{reTcClickSelectorText, `await page.locator($1).filter({ hasText: $2 }).click()`},
			{reTcClickSelectorNth, `await page.locator($1).nth($2).click()`},
			{reTcClickSelectorFind, `await page.locator($1).locator($2).click()`},
			{reTcClickSelector, `await page.locator($1).click()`},
			{reTcTypeTextSelector, `await page.locator($1).fill($2)`},
			{reTcDoubleClick, `await page.locator($1).dblclick()`},
			{reTcHover, `await page.locator($1).hover()`},
			{reTcNavigate, `await page.goto($1)`},
			{reTcWait, `await page.waitForTimeout($1)`},
			{reTcTakeScreenshot, `await page.screenshot()`},
			{reTcResizeWindow, `await page.setViewportSize({ width: $1, height: $2 })`},
			{reTcSetFilesToUpload, `await page.locator($1).setInputFiles($2)`},
			{reTcClick, `await page.locator($1).click()`},
			{reTcTypeText, `await page.locator($1).fill($2)`},
		}
		for _, replacement := range actionReplacements {
			result = replaceCodeRegexString(result, replacement.re, replacement.repl)
		}

		result = replaceCodeRegexString(result, reTcSelectorWithText, `page.locator($1).filter({ hasText: $2 })`)
		result = replaceCodeRegexString(result, reTcSelectorFind, `page.locator($1).locator($2)`)
		result = replaceCodeRegexString(result, reTcSelectorNth, `page.locator($1).nth($2)`)
		result = replaceCodeRegexString(result, reTcSelectorStandalone, `page.locator($1)`)
		result = replaceCodeRegexString(result, reTcPWExpectLocatorVar, `await expect($1).`)
		result = replaceCodeRegexString(result, reTcPWActionLocatorVar, `await $1.`)
		result = replaceCodeRegexString(result, reTcTestCallback, `test($1, async ({ page }) => {`)

		result = commentUnsupportedTestCafePlaywrightLines(result)
	}
	result = cleanupConvertedPlaywrightOutput(result)
	result = wrapTestCafePlaywrightSuite(result, suiteName, pageURL)
	result = prependImportPreservingHeader(result, "import { test, expect } from '@playwright/test';")
	return ensureTrailingNewline(result), nil
}

func extractTestCafeFixture(source string) (string, string, string) {
	if match := reTcFixturePage.FindStringSubmatch(source); len(match) == 3 {
		return reTcFixturePage.ReplaceAllString(source, ""), match[1], match[2]
	}
	if match := reTcFixtureOnly.FindStringSubmatch(source); len(match) == 2 {
		return reTcFixtureOnly.ReplaceAllString(source, ""), match[1], ""
	}
	return source, "", ""
}

func wrapTestCafePlaywrightSuite(body, suiteName, pageURL string) string {
	body = strings.TrimSpace(body)
	if suiteName == "" {
		return body
	}

	lines := []string{`test.describe(` + quoteSingle(suiteName) + `, () => {`}
	if pageURL != "" {
		lines = append(lines,
			"  test.beforeEach(async ({ page }) => {",
			"    await page.goto("+quoteSingle(pageURL)+")",
			"  });",
			"",
		)
	}
	if body != "" {
		lines = append(lines, body)
	}
	lines = append(lines, "});")
	return strings.Join(lines, "\n")
}

func commentUnsupportedTestCafePlaywrightLines(source string) string {
	lines := strings.Split(source, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "//") {
			continue
		}
		if reTcUnsupportedLine.MatchString(line) || strings.Contains(line, "await t.") || strings.Contains(line, "Selector(") {
			indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
			lines[i] = indent + "// TERRAIN-TODO: manual TestCafe conversion required\n" + indent + "// " + strings.TrimSpace(line)
		}
	}
	return strings.Join(lines, "\n")
}

func quoteSingle(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "\\'") + "'"
}
