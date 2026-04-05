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
	rePptrExpectTitleCont  = regexp.MustCompile(`expect\(await\s+page\.title\(\)\)\.toContain\(([^)]+)\)`)
	rePptrExpectTitleMatch = regexp.MustCompile(`expect\(await\s+page\.title\(\)\)\.toMatch\(([^)]+)\)`)
	rePptrExpectTruthy     = regexp.MustCompile(`expect\(await\s+page\.\$\(([^)]+)\)\)\.toBeTruthy\(\)`)
	rePptrExpectFalsy      = regexp.MustCompile(`expect\(await\s+page\.\$\(([^)]+)\)\)\.toBeFalsy\(\)`)
	rePptrExpectText       = regexp.MustCompile(`expect\(await\s+page\.\$eval\(([^,]+),\s*el\s*=>\s*el\.textContent\)\)\.toBe\(([^)]+)\)`)
	rePptrExpectContain    = regexp.MustCompile(`expect\(await\s+page\.\$eval\(([^,]+),\s*el\s*=>\s*el\.textContent\)\)\.toContain\(([^)]+)\)`)
	rePptrExpectValue      = regexp.MustCompile(`expect\(await\s+page\.\$eval\(([^,]+),\s*el\s*=>\s*el\.value\)\)\.toBe\(([^)]+)\)`)
	rePptrExpectCount      = regexp.MustCompile(`expect\(\(await\s+page\.\$\$\(([^)]+)\)\)\.length\)\.toBe\(([^)]+)\)`)
	rePptrPageHoverLine    = regexp.MustCompile(`^\s*await\s+page\.hover\(`)
	rePptrPageFocusLine    = regexp.MustCompile(`^\s*await\s+page\.focus\(`)
	rePptrPageEvalCall     = regexp.MustCompile(`(?s)await\s+page\.\$eval\((.*?)\)`)
	rePptrPageEvalAllCall  = regexp.MustCompile(`(?s)await\s+page\.\$\$eval\((.*?)\)`)
	rePptrWaitSelectorLine = regexp.MustCompile(`^\s*await\s+page\.waitForSelector\(`)
	rePptrWaitNavigation   = regexp.MustCompile(`^\s*await\s+page\.waitForNavigation\(`)
	rePptrCookiesLine      = regexp.MustCompile(`^\s*await\s+page\.cookies\(`)
	rePptrDeleteCookieLine = regexp.MustCompile(`^\s*await\s+page\.deleteCookie\(`)
	rePptrSetCookieLine    = regexp.MustCompile(`^\s*await\s+page\.setCookie\(`)
	rePptrSetViewportLine  = regexp.MustCompile(`^\s*await\s+page\.setViewport\(`)
	rePptrPageTypeLine     = regexp.MustCompile(`^\s*await\s+page\.type\(`)
	rePptrPageClickLine    = regexp.MustCompile(`^\s*await\s+page\.click\(`)
	rePptrPageSelectLine   = regexp.MustCompile(`^\s*await\s+page\.select\(`)
	rePptrPageEvalLine     = regexp.MustCompile(`\bawait\s+page\.\$eval\(`)
	rePptrPageEvalAllLine  = regexp.MustCompile(`\bawait\s+page\.\$\$eval\(`)
	rePptrBeforeAllCall    = regexp.MustCompile(`(?m)(^|[^\w.])beforeAll\(`)
	rePptrAfterAllCall     = regexp.MustCompile(`(?m)(^|[^\w.])afterAll\(`)
	rePptrTestFnCallback   = regexp.MustCompile(`(test(?:\.(?:only|skip))?\([^,\n]+,\s*)(?:async\s*)?function\s*\(\s*\)\s*\{`)
	rePptrDescribeFnCB     = regexp.MustCompile(`(test\.describe(?:\.(?:only|skip))?\([^,\n]+,\s*)(?:async\s*)?function\s*\(\s*\)\s*\{`)
	rePptrHookFnCallback   = regexp.MustCompile(`test\.(beforeAll|afterAll|beforeEach|afterEach)\(\s*(?:async\s*)?function\s*\(\s*\)\s*\{`)
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

	result = replaceCodeRegexMatches(result, rePptrExpectURLMatch, func(_ string, groups []string) string {
		if len(groups) != 1 {
			return ""
		}
		return "await expect(page).toHaveURL(" + playwrightPatternArg(groups[0]) + ")"
	})
	result = replaceCodeRegexMatches(result, rePptrExpectTitleMatch, func(_ string, groups []string) string {
		if len(groups) != 1 {
			return ""
		}
		return "await expect(page).toHaveTitle(" + playwrightPatternArg(groups[0]) + ")"
	})
	result = replaceCodeRegexMatches(result, rePptrExpectURLContain, func(_ string, groups []string) string {
		if len(groups) != 1 {
			return ""
		}
		return "await expect(page).toHaveURL(" + playwrightPatternArg(groups[0]) + ")"
	})
	result = replaceCodeRegexMatches(result, rePptrExpectTitleCont, func(_ string, groups []string) string {
		if len(groups) != 1 {
			return ""
		}
		return "await expect(page).toHaveTitle(" + playwrightPatternArg(groups[0]) + ")"
	})

	assertionReplacements := []struct {
		re   *regexp.Regexp
		repl string
	}{
		{rePptrExpectURL, `await expect(page).toHaveURL($1)`},
		{rePptrExpectTitle, `await expect(page).toHaveTitle($1)`},
		{rePptrExpectTruthy, `await expect(page.locator($1)).toBeVisible()`},
		{rePptrExpectFalsy, `await expect(page.locator($1)).toBeHidden()`},
		{rePptrExpectText, `await expect(page.locator($1)).toHaveText($2)`},
		{rePptrExpectContain, `await expect(page.locator($1)).toContainText($2)`},
		{rePptrExpectValue, `await expect(page.locator($1)).toHaveValue($2)`},
		{rePptrExpectCount, `await expect(page.locator($1)).toHaveCount($2)`},
	}
	for _, replacement := range assertionReplacements {
		repl := replacement.repl
		result = replaceCodeRegexMatches(result, replacement.re, func(match string, _ []string) string {
			return replacement.re.ReplaceAllString(match, repl)
		})
	}

	result = rewritePuppeteerHoverCallsFallback(result)
	result = rewritePuppeteerFocusCallsFallback(result)
	result = rewritePuppeteerTypeCallsFallback(result)
	result = rewritePuppeteerClickCallsFallback(result)
	result = rewritePuppeteerSelectCallsFallback(result)
	result = rewritePuppeteerEvalCallsFallback(result)
	result = rewritePuppeteerEvalAllCallsFallback(result)
	result = rewritePuppeteerWaitForSelectorCallsFallback(result)
	result = rewritePuppeteerLocatorCallsFallback(result)
	result = rewritePuppeteerCookiesCallsFallback(result)
	result = rewritePuppeteerDeleteCookieCallsFallback(result)
	result = rewritePuppeteerSetViewportCallsFallback(result)
	result = rewritePuppeteerSetCookieCallsFallback(result)

	result = replaceCodeRegexString(result, reDescribeOnly, "${1}test.describe.only(")
	result = replaceCodeRegexString(result, reDescribeSkip, "${1}test.describe.skip(")
	result = replaceCodeRegexString(result, reDescribe, "${1}test.describe(")
	result = replaceCodeRegexString(result, reItOnly, "${1}test.only(")
	result = replaceCodeRegexString(result, reItSkip, "${1}test.skip(")
	result = replaceCodeRegexString(result, reIt, "${1}test(")
	result = replaceCodeRegexString(result, reBeforeEach, "${1}test.beforeEach(")
	result = replaceCodeRegexString(result, reAfterEach, "${1}test.afterEach(")
	result = replaceCodeRegexString(result, rePptrBeforeAllCall, "${1}test.beforeAll(")
	result = replaceCodeRegexString(result, rePptrAfterAllCall, "${1}test.afterAll(")

	result = replaceCodeRegexString(result, rePlaywrightDescribeCallback, `${1}() => {`)
	result = replaceCodeRegexString(result, rePlaywrightTestEmptyCallback, `${1}async ({ page }) => {`)
	result = replaceCodeRegexString(result, rePlaywrightHookCallback, `test.$1(async ({ page }) => {`)
	result = replaceCodeRegexString(result, rePptrDescribeFnCB, `${1}() => {`)
	result = replaceCodeRegexString(result, rePptrTestFnCallback, `${1}async ({ page }) => {`)
	result = replaceCodeRegexString(result, rePptrHookFnCallback, `test.$1(async ({ page }) => {`)

	result = commentUnsupportedPuppeteerPlaywrightFallbackLines(result)
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

func rewritePuppeteerWaitForSelectorCallsFallback(source string) string {
	return rewriteSourceCalls(source, "await page.waitForSelector(", func(args []string) (string, bool) {
		if replacement, ok := puppeteerWaitForSelectorArgsToPlaywright(args); ok {
			return replacement, true
		}
		return "", false
	})
}

func rewritePuppeteerLocatorCallsFallback(source string) string {
	rewrite := func(args []string) (string, bool) {
		if len(args) != 1 {
			return "", false
		}
		return "page.locator(" + args[0] + ")", true
	}

	source = rewriteSourceCalls(source, "await page.$(", rewrite)
	source = rewriteSourceCalls(source, "page.$(", rewrite)
	source = rewriteSourceCalls(source, "await page.$$(", rewrite)
	return rewriteSourceCalls(source, "page.$$(", rewrite)
}

func rewritePuppeteerHoverCallsFallback(source string) string {
	return rewriteSourceCalls(source, "await page.hover(", func(args []string) (string, bool) {
		if len(args) != 1 {
			return "", false
		}
		return "await page.locator(" + args[0] + ").hover()", true
	})
}

func rewritePuppeteerFocusCallsFallback(source string) string {
	return rewriteSourceCalls(source, "await page.focus(", func(args []string) (string, bool) {
		if len(args) != 1 {
			return "", false
		}
		return "await page.locator(" + args[0] + ").focus()", true
	})
}

func rewritePuppeteerTypeCallsFallback(source string) string {
	return rewriteSourceCalls(source, "await page.type(", func(args []string) (string, bool) {
		if replacement, ok := puppeteerTypeArgsToPlaywright(args); ok {
			return replacement, true
		}
		return "", false
	})
}

func rewritePuppeteerClickCallsFallback(source string) string {
	return rewriteSourceCalls(source, "await page.click(", func(args []string) (string, bool) {
		if replacement, ok := puppeteerClickArgsToPlaywright(args); ok {
			return replacement, true
		}
		return "", false
	})
}

func rewritePuppeteerSelectCallsFallback(source string) string {
	return rewriteSourceCalls(source, "await page.select(", func(args []string) (string, bool) {
		if replacement, ok := puppeteerSelectArgsToPlaywright(args); ok {
			return replacement, true
		}
		return "", false
	})
}

func rewritePuppeteerEvalCallsFallback(source string) string {
	return rewriteSourceCalls(source, "await page.$eval(", func(args []string) (string, bool) {
		return puppeteerEvalArgsToPlaywright(args, false)
	})
}

func rewritePuppeteerEvalAllCallsFallback(source string) string {
	return rewriteSourceCalls(source, "await page.$$eval(", func(args []string) (string, bool) {
		return puppeteerEvalArgsToPlaywright(args, true)
	})
}

func rewritePuppeteerSetCookieCallsFallback(source string) string {
	return rewriteSourceCalls(source, "await page.setCookie(", func(args []string) (string, bool) {
		if cookies, ok := puppeteerCookieArgsToPlaywright(args); ok {
			return "await page.context().addCookies(" + cookies + ")", true
		}
		return "", false
	})
}

func rewritePuppeteerCookiesCallsFallback(source string) string {
	return rewriteSourceCalls(source, "await page.cookies(", func(args []string) (string, bool) {
		if len(args) != 0 {
			return "", false
		}
		return "await page.context().cookies()", true
	})
}

func rewritePuppeteerDeleteCookieCallsFallback(source string) string {
	return rewriteSourceCalls(source, "await page.deleteCookie(", func(args []string) (string, bool) {
		if len(args) != 0 {
			return "", false
		}
		return "await page.context().clearCookies()", true
	})
}

func rewritePuppeteerSetViewportCallsFallback(source string) string {
	return rewriteSourceCalls(source, "await page.setViewport(", func(args []string) (string, bool) {
		if len(args) != 1 {
			return "", false
		}
		width, height, ok := parseViewportSizeArg(args[0])
		if !ok {
			return "", false
		}
		return "await page.setViewportSize({ width: " + width + ", height: " + height + " })", true
	})
}

func commentUnsupportedPuppeteerPlaywrightFallbackLines(source string) string {
	return commentMatchedLines(source, func(line string) bool {
		return rePptrPageTypeLine.MatchString(line) ||
			rePptrPageClickLine.MatchString(line) ||
			rePptrPageSelectLine.MatchString(line) ||
			rePptrPageEvalLine.MatchString(line) ||
			rePptrPageEvalAllLine.MatchString(line) ||
			rePptrPageHoverLine.MatchString(line) ||
			rePptrPageFocusLine.MatchString(line) ||
			rePptrWaitSelectorLine.MatchString(line) ||
			rePptrWaitNavigation.MatchString(line) ||
			rePptrCookiesLine.MatchString(line) ||
			rePptrDeleteCookieLine.MatchString(line) ||
			rePptrSetCookieLine.MatchString(line) ||
			rePptrSetViewportLine.MatchString(line)
	}, "manual Puppeteer conversion required")
}
