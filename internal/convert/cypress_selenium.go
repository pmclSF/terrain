package convert

import (
	"regexp"
	"strings"
)

var (
	reCyToSelHookBeforeArrow    = regexp.MustCompile(`(?m)(^|[^\w.])before\(\s*\(\s*\)\s*=>\s*\{`)
	reCyToSelHookAfterArrow     = regexp.MustCompile(`(?m)(^|[^\w.])after\(\s*\(\s*\)\s*=>\s*\{`)
	reCyToSelHookBeforeEach     = regexp.MustCompile(`(?m)(^|[^\w.])beforeEach\(\s*\(\s*\)\s*=>\s*\{`)
	reCyToSelHookAfterEach      = regexp.MustCompile(`(?m)(^|[^\w.])afterEach\(\s*\(\s*\)\s*=>\s*\{`)
	reCyToSelHookBeforeFunc     = regexp.MustCompile(`(?m)(^|[^\w.])before\(\s*function\s*\(\s*\)\s*\{`)
	reCyToSelHookAfterFunc      = regexp.MustCompile(`(?m)(^|[^\w.])after\(\s*function\s*\(\s*\)\s*\{`)
	reCyToSelHookBeforeEachFunc = regexp.MustCompile(`(?m)(^|[^\w.])beforeEach\(\s*function\s*\(\s*\)\s*\{`)
	reCyToSelHookAfterEachFunc  = regexp.MustCompile(`(?m)(^|[^\w.])afterEach\(\s*function\s*\(\s*\)\s*\{`)
	reCyToSelItArrow            = regexp.MustCompile(`(?m)(^|[^\w.])(it(?:\.(?:only|skip))?\([^,\n]+,\s*)\(\s*\)\s*=>\s*\{`)
	reCyToSelItFunc             = regexp.MustCompile(`(?m)(^|[^\w.])(it(?:\.(?:only|skip))?\([^,\n]+,\s*)function\s*\(\s*\)\s*\{`)
	reCyToSelContainsClick      = regexp.MustCompile(`cy\.contains\(([^()\n]+)\)\.click\(\)`)
	reCyToSelContainsVisible    = regexp.MustCompile(`cy\.contains\(([^()\n]+)\)\.should\(['"]be\.visible['"]\)`)
	reCyToSelClearCookies       = regexp.MustCompile(`cy\.clearCookies\(\)`)
	reCyToSelClearLocalStorage  = regexp.MustCompile(`cy\.clearLocalStorage\(\)`)
	reCyToSelFindClick          = regexp.MustCompile(`cy\.get\(([^()\n]+)\)\.find\(([^()\n]+)\)\.click\(\)`)
	reCyToSelNumericBack        = regexp.MustCompile(`cy\.go\((-?\d+)\)`)
	reCyToSelUnsupportedLine    = regexp.MustCompile(`\b(?:cy\.(?:intercept|request|task|session|fixture|origin|wrap|stub|spy|clock|tick|screenshot)|Cypress\.)|\.within\(|\.each\(|\.its\(`)
)

const seleniumBoilerplateWithExpect = "const { Builder, By, Key, until } = require('selenium-webdriver');\nconst { expect } = require('@jest/globals');\n\nlet driver;\n\nbeforeAll(async () => {\n  driver = await new Builder().forBrowser('chrome').build();\n});\n\nafterAll(async () => {\n  await driver.quit();\n});"

// ConvertCypressToSeleniumSource rewrites the high-confidence Cypress browser
// surface into Go-native Selenium output.
func ConvertCypressToSeleniumSource(source string) (string, error) {
	if strings.TrimSpace(source) == "" {
		return source, nil
	}
	if !strings.Contains(source, "cy.") && !reCypressReference.MatchString(source) {
		return ensureTrailingNewline(strings.ReplaceAll(source, "\r\n", "\n")), nil
	}

	result := strings.ReplaceAll(source, "\r\n", "\n")
	result = reCypressReference.ReplaceAllString(result, "")
	astApplied := false
	if astResult, ok := convertCypressToSeleniumSourceAST(result); ok {
		result = astResult
		astApplied = true
	}

	if !astApplied {
		result = reCyJoinChains.ReplaceAllString(result, "cy.$1($2).")
		result = reCyJoinMethods.ReplaceAllString(result, ").$1")

		assertionReplacements := []struct {
			re   *regexp.Regexp
			repl string
		}{
			{reCyGetShouldVisible, `expect(await (await driver.findElement(By.css($1))).isDisplayed()).toBe(true)`},
			{reCyGetShouldHidden, `expect(await (await driver.findElement(By.css($1))).isDisplayed()).toBe(false)`},
			{reCyGetShouldExist, `expect((await driver.findElements(By.css($1))).length).toBeGreaterThan(0)`},
			{reCyGetShouldNotExist, `expect((await driver.findElements(By.css($1))).length).toBe(0)`},
			{reCyGetShouldText, `expect(await (await driver.findElement(By.css($1))).getText()).toBe($2)`},
			{reCyGetShouldContain, `expect(await (await driver.findElement(By.css($1))).getText()).toContain($2)`},
			{reCyGetShouldValue, `expect(await (await driver.findElement(By.css($1))).getAttribute("value")).toBe($2)`},
			{reCyGetShouldChecked, `expect(await (await driver.findElement(By.css($1))).isSelected()).toBe(true)`},
			{reCyGetShouldDisabled, `expect(await (await driver.findElement(By.css($1))).isEnabled()).toBe(false)`},
			{reCyGetShouldEnabled, `expect(await (await driver.findElement(By.css($1))).isEnabled()).toBe(true)`},
			{reCyGetShouldClass, `expect(await (await driver.findElement(By.css($1))).getAttribute("class")).toContain($2)`},
			{reCyGetShouldLength, `expect((await driver.findElements(By.css($1))).length).toBe($2)`},
			{reCyGetShouldLengthGT, `expect((await driver.findElements(By.css($1))).length).toBeGreaterThan($2)`},
			{reCyToSelContainsVisible, "expect(await (await driver.findElement(By.xpath(`//*[contains(text(),$1)]`))).isDisplayed()).toBe(true)"},
			{reCyURLShouldInclude, `expect(await driver.getCurrentUrl()).toContain($1)`},
			{reCyURLShouldEq, `expect(await driver.getCurrentUrl()).toBe($1)`},
			{reCyTitleShouldEq, `expect(await driver.getTitle()).toBe($1)`},
		}
		for _, replacement := range assertionReplacements {
			result = replacement.re.ReplaceAllString(result, replacement.repl)
		}

		actionReplacements := []struct {
			re   *regexp.Regexp
			repl string
		}{
			{reCyGetClearType, "await driver.findElement(By.css($1)).clear();\n    await driver.findElement(By.css($1)).sendKeys($2)"},
			{reCyGetFirstClick, `await (await driver.findElements(By.css($1)))[0].click()`},
			{reCyGetLastClick, `await (await driver.findElements(By.css($1))).slice(-1)[0].click()`},
			{reCyGetEqClick, `await (await driver.findElements(By.css($1)))[$2].click()`},
			{reCyGetClick, `await driver.findElement(By.css($1)).click()`},
			{reCyGetDoubleClick, "await driver.findElement(By.css($1)).click();\n    await driver.findElement(By.css($1)).click()"},
			{reCyGetType, `await driver.findElement(By.css($1)).sendKeys($2)`},
			{reCyGetClear, `await driver.findElement(By.css($1)).clear()`},
			{reCyGetCheck, "const checkbox = await driver.findElement(By.css($1));\n    if (!(await checkbox.isSelected())) await checkbox.click()"},
			{reCyGetUncheck, "const checkbox = await driver.findElement(By.css($1));\n    if (await checkbox.isSelected()) await checkbox.click()"},
			{reCyGetSelect, `await driver.findElement(By.css($1)).sendKeys($2)`},
			{reCyContainsClick, "await driver.findElement(By.xpath(`//*[contains(text(),$1)]`)).click()"},
			{reCyToSelFindClick, `await (await driver.findElement(By.css($1))).findElement(By.css($2)).click()`},
			{reCyVisit, `await driver.get($1)`},
			{reCyReload, `await driver.navigate().refresh()`},
			{reCyGoBack, `await driver.navigate().back()`},
			{reCyGoForward, `await driver.navigate().forward()`},
			{reCyWaitNumber, `await driver.sleep($1)`},
			{reCyToSelClearCookies, `await driver.manage().deleteAllCookies()`},
			{reCyToSelClearLocalStorage, `await driver.executeScript("localStorage.clear()")`},
		}
		for _, replacement := range actionReplacements {
			result = replacement.re.ReplaceAllString(result, replacement.repl)
		}

		result = reCyToSelNumericBack.ReplaceAllString(result, `await driver.navigate().back() /* cy.go($1) */`)
		result = reContext.ReplaceAllString(result, "${1}describe(")
		result = reSpecify.ReplaceAllString(result, "${1}it(")
		result = reCyToSelHookBeforeArrow.ReplaceAllString(result, "${1}beforeAll(async () => {")
		result = reCyToSelHookAfterArrow.ReplaceAllString(result, "${1}afterAll(async () => {")
		result = reCyToSelHookBeforeEach.ReplaceAllString(result, "${1}beforeEach(async () => {")
		result = reCyToSelHookAfterEach.ReplaceAllString(result, "${1}afterEach(async () => {")
		result = reCyToSelHookBeforeFunc.ReplaceAllString(result, "${1}beforeAll(async function() {")
		result = reCyToSelHookAfterFunc.ReplaceAllString(result, "${1}afterAll(async function() {")
		result = reCyToSelHookBeforeEachFunc.ReplaceAllString(result, "${1}beforeEach(async function() {")
		result = reCyToSelHookAfterEachFunc.ReplaceAllString(result, "${1}afterEach(async function() {")
		result = reCyToSelItArrow.ReplaceAllString(result, "${1}${2}async () => {")
		result = reCyToSelItFunc.ReplaceAllString(result, "${1}${2}async function() {")

		result = commentUnsupportedCypressSeleniumLines(result)
	}
	result = cleanupConvertedSeleniumOutput(result)
	result = prependImportPreservingHeader(result, seleniumBoilerplateWithExpect)
	return ensureTrailingNewline(result), nil
}

func commentUnsupportedCypressSeleniumLines(source string) string {
	lines := strings.Split(source, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "//") {
			continue
		}
		if reCyToSelUnsupportedLine.MatchString(line) ||
			reUnsupportedCyLine.MatchString(line) ||
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
