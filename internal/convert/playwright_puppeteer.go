package convert

import (
	"regexp"
	"strings"
)

var (
	rePWToPptrExpectURL       = regexp.MustCompile(`await expect\(page\)\.toHaveURL\(([^)]+)\)`)
	rePWToPptrExpectTitle     = regexp.MustCompile(`await expect\(page\)\.toHaveTitle\(([^)]+)\)`)
	rePWToPptrExpectVisible   = regexp.MustCompile(`await expect\(page\.locator\(([^)]+)\)\)\.toBeVisible\(\)`)
	rePWToPptrExpectHidden    = regexp.MustCompile(`await expect\(page\.locator\(([^)]+)\)\)\.toBeHidden\(\)`)
	rePWToPptrExpectAttached  = regexp.MustCompile(`await expect\(page\.locator\(([^)]+)\)\)\.toBeAttached\(\)`)
	rePWToPptrExpectText      = regexp.MustCompile(`await expect\(page\.locator\(([^)]+)\)\)\.toHaveText\(([^)]+)\)`)
	rePWToPptrExpectContain   = regexp.MustCompile(`await expect\(page\.locator\(([^)]+)\)\)\.toContainText\(([^)]+)\)`)
	rePWToPptrExpectValue     = regexp.MustCompile(`await expect\(page\.locator\(([^)]+)\)\)\.toHaveValue\(([^)]+)\)`)
	rePWToPptrExpectCount     = regexp.MustCompile(`await expect\(page\.locator\(([^)]+)\)\)\.toHaveCount\(([^)]+)\)`)
	rePWToPptrExpectChecked   = regexp.MustCompile(`await expect\(page\.locator\(([^)]+)\)\)\.toBeChecked\(\)`)
	rePWToPptrExpectAttribute = regexp.MustCompile(`await expect\(page\.locator\(([^)]+)\)\)\.toHaveAttribute\(([^,]+),\s*([^)]+)\)`)

	rePWToPptrFill        = regexp.MustCompile(`await page\.locator\(([^)]+)\)\.fill\(([^)]+)\)`)
	rePWToPptrClick       = regexp.MustCompile(`await page\.locator\(([^)]+)\)\.click\(\)`)
	rePWToPptrDblclick    = regexp.MustCompile(`await page\.locator\(([^)]+)\)\.dblclick\(\)`)
	rePWToPptrHover       = regexp.MustCompile(`await page\.locator\(([^)]+)\)\.hover\(\)`)
	rePWToPptrTextContent = regexp.MustCompile(`await page\.locator\(([^)]+)\)\.textContent\(\)`)
	rePWToPptrIsVisible   = regexp.MustCompile(`await page\.locator\(([^)]+)\)\.isVisible\(\)`)
	rePWToPptrWait        = regexp.MustCompile(`await page\.locator\(([^)]+)\)\.waitFor\(\)`)
	rePWToPptrWaitOpts    = regexp.MustCompile(`await page\.locator\(([^)]+)\)\.waitFor\(\{[^}]*\}\)`)
	rePWToPptrEvaluate    = regexp.MustCompile(`await page\.locator\(([^)]+)\)\.evaluate\(([^)]+)\)`)
	rePWToPptrEvaluateAll = regexp.MustCompile(`await page\.locator\(([^)]+)\)\.evaluateAll\(([^)]+)\)`)
	rePWToPptrSelect      = regexp.MustCompile(`await page\.locator\(([^)]+)\)\.selectOption\(([^)]+)\)`)
	rePWToPptrClear       = regexp.MustCompile(`await page\.locator\(([^)]+)\)\.clear\(\)`)
	rePWToPptrStandalone  = regexp.MustCompile(`page\.locator\(([^)]+)\)`)

	rePWToPptrViewport    = regexp.MustCompile(`await page\.setViewportSize\(\{\s*width:\s*(\d+),\s*height:\s*(\d+)\s*\}\)`)
	rePWToPptrAddCookies  = regexp.MustCompile(`await (?:page\.context\(\)|context)\.addCookies\(`)
	rePWToPptrCookies     = regexp.MustCompile(`await (?:page\.context\(\)|context)\.cookies\(\)`)
	rePWToPptrClearCook   = regexp.MustCompile(`await (?:page\.context\(\)|context)\.clearCookies\(\)`)
	rePWToPptrRoute       = regexp.MustCompile(`await page\.route\([^)]+,\s*[^)]+\)`)
	rePWToPptrUnsupported = regexp.MustCompile(`\b(?:page\.route|page\.getByText|page\.getByRole|page\.getByTestId|request\.|download\.|context\.)|page\.context\(`)

	rePuppeteerDescribeOpen = regexp.MustCompile(`describe(?:\.(?:only|skip))?\([^)]*,\s*(?:async\s*)?\(\s*\)\s*=>\s*\{\n`)
)

// ConvertPlaywrightToPuppeteerSource rewrites the high-confidence Playwright
// browser surface into Go-native Puppeteer output.
func ConvertPlaywrightToPuppeteerSource(source string) (string, error) {
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

	assertionReplacements := []struct {
		re   *regexp.Regexp
		repl string
	}{
		{rePWToPptrExpectURL, `expect(page.url()).toBe($1)`},
		{rePWToPptrExpectTitle, `expect(await page.title()).toBe($1)`},
		{rePWToPptrExpectVisible, `expect(await page.$$($1)).toBeTruthy()`},
		{rePWToPptrExpectHidden, `expect(await page.$$($1)).toBeFalsy()`},
		{rePWToPptrExpectAttached, `expect(await page.$$($1)).toBeTruthy()`},
		{rePWToPptrExpectText, `expect(await page.$$eval($1, el => el.textContent)).toBe($2)`},
		{rePWToPptrExpectContain, `expect(await page.$$eval($1, el => el.textContent)).toContain($2)`},
		{rePWToPptrExpectValue, `expect(await page.$$eval($1, el => el.value)).toBe($2)`},
		{rePWToPptrExpectCount, `expect((await page.$$$$($1)).length).toBe($2)`},
		{rePWToPptrExpectChecked, `expect(await page.$$eval($1, el => el.checked)).toBe(true)`},
		{rePWToPptrExpectAttribute, `expect(await page.$$eval($1, (el, a) => el.getAttribute(a), $2)).toBe($3)`},
	}
	for _, replacement := range assertionReplacements {
		result = replacement.re.ReplaceAllString(result, replacement.repl)
	}

	actionReplacements := []struct {
		re   *regexp.Regexp
		repl string
	}{
		{rePWToPptrFill, `await page.type($1, $2)`},
		{rePWToPptrDblclick, `await page.click($1, { clickCount: 2 })`},
		{rePWToPptrClick, `await page.click($1)`},
		{rePWToPptrHover, `await page.hover($1)`},
		{rePWToPptrTextContent, `await page.$$eval($1, el => el.textContent)`},
		{rePWToPptrIsVisible, `!!(await page.$$($1))`},
		{rePWToPptrWaitOpts, `await page.waitForSelector($1)`},
		{rePWToPptrWait, `await page.waitForSelector($1)`},
		{rePWToPptrEvaluate, `await page.$$eval($1, $2)`},
		{rePWToPptrEvaluateAll, `await page.$$$$eval($1, $2)`},
		{rePWToPptrSelect, `await page.select($1, $2)`},
		{rePWToPptrViewport, `await page.setViewport({ width: $1, height: $2 })`},
		{rePWToPptrAddCookies, `await page.setCookie(`},
		{rePWToPptrCookies, `await page.cookies()`},
		{rePWToPptrClearCook, `await page.deleteCookie()`},
	}
	for _, replacement := range actionReplacements {
		result = replacement.re.ReplaceAllString(result, replacement.repl)
	}

	if rePWToPptrClear.MatchString(result) {
		result = rePWToPptrClear.ReplaceAllString(result, "await page.click($1, { clickCount: 3 });\n    await page.keyboard.press('Backspace')")
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

	result = rePWToPptrStandalone.ReplaceAllString(result, `page.$$($1)`)
	result = commentUnsupportedPlaywrightPuppeteerLines(result)
	result = addPuppeteerLifecycleBoilerplate(result)
	result = cleanupConvertedPuppeteerOutput(result)
	result = prependImportPreservingHeader(result, "const puppeteer = require('puppeteer');")
	return ensureTrailingNewline(result), nil
}

func commentUnsupportedPlaywrightPuppeteerLines(source string) string {
	lines := strings.Split(source, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "//") {
			continue
		}
		if rePWToPptrRoute.MatchString(line) || rePWToPptrUnsupported.MatchString(line) {
			indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
			lines[i] = indent + "// TERRAIN-TODO: manual Playwright conversion required\n" + indent + "// " + strings.TrimSpace(line)
		}
	}
	return strings.Join(lines, "\n")
}

func addPuppeteerLifecycleBoilerplate(source string) string {
	if strings.Contains(source, "let browser, page;") || !strings.Contains(source, "describe(") {
		return source
	}
	loc := rePuppeteerDescribeOpen.FindStringIndex(source)
	if loc == nil {
		return source
	}

	lifecycle := "  let browser, page;\n\n  beforeAll(async () => {\n    browser = await puppeteer.launch();\n    page = await browser.newPage();\n  });\n\n  afterAll(async () => {\n    await browser.close();\n  });"
	return source[:loc[1]] + lifecycle + "\n\n" + source[loc[1]:]
}

func cleanupConvertedPuppeteerOutput(source string) string {
	for strings.Contains(source, "\n\n\n") {
		source = strings.ReplaceAll(source, "\n\n\n", "\n\n")
	}
	return strings.TrimSpace(source) + "\n"
}
