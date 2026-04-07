package convert

import (
	"regexp"
	"strings"
)

var (
	reCyToWdioGetVisible     = regexp.MustCompile(`cy\.get\(([^)]+)\)\.should\(['"]be\.visible['"]\)`)
	reCyToWdioGetHidden      = regexp.MustCompile(`cy\.get\(([^)]+)\)\.should\(['"]not\.be\.visible['"]\)`)
	reCyToWdioGetExist       = regexp.MustCompile(`cy\.get\(([^)]+)\)\.should\(['"]exist['"]\)`)
	reCyToWdioGetNotExist    = regexp.MustCompile(`cy\.get\(([^)]+)\)\.should\(['"]not\.exist['"]\)`)
	reCyToWdioGetText        = regexp.MustCompile(`cy\.get\(([^)]+)\)\.should\(['"]have\.text['"],\s*([^)]+)\)`)
	reCyToWdioGetContain     = regexp.MustCompile(`cy\.get\(([^)]+)\)\.should\(['"]contain['"],\s*([^)]+)\)`)
	reCyToWdioGetValue       = regexp.MustCompile(`cy\.get\(([^)]+)\)\.should\(['"]have\.value['"],\s*([^)]+)\)`)
	reCyToWdioGetLength      = regexp.MustCompile(`cy\.get\(([^)]+)\)\.should\(['"]have\.length['"],\s*(\d+)\)`)
	reCyToWdioGetChecked     = regexp.MustCompile(`cy\.get\(([^)]+)\)\.should\(['"]be\.checked['"]\)`)
	reCyToWdioGetDisabled    = regexp.MustCompile(`cy\.get\(([^)]+)\)\.should\(['"]be\.disabled['"]\)`)
	reCyToWdioGetEnabled     = regexp.MustCompile(`cy\.get\(([^)]+)\)\.should\(['"]be\.enabled['"]\)`)
	reCyToWdioGetAttribute   = regexp.MustCompile(`cy\.get\(([^)]+)\)\.should\(['"]have\.attr['"],\s*([^,]+),\s*([^)]+)\)`)
	reCyToWdioClearType      = regexp.MustCompile(`cy\.get\(([^)]+)\)\.clear\(\)\.type\(([^)]+)\)`)
	reCyToWdioType           = regexp.MustCompile(`cy\.get\(([^)]+)\)\.type\(([^)]+)\)`)
	reCyToWdioClick          = regexp.MustCompile(`cy\.get\(([^)]+)\)\.click\(\)`)
	reCyToWdioDoubleClick    = regexp.MustCompile(`cy\.get\(([^)]+)\)\.dblclick\(\)`)
	reCyToWdioClear          = regexp.MustCompile(`cy\.get\(([^)]+)\)\.clear\(\)`)
	reCyToWdioSelect         = regexp.MustCompile(`cy\.get\(([^)]+)\)\.select\(([^)]+)\)`)
	reCyToWdioCheck          = regexp.MustCompile(`cy\.get\(([^)]+)\)\.check\(\)`)
	reCyToWdioUncheck        = regexp.MustCompile(`cy\.get\(([^)]+)\)\.uncheck\(\)`)
	reCyToWdioHover          = regexp.MustCompile(`cy\.get\(([^)]+)\)\.trigger\(['"]mouseover['"]\)`)
	reCyToWdioInvokeText     = regexp.MustCompile(`cy\.get\(([^)]+)\)\.invoke\(['"]text['"]\)`)
	reCyToWdioInvokeAttr     = regexp.MustCompile(`cy\.get\(([^)]+)\)\.invoke\(['"]attr['"],\s*([^)]+)\)`)
	reCyToWdioContainsClick  = regexp.MustCompile(`cy\.contains\('([^'\n]+)'\)\.click\(\)`)
	reCyToWdioContainsClickD = regexp.MustCompile(`cy\.contains\("([^"\n]+)"\)\.click\(\)`)
	reCyToWdioContainsS      = regexp.MustCompile(`cy\.contains\('([^'\n]+)'\)`)
	reCyToWdioContainsD      = regexp.MustCompile(`cy\.contains\("([^"\n]+)"\)`)
	reCyToWdioVisit          = regexp.MustCompile(`cy\.visit\(([^)]+)\)`)
	reCyToWdioReload         = regexp.MustCompile(`cy\.reload\(\)`)
	reCyToWdioBack           = regexp.MustCompile(`cy\.go\(['"]back['"]\)`)
	reCyToWdioForward        = regexp.MustCompile(`cy\.go\(['"]forward['"]\)`)
	reCyToWdioURLInclude     = regexp.MustCompile(`cy\.url\(\)\.should\(['"]include['"],\s*([^)]+)\)`)
	reCyToWdioURLEq          = regexp.MustCompile(`cy\.url\(\)\.should\(['"]eq['"],\s*([^)]+)\)`)
	reCyToWdioTitleEq        = regexp.MustCompile(`cy\.title\(\)\.should\(['"]eq['"],\s*([^)]+)\)`)
	reCyToWdioWait           = regexp.MustCompile(`cy\.wait\((\d+)\)`)
	reCyToWdioClearCookies   = regexp.MustCompile(`cy\.clearCookies\(\)`)
	reCyToWdioGetCookies     = regexp.MustCompile(`cy\.getCookies\(\)`)
	reCyToWdioClearStorage   = regexp.MustCompile(`cy\.clearLocalStorage\(\)`)
	reCyToWdioLog            = regexp.MustCompile(`cy\.log\(([^)]+)\)`)
	reCyToWdioWindowThen     = regexp.MustCompile(`cy\.window\(\)\.then\(([^)]+)\)`)
	reCyToWdioUnsupported    = regexp.MustCompile(`\b(?:cy\.intercept|cy\.request|cy\.session|cy\.task|cy\.origin|cy\.wrap|Cypress\.)`)
	reCyToWdioTestCallback   = regexp.MustCompile(`((?:it|test|specify|it\.only|it\.skip)\s*\([^,]+,\s*)\(\s*\)\s*=>\s*\{`)
	reCyToWdioHookCallback   = regexp.MustCompile(`((?:beforeEach|afterEach|before|after)\s*\(\s*)\(\s*\)\s*=>\s*\{`)
)

// ConvertCypressToWdioSource rewrites the high-confidence Cypress browser
// surface into Go-native WebdriverIO output. Unsupported constructs are
// preserved as explicit TODO comments for manual follow-up.
func ConvertCypressToWdioSource(source string) (string, error) {
	if strings.TrimSpace(source) == "" {
		return source, nil
	}
	if !strings.Contains(source, "cy.") && !reCypressReference.MatchString(source) {
		return ensureTrailingNewline(strings.ReplaceAll(source, "\r\n", "\n")), nil
	}

	result := strings.ReplaceAll(source, "\r\n", "\n")
	result = reCypressReference.ReplaceAllString(result, "")
	astApplied := false
	if astResult, ok := convertCypressToWdioSourceAST(result); ok {
		result = astResult
		astApplied = true
	}

	if !astApplied {
		assertionReplacements := []struct {
			re   *regexp.Regexp
			repl string
		}{
			{reCyToWdioGetVisible, `await expect($($1)).toBeDisplayed()`},
			{reCyToWdioGetHidden, `await expect($($1)).not.toBeDisplayed()`},
			{reCyToWdioGetExist, `await expect($($1)).toExist()`},
			{reCyToWdioGetNotExist, `await expect($($1)).not.toExist()`},
			{reCyToWdioGetText, `await expect($($1)).toHaveText($2)`},
			{reCyToWdioGetContain, `await expect($($1)).toHaveTextContaining($2)`},
			{reCyToWdioGetValue, `await expect($($1)).toHaveValue($2)`},
			{reCyToWdioGetLength, `await expect($$$$($1)).toBeElementsArrayOfSize($2)`},
			{reCyToWdioGetChecked, `await expect($($1)).toBeSelected()`},
			{reCyToWdioGetDisabled, `await expect($($1)).toBeDisabled()`},
			{reCyToWdioGetEnabled, `await expect($($1)).toBeEnabled()`},
			{reCyToWdioGetAttribute, `await expect($($1)).toHaveAttribute($2, $3)`},
			{reCyToWdioURLInclude, `await expect(browser).toHaveUrlContaining($1)`},
			{reCyToWdioURLEq, `await expect(browser).toHaveUrl($1)`},
			{reCyToWdioTitleEq, `await expect(browser).toHaveTitle($1)`},
		}
		for _, replacement := range assertionReplacements {
			result = replaceCodeRegexString(result, replacement.re, replacement.repl)
		}

		actionReplacements := []struct {
			re   *regexp.Regexp
			repl string
		}{
			{reCyToWdioClearType, `await $($1).setValue($2)`},
			{reCyToWdioType, `await $($1).setValue($2)`},
			{reCyToWdioClick, `await $($1).click()`},
			{reCyToWdioDoubleClick, `await $($1).doubleClick()`},
			{reCyToWdioClear, `await $($1).clearValue()`},
			{reCyToWdioSelect, `await $($1).selectByVisibleText($2)`},
			{reCyToWdioCheck, `await $($1).click()`},
			{reCyToWdioUncheck, `await $($1).click()`},
			{reCyToWdioHover, `await $($1).moveTo()`},
			{reCyToWdioInvokeText, `await $($1).getText()`},
			{reCyToWdioInvokeAttr, `await $($1).getAttribute($2)`},
			{reCyToWdioVisit, `await browser.url($1)`},
			{reCyToWdioReload, `await browser.refresh()`},
			{reCyToWdioBack, `await browser.back()`},
			{reCyToWdioForward, `await browser.forward()`},
			{reCyToWdioWait, `await browser.pause($1)`},
			{reCyToWdioClearCookies, `await browser.deleteCookies()`},
			{reCyToWdioGetCookies, `await browser.getCookies()`},
			{reCyToWdioClearStorage, `await browser.execute(() => localStorage.clear())`},
			{reCyToWdioLog, `console.log($1)`},
			{reCyToWdioWindowThen, `await browser.execute($1)`},
		}
		for _, replacement := range actionReplacements {
			result = replaceCodeRegexString(result, replacement.re, replacement.repl)
		}

		result = replaceCodeRegexString(result, reCyToWdioContainsClick, "await $(`*=$1`).click()")
		result = replaceCodeRegexString(result, reCyToWdioContainsClickD, "await $(`*=$1`).click()")
		result = replaceCodeRegexString(result, reCyToWdioContainsS, "$(`*=$1`)")
		result = replaceCodeRegexString(result, reCyToWdioContainsD, "$(`*=$1`)")

		result = commentUnsupportedCypressWdioLines(result)
		result = replaceCodeRegexString(result, reCyToWdioHookCallback, `${1}async () => {`)
		result = replaceCodeRegexString(result, reCyToWdioTestCallback, `${1}async () => {`)
	}
	result = cleanupConvertedWdioOutput(result)
	return ensureTrailingNewline(result), nil
}

func commentUnsupportedCypressWdioLines(source string) string {
	lines := strings.Split(source, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "///") {
			continue
		}
		if reCyToWdioUnsupported.MatchString(line) {
			indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
			lines[i] = indent + "// TERRAIN-TODO: manual Cypress conversion required\n" + indent + "// " + strings.TrimSpace(line)
		}
	}
	return strings.Join(lines, "\n")
}
