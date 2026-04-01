import {
  SHORTHANDS,
  CONVERSION_CATEGORIES,
  FRAMEWORK_ABBREV,
} from '../../src/cli/shorthands.js';

describe('shorthands', () => {
  describe('FRAMEWORK_ABBREV', () => {
    it('should have abbreviations for all 16 frameworks', () => {
      const expected = [
        'cypress',
        'playwright',
        'selenium',
        'jest',
        'vitest',
        'mocha',
        'jasmine',
        'junit4',
        'junit5',
        'testng',
        'pytest',
        'unittest',
        'nose2',
        'webdriverio',
        'puppeteer',
        'testcafe',
      ];
      for (const fw of expected) {
        expect(FRAMEWORK_ABBREV[fw]).toBeDefined();
        expect(typeof FRAMEWORK_ABBREV[fw]).toBe('string');
      }
    });
  });

  describe('SHORTHANDS', () => {
    it('should be a non-empty object', () => {
      expect(Object.keys(SHORTHANDS).length).toBeGreaterThan(0);
    });

    it('should contain numeric aliases like cy2pw', () => {
      expect(SHORTHANDS['cy2pw']).toEqual({ from: 'cypress', to: 'playwright' });
    });

    it('should contain long aliases like cytopw', () => {
      expect(SHORTHANDS['cytopw']).toEqual({ from: 'cypress', to: 'playwright' });
    });

    it('should have from and to for every entry', () => {
      for (const [alias, { from, to }] of Object.entries(SHORTHANDS)) {
        expect(typeof from).toBe('string');
        expect(typeof to).toBe('string');
        expect(from).not.toBe(to);
      }
    });

    it('should not have duplicate from-to mappings under different names', () => {
      const seen = new Map();
      for (const [alias, { from, to }] of Object.entries(SHORTHANDS)) {
        const key = `${from}-${to}`;
        if (!seen.has(key)) {
          seen.set(key, []);
        }
        seen.get(key).push(alias);
      }
      // Each direction should have at most 2 aliases (numeric and long)
      for (const [_key, aliases] of seen) {
        expect(aliases.length).toBeLessThanOrEqual(2);
      }
    });
  });

  describe('CONVERSION_CATEGORIES', () => {
    it('should be an array of category objects', () => {
      expect(Array.isArray(CONVERSION_CATEGORIES)).toBe(true);
      expect(CONVERSION_CATEGORIES.length).toBeGreaterThan(0);
    });

    it('should have name and directions for each category', () => {
      for (const category of CONVERSION_CATEGORIES) {
        expect(typeof category.name).toBe('string');
        expect(Array.isArray(category.directions)).toBe(true);
        expect(category.directions.length).toBeGreaterThan(0);
      }
    });

    it('should include JavaScript E2E / Browser category', () => {
      const e2e = CONVERSION_CATEGORIES.find(
        (c) => c.name === 'JavaScript E2E / Browser'
      );
      expect(e2e).toBeDefined();
      expect(e2e.directions.length).toBeGreaterThan(0);
    });

    it('should include JavaScript Unit Testing category', () => {
      const unit = CONVERSION_CATEGORIES.find(
        (c) => c.name === 'JavaScript Unit Testing'
      );
      expect(unit).toBeDefined();
    });

    it('should include Java and Python categories', () => {
      const java = CONVERSION_CATEGORIES.find((c) => c.name === 'Java');
      const python = CONVERSION_CATEGORIES.find((c) => c.name === 'Python');
      expect(java).toBeDefined();
      expect(python).toBeDefined();
    });

    it('should have shorthands array for each direction', () => {
      for (const category of CONVERSION_CATEGORIES) {
        for (const direction of category.directions) {
          expect(typeof direction.from).toBe('string');
          expect(typeof direction.to).toBe('string');
          expect(Array.isArray(direction.shorthands)).toBe(true);
          expect(direction.shorthands.length).toBeGreaterThan(0);
        }
      }
    });
  });
});
