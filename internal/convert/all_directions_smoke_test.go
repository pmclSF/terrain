package convert

import "testing"

func TestLegacyAllDirectionsSmoke(t *testing.T) {
	t.Parallel()

	directions := SupportedDirections()
	if len(directions) != 25 {
		t.Fatalf("supported directions = %d, want 25", len(directions))
	}

	for _, direction := range directions {
		direction := direction
		t.Run(direction.From+"-"+direction.To, func(t *testing.T) {
			t.Parallel()

			fixture, ok := legacySourceFixture(direction.From)
			if !ok {
				t.Fatalf("missing fixture for %s", direction.From)
			}

			output, err := ConvertSource(direction, fixture)
			if err != nil {
				t.Fatalf("ConvertSource returned error: %v", err)
			}
			if len(trimWhitespace(output)) == 0 {
				t.Fatalf("expected non-empty output for %s -> %s", direction.From, direction.To)
			}
			if err := ValidateSyntax(smokeOutputPath(direction), direction.Language, output); err != nil {
				t.Fatalf("expected syntactically valid output for %s -> %s, got: %v\noutput:\n%s", direction.From, direction.To, err, output)
			}
		})
	}
}

func smokeOutputPath(direction Direction) string {
	switch direction.Language {
	case "java":
		return "ConvertedExample.java"
	case "python":
		return "converted_example.py"
	default:
		return "converted_example.js"
	}
}

func legacySourceFixture(framework string) (string, bool) {
	fixture, ok := legacySourceFixtures[NormalizeFramework(framework)]
	return fixture, ok
}

func trimWhitespace(s string) string {
	start := 0
	for start < len(s) && (s[start] == ' ' || s[start] == '\n' || s[start] == '\r' || s[start] == '\t') {
		start++
	}
	end := len(s)
	for end > start && (s[end-1] == ' ' || s[end-1] == '\n' || s[end-1] == '\r' || s[end-1] == '\t') {
		end--
	}
	return s[start:end]
}

var legacySourceFixtures = map[string]string{
	"cypress": `describe('test', () => {
  it('works', () => {
    cy.visit('/');
    cy.get('#btn').click();
  });
});
`,
	"playwright": `import { test, expect } from '@playwright/test';
test.describe('test', () => {
  test('works', async ({ page }) => {
    await page.goto('/');
    await page.locator('#btn').click();
  });
});
`,
	"selenium": `const { Builder, By } = require('selenium-webdriver');
describe('test', () => {
  it('works', async () => {
    const driver = await new Builder().forBrowser('chrome').build();
    await driver.get('http://localhost');
    await driver.findElement(By.css('#btn')).click();
  });
});
`,
	"jest": `describe('test', () => {
  it('works', () => {
    expect(1 + 1).toBe(2);
  });
});
`,
	"vitest": `import { describe, it, expect } from 'vitest';
describe('test', () => {
  it('works', () => {
    expect(1 + 1).toBe(2);
  });
});
`,
	"mocha": `const { expect } = require('chai');
describe('test', () => {
  it('works', () => {
    expect(1 + 1).to.equal(2);
  });
});
`,
	"jasmine": `describe('test', () => {
  it('works', () => {
    const spy = jasmine.createSpy();
    spy();
    expect(spy).toHaveBeenCalled();
  });
});
`,
	"webdriverio": `describe('test', () => {
  it('works', async () => {
    await browser.url('/');
    await $('#btn').click();
  });
});
`,
	"puppeteer": `const puppeteer = require('puppeteer');
describe('test', () => {
  it('works', async () => {
    const browser = await puppeteer.launch();
    const page = await browser.newPage();
    await page.goto('/');
    await page.click('#btn');
    await browser.close();
  });
});
`,
	"testcafe": "import { Selector } from 'testcafe';\n" +
		"fixture`Test`.page`http://localhost`;\n" +
		"test('works', async t => {\n" +
		"  await t.click('#btn');\n" +
		"});\n",
	"junit4": `import org.junit.Test;
import static org.junit.Assert.*;

public class MyTest {
    @Test
    public void testBasic() {
        assertEquals(1, 1);
    }
}
`,
	"junit5": `import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.Assertions;

public class MyTest {
    @Test
    public void testBasic() {
        Assertions.assertEquals(1, 1);
    }
}
`,
	"testng": `import org.testng.annotations.Test;
import org.testng.Assert;

public class MyTest {
    @Test
    public void testBasic() {
        Assert.assertEquals(1, 1);
    }
}
`,
	"pytest": `import pytest

def test_basic():
    assert 1 == 1
`,
	"unittest": `import unittest

class TestBasic(unittest.TestCase):
    def test_basic(self):
        self.assertEqual(1, 1)
`,
	"nose2": `from nose.tools import assert_equal

def test_basic():
    assert_equal(1, 1)
`,
}
