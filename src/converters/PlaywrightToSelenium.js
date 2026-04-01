import { BaseConverter } from '../core/BaseConverter.js';

/**
 * Converts Playwright tests to Selenium WebDriver format
 */
export class PlaywrightToSelenium extends BaseConverter {
  constructor(options = {}) {
    super(options);
    this.sourceFramework = 'playwright';
    this.targetFramework = 'selenium';
  }

  async convert(content, _options = {}) {
    let result = content;

    // Remove Playwright imports
    result = result.replace(
      /import\s*\{[^{}\n]*\}\s*from\s*['"]@playwright\/test['"];?\n?/g,
      ''
    );

    // Convert Playwright commands to Selenium
    result = this.convertPlaywrightCommands(result);

    // Convert test structure
    result = this.convertTestStructure(result);

    // Transform test callbacks
    result = this.transformTestCallbacks(result);

    // Add imports and setup
    const imports = this.getImports([]);
    result = imports.join('\n') + '\n\n' + this.getSetupTeardown() + result;

    // Clean up
    result = this.cleanupOutput(result);

    this.stats.conversions++;
    return result;
  }

  /**
   * Convert Playwright commands to Selenium equivalents
   * @param {string} content - Content to convert
   * @returns {string}
   */
  convertPlaywrightCommands(content) {
    let result = content;

    // Convert assertions first
    // Note: Using [^()\n]+ to prevent ReDoS by excluding nested parens
    result = result.replace(
      /await expect\(page\.locator\(([^()\n]+)\)\)\.toBeVisible\(\)/g,
      'expect(await (await driver.findElement(By.css($1))).isDisplayed()).toBe(true)'
    );

    result = result.replace(
      /await expect\(page\.locator\(([^()\n]+)\)\)\.toBeHidden\(\)/g,
      'expect(await (await driver.findElement(By.css($1))).isDisplayed()).toBe(false)'
    );

    result = result.replace(
      /await expect\(page\.locator\(([^()\n]+)\)\)\.toBeAttached\(\)/g,
      'expect((await driver.findElements(By.css($1))).length).toBeGreaterThan(0)'
    );

    result = result.replace(
      /await expect\(page\.locator\(([^()\n]+)\)\)\.not\.toBeAttached\(\)/g,
      'expect((await driver.findElements(By.css($1))).length).toBe(0)'
    );

    result = result.replace(
      /await expect\(page\.locator\(([^()\n]+)\)\)\.toHaveText\(([^()\n]+)\)/g,
      'expect(await (await driver.findElement(By.css($1))).getText()).toBe($2)'
    );

    result = result.replace(
      /await expect\(page\.locator\(([^()\n]+)\)\)\.toContainText\(([^()\n]+)\)/g,
      'expect(await (await driver.findElement(By.css($1))).getText()).toContain($2)'
    );

    result = result.replace(
      /await expect\(page\.locator\(([^()\n]+)\)\)\.toHaveValue\(([^()\n]+)\)/g,
      'expect(await (await driver.findElement(By.css($1))).getAttribute("value")).toBe($2)'
    );

    result = result.replace(
      /await expect\(page\.locator\(([^()\n]+)\)\)\.toHaveClass\(([^()\n]+)\)/g,
      'expect(await (await driver.findElement(By.css($1))).getAttribute("class")).toContain($2)'
    );

    result = result.replace(
      /await expect\(page\.locator\(([^()\n]+)\)\)\.toBeChecked\(\)/g,
      'expect(await (await driver.findElement(By.css($1))).isSelected()).toBe(true)'
    );

    result = result.replace(
      /await expect\(page\.locator\(([^()\n]+)\)\)\.toBeDisabled\(\)/g,
      'expect(await (await driver.findElement(By.css($1))).isEnabled()).toBe(false)'
    );

    result = result.replace(
      /await expect\(page\.locator\(([^()\n]+)\)\)\.toBeEnabled\(\)/g,
      'expect(await (await driver.findElement(By.css($1))).isEnabled()).toBe(true)'
    );

    result = result.replace(
      /await expect\(page\.locator\(([^()\n]+)\)\)\.toHaveCount\((\d+)\)/g,
      'expect((await driver.findElements(By.css($1))).length).toBe($2)'
    );

    // Convert page URL/title assertions
    result = result.replace(
      /await expect\(page\)\.toHaveURL\(([^()\n]+)\)/g,
      'expect(await driver.getCurrentUrl()).toContain($1)'
    );

    result = result.replace(
      /await expect\(page\)\.toHaveTitle\(([^)]+)\)/g,
      'expect(await driver.getTitle()).toBe($1)'
    );

    // Convert interactions
    result = result.replace(
      /await page\.locator\(([^)]+)\)\.fill\(([^)]+)\)/g,
      'await driver.findElement(By.css($1)).sendKeys($2)'
    );

    result = result.replace(
      /await page\.locator\(([^)]+)\)\.click\(\)/g,
      'await driver.findElement(By.css($1)).click()'
    );

    result = result.replace(
      /await page\.locator\(([^)]+)\)\.clear\(\)/g,
      'await driver.findElement(By.css($1)).clear()'
    );

    result = result.replace(
      /await page\.locator\(([^)]+)\)\.check\(\)/g,
      'const checkbox = await driver.findElement(By.css($1));\n      if (!(await checkbox.isSelected())) await checkbox.click()'
    );

    result = result.replace(
      /await page\.locator\(([^)]+)\)\.uncheck\(\)/g,
      'const checkbox = await driver.findElement(By.css($1));\n      if (await checkbox.isSelected()) await checkbox.click()'
    );

    result = result.replace(
      /await page\.locator\(([^)]+)\)\.selectOption\(([^)]+)\)/g,
      'const select = await driver.findElement(By.css($1));\n      await select.findElement(By.css(`option[value=${$2}]`)).click()'
    );

    result = result.replace(
      /await page\.getByText\(([^)]+)\)\.click\(\)/g,
      'await driver.findElement(By.xpath(`//*[contains(text(),$1)]`)).click()'
    );

    // Convert navigation
    result = result.replace(
      /await page\.goto\(([^)]+)\)/g,
      'await driver.get($1)'
    );

    result = result.replace(
      /await page\.reload\(\)/g,
      'await driver.navigate().refresh()'
    );
    result = result.replace(
      /await page\.goBack\(\)/g,
      'await driver.navigate().back()'
    );
    result = result.replace(
      /await page\.goForward\(\)/g,
      'await driver.navigate().forward()'
    );

    // Convert viewport
    result = result.replace(
      /await page\.setViewportSize\(\{\s*width:\s*(\d+),\s*height:\s*(\d+)\s*\}\)/g,
      'await driver.manage().window().setRect({ width: $1, height: $2 })'
    );

    // Convert waits
    result = result.replace(
      /await page\.waitForTimeout\((\d+)\)/g,
      'await driver.sleep($1)'
    );

    // Convert storage/cookies
    result = result.replace(
      /await context\.clearCookies\(\)/g,
      'await driver.manage().deleteAllCookies()'
    );

    result = result.replace(
      /await page\.evaluate\(\(\)\s*=>\s*localStorage\.clear\(\)\)/g,
      'await driver.executeScript("localStorage.clear()")'
    );

    return result;
  }

  /**
   * Convert test structure
   * @param {string} content - Content to convert
   * @returns {string}
   */
  convertTestStructure(content) {
    let result = content;

    // Convert describe
    result = result.replace(/test\.describe\.only\(/g, 'describe.only(');
    result = result.replace(/test\.describe\.skip\(/g, 'describe.skip(');
    result = result.replace(/test\.describe\(/g, 'describe(');

    // Convert tests
    result = result.replace(/test\.only\(/g, 'it.only(');
    result = result.replace(/test\.skip\(/g, 'it.skip(');
    result = result.replace(/test\(/g, 'it(');

    // Convert hooks
    result = result.replace(/test\.beforeAll\(/g, 'beforeAll(');
    result = result.replace(/test\.afterAll\(/g, 'afterAll(');
    result = result.replace(/test\.beforeEach\(/g, 'beforeEach(');
    result = result.replace(/test\.afterEach\(/g, 'afterEach(');

    return result;
  }

  /**
   * Transform test callbacks
   * @param {string} content - Content to transform
   * @returns {string}
   */
  transformTestCallbacks(content) {
    // Remove page/request destructuring from test callbacks
    // Note: Using [^,()\n]+ and [^{}\n]+ to prevent ReDoS
    content = content.replace(
      /it\(([^,()\n]+),\s*async\s*\(\s*\{[^{}\n]+\}\s*\)\s*=>\s*\{/g,
      'it($1, async () => {'
    );

    // Remove page/request destructuring from hooks
    content = content.replace(
      /beforeEach\s*\(\s*async\s*\(\s*\{[^{}\n]+\}\s*\)\s*=>\s*\{/g,
      'beforeEach(async () => {'
    );

    content = content.replace(
      /afterEach\s*\(\s*async\s*\(\s*\{[^{}\n]+\}\s*\)\s*=>\s*\{/g,
      'afterEach(async () => {'
    );

    content = content.replace(
      /beforeAll\s*\(\s*async\s*\(\s*\{[^}]+\}\s*\)\s*=>\s*\{/g,
      'beforeAll(async () => {'
    );

    content = content.replace(
      /afterAll\s*\(\s*async\s*\(\s*\{[^}]+\}\s*\)\s*=>\s*\{/g,
      'afterAll(async () => {'
    );

    // Remove async callbacks for describe
    content = content.replace(
      /describe\(([^,\n]+),\s*\(\)\s*=>\s*\{/g,
      'describe($1, () => {'
    );

    return content;
  }

  /**
   * Clean up output
   * @param {string} content - Content to clean
   * @returns {string}
   */
  cleanupOutput(content) {
    return (
      content
        // Remove double awaits
        .replace(/await\s+await/g, 'await')
        // Remove empty lines
        .replace(/\n{3,}/g, '\n\n')
        // Trim
        .trim() + '\n'
    );
  }

  getSetupTeardown() {
    return `let driver;

beforeAll(async () => {
  driver = await new Builder().forBrowser('chrome').build();
});

afterAll(async () => {
  await driver.quit();
});

`;
  }

  detectTestTypes(_content) {
    return ['e2e'];
  }

  getImports(_testTypes) {
    return [
      "const { Builder, By, Key, until } = require('selenium-webdriver');",
      "const { expect } = require('@jest/globals');",
    ];
  }

  async convertConfig(configPath, _options = {}) {
    return `// Selenium WebDriver configuration
// Converted from Playwright config

module.exports = {
  capabilities: {
    browserName: 'chrome'
  },
  baseUrl: 'http://localhost:3000',
  timeout: 30000
};
`;
  }
}

export default PlaywrightToSelenium;
