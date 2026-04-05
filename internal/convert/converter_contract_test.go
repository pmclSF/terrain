package convert

import (
	"strings"
	"testing"
)

func TestConverters_EmptyInputReturnsEmptyWithoutError(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		fn   func(string) (string, error)
	}{
		{name: "jest-vitest", fn: ConvertJestToVitestSource},
		{name: "jasmine-jest", fn: ConvertJasmineToJestSource},
		{name: "jest-jasmine", fn: ConvertJestToJasmineSource},
		{name: "mocha-jest", fn: ConvertMochaToJestSource},
		{name: "jest-mocha", fn: ConvertJestToMochaSource},
		{name: "cypress-playwright", fn: ConvertCypressToPlaywrightSource},
		{name: "playwright-cypress", fn: ConvertPlaywrightToCypressSource},
		{name: "playwright-puppeteer", fn: ConvertPlaywrightToPuppeteerSource},
		{name: "puppeteer-playwright", fn: ConvertPuppeteerToPlaywrightSource},
		{name: "testcafe-playwright", fn: ConvertTestCafeToPlaywrightSource},
		{name: "testcafe-cypress", fn: ConvertTestCafeToCypressSource},
		{name: "junit4-junit5", fn: ConvertJUnit4ToJunit5Source},
		{name: "pytest-unittest", fn: ConvertPytestToUnittestSource},
		{name: "nose2-pytest", fn: ConvertNose2ToPytestSource},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := tc.fn("")
			if err != nil {
				t.Fatalf("expected nil error, got %v", err)
			}
			if got != "" {
				t.Fatalf("expected empty output, got %q", got)
			}
		})
	}
}

func TestConverters_MalformedInputDoesNotReturnError(t *testing.T) {
	t.Parallel()

	for _, direction := range SupportedDirections() {
		direction := direction
		if direction.Language != "javascript" {
			continue
		}
		fixture, ok := malformedJSFixture(direction.From)
		if !ok {
			t.Fatalf("missing malformed fixture for %s", direction.From)
		}
		t.Run(direction.From+"-"+direction.To, func(t *testing.T) {
			t.Parallel()

			got, err := ConvertSource(direction, fixture.input)
			if err != nil {
				t.Fatalf("expected nil error, got %v", err)
			}
			if got == "" {
				t.Fatal("expected non-empty output for malformed input fallback")
			}
			if !strings.Contains(got, fixture.comment) {
				t.Fatalf("expected fallback to preserve comment %q, got:\n%s", fixture.comment, got)
			}
			if !strings.Contains(got, fixture.literal) {
				t.Fatalf("expected fallback to preserve literal %q, got:\n%s", fixture.literal, got)
			}
		})
	}
}

type malformedFixture struct {
	input   string
	comment string
	literal string
}

func malformedJSFixture(framework string) (malformedFixture, bool) {
	fixture, ok := malformedJSFixtures[NormalizeFramework(framework)]
	return fixture, ok
}

var malformedJSFixtures = map[string]malformedFixture{
	"cypress": {
		input: `describe('x', () => {
  // KEEP_COMMENT: cy.get('#btn').click() should stay in comments
  const keep = "KEEP_LITERAL: cy.get('#btn').click() should stay literal";
  cy.get('#btn').click(
`,
		comment: "// KEEP_COMMENT: cy.get('#btn').click() should stay in comments",
		literal: `"KEEP_LITERAL: cy.get('#btn').click() should stay literal"`,
	},
	"playwright": {
		input: `import { test } from '@playwright/test';
// KEEP_COMMENT: await page.locator('#btn').click() should stay in comments
const keep = "KEEP_LITERAL: page.locator('#btn').click() should stay literal";
test('x', async ({ page }) => {
  await page.locator('#btn').click(
`,
		comment: "// KEEP_COMMENT: await page.locator('#btn').click() should stay in comments",
		literal: `"KEEP_LITERAL: page.locator('#btn').click() should stay literal"`,
	},
	"selenium": {
		input: `const { Builder, By } = require('selenium-webdriver');
// KEEP_COMMENT: driver.findElement(By.css('#btn')).click() should stay in comments
const keep = "KEEP_LITERAL: driver.findElement(By.css('#btn')).click() should stay literal";
describe('x', () => {
  await driver.findElement(By.css('#btn')).click(
`,
		comment: "// KEEP_COMMENT: driver.findElement(By.css('#btn')).click() should stay in comments",
		literal: `"KEEP_LITERAL: driver.findElement(By.css('#btn')).click() should stay literal"`,
	},
	"jest": {
		input: `// KEEP_COMMENT: jest.fn should stay in comments
const keep = "KEEP_LITERAL: jest.spyOn should stay literal";
describe('x', () => {
  const callback = jest.fn(
`,
		comment: "// KEEP_COMMENT: jest.fn should stay in comments",
		literal: `"KEEP_LITERAL: jest.spyOn should stay literal"`,
	},
	"mocha": {
		input: `const { expect } = require('chai');
// KEEP_COMMENT: sinon.spy should stay in comments
const keep = "KEEP_LITERAL: sinon.spy should stay literal";
describe('x', () => {
  const spy = sinon.spy(
`,
		comment: "// KEEP_COMMENT: sinon.spy should stay in comments",
		literal: `"KEEP_LITERAL: sinon.spy should stay literal"`,
	},
	"jasmine": {
		input: `// KEEP_COMMENT: jasmine.createSpy should stay in comments
const keep = "KEEP_LITERAL: jasmine.clock().install should stay literal";
describe('x', function() {
  jasmine.createSpy(
`,
		comment: "// KEEP_COMMENT: jasmine.createSpy should stay in comments",
		literal: `"KEEP_LITERAL: jasmine.clock().install should stay literal"`,
	},
	"webdriverio": {
		input: `describe('x', async () => {
  // KEEP_COMMENT: await $('#btn').click() should stay in comments
  const keep = "KEEP_LITERAL: $('#btn').click() should stay literal";
  await $('#btn').click(
`,
		comment: "// KEEP_COMMENT: await $('#btn').click() should stay in comments",
		literal: `"KEEP_LITERAL: $('#btn').click() should stay literal"`,
	},
	"puppeteer": {
		input: `const puppeteer = require('puppeteer');
// KEEP_COMMENT: await page.click('#btn') should stay in comments
const keep = "KEEP_LITERAL: page.click('#btn') should stay literal";
describe('x', async () => {
  await page.click('#btn'
`,
		comment: "// KEEP_COMMENT: await page.click('#btn') should stay in comments",
		literal: `"KEEP_LITERAL: page.click('#btn') should stay literal"`,
	},
	"testcafe": {
		input: "import { Selector } from 'testcafe';\n" +
			"// KEEP_COMMENT: await t.click('#btn') should stay in comments\n" +
			"const keep = \"KEEP_LITERAL: Selector('#btn') should stay literal\";\n" +
			"fixture`Test`;\n" +
			"test('x', async t => {\n" +
			"  await t.click('#btn'\n",
		comment: "// KEEP_COMMENT: await t.click('#btn') should stay in comments",
		literal: `"KEEP_LITERAL: Selector('#btn') should stay literal"`,
	},
}
