package convert

import (
	"regexp"
	"strings"
)

var (
	rePlaywrightTestImport = regexp.MustCompile(`(?m)^import\s+\{[^}]*\}\s+from\s+['"]@playwright/test['"];\s*\n?`)
	reCypressReference     = regexp.MustCompile(`(?m)^///\s*<reference\s+types="cypress"\s*/>\s*\n?`)

	reCyGetShouldVisible  = regexp.MustCompile(`cy\.get\(([^()\n]+)\)\.should\(['"]be\.visible['"]\)`)
	reCyGetShouldHidden   = regexp.MustCompile(`cy\.get\(([^()\n]+)\)\.should\(['"]not\.be\.visible['"]\)`)
	reCyGetShouldExist    = regexp.MustCompile(`cy\.get\(([^()\n]+)\)\.should\(['"]exist['"]\)`)
	reCyGetShouldNotExist = regexp.MustCompile(`cy\.get\(([^()\n]+)\)\.should\(['"]not\.exist['"]\)`)
	reCyGetShouldText     = regexp.MustCompile(`cy\.get\(([^()\n]+)\)\.should\(['"]have\.text['"],\s*([^()\n]+)\)`)
	reCyGetShouldContain  = regexp.MustCompile(`cy\.get\(([^()\n]+)\)\.should\(['"]contain(?:\.text)?['"],\s*([^()\n]+)\)`)
	reCyGetShouldValue    = regexp.MustCompile(`cy\.get\(([^()\n]+)\)\.should\(['"]have\.value['"],\s*([^()\n]+)\)`)
	reCyGetShouldChecked  = regexp.MustCompile(`cy\.get\(([^()\n]+)\)\.should\(['"]be\.checked['"]\)`)
	reCyGetShouldDisabled = regexp.MustCompile(`cy\.get\(([^()\n]+)\)\.should\(['"]be\.disabled['"]\)`)
	reCyGetShouldEnabled  = regexp.MustCompile(`cy\.get\(([^()\n]+)\)\.should\(['"]be\.enabled['"]\)`)
	reCyGetShouldClass    = regexp.MustCompile(`cy\.get\(([^()\n]+)\)\.should\(['"]have\.class['"],\s*([^()\n]+)\)`)
	reCyGetShouldLength   = regexp.MustCompile(`cy\.get\(([^()\n]+)\)\.should\(['"]have\.length['"],\s*([^()\n]+)\)`)
	reCyGetShouldLengthGT = regexp.MustCompile(`cy\.get\(([^()\n]+)\)\.should\(['"]have\.length\.greaterThan['"],\s*([^()\n]+)\)`)
	reCyGetShouldNotEmpty = regexp.MustCompile(`cy\.get\(([^()\n]+)\)\.should\(['"]not\.be\.empty['"]\)`)
	reCyContainsShouldVis = regexp.MustCompile(`cy\.contains\(([^()\n]+)\)\.should\(['"]be\.visible['"]\)`)
	reCyURLShouldInclude  = regexp.MustCompile(`cy\.url\(\)\.should\(['"]include['"],\s*([^()\n]+)\)`)
	reCyURLShouldEq       = regexp.MustCompile(`cy\.url\(\)\.should\(['"]eq['"],\s*([^()\n]+)\)`)
	reCyTitleShouldEq     = regexp.MustCompile(`cy\.title\(\)\.should\(['"]eq['"],\s*([^()\n]+)\)`)

	reCyGetClearType   = regexp.MustCompile(`cy\.get\(([^()\n]+)\)\.clear\(\)\.type\(([^()\n]+)\)`)
	reCyGetClick       = regexp.MustCompile(`cy\.get\(([^()\n]+)\)\.click\(\)`)
	reCyGetDoubleClick = regexp.MustCompile(`cy\.get\(([^()\n]+)\)\.dblclick\(\)`)
	reCyGetType        = regexp.MustCompile(`cy\.get\(([^()\n]+)\)\.type\(([^()\n]+)\)`)
	reCyGetClear       = regexp.MustCompile(`cy\.get\(([^()\n]+)\)\.clear\(\)`)
	reCyGetCheck       = regexp.MustCompile(`cy\.get\(([^()\n]+)\)\.check\(\)`)
	reCyGetUncheck     = regexp.MustCompile(`cy\.get\(([^()\n]+)\)\.uncheck\(\)`)
	reCyGetSelect      = regexp.MustCompile(`cy\.get\(([^()\n]+)\)\.select\(([^()\n]+)\)`)
	reCyGetFocus       = regexp.MustCompile(`cy\.get\(([^()\n]+)\)\.focus\(\)`)
	reCyGetBlur        = regexp.MustCompile(`cy\.get\(([^()\n]+)\)\.blur\(\)`)
	reCyGetScrollInto  = regexp.MustCompile(`cy\.get\(([^()\n]+)\)\.scrollIntoView\(\)`)
	reCyGetFirstClick  = regexp.MustCompile(`cy\.get\(([^()\n]+)\)\.first\(\)\.click\(\)`)
	reCyGetLastClick   = regexp.MustCompile(`cy\.get\(([^()\n]+)\)\.last\(\)\.click\(\)`)
	reCyGetEqClick     = regexp.MustCompile(`cy\.get\(([^()\n]+)\)\.eq\(([^()\n]+)\)\.click\(\)`)
	reCyContainsClick  = regexp.MustCompile(`cy\.contains\(([^()\n]+)\)\.click\(\)`)
	reCyContainsOnly   = regexp.MustCompile(`cy\.contains\(([^()\n]+)\)`)
	reCyVisit          = regexp.MustCompile(`cy\.visit\(([^()\n]+)\)`)
	reCyReload         = regexp.MustCompile(`cy\.reload\(\)`)
	reCyGoBack         = regexp.MustCompile(`cy\.go\(['"]back['"]\)`)
	reCyGoForward      = regexp.MustCompile(`cy\.go\(['"]forward['"]\)`)
	reCyViewport       = regexp.MustCompile(`cy\.viewport\(([^,\n]+),\s*([^()\n]+)\)`)
	reCyScreenshotPath = regexp.MustCompile(`cy\.screenshot\(([^()\n]+)\)`)
	reCyScreenshot     = regexp.MustCompile(`cy\.screenshot\(\)`)
	reCyGetFirst       = regexp.MustCompile(`cy\.get\(([^()\n]+)\)\.first\(\)`)
	reCyGetLast        = regexp.MustCompile(`cy\.get\(([^()\n]+)\)\.last\(\)`)
	reCyGetEq          = regexp.MustCompile(`cy\.get\(([^()\n]+)\)\.eq\(([^()\n]+)\)`)
	reCyGetOnly        = regexp.MustCompile(`cy\.get\(([^()\n]+)\)`)
	reCyWaitNumber     = regexp.MustCompile(`cy\.wait\((\d+)\)`)
	reCyJoinChains     = regexp.MustCompile(`cy\.(get|contains)\(([^)]*)\)\s*\n\s*\.`)
	reCyJoinMethods    = regexp.MustCompile(`\)\s*\n\s*\.(should|and|click|dblclick|type|clear|check|uncheck|select|focus|blur|first|last|eq|scrollIntoView)`)

	reDescribeOnly = regexp.MustCompile(`(?m)(^|[^\w.])describe\.only\(`)
	reDescribeSkip = regexp.MustCompile(`(?m)(^|[^\w.])describe\.skip\(`)
	reDescribe     = regexp.MustCompile(`(?m)(^|[^\w.])describe\(`)
	reContext      = regexp.MustCompile(`(?m)(^|[^\w.])context\(`)
	reItOnly       = regexp.MustCompile(`(?m)(^|[^\w.])it\.only\(`)
	reItSkip       = regexp.MustCompile(`(?m)(^|[^\w.])it\.skip\(`)
	reSpecify      = regexp.MustCompile(`(?m)(^|[^\w.])specify\(`)
	reIt           = regexp.MustCompile(`(?m)(^|[^\w.])it\(`)
	reBefore       = regexp.MustCompile(`(?m)(^|[^\w.])before\(`)
	reAfter        = regexp.MustCompile(`(?m)(^|[^\w.])after\(`)
	reBeforeEach   = regexp.MustCompile(`(?m)(^|[^\w.])beforeEach\(`)
	reAfterEach    = regexp.MustCompile(`(?m)(^|[^\w.])afterEach\(`)

	rePlaywrightTestEmptyCallback = regexp.MustCompile(`(test(?:\.(?:only|skip))?\([^,\n]+,\s*)(?:async\s*)?\(\s*\)\s*=>\s*\{`)
	rePlaywrightDescribeCallback  = regexp.MustCompile(`(test\.describe(?:\.(?:only|skip))?\([^,\n]+,\s*)(?:async\s*)?\(\s*\)\s*=>\s*\{`)
	rePlaywrightHookCallback      = regexp.MustCompile(`test\.(beforeAll|afterAll|beforeEach|afterEach)\(\s*(?:async\s*)?\(\s*\)\s*=>\s*\{`)

	reUnsupportedCyLine      = regexp.MustCompile(`\bcy\.`)
	reUnsupportedShouldLine  = regexp.MustCompile(`\.(?:should|and)\(`)
	reUnsupportedWithinLine  = regexp.MustCompile(`\.(?:within|each|its)\(`)
	reUnsupportedWrapLine    = regexp.MustCompile(`\bcy\.wrap\(`)
	reUnsupportedCypressLine = regexp.MustCompile(`\bCypress\.`)
)

// ConvertCypressToPlaywrightSource rewrites the high-confidence Cypress browser
// surface into Go-native Playwright output. Unsupported constructs are turned
// into explicit manual-review comments rather than being left as broken code.
func ConvertCypressToPlaywrightSource(source string) (string, error) {
	if strings.TrimSpace(source) == "" {
		return source, nil
	}
	if !strings.Contains(source, "cy.") && !reCypressReference.MatchString(source) {
		return ensureTrailingNewline(strings.ReplaceAll(source, "\r\n", "\n")), nil
	}

	result := strings.ReplaceAll(source, "\r\n", "\n")
	result = rePlaywrightTestImport.ReplaceAllString(result, "")
	result = reCypressReference.ReplaceAllString(result, "")

	retryWarning := false
	astApplied := false
	var astUnsupportedRows map[int]bool
	if astResult, ok := convertCypressToPlaywrightSourceAST(result); ok {
		result = astResult.source
		retryWarning = retryWarning || astResult.retryWarning
		astUnsupportedRows = astResult.unsupportedRows
		astApplied = true
	}

	if !astApplied {
		result = reCyJoinChains.ReplaceAllString(result, "cy.$1($2).")
		result = reCyJoinMethods.ReplaceAllString(result, ").$1")

		assertionReplacements := []struct {
			re   *regexp.Regexp
			repl string
		}{
			{reCyGetShouldVisible, `await expect(page.locator($1)).toBeVisible()`},
			{reCyGetShouldHidden, `await expect(page.locator($1)).toBeHidden()`},
			{reCyGetShouldExist, `await expect(page.locator($1)).toBeAttached()`},
			{reCyGetShouldNotExist, `await expect(page.locator($1)).not.toBeAttached()`},
			{reCyGetShouldText, `await expect(page.locator($1)).toHaveText($2)`},
			{reCyGetShouldContain, `await expect(page.locator($1)).toContainText($2)`},
			{reCyGetShouldValue, `await expect(page.locator($1)).toHaveValue($2)`},
			{reCyGetShouldChecked, `await expect(page.locator($1)).toBeChecked()`},
			{reCyGetShouldDisabled, `await expect(page.locator($1)).toBeDisabled()`},
			{reCyGetShouldEnabled, `await expect(page.locator($1)).toBeEnabled()`},
			{reCyGetShouldClass, `await expect(page.locator($1)).toHaveClass($2)`},
			{reCyGetShouldLength, `await expect(page.locator($1)).toHaveCount($2)`},
			{reCyGetShouldNotEmpty, `await expect(page.locator($1)).not.toBeEmpty()`},
			{reCyContainsShouldVis, `await expect(page.getByText($1)).toBeVisible()`},
			{reCyURLShouldInclude, `expect(page.url()).toContain($1)`},
			{reCyURLShouldEq, `expect(page.url()).toBe($1)`},
			{reCyTitleShouldEq, `await expect(page).toHaveTitle($1)`},
		}
		for _, replacement := range assertionReplacements {
			if replacement.re.MatchString(result) {
				retryWarning = true
				result = replacement.re.ReplaceAllString(result, replacement.repl)
			}
		}
		if reCyGetShouldLengthGT.MatchString(result) {
			retryWarning = true
			result = reCyGetShouldLengthGT.ReplaceAllString(result, `expect(await page.locator($1).count()).toBeGreaterThan($2)`)
		}

		actionReplacements := []struct {
			re   *regexp.Regexp
			repl string
		}{
			{reCyGetClearType, `await page.locator($1).fill($2)`},
			{reCyGetFirstClick, `await page.locator($1).first().click()`},
			{reCyGetLastClick, `await page.locator($1).last().click()`},
			{reCyGetEqClick, `await page.locator($1).nth($2).click()`},
			{reCyGetClick, `await page.locator($1).click()`},
			{reCyGetDoubleClick, `await page.locator($1).dblclick()`},
			{reCyGetType, `await page.locator($1).fill($2)`},
			{reCyGetClear, `await page.locator($1).clear()`},
			{reCyGetCheck, `await page.locator($1).check()`},
			{reCyGetUncheck, `await page.locator($1).uncheck()`},
			{reCyGetSelect, `await page.locator($1).selectOption($2)`},
			{reCyGetFocus, `await page.locator($1).focus()`},
			{reCyGetBlur, `await page.locator($1).blur()`},
			{reCyGetScrollInto, `await page.locator($1).scrollIntoViewIfNeeded()`},
			{reCyContainsClick, `await page.getByText($1).click()`},
			{reCyVisit, `await page.goto($1)`},
			{reCyReload, `await page.reload()`},
			{reCyGoBack, `await page.goBack()`},
			{reCyGoForward, `await page.goForward()`},
			{reCyViewport, `await page.setViewportSize({ width: $1, height: $2 })`},
			{reCyScreenshotPath, `await page.screenshot({ path: $1 })`},
			{reCyScreenshot, `await page.screenshot()`},
			{reCyWaitNumber, `await page.waitForTimeout($1)`},
			{reCyGetFirst, `page.locator($1).first()`},
			{reCyGetLast, `page.locator($1).last()`},
			{reCyGetEq, `page.locator($1).nth($2)`},
			{reCyContainsOnly, `page.getByText($1)`},
			{reCyGetOnly, `page.locator($1)`},
		}
		for _, replacement := range actionReplacements {
			result = replacement.re.ReplaceAllString(result, replacement.repl)
		}

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
	}

	if astApplied {
		if len(astUnsupportedRows) > 0 {
			result = commentSpecificLines(result, astUnsupportedRows, "manual Cypress conversion required")
		}
	} else {
		result = commentUnsupportedCypressLines(result)
	}

	prelude := "import { test, expect } from '@playwright/test';"
	if retryWarning {
		prelude += "\n// TERRAIN-WARNING: Cypress .should() retries until timeout; review Playwright expect() semantics."
	}
	result = cleanupConvertedPlaywrightOutput(result)
	result = prependImportPreservingHeader(result, prelude)
	return ensureTrailingNewline(result), nil
}

func commentUnsupportedCypressLines(source string) string {
	if lines, ok := unsupportedCypressLineRowsAST(source); ok {
		if len(lines) == 0 {
			return source
		}
		return commentSpecificLines(source, lines, "manual Cypress conversion required")
	}

	lines := strings.Split(source, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "//") {
			continue
		}
		if reUnsupportedCyLine.MatchString(line) ||
			reUnsupportedShouldLine.MatchString(line) ||
			reUnsupportedWithinLine.MatchString(line) ||
			reUnsupportedWrapLine.MatchString(line) ||
			reUnsupportedCypressLine.MatchString(line) {
			indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
			lines[i] = indent + "// TERRAIN-TODO: manual Cypress conversion required\n" + indent + "// " + strings.TrimSpace(line)
		}
	}
	return strings.Join(lines, "\n")
}

func commentSpecificLines(source string, rows map[int]bool, todo string) string {
	lines := strings.Split(source, "\n")
	for i, line := range lines {
		if !rows[i] {
			continue
		}
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "//") {
			continue
		}
		indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
		lines[i] = indent + "// TERRAIN-TODO: " + todo + "\n" + indent + "// " + strings.TrimSpace(line)
	}
	return strings.Join(lines, "\n")
}

func cleanupConvertedPlaywrightOutput(source string) string {
	for strings.Contains(source, "\n\n\n") {
		source = strings.ReplaceAll(source, "\n\n\n", "\n\n")
	}
	return strings.TrimSpace(source) + "\n"
}
