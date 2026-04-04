package convert

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type RawJS string

type configAssignment struct {
	KeyPath string
	Value   any
}

type configMapper func(value any, all map[string]any) *configAssignment

var (
	jestToVitestKeys = map[string]configMapper{
		"testEnvironment": func(value any, _ map[string]any) *configAssignment {
			return &configAssignment{KeyPath: "environment", Value: value}
		},
		"setupFiles": func(value any, _ map[string]any) *configAssignment {
			return &configAssignment{KeyPath: "setupFiles", Value: value}
		},
		"setupFilesAfterFramework": func(value any, _ map[string]any) *configAssignment {
			return &configAssignment{KeyPath: "setupFiles", Value: value}
		},
		"testMatch": func(value any, _ map[string]any) *configAssignment {
			return &configAssignment{KeyPath: "include", Value: value}
		},
		"coverageThreshold": func(value any, _ map[string]any) *configAssignment {
			return &configAssignment{KeyPath: "coverage.thresholds", Value: value}
		},
		"testTimeout": func(value any, _ map[string]any) *configAssignment {
			return &configAssignment{KeyPath: "testTimeout", Value: value}
		},
		"clearMocks": func(value any, _ map[string]any) *configAssignment {
			return &configAssignment{KeyPath: "clearMocks", Value: value}
		},
		"resetMocks": func(value any, _ map[string]any) *configAssignment {
			return &configAssignment{KeyPath: "restoreMocks", Value: value}
		},
		"restoreMocks": func(value any, _ map[string]any) *configAssignment {
			return &configAssignment{KeyPath: "restoreMocks", Value: value}
		},
	}
	vitestToJestKeys = map[string]configMapper{
		"environment": func(value any, _ map[string]any) *configAssignment {
			return &configAssignment{KeyPath: "testEnvironment", Value: value}
		},
		"setupFiles": func(value any, _ map[string]any) *configAssignment {
			return &configAssignment{KeyPath: "setupFiles", Value: value}
		},
		"include": func(value any, _ map[string]any) *configAssignment {
			return &configAssignment{KeyPath: "testMatch", Value: value}
		},
		"testTimeout": func(value any, _ map[string]any) *configAssignment {
			return &configAssignment{KeyPath: "testTimeout", Value: value}
		},
		"clearMocks": func(value any, _ map[string]any) *configAssignment {
			return &configAssignment{KeyPath: "clearMocks", Value: value}
		},
		"restoreMocks": func(value any, _ map[string]any) *configAssignment {
			return &configAssignment{KeyPath: "restoreMocks", Value: value}
		},
	}
	cypressToPlaywrightKeys = map[string]configMapper{
		"baseUrl": func(value any, _ map[string]any) *configAssignment {
			return &configAssignment{KeyPath: "use.baseURL", Value: value}
		},
		"viewportWidth": func(value any, all map[string]any) *configAssignment {
			height := intValue(all["viewportHeight"], 720)
			return &configAssignment{
				KeyPath: "use.viewport",
				Value:   RawJS(fmt.Sprintf("{ width: %d, height: %d }", intValue(value, 1280), height)),
			}
		},
		"viewportHeight": func(_ any, _ map[string]any) *configAssignment { return nil },
		"retries": func(value any, _ map[string]any) *configAssignment {
			return &configAssignment{KeyPath: "retries", Value: value}
		},
		"specPattern": func(value any, _ map[string]any) *configAssignment {
			return &configAssignment{KeyPath: "testMatch", Value: value}
		},
		"defaultCommandTimeout": func(value any, _ map[string]any) *configAssignment {
			return &configAssignment{KeyPath: "timeout", Value: value}
		},
	}
	playwrightToCypressKeys = map[string]configMapper{
		"baseURL": func(value any, _ map[string]any) *configAssignment {
			return &configAssignment{KeyPath: "baseUrl", Value: value}
		},
		"timeout": func(value any, _ map[string]any) *configAssignment {
			return &configAssignment{KeyPath: "defaultCommandTimeout", Value: value}
		},
		"retries": func(value any, _ map[string]any) *configAssignment {
			return &configAssignment{KeyPath: "retries", Value: value}
		},
		"testMatch": func(value any, _ map[string]any) *configAssignment {
			return &configAssignment{KeyPath: "specPattern", Value: value}
		},
		"testDir": func(value any, _ map[string]any) *configAssignment {
			return &configAssignment{KeyPath: "specPattern", Value: value}
		},
	}
	wdioToPlaywrightKeys = map[string]configMapper{
		"baseUrl": func(value any, _ map[string]any) *configAssignment {
			return &configAssignment{KeyPath: "use.baseURL", Value: value}
		},
		"waitforTimeout": func(value any, _ map[string]any) *configAssignment {
			return &configAssignment{KeyPath: "timeout", Value: value}
		},
		"specs": func(value any, _ map[string]any) *configAssignment {
			return &configAssignment{KeyPath: "testMatch", Value: value}
		},
		"maxInstances": func(value any, _ map[string]any) *configAssignment {
			return &configAssignment{KeyPath: "workers", Value: value}
		},
	}
	playwrightToWdioKeys = map[string]configMapper{
		"baseURL": func(value any, _ map[string]any) *configAssignment {
			return &configAssignment{KeyPath: "baseUrl", Value: value}
		},
		"timeout": func(value any, _ map[string]any) *configAssignment {
			return &configAssignment{KeyPath: "waitforTimeout", Value: value}
		},
		"testMatch": func(value any, _ map[string]any) *configAssignment {
			return &configAssignment{KeyPath: "specs", Value: value}
		},
		"workers": func(value any, _ map[string]any) *configAssignment {
			return &configAssignment{KeyPath: "maxInstances", Value: value}
		},
	}
	wdioToCypressKeys = map[string]configMapper{
		"baseUrl": func(value any, _ map[string]any) *configAssignment {
			return &configAssignment{KeyPath: "baseUrl", Value: value}
		},
		"waitforTimeout": func(value any, _ map[string]any) *configAssignment {
			return &configAssignment{KeyPath: "defaultCommandTimeout", Value: value}
		},
		"specs": func(value any, _ map[string]any) *configAssignment {
			return &configAssignment{KeyPath: "specPattern", Value: value}
		},
	}
	cypressToWdioKeys = map[string]configMapper{
		"baseUrl": func(value any, _ map[string]any) *configAssignment {
			return &configAssignment{KeyPath: "baseUrl", Value: value}
		},
		"defaultCommandTimeout": func(value any, _ map[string]any) *configAssignment {
			return &configAssignment{KeyPath: "waitforTimeout", Value: value}
		},
		"specPattern": func(value any, _ map[string]any) *configAssignment {
			return &configAssignment{KeyPath: "specs", Value: value}
		},
	}
	cypressToSeleniumKeys = map[string]configMapper{
		"baseUrl": func(value any, _ map[string]any) *configAssignment {
			return &configAssignment{KeyPath: "baseUrl", Value: value}
		},
		"defaultCommandTimeout": func(value any, _ map[string]any) *configAssignment {
			return &configAssignment{KeyPath: "implicitWait", Value: value}
		},
		"viewportWidth": func(value any, all map[string]any) *configAssignment {
			height := intValue(all["viewportHeight"], 720)
			return &configAssignment{
				KeyPath: "windowSize",
				Value:   RawJS(fmt.Sprintf("{ width: %d, height: %d }", intValue(value, 1280), height)),
			}
		},
		"viewportHeight": func(_ any, _ map[string]any) *configAssignment { return nil },
	}
	seleniumToCypressKeys = map[string]configMapper{
		"baseUrl": func(value any, _ map[string]any) *configAssignment {
			return &configAssignment{KeyPath: "baseUrl", Value: value}
		},
		"implicitWait": func(value any, _ map[string]any) *configAssignment {
			return &configAssignment{KeyPath: "defaultCommandTimeout", Value: value}
		},
		"browserName": func(value any, _ map[string]any) *configAssignment {
			return &configAssignment{KeyPath: "browser", Value: value}
		},
	}
	playwrightToSeleniumKeys = map[string]configMapper{
		"baseURL": func(value any, _ map[string]any) *configAssignment {
			return &configAssignment{KeyPath: "baseUrl", Value: value}
		},
		"timeout": func(value any, _ map[string]any) *configAssignment {
			return &configAssignment{KeyPath: "implicitWait", Value: value}
		},
		"testMatch": func(value any, _ map[string]any) *configAssignment {
			return &configAssignment{KeyPath: "specs", Value: value}
		},
	}
	seleniumToPlaywrightKeys = map[string]configMapper{
		"baseUrl": func(value any, _ map[string]any) *configAssignment {
			return &configAssignment{KeyPath: "use.baseURL", Value: value}
		},
		"implicitWait": func(value any, _ map[string]any) *configAssignment {
			return &configAssignment{KeyPath: "timeout", Value: value}
		},
		"browserName": func(value any, _ map[string]any) *configAssignment {
			return &configAssignment{KeyPath: "use.browserName", Value: value}
		},
	}
	mochaToJestKeys = map[string]configMapper{
		"timeout": func(value any, _ map[string]any) *configAssignment {
			return &configAssignment{KeyPath: "testTimeout", Value: value}
		},
		"spec": func(value any, _ map[string]any) *configAssignment {
			return &configAssignment{KeyPath: "testMatch", Value: value}
		},
		"require": func(value any, _ map[string]any) *configAssignment {
			return &configAssignment{KeyPath: "setupFiles", Value: value}
		},
		"bail": func(value any, _ map[string]any) *configAssignment {
			return &configAssignment{KeyPath: "bail", Value: value}
		},
	}
	jasmineToJestKeys = map[string]configMapper{
		"spec_dir": func(value any, _ map[string]any) *configAssignment {
			return &configAssignment{KeyPath: "roots", Value: value}
		},
		"spec_files": func(value any, _ map[string]any) *configAssignment {
			return &configAssignment{KeyPath: "testMatch", Value: value}
		},
		"helpers": func(value any, _ map[string]any) *configAssignment {
			return &configAssignment{KeyPath: "setupFiles", Value: value}
		},
		"random": func(value any, _ map[string]any) *configAssignment {
			return &configAssignment{KeyPath: "randomize", Value: value}
		},
	}
	configKeyOrder = []string{
		"environment",
		"setupFiles",
		"include",
		"coverage",
		"testTimeout",
		"clearMocks",
		"restoreMocks",
		"use",
		"baseURL",
		"viewport",
		"timeout",
		"retries",
		"testMatch",
		"projects",
		"baseUrl",
		"defaultCommandTimeout",
		"specPattern",
		"testEnvironment",
		"waitforTimeout",
		"specs",
		"workers",
		"implicitWait",
		"windowSize",
		"browser",
		"browserName",
		"roots",
		"bail",
	}
	defaultPlaywrightProjects = RawJS(`[
    { name: 'chromium', use: { browserName: 'chromium' } },
    { name: 'firefox', use: { browserName: 'firefox' } },
    { name: 'webkit', use: { browserName: 'webkit' } },
  ]`)
)

// DetectConfigFramework infers the source framework from a config filename.
func DetectConfigFramework(path string) string {
	name := strings.ToLower(filepath.Base(path))
	switch {
	case strings.Contains(name, "jest.config"):
		return "jest"
	case strings.Contains(name, "vitest.config"):
		return "vitest"
	case strings.Contains(name, "playwright.config"):
		return "playwright"
	case strings.Contains(name, "cypress.config") || name == "cypress.json":
		return "cypress"
	case strings.Contains(name, "wdio") || name == "wdio.conf.js" || name == "wdio.conf.ts":
		return "webdriverio"
	case strings.HasPrefix(name, ".mocharc") || name == "mocha.opts":
		return "mocha"
	case strings.Contains(name, "jasmine.json") || strings.Contains(name, "jasmine.config"):
		return "jasmine"
	case strings.Contains(name, "selenium.config"):
		return "selenium"
	case name == "pytest.ini" || name == "pyproject.toml" || name == "setup.cfg":
		return "pytest"
	default:
		return ""
	}
}

// SupportsConfigConversion reports whether the Go-native config runtime supports
// a given framework direction.
func SupportsConfigConversion(fromFramework, toFramework string) bool {
	switch NormalizeFramework(fromFramework) + "-" + NormalizeFramework(toFramework) {
	case "jest-vitest",
		"vitest-jest",
		"cypress-playwright",
		"playwright-cypress",
		"webdriverio-playwright",
		"playwright-webdriverio",
		"webdriverio-cypress",
		"cypress-webdriverio",
		"cypress-selenium",
		"selenium-cypress",
		"playwright-selenium",
		"selenium-playwright",
		"mocha-jest",
		"jasmine-jest":
		return true
	default:
		return false
	}
}

// TargetConfigFileName returns the conventional target config filename for a
// framework, falling back to the existing basename when Terrain does not have a
// stronger convention.
func TargetConfigFileName(toFramework, fallbackBase string) string {
	switch NormalizeFramework(toFramework) {
	case "jest":
		return "jest.config.js"
	case "vitest":
		return "vitest.config.ts"
	case "playwright":
		return "playwright.config.ts"
	case "cypress":
		return "cypress.config.js"
	case "webdriverio":
		return "wdio.conf.js"
	case "mocha":
		return ".mocharc.yml"
	case "jasmine":
		return "jasmine.json"
	case "pytest":
		return "pytest.ini"
	case "unittest":
		return "unittest.cfg"
	case "testng":
		return "testng.xml"
	case "junit5":
		return "junit-platform.properties"
	default:
		if strings.TrimSpace(fallbackBase) != "" {
			return fallbackBase
		}
		return "terrain.config"
	}
}

// ConvertConfig converts framework config text between supported directions.
func ConvertConfig(content, fromFramework, toFramework string) (string, error) {
	fromFramework = NormalizeFramework(fromFramework)
	toFramework = NormalizeFramework(toFramework)
	direction := fromFramework + "-" + toFramework

	switch direction {
	case "jest-vitest":
		return convertJSConfig(content, jestToVitestKeys, "Jest", "vitest")
	case "vitest-jest":
		return convertJSConfig(content, vitestToJestKeys, "Vitest", "jest")
	case "cypress-playwright":
		out, err := convertJSConfig(content, cypressToPlaywrightKeys, "Cypress", "playwright")
		if err != nil {
			return "", err
		}
		return EnsureDefaultPlaywrightProjects(out), nil
	case "playwright-cypress":
		return convertJSConfig(content, playwrightToCypressKeys, "Playwright", "cypress")
	case "webdriverio-playwright":
		out, err := convertJSConfig(content, wdioToPlaywrightKeys, "WebdriverIO", "playwright")
		if err != nil {
			return "", err
		}
		return EnsureDefaultPlaywrightProjects(out), nil
	case "playwright-webdriverio":
		return convertJSConfig(content, playwrightToWdioKeys, "Playwright", "webdriverio")
	case "webdriverio-cypress":
		return convertJSConfig(content, wdioToCypressKeys, "WebdriverIO", "cypress")
	case "cypress-webdriverio":
		return convertJSConfig(content, cypressToWdioKeys, "Cypress", "webdriverio")
	case "cypress-selenium":
		return convertJSConfig(content, cypressToSeleniumKeys, "Cypress", "selenium")
	case "selenium-cypress":
		return convertJSConfig(content, seleniumToCypressKeys, "Selenium", "cypress")
	case "playwright-selenium":
		return convertJSConfig(content, playwrightToSeleniumKeys, "Playwright", "selenium")
	case "selenium-playwright":
		out, err := convertJSConfig(content, seleniumToPlaywrightKeys, "Selenium", "playwright")
		if err != nil {
			return "", err
		}
		return EnsureDefaultPlaywrightProjects(out), nil
	case "mocha-jest":
		return convertYAMLOrJSConfig(content, mochaToJestKeys, "Mocha", "jest")
	case "jasmine-jest":
		return convertYAMLOrJSConfig(content, jasmineToJestKeys, "Jasmine", "jest")
	default:
		return addConfigTodoHeader(content, fromFramework, toFramework), nil
	}
}

// EnsureDefaultPlaywrightProjects injects a default browser matrix when a generated
// Playwright config does not already define projects.
func EnsureDefaultPlaywrightProjects(configText string) string {
	if !strings.Contains(configText, "defineConfig({") {
		return configText
	}
	if matched, _ := regexp.MatchString(`(?m)^\s*projects\s*:`, configText); matched {
		return configText
	}
	needle := "\n});"
	if idx := strings.LastIndex(configText, needle); idx >= 0 {
		return configText[:idx] + "\n  projects: " + string(defaultPlaywrightProjects) + ",\n});" + configText[idx+len(needle):]
	}
	return configText
}

func convertYAMLOrJSConfig(content string, keyMap map[string]configMapper, sourceName, target string) (string, error) {
	parsed, ok := parseYAMLSimple(content)
	if !ok {
		parsed, ok = parseJSONSimple(content)
	}
	if !ok {
		parsed, ok = extractConfigKeys(content)
	}
	if !ok {
		return addConfigTodoHeader(content, strings.ToLower(sourceName), target), nil
	}
	return renderConfig(parsed, keyMap, sourceName, target), nil
}

func convertJSConfig(content string, keyMap map[string]configMapper, sourceName, target string) (string, error) {
	parsed, ok := extractConfigKeys(content)
	if !ok {
		parsed, ok = parseJSONSimple(content)
	}
	if !ok {
		return addConfigTodoHeader(content, strings.ToLower(sourceName), target), nil
	}
	return renderConfig(parsed, keyMap, sourceName, target), nil
}

func renderConfig(keys map[string]any, keyMap map[string]configMapper, sourceName, target string) string {
	mapped := map[string]any{}
	todos := make([]string, 0)

	for key, value := range keys {
		if mapper, ok := keyMap[key]; ok {
			if assignment := mapper(value, keys); assignment != nil {
				assignNestedValue(mapped, assignment.KeyPath, assignment.Value)
			}
			continue
		}
		todos = append(todos, formatConfigTodo("CONFIG-UNSUPPORTED", fmt.Sprintf("Unsupported %s config key: %s", sourceName, key), fmt.Sprintf("%s: %s", key, renderInlineValue(value)), fmt.Sprintf("Manually convert this option to %s equivalent", titleFramework(target))))
	}
	sort.Strings(todos)

	switch target {
	case "vitest":
		return renderVitestConfig(mapped, todos)
	case "jest":
		return renderJestConfig(mapped, todos)
	case "playwright":
		return renderPlaywrightConfig(mapped, todos)
	case "cypress":
		return renderCypressConfig(mapped, todos)
	case "webdriverio":
		return renderWdioConfig(mapped, todos)
	case "selenium":
		return renderSeleniumConfig(mapped, todos)
	default:
		return addConfigTodoHeader("", strings.ToLower(sourceName), target)
	}
}

func renderVitestConfig(mapped map[string]any, todos []string) string {
	lines := []string{
		"import { defineConfig } from 'vitest/config';",
		"",
		"export default defineConfig({",
		"  test: {",
	}
	lines = append(lines, renderObjectEntries(mapped, 2)...)
	lines = append(lines, "  },", "});")
	if len(todos) > 0 {
		lines = append(lines, "")
		lines = append(lines, todos...)
	}
	return strings.Join(lines, "\n") + "\n"
}

func renderJestConfig(mapped map[string]any, todos []string) string {
	lines := []string{"module.exports = {"}
	lines = append(lines, renderObjectEntries(mapped, 1)...)
	lines = append(lines, "};")
	if len(todos) > 0 {
		lines = append(lines, "")
		lines = append(lines, todos...)
	}
	return strings.Join(lines, "\n") + "\n"
}

func renderPlaywrightConfig(mapped map[string]any, todos []string) string {
	lines := []string{
		"import { defineConfig, devices } from '@playwright/test';",
		"",
		"export default defineConfig({",
	}
	lines = append(lines, renderObjectEntries(mapped, 1)...)
	lines = append(lines, "});")
	if len(todos) > 0 {
		lines = append(lines, "")
		lines = append(lines, todos...)
	}
	return strings.Join(lines, "\n") + "\n"
}

func renderCypressConfig(mapped map[string]any, todos []string) string {
	lines := []string{
		"const { defineConfig } = require('cypress');",
		"",
		"module.exports = defineConfig({",
		"  e2e: {",
	}
	lines = append(lines, renderObjectEntries(mapped, 2)...)
	lines = append(lines, "  },", "});")
	if len(todos) > 0 {
		lines = append(lines, "")
		lines = append(lines, todos...)
	}
	return strings.Join(lines, "\n") + "\n"
}

func renderWdioConfig(mapped map[string]any, todos []string) string {
	lines := []string{"exports.config = {"}
	lines = append(lines, renderObjectEntries(mapped, 1)...)
	lines = append(lines, "};")
	if len(todos) > 0 {
		lines = append(lines, "")
		lines = append(lines, todos...)
	}
	return strings.Join(lines, "\n") + "\n"
}

func renderSeleniumConfig(mapped map[string]any, todos []string) string {
	lines := []string{
		"const { Builder, By, until } = require('selenium-webdriver');",
		"",
		"module.exports = {",
	}
	if _, ok := mapped["capabilities"]; !ok {
		mapped["capabilities"] = RawJS("{ browserName: 'chrome' }")
	}
	lines = append(lines, renderObjectEntries(mapped, 1)...)
	lines = append(lines, "};")
	if len(todos) > 0 {
		lines = append(lines, "")
		lines = append(lines, todos...)
	}
	return strings.Join(lines, "\n") + "\n"
}

func extractConfigKeys(content string) (map[string]any, bool) {
	if parsed, ok := parseJSONSimple(content); ok {
		return parsed, true
	}

	body, ok := extractConfigBody(content)
	if !ok {
		return nil, false
	}
	keys := parseJSObjectBody(body)
	if len(keys) == 0 {
		return nil, false
	}
	for _, wrapper := range []string{"test", "use", "e2e"} {
		raw, ok := keys[wrapper].(RawJS)
		if !ok {
			continue
		}
		nested := parseJSObjectBody(trimOuterBraces(string(raw)))
		for key, value := range nested {
			if _, exists := keys[key]; !exists {
				keys[key] = value
			}
		}
	}
	return keys, true
}

func extractConfigBody(content string) (string, bool) {
	markers := []string{
		"defineConfig(",
		"module.exports",
		"export default",
		"exports.config",
	}
	for _, marker := range markers {
		idx := strings.Index(content, marker)
		if idx < 0 {
			continue
		}
		start := strings.Index(content[idx:], "{")
		if start < 0 {
			continue
		}
		absStart := idx + start
		end := findMatchingBrace(content, absStart)
		if end <= absStart {
			continue
		}
		return content[absStart+1 : end], true
	}
	return "", false
}

func findMatchingBrace(s string, start int) int {
	depth := 0
	inSingle := false
	inDouble := false
	escape := false
	for i := start; i < len(s); i++ {
		ch := s[i]
		if escape {
			escape = false
			continue
		}
		if ch == '\\' && (inSingle || inDouble) {
			escape = true
			continue
		}
		if ch == '\'' && !inDouble {
			inSingle = !inSingle
			continue
		}
		if ch == '"' && !inSingle {
			inDouble = !inDouble
			continue
		}
		if inSingle || inDouble {
			continue
		}
		switch ch {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

func parseJSObjectBody(body string) map[string]any {
	fields := splitTopLevelFields(body)
	keys := map[string]any{}
	for _, field := range fields {
		key, value, ok := splitField(field)
		if !ok {
			continue
		}
		keys[normalizeConfigKey(key)] = parseConfigValue(value)
	}
	return keys
}

func splitTopLevelFields(body string) []string {
	var fields []string
	start := 0
	braces := 0
	brackets := 0
	inSingle := false
	inDouble := false
	escape := false
	for i := 0; i < len(body); i++ {
		ch := body[i]
		if escape {
			escape = false
			continue
		}
		if ch == '\\' && (inSingle || inDouble) {
			escape = true
			continue
		}
		if ch == '\'' && !inDouble {
			inSingle = !inSingle
			continue
		}
		if ch == '"' && !inSingle {
			inDouble = !inDouble
			continue
		}
		if inSingle || inDouble {
			continue
		}
		switch ch {
		case '{':
			braces++
		case '}':
			if braces > 0 {
				braces--
			}
		case '[':
			brackets++
		case ']':
			if brackets > 0 {
				brackets--
			}
		case ',':
			if braces == 0 && brackets == 0 {
				fields = append(fields, body[start:i])
				start = i + 1
			}
		}
	}
	if start < len(body) {
		fields = append(fields, body[start:])
	}
	return fields
}

func splitField(field string) (string, string, bool) {
	inSingle := false
	inDouble := false
	escape := false
	braces := 0
	brackets := 0
	for i := 0; i < len(field); i++ {
		ch := field[i]
		if escape {
			escape = false
			continue
		}
		if ch == '\\' && (inSingle || inDouble) {
			escape = true
			continue
		}
		if ch == '\'' && !inDouble {
			inSingle = !inSingle
			continue
		}
		if ch == '"' && !inSingle {
			inDouble = !inDouble
			continue
		}
		if inSingle || inDouble {
			continue
		}
		switch ch {
		case '{':
			braces++
		case '}':
			if braces > 0 {
				braces--
			}
		case '[':
			brackets++
		case ']':
			if brackets > 0 {
				brackets--
			}
		case ':':
			if braces == 0 && brackets == 0 {
				key := strings.TrimSpace(field[:i])
				value := strings.TrimSpace(field[i+1:])
				if key == "" || value == "" {
					return "", "", false
				}
				return key, value, true
			}
		}
	}
	return "", "", false
}

func parseConfigValue(value string) any {
	value = strings.TrimSpace(strings.TrimSuffix(value, ";"))
	switch {
	case len(value) >= 2 && ((value[0] == '\'' && value[len(value)-1] == '\'') || (value[0] == '"' && value[len(value)-1] == '"')):
		return value[1 : len(value)-1]
	case value == "true":
		return true
	case value == "false":
		return false
	case regexp.MustCompile(`^\d+$`).MatchString(value):
		n, _ := strconv.Atoi(value)
		return n
	case strings.HasPrefix(value, "{") && strings.HasSuffix(value, "}"):
		return RawJS(value)
	case strings.HasPrefix(value, "[") && strings.HasSuffix(value, "]"):
		return RawJS(value)
	default:
		return RawJS(value)
	}
}

func parseJSONSimple(content string) (map[string]any, bool) {
	trimmed := strings.TrimSpace(content)
	if !strings.HasPrefix(trimmed, "{") {
		return nil, false
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(trimmed), &parsed); err != nil {
		return nil, false
	}
	return parsed, true
}

func parseYAMLSimple(content string) (map[string]any, bool) {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" || strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "module") || strings.HasPrefix(trimmed, "export") {
		return nil, false
	}
	keys := map[string]any{}
	found := false
	for _, line := range strings.Split(trimmed, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := normalizeConfigKey(strings.TrimSpace(parts[0]))
		value := strings.TrimSpace(parts[1])
		keys[key] = parseYAMLValue(value)
		found = true
	}
	return keys, found
}

func parseYAMLValue(value string) any {
	value = strings.TrimSpace(value)
	switch {
	case value == "":
		return ""
	case value == "true":
		return true
	case value == "false":
		return false
	case regexp.MustCompile(`^\d+$`).MatchString(value):
		n, _ := strconv.Atoi(value)
		return n
	case len(value) >= 2 && ((value[0] == '\'' && value[len(value)-1] == '\'') || (value[0] == '"' && value[len(value)-1] == '"')):
		return value[1 : len(value)-1]
	default:
		return value
	}
}

func assignNestedValue(target map[string]any, keyPath string, value any) {
	parts := strings.Split(keyPath, ".")
	current := target
	for i, part := range parts {
		if i == len(parts)-1 {
			current[part] = value
			return
		}
		next, ok := current[part].(map[string]any)
		if !ok {
			next = map[string]any{}
			current[part] = next
		}
		current = next
	}
}

func renderObjectEntries(obj map[string]any, indentLevel int) []string {
	indent := strings.Repeat("  ", indentLevel)
	keys := sortedConfigKeys(obj)
	lines := make([]string, 0, len(keys))
	for _, key := range keys {
		value := obj[key]
		renderedKey := formatJSKey(key)
		if nested, ok := value.(map[string]any); ok {
			lines = append(lines, fmt.Sprintf("%s%s: {", indent, renderedKey))
			lines = append(lines, renderObjectEntries(nested, indentLevel+1)...)
			lines = append(lines, fmt.Sprintf("%s},", indent))
			continue
		}
		lines = append(lines, fmt.Sprintf("%s%s: %s,", indent, renderedKey, formatJSValue(value, indentLevel)))
	}
	return lines
}

func formatJSValue(value any, indentLevel int) string {
	switch v := value.(type) {
	case RawJS:
		return string(v)
	case string:
		return "'" + strings.ReplaceAll(v, "'", "\\'") + "'"
	case bool:
		if v {
			return "true"
		}
		return "false"
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case []any:
		parts := make([]string, 0, len(v))
		for _, item := range v {
			parts = append(parts, formatJSValue(item, indentLevel+1))
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case map[string]any:
		lines := []string{"{"}
		lines = append(lines, renderObjectEntries(v, indentLevel+1)...)
		lines = append(lines, strings.Repeat("  ", indentLevel)+"}")
		return strings.Join(lines, "\n")
	default:
		return renderInlineValue(value)
	}
}

func renderInlineValue(value any) string {
	switch v := value.(type) {
	case RawJS:
		return string(v)
	case string:
		return "'" + strings.ReplaceAll(v, "'", "\\'") + "'"
	case bool:
		if v {
			return "true"
		}
		return "false"
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case int:
		return strconv.Itoa(v)
	default:
		bytes, err := json.Marshal(v)
		if err != nil {
			return fmt.Sprint(v)
		}
		return string(bytes)
	}
}

func formatJSKey(key string) string {
	if regexp.MustCompile(`^[A-Za-z_$][A-Za-z0-9_$]*$`).MatchString(key) {
		return key
	}
	return "'" + key + "'"
}

func sortedConfigKeys(obj map[string]any) []string {
	keys := make([]string, 0, len(obj))
	for key := range obj {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		leftWeight := configKeyWeight(keys[i])
		rightWeight := configKeyWeight(keys[j])
		if leftWeight == rightWeight {
			return keys[i] < keys[j]
		}
		return leftWeight < rightWeight
	})
	return keys
}

func configKeyWeight(key string) int {
	for i, candidate := range configKeyOrder {
		if candidate == key {
			return i
		}
	}
	return len(configKeyOrder) + int(key[0])
}

func addConfigTodoHeader(content, from, to string) string {
	header := formatConfigTodo("CONFIG-MANUAL", fmt.Sprintf("Config conversion from %s to %s requires manual review", from, to), fmt.Sprintf("Full config file (%s)", from), fmt.Sprintf("Rewrite this config for %s", to))
	if strings.TrimSpace(content) == "" {
		return header + "\n"
	}
	return header + "\n\n" + content
}

func formatConfigTodo(id, description, original, action string) string {
	return fmt.Sprintf("// TERRAIN-TODO [%s]: %s\n// Original: %s\n// Action: %s", id, description, original, action)
}

func normalizeConfigKey(key string) string {
	key = strings.TrimSpace(key)
	key = strings.Trim(key, `"'`)
	return strings.ReplaceAll(key, "-", "_")
}

func trimOuterBraces(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "{")
	value = strings.TrimSuffix(value, "}")
	return value
}

func intValue(value any, fallback int) int {
	switch v := value.(type) {
	case int:
		return v
	case float64:
		return int(v)
	case string:
		n, err := strconv.Atoi(v)
		if err == nil {
			return n
		}
	case RawJS:
		n, err := strconv.Atoi(string(v))
		if err == nil {
			return n
		}
	}
	return fallback
}

func titleFramework(name string) string {
	if name == "" {
		return ""
	}
	return strings.ToUpper(name[:1]) + name[1:]
}
