package convert

import (
	"regexp"
	"strings"
)

var (
	reWdioGlobalsImport = regexp.MustCompile(`(?m)^import\s+\{[^}]*\}\s+from\s+['"]@wdio\/globals['"];\s*\n?`)
	reWdioImportLine    = regexp.MustCompile(`(?m)^import\s+\{[^}]*\}\s+from\s+['"]webdriverio['"];\s*\n?`)

	reWdioBrowserURL           = regexp.MustCompile(`await expect\(browser\)\.toHaveUrl\(([^)]+)\)`)
	reWdioBrowserURLContaining = regexp.MustCompile(`await expect\(browser\)\.toHaveUrlContaining\(([^)]+)\)`)
	reWdioBrowserTitle         = regexp.MustCompile(`await expect\(browser\)\.toHaveTitle\(([^)]+)\)`)
	reWdioDisplayed            = regexp.MustCompile(`await expect\(\$\(([^)]+)\)\)\.toBeDisplayed\(\)`)
	reWdioNotDisplayed         = regexp.MustCompile(`await expect\(\$\(([^)]+)\)\)\.not\.toBeDisplayed\(\)`)
	reWdioExists               = regexp.MustCompile(`await expect\(\$\(([^)]+)\)\)\.toExist\(\)`)
	reWdioNotExists            = regexp.MustCompile(`await expect\(\$\(([^)]+)\)\)\.not\.toExist\(\)`)
	reWdioHaveText             = regexp.MustCompile(`await expect\(\$\(([^)]+)\)\)\.toHaveText\(([^)]+)\)`)
	reWdioHaveTextContaining   = regexp.MustCompile(`await expect\(\$\(([^)]+)\)\)\.toHaveTextContaining\(([^)]+)\)`)
	reWdioHaveValue            = regexp.MustCompile(`await expect\(\$\(([^)]+)\)\)\.toHaveValue\(([^)]+)\)`)
	reWdioElementsArraySize    = regexp.MustCompile(`await expect\(\$\$\(([^)]+)\)\)\.toBeElementsArrayOfSize\(([^)]+)\)`)
	reWdioSelected             = regexp.MustCompile(`await expect\(\$\(([^)]+)\)\)\.toBeSelected\(\)`)
	reWdioEnabled              = regexp.MustCompile(`await expect\(\$\(([^)]+)\)\)\.toBeEnabled\(\)`)
	reWdioDisabled             = regexp.MustCompile(`await expect\(\$\(([^)]+)\)\)\.toBeDisabled\(\)`)
	reWdioHaveAttribute        = regexp.MustCompile(`await expect\(\$\(([^)]+)\)\)\.toHaveAttribute\(([^,]+),\s*([^)]+)\)`)

	reWdioExactTextSelectorSingle   = regexp.MustCompile(`\$\('=([^'\n]+)'\)`)
	reWdioExactTextSelectorDouble   = regexp.MustCompile(`\$\("=([^"\n]+)"\)`)
	reWdioPartialTextSelectorSingle = regexp.MustCompile(`\$\('\*=([^'\n]+)'\)`)
	reWdioPartialTextSelectorDouble = regexp.MustCompile(`\$\("\*=([^"\n]+)"\)`)

	reWdioSetValue            = regexp.MustCompile(`await \$\(([^)]+)\)\.setValue\(([^)]+)\)`)
	reWdioClick               = regexp.MustCompile(`await \$\(([^)]+)\)\.click\(\)`)
	reWdioDoubleClick         = regexp.MustCompile(`await \$\(([^)]+)\)\.doubleClick\(\)`)
	reWdioClearValue          = regexp.MustCompile(`await \$\(([^)]+)\)\.clearValue\(\)`)
	reWdioMoveTo              = regexp.MustCompile(`await \$\(([^)]+)\)\.moveTo\(\)`)
	reWdioGetText             = regexp.MustCompile(`await \$\(([^)]+)\)\.getText\(\)`)
	reWdioIsDisplayed         = regexp.MustCompile(`await \$\(([^)]+)\)\.isDisplayed\(\)`)
	reWdioIsExisting          = regexp.MustCompile(`await \$\(([^)]+)\)\.isExisting\(\)`)
	reWdioWaitForDisplayed    = regexp.MustCompile(`await \$\(([^)]+)\)\.waitForDisplayed\(\)`)
	reWdioWaitForExist        = regexp.MustCompile(`await \$\(([^)]+)\)\.waitForExist\(\)`)
	reWdioSelectByVisibleText = regexp.MustCompile(`await \$\(([^)]+)\)\.selectByVisibleText\(([^)]+)\)`)
	reWdioSelectByValue       = regexp.MustCompile(`await \$\(([^)]+)\)\.selectByAttribute\(['"]value['"],\s*([^)]+)\)`)
	reWdioGetAttribute        = regexp.MustCompile(`await \$\(([^)]+)\)\.getAttribute\(([^)]+)\)`)
	reWdioManySelectors       = regexp.MustCompile(`\$\$\(([^)]+)\)`)
	reWdioSingleSelectors     = regexp.MustCompile(`\$\(([^)]+)\)`)

	reWdioBrowserGoto          = regexp.MustCompile(`await browser\.url\(([^)]+)\)`)
	reWdioBrowserPause         = regexp.MustCompile(`await browser\.pause\(([^)]+)\)`)
	reWdioBrowserExecute       = regexp.MustCompile(`await browser\.execute\(`)
	reWdioBrowserRefresh       = regexp.MustCompile(`await browser\.refresh\(\)`)
	reWdioBrowserBack          = regexp.MustCompile(`await browser\.back\(\)`)
	reWdioBrowserForward       = regexp.MustCompile(`await browser\.forward\(\)`)
	reWdioBrowserGetTitle      = regexp.MustCompile(`await browser\.getTitle\(\)`)
	reWdioBrowserGetURL        = regexp.MustCompile(`await browser\.getUrl\(\)`)
	reWdioBrowserKeysCall      = regexp.MustCompile(`await browser\.keys\(([^)]*)\)`)
	reWdioBrowserSetCookies    = regexp.MustCompile(`await browser\.setCookies\(([^)]*)\)`)
	reWdioBrowserGetCookies    = regexp.MustCompile(`await browser\.getCookies\(([^)]*)\)`)
	reWdioBrowserDeleteCookies = regexp.MustCompile(`await browser\.deleteCookies\(([^)]*)\)`)
	reWdioBrowserMock          = regexp.MustCompile(`await browser\.mock\(`)

	reUnsupportedWdioLine = regexp.MustCompile(`(?:\bbrowser\.|expect\(\s*browser\s*\)|expect\(\s*\$\(|expect\(\s*\$\$\(|\$\(|\$\$\(|@wdio/|webdriverio\b)`)
)

// ConvertWdioToPlaywrightSource rewrites the high-confidence WebdriverIO
// browser surface into Go-native Playwright output. Unsupported constructs are
// preserved as explicit TODO comments for manual follow-up.
func ConvertWdioToPlaywrightSource(source string) (string, error) {
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
	result = rePlaywrightTestImport.ReplaceAllString(result, "")
	result = reWdioGlobalsImport.ReplaceAllString(result, "")
	result = reWdioImportLine.ReplaceAllString(result, "")
	astApplied := false
	var astUnsupportedRows map[int]bool
	if astResult, ok := convertWdioToPlaywrightSourceAST(result); ok {
		result = astResult.source
		astUnsupportedRows = astResult.unsupportedRows
		astApplied = true
	}

	if !astApplied {
		retryWarning := false
		assertionReplacements := []struct {
			re   *regexp.Regexp
			repl string
		}{
			{reWdioBrowserURL, `await expect(page).toHaveURL($1)`},
			{reWdioBrowserURLContaining, `await expect(page).toHaveURL(new RegExp($1))`},
			{reWdioBrowserTitle, `await expect(page).toHaveTitle($1)`},
			{reWdioNotDisplayed, `await expect(page.locator($1)).toBeHidden()`},
			{reWdioDisplayed, `await expect(page.locator($1)).toBeVisible()`},
			{reWdioNotExists, `await expect(page.locator($1)).not.toBeAttached()`},
			{reWdioExists, `await expect(page.locator($1)).toBeAttached()`},
			{reWdioHaveTextContaining, `await expect(page.locator($1)).toContainText($2)`},
			{reWdioHaveText, `await expect(page.locator($1)).toHaveText($2)`},
			{reWdioHaveValue, `await expect(page.locator($1)).toHaveValue($2)`},
			{reWdioElementsArraySize, `await expect(page.locator($1)).toHaveCount($2)`},
			{reWdioSelected, `await expect(page.locator($1)).toBeChecked()`},
			{reWdioEnabled, `await expect(page.locator($1)).toBeEnabled()`},
			{reWdioDisabled, `await expect(page.locator($1)).toBeDisabled()`},
			{reWdioHaveAttribute, `await expect(page.locator($1)).toHaveAttribute($2, $3)`},
		}
		for _, replacement := range assertionReplacements {
			if replacement.re.MatchString(result) {
				retryWarning = true
				result = replacement.re.ReplaceAllString(result, replacement.repl)
			}
		}

		result = reWdioPartialTextSelectorSingle.ReplaceAllString(result, `page.getByText('$1')`)
		result = reWdioPartialTextSelectorDouble.ReplaceAllString(result, `page.getByText("$1")`)
		result = reWdioExactTextSelectorSingle.ReplaceAllString(result, `page.getByText('$1')`)
		result = reWdioExactTextSelectorDouble.ReplaceAllString(result, `page.getByText("$1")`)

		actionReplacements := []struct {
			re   *regexp.Regexp
			repl string
		}{
			{reWdioSetValue, `await page.locator($1).fill($2)`},
			{reWdioClick, `await page.locator($1).click()`},
			{reWdioDoubleClick, `await page.locator($1).dblclick()`},
			{reWdioClearValue, `await page.locator($1).clear()`},
			{reWdioMoveTo, `await page.locator($1).hover()`},
			{reWdioGetText, `await page.locator($1).textContent()`},
			{reWdioIsDisplayed, `await page.locator($1).isVisible()`},
			{reWdioIsExisting, `await page.locator($1).isVisible()`},
			{reWdioWaitForDisplayed, `await page.locator($1).waitFor({ state: 'visible' })`},
			{reWdioWaitForExist, `await page.locator($1).waitFor()`},
			{reWdioSelectByVisibleText, `await page.locator($1).selectOption({ label: $2 })`},
			{reWdioSelectByValue, `await page.locator($1).selectOption($2)`},
			{reWdioGetAttribute, `await page.locator($1).getAttribute($2)`},
			{reWdioBrowserGoto, `await page.goto($1)`},
			{reWdioBrowserPause, `await page.waitForTimeout($1)`},
			{reWdioBrowserRefresh, `await page.reload()`},
			{reWdioBrowserBack, `await page.goBack()`},
			{reWdioBrowserForward, `await page.goForward()`},
			{reWdioBrowserGetTitle, `await page.title()`},
			{reWdioBrowserGetURL, `page.url()`},
		}
		for _, replacement := range actionReplacements {
			result = replacement.re.ReplaceAllString(result, replacement.repl)
		}

		if reWdioBrowserKeysCall.MatchString(result) {
			result = reWdioBrowserKeysCall.ReplaceAllStringFunc(result, func(match string) string {
				parts := reWdioBrowserKeysCall.FindStringSubmatch(match)
				if len(parts) != 2 {
					return match
				}
				replacement, ok := wdioBrowserKeysArgToPlaywright(parts[1])
				if !ok {
					return match
				}
				return replacement
			})
		}
		if reWdioBrowserSetCookies.MatchString(result) {
			result = reWdioBrowserSetCookies.ReplaceAllStringFunc(result, func(match string) string {
				parts := reWdioBrowserSetCookies.FindStringSubmatch(match)
				if len(parts) != 2 {
					return match
				}
				cookies, ok := wdioCookieArgToPlaywright(parts[1])
				if !ok {
					return match
				}
				return "await page.context().addCookies(" + cookies + ")"
			})
		}
		if reWdioBrowserGetCookies.MatchString(result) {
			result = reWdioBrowserGetCookies.ReplaceAllStringFunc(result, func(match string) string {
				parts := reWdioBrowserGetCookies.FindStringSubmatch(match)
				if len(parts) != 2 {
					return match
				}
				if strings.TrimSpace(parts[1]) != "" {
					return match
				}
				return "await page.context().cookies()"
			})
		}
		if reWdioBrowserDeleteCookies.MatchString(result) {
			result = reWdioBrowserDeleteCookies.ReplaceAllStringFunc(result, func(match string) string {
				parts := reWdioBrowserDeleteCookies.FindStringSubmatch(match)
				if len(parts) != 2 {
					return match
				}
				if strings.TrimSpace(parts[1]) != "" {
					return match
				}
				return "await page.context().clearCookies()"
			})
		}

		result = reWdioBrowserExecute.ReplaceAllString(result, `await page.evaluate(`)
		result = reWdioManySelectors.ReplaceAllString(result, `page.locator($1)`)
		result = reWdioSingleSelectors.ReplaceAllString(result, `page.locator($1)`)

		result = reDescribeOnly.ReplaceAllString(result, "${1}test.describe.only(")
		result = reDescribeSkip.ReplaceAllString(result, "${1}test.describe.skip(")
		result = reDescribe.ReplaceAllString(result, "${1}test.describe(")
		result = reContext.ReplaceAllString(result, "${1}test.describe(")
		result = reItOnly.ReplaceAllString(result, "${1}test.only(")
		result = reItSkip.ReplaceAllString(result, "${1}test.skip(")
		result = reSpecify.ReplaceAllString(result, "${1}test(")
		result = reIt.ReplaceAllString(result, "${1}test(")
		result = reBeforeEach.ReplaceAllString(result, "${1}test.beforeEach(")
		result = reAfterEach.ReplaceAllString(result, "${1}test.afterEach(")
		result = reBefore.ReplaceAllString(result, "${1}test.beforeAll(")
		result = reAfter.ReplaceAllString(result, "${1}test.afterAll(")

		result = rePlaywrightDescribeCallback.ReplaceAllString(result, `${1}() => {`)
		result = rePlaywrightTestEmptyCallback.ReplaceAllString(result, `${1}async ({ page }) => {`)
		result = rePlaywrightHookCallback.ReplaceAllString(result, `test.$1(async ({ page }) => {`)
		if retryWarning {
			// Keep output stable for parity; the benchmark and tests carry the warning signal.
		}
	}

	if astApplied {
		if len(astUnsupportedRows) > 0 {
			result = commentSpecificLines(result, astUnsupportedRows, "manual WebdriverIO conversion required")
		}
	} else {
		result = commentUnsupportedWdioLines(result)
	}
	result = cleanupConvertedPlaywrightOutput(result)
	result = prependImportPreservingHeader(result, "import { test, expect } from '@playwright/test';")
	return ensureTrailingNewline(result), nil
}

func commentUnsupportedWdioLines(source string) string {
	if rows, ok := unsupportedWdioLineRowsAST(source); ok {
		if len(rows) == 0 {
			return source
		}
		return commentSpecificLines(source, rows, "manual WebdriverIO conversion required")
	}

	lines := strings.Split(source, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "//") {
			continue
		}
		if reWdioBrowserMock.MatchString(line) || reUnsupportedWdioLine.MatchString(line) {
			indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
			lines[i] = indent + "// TERRAIN-TODO: manual WebdriverIO conversion required\n" + indent + "// " + strings.TrimSpace(line)
		}
	}
	return strings.Join(lines, "\n")
}

func wdioCookieArgToPlaywright(arg string) (string, bool) {
	arg = strings.TrimSpace(arg)
	switch {
	case strings.HasPrefix(arg, "[") && strings.HasSuffix(arg, "]"):
		return arg, true
	case strings.HasPrefix(arg, "{") && strings.HasSuffix(arg, "}"):
		return "[" + arg + "]", true
	default:
		return "", false
	}
}

func wdioBrowserKeysArgToPlaywright(arg string) (string, bool) {
	arg = strings.TrimSpace(arg)
	if isJSStringLiteral(arg) {
		return "await page.keyboard.press(" + arg + ")", true
	}
	if strings.HasPrefix(arg, "[") && strings.HasSuffix(arg, "]") {
		items := splitTopLevelArgs(arg[1 : len(arg)-1])
		if len(items) == 1 && isJSStringLiteral(strings.TrimSpace(items[0])) {
			return "await page.keyboard.press(" + strings.TrimSpace(items[0]) + ")", true
		}
	}
	return "", false
}

func isJSStringLiteral(value string) bool {
	value = strings.TrimSpace(value)
	if len(value) < 2 {
		return false
	}
	quote := value[0]
	switch quote {
	case '\'', '"', '`':
		return value[len(value)-1] == quote
	default:
		return false
	}
}
