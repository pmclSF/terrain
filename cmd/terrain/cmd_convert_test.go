package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunListConversions_JSON(t *testing.T) {
	t.Parallel()

	out, err := captureRun(func() error {
		return runListConversions(true)
	})
	if err != nil {
		t.Fatalf("runListConversions returned error: %v", err)
	}

	var payload struct {
		Categories []struct {
			Name       string `json:"name"`
			Directions []struct {
				From string `json:"from"`
				To   string `json:"to"`
			} `json:"directions"`
		} `json:"categories"`
	}
	if err := json.Unmarshal(out, &payload); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, out)
	}
	if len(payload.Categories) != 4 {
		t.Fatalf("category count = %d, want 4", len(payload.Categories))
	}
}

func TestRunShorthands_Text(t *testing.T) {
	t.Parallel()

	out, err := captureRun(func() error {
		return runShorthands(false)
	})
	if err != nil {
		t.Fatalf("runShorthands returned error: %v", err)
	}
	text := string(out)
	if !strings.Contains(text, "cy2pw") {
		t.Fatalf("expected cy2pw in output, got:\n%s", text)
	}
	if !strings.Contains(text, "jesttovt") {
		t.Fatalf("expected jesttovt in output, got:\n%s", text)
	}
}

func TestRunDetect_JSON(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	path := filepath.Join(root, "auth.spec.ts")
	content := "import { test, expect } from '@playwright/test';\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	out, err := captureRun(func() error {
		return runDetect(path, true)
	})
	if err != nil {
		t.Fatalf("runDetect returned error: %v", err)
	}

	var detection struct {
		Framework string `json:"framework"`
		Mode      string `json:"mode"`
	}
	if err := json.Unmarshal(out, &detection); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, out)
	}
	if detection.Framework != "playwright" {
		t.Fatalf("framework = %q, want playwright", detection.Framework)
	}
	if detection.Mode != "file" {
		t.Fatalf("mode = %q, want file", detection.Mode)
	}
}

func TestRunConvert_PlanWithAutoDetect(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	path := filepath.Join(root, "auth.test.ts")
	content := "import { describe, it, expect } from '@jest/globals';\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	out, err := captureRun(func() error {
		return runConvert(path, convertCommandOptions{
			To:         "vitest",
			Plan:       true,
			AutoDetect: true,
			JSON:       true,
		})
	})
	if err != nil {
		t.Fatalf("runConvert returned error: %v", err)
	}

	var payload struct {
		Direction struct {
			From string `json:"from"`
			To   string `json:"to"`
		} `json:"direction"`
		SourceDetection struct {
			Framework string `json:"framework"`
		} `json:"sourceDetection"`
		ExecutionStatus string `json:"executionStatus"`
	}
	if err := json.Unmarshal(out, &payload); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, out)
	}
	if payload.Direction.From != "jest" || payload.Direction.To != "vitest" {
		t.Fatalf("direction = %s -> %s, want jest -> vitest", payload.Direction.From, payload.Direction.To)
	}
	if payload.SourceDetection.Framework != "jest" {
		t.Fatalf("detected framework = %q, want jest", payload.SourceDetection.Framework)
	}
	if payload.ExecutionStatus == "" {
		t.Fatal("expected execution status to be populated")
	}
}

func TestRunConvert_ExecutesPytestToUnittestNowThatGoRuntimeExists(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	path := filepath.Join(root, "test_example.py")
	input := `import pytest

def test_example():
    assert True
`
	if err := os.WriteFile(path, []byte(input), 0o644); err != nil {
		t.Fatalf("write input: %v", err)
	}

	out, err := captureRun(func() error {
		return runConvert(path, convertCommandOptions{
			From: "pytest",
			To:   "unittest",
		})
	})
	if err != nil {
		t.Fatalf("runConvert returned error: %v", err)
	}
	text := string(out)
	if !strings.Contains(text, "class TestExample(unittest.TestCase):") {
		t.Fatalf("expected unittest class output, got:\n%s", text)
	}
	if !strings.Contains(text, "self.assertTrue(True)") {
		t.Fatalf("expected unittest assertion output, got:\n%s", text)
	}
}

func TestRunConvert_ExecutesJunit4ToJunit5ToOutputFile(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	inputPath := filepath.Join(root, "ExampleTest.java")
	outputDir := filepath.Join(root, "converted")
	input := `import org.junit.Test;
import org.junit.Assert;

public class ExampleTest {
    @Test
    public void testValue() {
        Assert.assertEquals(42, getValue());
    }
}
`
	if err := os.WriteFile(inputPath, []byte(input), 0o644); err != nil {
		t.Fatalf("write input: %v", err)
	}

	if err := runCaptured(func() error {
		return runConvert(inputPath, convertCommandOptions{
			From:   "junit4",
			To:     "junit5",
			Output: outputDir,
		})
	}); err != nil {
		t.Fatalf("runConvert returned error: %v", err)
	}

	output, err := os.ReadFile(filepath.Join(outputDir, "ExampleTest.java"))
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	text := string(output)
	if !strings.Contains(text, "org.junit.jupiter.api.Test") {
		t.Fatalf("expected junit5 import, got:\n%s", text)
	}
	if !strings.Contains(text, "Assertions.assertEquals(42, getValue())") {
		t.Fatalf("expected junit assertion conversion, got:\n%s", text)
	}
}

func TestRunConvert_ExecutesUnittestToPytestToOutputFile(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	inputPath := filepath.Join(root, "test_example.py")
	outputPath := filepath.Join(root, "converted", "test_example.py")
	input := `import unittest

class TestExample(unittest.TestCase):
    def test_math(self):
        self.assertEqual(2 + 2, 4)
`
	if err := os.WriteFile(inputPath, []byte(input), 0o644); err != nil {
		t.Fatalf("write input: %v", err)
	}

	if err := runCaptured(func() error {
		return runConvert(inputPath, convertCommandOptions{
			From:   "unittest",
			To:     "pytest",
			Output: outputPath,
		})
	}); err != nil {
		t.Fatalf("runConvert returned error: %v", err)
	}

	output, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	text := string(output)
	if !strings.Contains(text, "def test_math():") {
		t.Fatalf("expected pytest function, got:\n%s", text)
	}
	if !strings.Contains(text, "assert 2 + 2 == 4") {
		t.Fatalf("expected pytest assert, got:\n%s", text)
	}
}

func TestRunConvert_ExecutesJestToVitestToStdout(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	path := filepath.Join(root, "auth.test.js")
	input := `describe('auth', () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  it('uses a mock', () => {
    const fn = jest.fn();
    expect(fn).toBeDefined();
  });
});
`
	if err := os.WriteFile(path, []byte(input), 0o644); err != nil {
		t.Fatalf("write input: %v", err)
	}

	out, err := captureRun(func() error {
		return runConvert(path, convertCommandOptions{
			From: "jest",
			To:   "vitest",
		})
	})
	if err != nil {
		t.Fatalf("runConvert returned error: %v", err)
	}
	text := string(out)
	if !strings.Contains(text, "import { describe, it, expect, beforeEach, vi } from 'vitest';") {
		t.Fatalf("expected vitest import, got:\n%s", text)
	}
	if !strings.Contains(text, "vi.clearAllMocks();") {
		t.Fatalf("expected jest.clearAllMocks to convert, got:\n%s", text)
	}
	if !strings.Contains(text, "const fn = vi.fn();") {
		t.Fatalf("expected jest.fn to convert, got:\n%s", text)
	}
}

func TestRunConvert_ExecutesJestToVitestToOutputFile(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	inputPath := filepath.Join(root, "service.test.js")
	outputPath := filepath.Join(root, "converted", "service.test.js")
	input := `jest.setTimeout(30000);

describe('service', () => {
  it('uses timers', () => {
    expect(true).toBe(true);
  });
});
`
	if err := os.WriteFile(inputPath, []byte(input), 0o644); err != nil {
		t.Fatalf("write input: %v", err)
	}

	if err := runCaptured(func() error {
		return runConvert(inputPath, convertCommandOptions{
			From:   "jest",
			To:     "vitest",
			Output: outputPath,
		})
	}); err != nil {
		t.Fatalf("runConvert returned error: %v", err)
	}

	output, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	text := string(output)
	if !strings.Contains(text, "import { describe, it, expect, vi } from 'vitest';") {
		t.Fatalf("expected vitest import, got:\n%s", text)
	}
	if !strings.Contains(text, "vi.setConfig({ testTimeout: 30000 })") {
		t.Fatalf("expected setTimeout rewrite, got:\n%s", text)
	}
}

func TestRunConvert_ExecutesCypressToPlaywrightAndRenamesOutput(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	inputPath := filepath.Join(root, "checkout.cy.js")
	outputDir := filepath.Join(root, "converted")
	input := `describe('Checkout', () => {
  it('submits', () => {
    cy.visit('/checkout');
    cy.get('[data-testid="submit"]').click();
    cy.get('.notice').should('be.visible');
  });
});
`
	if err := os.WriteFile(inputPath, []byte(input), 0o644); err != nil {
		t.Fatalf("write input: %v", err)
	}

	if err := runCaptured(func() error {
		return runConvert(inputPath, convertCommandOptions{
			From:   "cypress",
			To:     "playwright",
			Output: outputDir,
		})
	}); err != nil {
		t.Fatalf("runConvert returned error: %v", err)
	}

	output, err := os.ReadFile(filepath.Join(outputDir, "checkout.spec.js"))
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	text := string(output)
	if !strings.Contains(text, "import { test, expect } from '@playwright/test';") {
		t.Fatalf("expected playwright import, got:\n%s", text)
	}
	if !strings.Contains(text, "await page.goto('/checkout')") {
		t.Fatalf("expected visit conversion, got:\n%s", text)
	}
	if !strings.Contains(text, "await page.locator('[data-testid=\"submit\"]').click()") {
		t.Fatalf("expected click conversion, got:\n%s", text)
	}
	if !strings.Contains(text, "await expect(page.locator('.notice')).toBeVisible()") {
		t.Fatalf("expected assertion conversion, got:\n%s", text)
	}
}

func TestRunConvert_ExecutesPlaywrightToCypressAndRenamesOutput(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	inputPath := filepath.Join(root, "dashboard.spec.ts")
	outputDir := filepath.Join(root, "converted")
	input := `import { test, expect } from '@playwright/test';

test.describe('Dashboard', () => {
  test('opens', async ({ page }) => {
    await page.goto('/dashboard');
    await page.locator('[data-testid="menu"]').click();
    await expect(page.locator('.panel')).toBeVisible();
  });
});
`
	if err := os.WriteFile(inputPath, []byte(input), 0o644); err != nil {
		t.Fatalf("write input: %v", err)
	}

	if err := runCaptured(func() error {
		return runConvert(inputPath, convertCommandOptions{
			From:   "playwright",
			To:     "cypress",
			Output: outputDir,
		})
	}); err != nil {
		t.Fatalf("runConvert returned error: %v", err)
	}

	output, err := os.ReadFile(filepath.Join(outputDir, "dashboard.cy.ts"))
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	text := string(output)
	if !strings.Contains(text, "/// <reference types=\"cypress\" />") {
		t.Fatalf("expected cypress reference, got:\n%s", text)
	}
	if !strings.Contains(text, "cy.visit('/dashboard')") {
		t.Fatalf("expected goto conversion, got:\n%s", text)
	}
	if !strings.Contains(text, "cy.get('[data-testid=\"menu\"]').click()") {
		t.Fatalf("expected click conversion, got:\n%s", text)
	}
	if !strings.Contains(text, "cy.get('.panel').should('be.visible')") {
		t.Fatalf("expected assertion conversion, got:\n%s", text)
	}
}

func TestRunConvert_ExecutesWdioToPlaywright(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	inputPath := filepath.Join(root, "account.spec.js")
	outputDir := filepath.Join(root, "converted")
	input := `import { browser, $, expect } from '@wdio/globals';

describe('Account', () => {
  it('opens', async () => {
    await browser.url('/account');
    await $('#save').click();
    await expect($('#notice')).toBeDisplayed();
  });
});
`
	if err := os.WriteFile(inputPath, []byte(input), 0o644); err != nil {
		t.Fatalf("write input: %v", err)
	}

	if err := runCaptured(func() error {
		return runConvert(inputPath, convertCommandOptions{
			From:   "webdriverio",
			To:     "playwright",
			Output: outputDir,
		})
	}); err != nil {
		t.Fatalf("runConvert returned error: %v", err)
	}

	output, err := os.ReadFile(filepath.Join(outputDir, "account.spec.js"))
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	text := string(output)
	if !strings.Contains(text, "import { test, expect } from '@playwright/test';") {
		t.Fatalf("expected playwright import, got:\n%s", text)
	}
	if !strings.Contains(text, "await page.goto('/account')") {
		t.Fatalf("expected goto conversion, got:\n%s", text)
	}
	if !strings.Contains(text, "await page.locator('#save').click()") {
		t.Fatalf("expected click conversion, got:\n%s", text)
	}
	if !strings.Contains(text, "await expect(page.locator('#notice')).toBeVisible()") {
		t.Fatalf("expected assertion conversion, got:\n%s", text)
	}
}

func TestRunConvert_ExecutesPlaywrightToWdio(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	inputPath := filepath.Join(root, "profile.spec.ts")
	outputDir := filepath.Join(root, "converted")
	input := `import { test, expect } from '@playwright/test';

test.describe('Profile', () => {
  test('opens', async ({ page }) => {
    await page.goto('/profile');
    await page.locator('#save').click();
    await expect(page.locator('#notice')).toBeVisible();
  });
});
`
	if err := os.WriteFile(inputPath, []byte(input), 0o644); err != nil {
		t.Fatalf("write input: %v", err)
	}

	if err := runCaptured(func() error {
		return runConvert(inputPath, convertCommandOptions{
			From:   "playwright",
			To:     "webdriverio",
			Output: outputDir,
		})
	}); err != nil {
		t.Fatalf("runConvert returned error: %v", err)
	}

	output, err := os.ReadFile(filepath.Join(outputDir, "profile.spec.ts"))
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	text := string(output)
	if strings.Contains(text, "@playwright/test") {
		t.Fatalf("expected playwright import to be removed, got:\n%s", text)
	}
	if !strings.Contains(text, "await browser.url('/profile')") {
		t.Fatalf("expected goto conversion, got:\n%s", text)
	}
	if !strings.Contains(text, "await $('#save').click()") {
		t.Fatalf("expected click conversion, got:\n%s", text)
	}
	if !strings.Contains(text, "await expect($('#notice')).toBeDisplayed()") {
		t.Fatalf("expected assertion conversion, got:\n%s", text)
	}
}

func TestRunConvert_ExecutesCypressToWdio(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	inputPath := filepath.Join(root, "checkout.cy.js")
	outputDir := filepath.Join(root, "converted")
	input := `describe('Checkout', () => {
  it('submits', () => {
    cy.visit('/checkout');
    cy.get('[data-testid="submit"]').click();
    cy.get('.notice').should('be.visible');
  });
});
`
	if err := os.WriteFile(inputPath, []byte(input), 0o644); err != nil {
		t.Fatalf("write input: %v", err)
	}

	if err := runCaptured(func() error {
		return runConvert(inputPath, convertCommandOptions{
			From:   "cypress",
			To:     "webdriverio",
			Output: outputDir,
		})
	}); err != nil {
		t.Fatalf("runConvert returned error: %v", err)
	}

	output, err := os.ReadFile(filepath.Join(outputDir, "checkout.cy.js"))
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	text := string(output)
	if !strings.Contains(text, "await browser.url('/checkout')") {
		t.Fatalf("expected visit conversion, got:\n%s", text)
	}
	if !strings.Contains(text, "await $('[data-testid=\"submit\"]').click()") {
		t.Fatalf("expected click conversion, got:\n%s", text)
	}
	if !strings.Contains(text, "await expect($('.notice')).toBeDisplayed()") {
		t.Fatalf("expected assertion conversion, got:\n%s", text)
	}
}

func TestRunConvert_ExecutesWdioToCypress(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	inputPath := filepath.Join(root, "account.spec.js")
	outputDir := filepath.Join(root, "converted")
	input := `import { browser, $, expect } from '@wdio/globals';

describe('Account', () => {
  it('opens', async () => {
    await browser.url('/account');
    await $('#save').click();
    await expect($('#notice')).toBeDisplayed();
  });
});
`
	if err := os.WriteFile(inputPath, []byte(input), 0o644); err != nil {
		t.Fatalf("write input: %v", err)
	}

	if err := runCaptured(func() error {
		return runConvert(inputPath, convertCommandOptions{
			From:   "webdriverio",
			To:     "cypress",
			Output: outputDir,
		})
	}); err != nil {
		t.Fatalf("runConvert returned error: %v", err)
	}

	output, err := os.ReadFile(filepath.Join(outputDir, "account.spec.js"))
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	text := string(output)
	if strings.Contains(text, "@wdio/globals") {
		t.Fatalf("expected wdio import to be removed, got:\n%s", text)
	}
	if !strings.Contains(text, "cy.visit('/account')") {
		t.Fatalf("expected visit conversion, got:\n%s", text)
	}
	if !strings.Contains(text, "cy.get('#save').click()") {
		t.Fatalf("expected click conversion, got:\n%s", text)
	}
	if !strings.Contains(text, "cy.get('#notice').should('be.visible')") {
		t.Fatalf("expected assertion conversion, got:\n%s", text)
	}
}

func TestRunConvert_ExecutesPuppeteerToPlaywright(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	inputPath := filepath.Join(root, "flow.test.js")
	outputDir := filepath.Join(root, "converted")
	input := `const puppeteer = require('puppeteer');

describe('Flow', () => {
  let browser, page;

  beforeAll(async () => {
    browser = await puppeteer.launch();
    page = await browser.newPage();
  });

  afterAll(async () => {
    await browser.close();
  });

  it('opens', async () => {
    await page.goto('/flow');
    await page.click('#open');
    expect(page.url()).toBe('/flow');
  });
});
`
	if err := os.WriteFile(inputPath, []byte(input), 0o644); err != nil {
		t.Fatalf("write input: %v", err)
	}

	if err := runCaptured(func() error {
		return runConvert(inputPath, convertCommandOptions{
			From:   "puppeteer",
			To:     "playwright",
			Output: outputDir,
		})
	}); err != nil {
		t.Fatalf("runConvert returned error: %v", err)
	}

	output, err := os.ReadFile(filepath.Join(outputDir, "flow.test.js"))
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	text := string(output)
	if !strings.Contains(text, "import { test, expect } from '@playwright/test';") {
		t.Fatalf("expected playwright import, got:\n%s", text)
	}
	if !strings.Contains(text, "await page.locator('#open').click()") {
		t.Fatalf("expected click conversion, got:\n%s", text)
	}
	if !strings.Contains(text, "await expect(page).toHaveURL('/flow')") {
		t.Fatalf("expected URL assertion conversion, got:\n%s", text)
	}
}

func TestRunConvert_ExecutesPlaywrightToPuppeteer(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	inputPath := filepath.Join(root, "nav.spec.ts")
	outputDir := filepath.Join(root, "converted")
	input := `import { test, expect } from '@playwright/test';

test.describe('Nav', () => {
  test('opens', async ({ page }) => {
    await page.goto('/nav');
    await page.locator('#open').click();
    await expect(page).toHaveURL('/nav');
  });
});
`
	if err := os.WriteFile(inputPath, []byte(input), 0o644); err != nil {
		t.Fatalf("write input: %v", err)
	}

	if err := runCaptured(func() error {
		return runConvert(inputPath, convertCommandOptions{
			From:   "playwright",
			To:     "puppeteer",
			Output: outputDir,
		})
	}); err != nil {
		t.Fatalf("runConvert returned error: %v", err)
	}

	output, err := os.ReadFile(filepath.Join(outputDir, "nav.spec.ts"))
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	text := string(output)
	if !strings.Contains(text, "const puppeteer = require('puppeteer');") {
		t.Fatalf("expected puppeteer import, got:\n%s", text)
	}
	if !strings.Contains(text, "browser = await puppeteer.launch();") {
		t.Fatalf("expected lifecycle boilerplate, got:\n%s", text)
	}
	if !strings.Contains(text, "await page.click('#open')") {
		t.Fatalf("expected click conversion, got:\n%s", text)
	}
	if !strings.Contains(text, "expect(page.url()).toBe('/nav')") {
		t.Fatalf("expected URL assertion conversion, got:\n%s", text)
	}
}

func TestRunConvert_ExecutesSeleniumToPlaywright(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	inputPath := filepath.Join(root, "login.spec.js")
	outputDir := filepath.Join(root, "converted")
	input := `const { Builder, By } = require('selenium-webdriver');

describe('Login', () => {
  it('opens', async () => {
    await driver.get('/login');
    await driver.findElement(By.css('#submit')).click();
    expect(await (await driver.findElement(By.css('.notice'))).isDisplayed()).toBe(true);
  });
});
`
	if err := os.WriteFile(inputPath, []byte(input), 0o644); err != nil {
		t.Fatalf("write input: %v", err)
	}

	if err := runCaptured(func() error {
		return runConvert(inputPath, convertCommandOptions{
			From:   "selenium",
			To:     "playwright",
			Output: outputDir,
		})
	}); err != nil {
		t.Fatalf("runConvert returned error: %v", err)
	}

	output, err := os.ReadFile(filepath.Join(outputDir, "login.spec.js"))
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	text := string(output)
	if !strings.Contains(text, "import { test, expect } from '@playwright/test';") {
		t.Fatalf("expected playwright import, got:\n%s", text)
	}
	if !strings.Contains(text, "await page.goto('/login')") {
		t.Fatalf("expected navigation conversion, got:\n%s", text)
	}
	if !strings.Contains(text, "await page.locator('#submit').click()") {
		t.Fatalf("expected click conversion, got:\n%s", text)
	}
	if !strings.Contains(text, "await expect(page.locator('.notice')).toBeVisible()") {
		t.Fatalf("expected visibility assertion conversion, got:\n%s", text)
	}
}

func TestRunConvert_ExecutesPlaywrightToSelenium(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	inputPath := filepath.Join(root, "nav.spec.ts")
	outputDir := filepath.Join(root, "converted")
	input := `import { test, expect } from '@playwright/test';

test.describe('Nav', () => {
  test('opens', async ({ page }) => {
    await page.goto('/nav');
    await page.locator('#open').click();
    await expect(page).toHaveURL('/nav');
  });
});
`
	if err := os.WriteFile(inputPath, []byte(input), 0o644); err != nil {
		t.Fatalf("write input: %v", err)
	}

	if err := runCaptured(func() error {
		return runConvert(inputPath, convertCommandOptions{
			From:   "playwright",
			To:     "selenium",
			Output: outputDir,
		})
	}); err != nil {
		t.Fatalf("runConvert returned error: %v", err)
	}

	output, err := os.ReadFile(filepath.Join(outputDir, "nav.spec.ts"))
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	text := string(output)
	if !strings.Contains(text, "const { Builder, By, Key, until } = require('selenium-webdriver');") {
		t.Fatalf("expected selenium import, got:\n%s", text)
	}
	if !strings.Contains(text, "driver = await new Builder().forBrowser('chrome').build();") {
		t.Fatalf("expected lifecycle boilerplate, got:\n%s", text)
	}
	if !strings.Contains(text, "await driver.findElement(By.css('#open')).click()") {
		t.Fatalf("expected click conversion, got:\n%s", text)
	}
	if !strings.Contains(text, "expect(await driver.getCurrentUrl()).toBe('/nav')") {
		t.Fatalf("expected URL assertion conversion, got:\n%s", text)
	}
}

func TestRunConvert_ExecutesCypressToSelenium(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	inputPath := filepath.Join(root, "checkout.cy.js")
	outputDir := filepath.Join(root, "converted")
	input := `describe('Checkout', () => {
  it('submits', () => {
    cy.visit('/checkout');
    cy.get('#email').type('user@test.com');
    cy.get('#submit').click();
    cy.get('.notice').should('be.visible');
  });
});
`
	if err := os.WriteFile(inputPath, []byte(input), 0o644); err != nil {
		t.Fatalf("write input: %v", err)
	}

	if err := runCaptured(func() error {
		return runConvert(inputPath, convertCommandOptions{
			From:   "cypress",
			To:     "selenium",
			Output: outputDir,
		})
	}); err != nil {
		t.Fatalf("runConvert returned error: %v", err)
	}

	output, err := os.ReadFile(filepath.Join(outputDir, "checkout.cy.js"))
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	text := string(output)
	if !strings.Contains(text, "const { Builder, By, Key, until } = require('selenium-webdriver');") {
		t.Fatalf("expected selenium import, got:\n%s", text)
	}
	if !strings.Contains(text, "await driver.get('/checkout')") {
		t.Fatalf("expected visit conversion, got:\n%s", text)
	}
	if !strings.Contains(text, "await (await driver.findElement(By.css('#submit'))).click()") {
		t.Fatalf("expected click conversion, got:\n%s", text)
	}
	if !strings.Contains(text, "expect(await (await driver.findElement(By.css('.notice'))).isDisplayed()).toBe(true)") {
		t.Fatalf("expected visibility assertion conversion, got:\n%s", text)
	}
}

func TestRunConvert_ExecutesSeleniumToCypress(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	inputPath := filepath.Join(root, "account.spec.js")
	outputDir := filepath.Join(root, "converted")
	input := `const { Builder, By } = require('selenium-webdriver');

describe('Account', () => {
  it('opens', async () => {
    await driver.get('/account');
    await driver.findElement(By.css('#save')).click();
    expect(await (await driver.findElement(By.css('.notice'))).isDisplayed()).toBe(true);
  });
});
`
	if err := os.WriteFile(inputPath, []byte(input), 0o644); err != nil {
		t.Fatalf("write input: %v", err)
	}

	if err := runCaptured(func() error {
		return runConvert(inputPath, convertCommandOptions{
			From:   "selenium",
			To:     "cypress",
			Output: outputDir,
		})
	}); err != nil {
		t.Fatalf("runConvert returned error: %v", err)
	}

	output, err := os.ReadFile(filepath.Join(outputDir, "account.spec.js"))
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	text := string(output)
	if !strings.Contains(text, "/// <reference types=\"cypress\" />") {
		t.Fatalf("expected cypress reference, got:\n%s", text)
	}
	if !strings.Contains(text, "cy.visit('/account')") {
		t.Fatalf("expected visit conversion, got:\n%s", text)
	}
	if !strings.Contains(text, "cy.get('#save').click()") {
		t.Fatalf("expected click conversion, got:\n%s", text)
	}
	if !strings.Contains(text, "cy.get('.notice').should('be.visible')") {
		t.Fatalf("expected visibility assertion conversion, got:\n%s", text)
	}
}

func TestRunConvert_ExecutesMochaToJest(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	inputPath := filepath.Join(root, "service.test.js")
	outputDir := filepath.Join(root, "converted")
	input := "const { expect } = require('chai');\nconst sinon = require('sinon');\n\ndescribe('service', () => {\n  it('tracks calls', () => {\n    const fn = sinon.stub();\n    fn();\n    sinon.assert.calledOnce(fn);\n    expect(true).to.be.true;\n  });\n});\n"
	if err := os.WriteFile(inputPath, []byte(input), 0o644); err != nil {
		t.Fatalf("write input: %v", err)
	}

	if err := runCaptured(func() error {
		return runConvert(inputPath, convertCommandOptions{
			From:   "mocha",
			To:     "jest",
			Output: outputDir,
		})
	}); err != nil {
		t.Fatalf("runConvert returned error: %v", err)
	}

	output, err := os.ReadFile(filepath.Join(outputDir, "service.test.js"))
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	text := string(output)
	if !strings.Contains(text, "const fn = jest.fn()") {
		t.Fatalf("expected jest mock conversion, got:\n%s", text)
	}
	if !strings.Contains(text, "expect(fn).toHaveBeenCalledTimes(1)") {
		t.Fatalf("expected sinon assert conversion, got:\n%s", text)
	}
	if !strings.Contains(text, "expect(true).toBe(true)") {
		t.Fatalf("expected chai assertion conversion, got:\n%s", text)
	}
}

func TestRunConvert_ExecutesJasmineToJest(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	inputPath := filepath.Join(root, "clock.spec.js")
	outputDir := filepath.Join(root, "converted")
	input := "describe('clock', () => {\n  beforeEach(() => {\n    jasmine.clock().install();\n  });\n  it('ticks', () => {\n    const fn = jasmine.createSpy('fn');\n    fn();\n    expect(fn).toHaveBeenCalled();\n  });\n});\n"
	if err := os.WriteFile(inputPath, []byte(input), 0o644); err != nil {
		t.Fatalf("write input: %v", err)
	}

	if err := runCaptured(func() error {
		return runConvert(inputPath, convertCommandOptions{
			From:   "jasmine",
			To:     "jest",
			Output: outputDir,
		})
	}); err != nil {
		t.Fatalf("runConvert returned error: %v", err)
	}

	output, err := os.ReadFile(filepath.Join(outputDir, "clock.spec.js"))
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	text := string(output)
	if !strings.Contains(text, "jest.useFakeTimers()") {
		t.Fatalf("expected timer conversion, got:\n%s", text)
	}
	if !strings.Contains(text, "const fn = jest.fn()") {
		t.Fatalf("expected jasmine spy conversion, got:\n%s", text)
	}
}

func TestRunConvert_ExecutesJestToMocha(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	inputPath := filepath.Join(root, "service.test.js")
	outputDir := filepath.Join(root, "converted")
	input := "describe('service', () => {\n  beforeAll(() => {\n    jest.useFakeTimers();\n  });\n  it('tracks calls', () => {\n    const fn = jest.fn();\n    fn();\n    expect(fn).toHaveBeenCalled();\n    expect(true).toBe(true);\n  });\n});\n"
	if err := os.WriteFile(inputPath, []byte(input), 0o644); err != nil {
		t.Fatalf("write input: %v", err)
	}

	if err := runCaptured(func() error {
		return runConvert(inputPath, convertCommandOptions{
			From:   "jest",
			To:     "mocha",
			Output: outputDir,
		})
	}); err != nil {
		t.Fatalf("runConvert returned error: %v", err)
	}

	output, err := os.ReadFile(filepath.Join(outputDir, "service.test.js"))
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	text := string(output)
	if !strings.Contains(text, "const { expect } = require('chai');") {
		t.Fatalf("expected chai prelude, got:\n%s", text)
	}
	if !strings.Contains(text, "const sinon = require('sinon');") {
		t.Fatalf("expected sinon prelude, got:\n%s", text)
	}
	if !strings.Contains(text, "const fn = sinon.stub()") {
		t.Fatalf("expected jest mock conversion, got:\n%s", text)
	}
	if !strings.Contains(text, "expect(true).to.be.true") {
		t.Fatalf("expected jest assertion conversion, got:\n%s", text)
	}
}

func TestRunConvert_ExecutesJestToJasmine(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	inputPath := filepath.Join(root, "service.test.js")
	outputDir := filepath.Join(root, "converted")
	input := "describe('service', () => {\n  beforeEach(() => {\n    jest.useFakeTimers();\n  });\n  it('tracks calls', () => {\n    const fn = jest.fn();\n    setTimeout(fn, 1000);\n    jest.advanceTimersByTime(1000);\n    expect(fn).toHaveBeenCalled();\n  });\n});\n"
	if err := os.WriteFile(inputPath, []byte(input), 0o644); err != nil {
		t.Fatalf("write input: %v", err)
	}

	if err := runCaptured(func() error {
		return runConvert(inputPath, convertCommandOptions{
			From:   "jest",
			To:     "jasmine",
			Output: outputDir,
		})
	}); err != nil {
		t.Fatalf("runConvert returned error: %v", err)
	}

	output, err := os.ReadFile(filepath.Join(outputDir, "service.test.js"))
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	text := string(output)
	if !strings.Contains(text, "jasmine.clock().install()") {
		t.Fatalf("expected timer install conversion, got:\n%s", text)
	}
	if !strings.Contains(text, "const fn = jasmine.createSpy()") {
		t.Fatalf("expected jest.fn conversion, got:\n%s", text)
	}
	if !strings.Contains(text, "jasmine.clock().tick(1000)") {
		t.Fatalf("expected timer advance conversion, got:\n%s", text)
	}
}

func TestRunConvert_ExecutesTestCafeToPlaywright(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	inputPath := filepath.Join(root, "checkout.test.js")
	outputDir := filepath.Join(root, "converted")
	input := "import { Selector } from 'testcafe';\n\nfixture`Checkout`.page`/checkout`;\n\ntest('submits', async t => {\n  await t.click(Selector('#submit'));\n  await t.expect(Selector('.notice').visible).ok();\n});\n"
	if err := os.WriteFile(inputPath, []byte(input), 0o644); err != nil {
		t.Fatalf("write input: %v", err)
	}

	if err := runCaptured(func() error {
		return runConvert(inputPath, convertCommandOptions{
			From:   "testcafe",
			To:     "playwright",
			Output: outputDir,
		})
	}); err != nil {
		t.Fatalf("runConvert returned error: %v", err)
	}

	output, err := os.ReadFile(filepath.Join(outputDir, "checkout.test.js"))
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	text := string(output)
	if !strings.Contains(text, "import { test, expect } from '@playwright/test';") {
		t.Fatalf("expected playwright import, got:\n%s", text)
	}
	if !strings.Contains(text, "await page.goto('/checkout')") {
		t.Fatalf("expected fixture page conversion, got:\n%s", text)
	}
	if !strings.Contains(text, "await page.locator('#submit').click()") {
		t.Fatalf("expected click conversion, got:\n%s", text)
	}
	if !strings.Contains(text, "await expect(page.locator('.notice')).toBeVisible()") {
		t.Fatalf("expected visibility assertion conversion, got:\n%s", text)
	}
}

func TestRunConvert_ExecutesTestCafeToCypress(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	inputPath := filepath.Join(root, "checkout.test.js")
	outputDir := filepath.Join(root, "converted")
	input := "import { Selector } from 'testcafe';\n\nfixture`Checkout`.page`/checkout`;\n\ntest('submits', async t => {\n  await t.click(Selector('#submit'));\n  await t.expect(Selector('.notice').visible).ok();\n});\n"
	if err := os.WriteFile(inputPath, []byte(input), 0o644); err != nil {
		t.Fatalf("write input: %v", err)
	}

	if err := runCaptured(func() error {
		return runConvert(inputPath, convertCommandOptions{
			From:   "testcafe",
			To:     "cypress",
			Output: outputDir,
		})
	}); err != nil {
		t.Fatalf("runConvert returned error: %v", err)
	}

	output, err := os.ReadFile(filepath.Join(outputDir, "checkout.test.js"))
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	text := string(output)
	if !strings.Contains(text, "/// <reference types=\"cypress\" />") {
		t.Fatalf("expected cypress reference, got:\n%s", text)
	}
	if !strings.Contains(text, "cy.visit('/checkout')") {
		t.Fatalf("expected fixture page conversion, got:\n%s", text)
	}
	if !strings.Contains(text, "cy.get('#submit').click()") {
		t.Fatalf("expected click conversion, got:\n%s", text)
	}
	if !strings.Contains(text, "cy.get('.notice').should('be.visible')") {
		t.Fatalf("expected visibility assertion conversion, got:\n%s", text)
	}
}
