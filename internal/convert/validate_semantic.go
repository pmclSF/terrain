package convert

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

type semanticLeakRule struct {
	label string
	re    *regexp.Regexp
}

var jsSemanticLeakRules = map[string][]semanticLeakRule{
	"cypress": {
		{label: "Cypress API", re: regexp.MustCompile(`\bcy\.[A-Za-z_]\w*\s*\(`)},
	},
	"playwright": {
		{label: "Playwright test import", re: regexp.MustCompile(`\b(?:from\s+['"]@playwright/test['"]|require\(\s*['"]@playwright/test['"]\s*\))`)},
		{label: "Playwright locator API", re: regexp.MustCompile(`\bpage\.(?:locator|getByRole|getByText|getByLabel|getByPlaceholder|getByAltText|getByTitle|getByTestId)\s*\(`)},
		{label: "Playwright test lifecycle API", re: regexp.MustCompile(`\btest\.(?:describe|beforeAll|afterAll|beforeEach|afterEach|only|skip)\s*\(`)},
	},
	"selenium": {
		{label: "Selenium import", re: regexp.MustCompile(`\b(?:from\s+['"]selenium-webdriver['"]|require\(\s*['"]selenium-webdriver['"]\s*\))`)},
		{label: "Selenium driver lookup API", re: regexp.MustCompile(`\bdriver\.findElements?\s*\(`)},
		{label: "Selenium By locator API", re: regexp.MustCompile(`\bBy\.(?:css|id|xpath|name|linkText|partialLinkText|tagName)\s*\(`)},
	},
	"webdriverio": {
		{label: "WebdriverIO browser API", re: regexp.MustCompile(`\bbrowser\.(?:url|pause|refresh|execute|setCookies|getCookies|deleteCookies|keys)\s*\(`)},
	},
	"puppeteer": {
		{label: "Puppeteer import", re: regexp.MustCompile(`\b(?:from\s+['"]puppeteer['"]|require\(\s*['"]puppeteer['"]\s*\))`)},
		{label: "Puppeteer element handle API", re: regexp.MustCompile(`\bpage\.\$\$?\s*\(`)},
		{label: "Puppeteer page API", re: regexp.MustCompile(`\bpage\.(?:waitForSelector|waitForNavigation|setViewport|setCookie|deleteCookie|cookies)\s*\(`)},
	},
	"testcafe": {
		{label: "TestCafe import", re: regexp.MustCompile(`\b(?:from\s+['"]testcafe['"]|require\(\s*['"]testcafe['"]\s*\))`)},
		{label: "TestCafe selector API", re: regexp.MustCompile(`\bSelector\s*\(`)},
		{label: "TestCafe fixture API", re: regexp.MustCompile(`\bfixture(?:\.page)?\b`)},
		{label: "TestCafe controller API", re: regexp.MustCompile(`\bt\.(?:click|typeText|expect|navigateTo|wait|setFilesToUpload)\s*\(`)},
	},
	"jest": {
		{label: "Jest globals import", re: regexp.MustCompile(`\b(?:from\s+['"]@jest/globals['"]|require\(\s*['"]@jest/globals['"]\s*\))`)},
		{label: "Jest runtime API", re: regexp.MustCompile(`\bjest\.(?:fn|spyOn|mock|useFakeTimers|useRealTimers|advanceTimersByTime|clearAllMocks|restoreAllMocks|setTimeout)\s*\(`)},
	},
	"mocha": {
		{label: "Chai import", re: regexp.MustCompile(`\b(?:from\s+['"]chai['"]|require\(\s*['"]chai['"]\s*\))`)},
		{label: "Sinon import", re: regexp.MustCompile(`\b(?:from\s+['"]sinon['"]|require\(\s*['"]sinon['"]\s*\))`)},
		{label: "Sinon runtime API", re: regexp.MustCompile(`\bsinon\.(?:spy|stub|restore|reset|useFakeTimers)\s*\(`)},
	},
	"jasmine": {
		{label: "Jasmine runtime API", re: regexp.MustCompile(`\bjasmine\.(?:createSpy|createSpyObj|clock|anything|objectContaining|stringMatching)\b`)},
	},
}

// ValidateConvertedOutput checks both parseability and framework-specific leakage
// for native conversion output.
func ValidateConvertedOutput(path string, direction Direction, source string) error {
	if err := ValidateSyntax(path, direction.Language, source); err != nil {
		return err
	}

	if warning := semanticValidationWarning(direction, source); warning != "" {
		target := strings.TrimSpace(path)
		if target == "" {
			target = "converted output"
		}
		return fmt.Errorf(
			"semantic validation failed for %s (%s -> %s): %s",
			target,
			direction.From,
			direction.To,
			warning,
		)
	}

	return nil
}

// ValidateExecutionResultForDirection checks Execute output using both syntax and
// framework-specific semantic validation.
func ValidateExecutionResultForDirection(result ExecutionResult, direction Direction) error {
	if result.Mode == "stdout" {
		path := result.Source
		if len(result.Files) > 0 && strings.TrimSpace(result.Files[0].SourcePath) != "" {
			path = result.Files[0].SourcePath
		}
		return ValidateConvertedOutput(path, direction, result.StdoutContent)
	}

	for _, file := range result.Files {
		if strings.TrimSpace(file.OutputPath) == "" {
			continue
		}
		content, err := os.ReadFile(file.OutputPath)
		if err != nil {
			return fmt.Errorf("read converted output for validation: %w", err)
		}
		if err := ValidateConvertedOutput(file.OutputPath, direction, string(content)); err != nil {
			return err
		}
	}
	return nil
}

func semanticValidationWarnings(direction Direction, source string) []string {
	warning := semanticValidationWarning(direction, source)
	if warning == "" {
		return nil
	}
	return []string{warning}
}

func semanticValidationWarning(direction Direction, source string) string {
	if strings.ToLower(strings.TrimSpace(direction.Language)) != "javascript" {
		return ""
	}

	rules := jsSemanticLeakRules[NormalizeFramework(direction.From)]
	if len(rules) == 0 {
		return ""
	}

	mask := jsCodeMask(source)
	for _, rule := range rules {
		if match := firstCodeRegexMatch(source, mask, rule.re); match != "" {
			return fmt.Sprintf("leftover %s detected in converted code (%s)", rule.label, compactValidationSnippet(match))
		}
	}

	return ""
}

func firstCodeRegexMatch(source string, mask []bool, re *regexp.Regexp) string {
	if !re.MatchString(source) {
		return ""
	}

	for _, idxs := range re.FindAllStringIndex(source, -1) {
		start, end := idxs[0], idxs[1]
		if start < 0 || end <= start || start >= len(mask) || !mask[start] {
			continue
		}
		return source[start:end]
	}

	return ""
}

func compactValidationSnippet(value string) string {
	value = strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
	if len(value) > 72 {
		return value[:69] + "..."
	}
	return value
}
