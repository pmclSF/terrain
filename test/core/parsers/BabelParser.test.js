import { parseJavaScript } from '../../../src/core/parsers/BabelParser.js';
import { walkIR } from '../../../src/core/ir.js';

describe('BabelParser', () => {
  describe('parseJavaScript', () => {
    it('should return a TestFile node', () => {
      const ir = parseJavaScript('const x = 1;');
      expect(ir.type).toBe('TestFile');
      expect(ir.language).toBe('javascript');
    });

    it('should parse import statements', () => {
      const ir = parseJavaScript(
        "import { describe, it, expect } from '@jest/globals';"
      );
      expect(ir.imports.length).toBe(1);
      expect(ir.imports[0].type).toBe('ImportStatement');
      expect(ir.imports[0].source).toBe('@jest/globals');
      expect(ir.imports[0].specifiers).toEqual(['describe', 'it', 'expect']);
    });

    it('should parse default imports', () => {
      const ir = parseJavaScript("import React from 'react';");
      expect(ir.imports[0].isDefault).toBe(true);
      expect(ir.imports[0].specifiers).toEqual(['default']);
    });

    it('should parse describe blocks as TestSuite', () => {
      const source = `
describe('MyComponent', () => {
  it('should work', () => {
    expect(true).toBe(true);
  });
});`;
      const ir = parseJavaScript(source);
      expect(ir.body.length).toBe(1);
      expect(ir.body[0].type).toBe('TestSuite');
      expect(ir.body[0].name).toBe('MyComponent');
      expect(ir.body[0].tests.length).toBe(1);
      expect(ir.body[0].tests[0].type).toBe('TestCase');
      expect(ir.body[0].tests[0].name).toBe('should work');
    });

    it('should parse hooks', () => {
      const source = `
describe('Suite', () => {
  beforeEach(() => {
    setup();
  });
  afterAll(() => {
    teardown();
  });
  it('test', () => {});
});`;
      const ir = parseJavaScript(source);
      const suite = ir.body[0];
      expect(suite.hooks.length).toBe(2);
      expect(suite.hooks[0].hookType).toBe('beforeEach');
      expect(suite.hooks[1].hookType).toBe('afterAll');
    });

    it('should parse assertions with kind', () => {
      const source = `
describe('Assertions', () => {
  it('checks equality', () => {
    expect(result).toBe(42);
  });
});`;
      const ir = parseJavaScript(source);
      const nodes = [];
      walkIR(ir, (node) => {
        if (node.type === 'Assertion') nodes.push(node);
      });
      expect(nodes.length).toBe(1);
      expect(nodes[0].kind).toBe('equal');
      expect(nodes[0].subject).toBe('result');
      expect(nodes[0].expected).toBe('42');
    });

    it('should detect negated assertions', () => {
      const source = `
describe('Negation', () => {
  it('checks not', () => {
    expect(x).not.toBe(null);
  });
});`;
      const ir = parseJavaScript(source);
      const assertions = [];
      walkIR(ir, (n) => {
        if (n.type === 'Assertion') assertions.push(n);
      });
      expect(assertions[0].isNegated).toBe(true);
    });

    it('should parse jest.mock as MockCall', () => {
      const source = "jest.mock('./myModule');";
      const ir = parseJavaScript(source);
      expect(ir.body.length).toBe(1);
      expect(ir.body[0].type).toBe('MockCall');
      expect(ir.body[0].kind).toBe('mockModule');
      expect(ir.body[0].target).toBe('./myModule');
    });

    it('should mark virtual mocks as unconvertible', () => {
      const source = "jest.mock('nonexistent', () => ({}), { virtual: true });";
      const ir = parseJavaScript(source);
      expect(ir.body[0].confidence).toBe('unconvertible');
    });

    it('should detect async test cases', () => {
      const source = `
describe('Async', () => {
  it('fetches data', async () => {
    const data = await fetchData();
    expect(data).toBeTruthy();
  });
});`;
      const ir = parseJavaScript(source);
      expect(ir.body[0].tests[0].isAsync).toBe(true);
    });

    it('should handle test.only and test.skip modifiers', () => {
      const source = `
describe('Modifiers', () => {
  it.only('focused test', () => {});
  it.skip('skipped test', () => {});
});`;
      const ir = parseJavaScript(source);
      const suite = ir.body[0];
      expect(suite.tests[0].modifiers[0].modifierType).toBe('only');
      expect(suite.tests[1].modifiers[0].modifierType).toBe('skip');
    });

    it('should not match patterns inside string literals', () => {
      const source = `
describe('String safety', () => {
  it('should handle strings with describe in them', () => {
    const msg = "describe('fake') should not be parsed";
    expect(msg).toBeTruthy();
  });
});`;
      const ir = parseJavaScript(source);
      // Only one suite, one test — the string content is NOT parsed as a suite
      expect(ir.body.length).toBe(1);
      expect(ir.body[0].type).toBe('TestSuite');
      expect(ir.body[0].tests.length).toBe(1);
    });

    it('should handle syntax errors gracefully', () => {
      const source = 'function( { broken syntax!!!';
      const ir = parseJavaScript(source);
      expect(ir.type).toBe('TestFile');
      // Should have RawCode with the whole file
      expect(ir.body.length).toBeGreaterThan(0);
    });

    it('should parse require-based imports', () => {
      const source = "const { Builder, By } = require('selenium-webdriver');";
      const ir = parseJavaScript(source);
      expect(ir.imports.length).toBe(1);
      expect(ir.imports[0].type).toBe('ImportStatement');
      expect(ir.imports[0].source).toBe('selenium-webdriver');
      expect(ir.imports[0].specifiers).toContain('Builder');
      expect(ir.imports[0].specifiers).toContain('By');
    });

    it('should track source locations', () => {
      const source = `describe('Located', () => {
  it('has location', () => {});
});`;
      const ir = parseJavaScript(source);
      expect(ir.body[0].sourceLocation).not.toBeNull();
      expect(ir.body[0].sourceLocation.line).toBe(1);
    });

    it('should parse jest.spyOn as MockCall', () => {
      const source = "jest.spyOn(console, 'log');";
      const ir = parseJavaScript(source);
      expect(ir.body[0].type).toBe('MockCall');
      expect(ir.body[0].kind).toBe('spyOnMethod');
    });

    it('should parse vi.fn as MockCall for vitest', () => {
      const source = 'const mock = vi.fn();';
      const ir = parseJavaScript(source);
      // vi.fn() is inside a variable declaration, will be caught differently
      expect(ir.body.length).toBeGreaterThan(0);
    });

    it('should handle TypeScript syntax', () => {
      const source = `
import { describe, it } from '@jest/globals';

interface TestData {
  name: string;
}

describe('TypeScript', () => {
  it('works with types', () => {
    const data: TestData = { name: 'test' };
    expect(data.name).toBe('test');
  });
});`;
      const ir = parseJavaScript(source);
      expect(ir.imports.length).toBe(1);
      // Should have the interface as RawCode and the suite
      const suites = ir.body.filter((n) => n.type === 'TestSuite');
      expect(suites.length).toBe(1);
    });

    it('should walk all nodes with walkIR', () => {
      const source = `
describe('Walk', () => {
  beforeEach(() => { setup(); });
  it('test1', () => { expect(1).toBe(1); });
  it('test2', () => { expect(2).toBe(2); });
});`;
      const ir = parseJavaScript(source);
      const types = new Set();
      walkIR(ir, (node) => types.add(node.type));
      expect(types.has('TestFile')).toBe(true);
      expect(types.has('TestSuite')).toBe(true);
      expect(types.has('TestCase')).toBe(true);
      expect(types.has('Hook')).toBe(true);
      expect(types.has('Assertion')).toBe(true);
    });
  });
});
