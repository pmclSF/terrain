package convert

import (
	"regexp"
	"strings"
)

var (
	rePptrRequireImport    = regexp.MustCompile(`(?m)^const\s+puppeteer\s*=\s*require\(\s*['"]puppeteer['"]\s*\)\s*;?\s*\n?`)
	rePptrESMImport        = regexp.MustCompile(`(?m)^import\s+puppeteer\s+from\s+['"]puppeteer['"];\s*\n?`)
	rePptrBrowserPageDecl  = regexp.MustCompile(`(?m)^\s*let\s+browser\s*,\s*page\s*;?\s*\n?`)
	rePptrBeforeAllBlock   = regexp.MustCompile(`(?s)\s*beforeAll\(async\s*\(\)\s*=>\s*\{\s*browser\s*=\s*await\s+puppeteer\.launch\([^)]*\)\s*;?\s*page\s*=\s*await\s+browser\.newPage\(\)\s*;?\s*\}\)\s*;?\s*\n?`)
	rePptrAfterAllBlock    = regexp.MustCompile(`(?s)\s*afterAll\(async\s*\(\)\s*=>\s*\{\s*await\s+browser\.close\(\)\s*;?\s*\}\)\s*;?\s*\n?`)
	rePptrLaunchLine       = regexp.MustCompile(`(?m)^\s*browser\s*=\s*await\s+puppeteer\.launch\([^)]*\)\s*;?\s*$`)
	rePptrNewPageLine      = regexp.MustCompile(`(?m)^\s*page\s*=\s*await\s+browser\.newPage\(\)\s*;?\s*$`)
	rePptrCloseLine        = regexp.MustCompile(`(?m)^\s*await\s+browser\.close\(\)\s*;?\s*$`)
	rePptrExpectURL        = regexp.MustCompile(`expect\(page\.url\(\)\)\.toBe\(([^)]+)\)`)
	rePptrExpectURLContain = regexp.MustCompile(`expect\(page\.url\(\)\)\.toContain\(([^)]+)\)`)
	rePptrExpectURLMatch   = regexp.MustCompile(`expect\(page\.url\(\)\)\.toMatch\(([^)]+)\)`)
	rePptrExpectTitle      = regexp.MustCompile(`expect\(await\s+page\.title\(\)\)\.toBe\(([^)]+)\)`)
	rePptrExpectTitleMatch = regexp.MustCompile(`expect\(await\s+page\.title\(\)\)\.toMatch\(([^)]+)\)`)
	rePptrExpectTruthy     = regexp.MustCompile(`expect\(await\s+page\.\$\(([^)]+)\)\)\.toBeTruthy\(\)`)
	rePptrExpectFalsy      = regexp.MustCompile(`expect\(await\s+page\.\$\(([^)]+)\)\)\.toBeFalsy\(\)`)
	rePptrExpectText       = regexp.MustCompile(`expect\(await\s+page\.\$eval\(([^,]+),\s*el\s*=>\s*el\.textContent\)\)\.toBe\(([^)]+)\)`)
	rePptrExpectContain    = regexp.MustCompile(`expect\(await\s+page\.\$eval\(([^,]+),\s*el\s*=>\s*el\.textContent\)\)\.toContain\(([^)]+)\)`)
	rePptrExpectValue      = regexp.MustCompile(`expect\(await\s+page\.\$eval\(([^,]+),\s*el\s*=>\s*el\.value\)\)\.toBe\(([^)]+)\)`)
	rePptrExpectCount      = regexp.MustCompile(`expect\(\(await\s+page\.\$\$\(([^)]+)\)\)\.length\)\.toBe\(([^)]+)\)`)
	rePptrPageType         = regexp.MustCompile(`await page\.type\(([^,]+),\s*([^)]+)\)`)
	rePptrPageClickDouble  = regexp.MustCompile(`await page\.click\(([^,]+),\s*\{\s*clickCount:\s*2\s*\}\)`)
	rePptrPageClick        = regexp.MustCompile(`await page\.click\(([^)]+)\)`)
	rePptrPageHover        = regexp.MustCompile(`await page\.hover\(([^)]+)\)`)
	rePptrPageSelect       = regexp.MustCompile(`await page\.select\(([^,]+),\s*([^)]+)\)`)
	rePptrPageFocus        = regexp.MustCompile(`await page\.focus\(([^)]+)\)`)
	rePptrPageEval         = regexp.MustCompile(`await page\.\$eval\(([^,]+),\s*`)
	rePptrPageEvalAll      = regexp.MustCompile(`await page\.\$\$eval\(([^,]+),\s*`)
	rePptrPageMany         = regexp.MustCompile(`await page\.\$\$\(([^)]+)\)`)
	rePptrPageSingle       = regexp.MustCompile(`await page\.\$\(([^)]+)\)`)
	rePptrWaitSelector     = regexp.MustCompile(`await page\.waitForSelector\(([^)]+)\)`)
	rePptrWaitNavigation   = regexp.MustCompile(`await page\.waitForNavigation\(\)`)
	rePptrSetViewport      = regexp.MustCompile(`await page\.setViewport\(\{`)
	rePptrSetCookie        = regexp.MustCompile(`await page\.setCookie\(`)
	rePptrCookies          = regexp.MustCompile(`await page\.cookies\(\)`)
	rePptrDeleteCookie     = regexp.MustCompile(`await page\.deleteCookie\(\)`)
	rePptrStandaloneSingle = regexp.MustCompile(`page\.\$\(([^)]+)\)`)
	rePptrBeforeAllCall    = regexp.MustCompile(`(?m)(^|[^\w.])beforeAll\(`)
	rePptrAfterAllCall     = regexp.MustCompile(`(?m)(^|[^\w.])afterAll\(`)
)

// ConvertPuppeteerToPlaywrightSource rewrites the high-confidence Puppeteer
// browser surface into Go-native Playwright output.
func ConvertPuppeteerToPlaywrightSource(source string) (string, error) {
	if strings.TrimSpace(source) == "" {
		return source, nil
	}
	if !strings.Contains(source, "puppeteer") &&
		!strings.Contains(source, "page.$") &&
		!strings.Contains(source, "page.type(") &&
		!strings.Contains(source, "page.waitForSelector(") {
		return ensureTrailingNewline(strings.ReplaceAll(source, "\r\n", "\n")), nil
	}

	result := strings.ReplaceAll(source, "\r\n", "\n")
	if astResult, ok := convertPuppeteerToPlaywrightSourceAST(result); ok {
		result = astResult
		result = cleanupConvertedPlaywrightOutput(result)
		result = prependImportPreservingHeader(result, "import { test, expect } from '@playwright/test';")
		return ensureTrailingNewline(result), nil
	}

	result = rePlaywrightTestImport.ReplaceAllString(result, "")
	result = rePptrRequireImport.ReplaceAllString(result, "")
	result = rePptrESMImport.ReplaceAllString(result, "")
	result = rePptrBrowserPageDecl.ReplaceAllString(result, "")
	result = rePptrBeforeAllBlock.ReplaceAllString(result, "\n")
	result = rePptrAfterAllBlock.ReplaceAllString(result, "\n")
	result = rePptrLaunchLine.ReplaceAllString(result, "")
	result = rePptrNewPageLine.ReplaceAllString(result, "")
	result = rePptrCloseLine.ReplaceAllString(result, "")

	if rePptrExpectURLMatch.MatchString(result) {
		result = rePptrExpectURLMatch.ReplaceAllStringFunc(result, func(match string) string {
			parts := rePptrExpectURLMatch.FindStringSubmatch(match)
			if len(parts) != 2 {
				return match
			}
			return "await expect(page).toHaveURL(" + playwrightPatternArg(parts[1]) + ")"
		})
	}
	if rePptrExpectTitleMatch.MatchString(result) {
		result = rePptrExpectTitleMatch.ReplaceAllStringFunc(result, func(match string) string {
			parts := rePptrExpectTitleMatch.FindStringSubmatch(match)
			if len(parts) != 2 {
				return match
			}
			return "await expect(page).toHaveTitle(" + playwrightPatternArg(parts[1]) + ")"
		})
	}

	assertionReplacements := []struct {
		re   *regexp.Regexp
		repl string
	}{
		{rePptrExpectURL, `await expect(page).toHaveURL($1)`},
		{rePptrExpectURLContain, `await expect(page).toHaveURL(new RegExp($1))`},
		{rePptrExpectTitle, `await expect(page).toHaveTitle($1)`},
		{rePptrExpectTruthy, `await expect(page.locator($1)).toBeVisible()`},
		{rePptrExpectFalsy, `await expect(page.locator($1)).toBeHidden()`},
		{rePptrExpectText, `await expect(page.locator($1)).toHaveText($2)`},
		{rePptrExpectContain, `await expect(page.locator($1)).toContainText($2)`},
		{rePptrExpectValue, `await expect(page.locator($1)).toHaveValue($2)`},
		{rePptrExpectCount, `await expect(page.locator($1)).toHaveCount($2)`},
	}
	for _, replacement := range assertionReplacements {
		result = replacement.re.ReplaceAllString(result, replacement.repl)
	}

	actionReplacements := []struct {
		re   *regexp.Regexp
		repl string
	}{
		{rePptrPageType, `await page.locator($1).fill($2)`},
		{rePptrPageClickDouble, `await page.locator($1).dblclick()`},
		{rePptrPageClick, `await page.locator($1).click()`},
		{rePptrPageHover, `await page.locator($1).hover()`},
		{rePptrPageSelect, `await page.locator($1).selectOption($2)`},
		{rePptrPageFocus, `await page.locator($1).focus()`},
		{rePptrWaitSelector, `await page.locator($1).waitFor()`},
		{rePptrSetViewport, `await page.setViewportSize({`},
		{rePptrSetCookie, `await page.context().addCookies(`},
		{rePptrCookies, `await page.context().cookies()`},
		{rePptrDeleteCookie, `await page.context().clearCookies()`},
	}
	for _, replacement := range actionReplacements {
		result = replacement.re.ReplaceAllString(result, replacement.repl)
	}

	result = rePptrPageEval.ReplaceAllString(result, `await page.locator($1).evaluate(`)
	result = rePptrPageEvalAll.ReplaceAllString(result, `await page.locator($1).evaluateAll(`)
	result = rePptrPageMany.ReplaceAllString(result, `page.locator($1)`)
	result = rePptrPageSingle.ReplaceAllString(result, `page.locator($1)`)
	result = rePptrWaitNavigation.ReplaceAllString(result, "")
	result = rePptrStandaloneSingle.ReplaceAllString(result, `page.locator($1)`)

	result = reDescribeOnly.ReplaceAllString(result, "${1}test.describe.only(")
	result = reDescribeSkip.ReplaceAllString(result, "${1}test.describe.skip(")
	result = reDescribe.ReplaceAllString(result, "${1}test.describe(")
	result = reItOnly.ReplaceAllString(result, "${1}test.only(")
	result = reItSkip.ReplaceAllString(result, "${1}test.skip(")
	result = reIt.ReplaceAllString(result, "${1}test(")
	result = reBeforeEach.ReplaceAllString(result, "${1}test.beforeEach(")
	result = reAfterEach.ReplaceAllString(result, "${1}test.afterEach(")
	result = rePptrBeforeAllCall.ReplaceAllString(result, "${1}test.beforeAll(")
	result = rePptrAfterAllCall.ReplaceAllString(result, "${1}test.afterAll(")

	result = rePlaywrightDescribeCallback.ReplaceAllString(result, `${1}() => {`)
	result = rePlaywrightTestEmptyCallback.ReplaceAllString(result, `${1}async ({ page }) => {`)
	result = rePlaywrightHookCallback.ReplaceAllString(result, `test.$1(async ({ page }) => {`)

	result = cleanupConvertedPlaywrightOutput(result)
	result = prependImportPreservingHeader(result, "import { test, expect } from '@playwright/test';")
	return ensureTrailingNewline(result), nil
}

func playwrightPatternArg(value string) string {
	if isJSRegexLiteral(value) {
		return value
	}
	return "new RegExp(" + value + ")"
}
