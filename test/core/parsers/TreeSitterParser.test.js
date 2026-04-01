import {
  initTreeSitter,
  isInitialized,
  parsePython,
  parseJava,
} from '../../../src/core/parsers/TreeSitterParser.js';
import { walkIR } from '../../../src/core/ir.js';

beforeAll(async () => {
  await initTreeSitter();
});

describe('TreeSitterParser', () => {
  describe('initialization', () => {
    it('should be initialized after init call', () => {
      expect(isInitialized()).toBe(true);
    });
  });

  describe('parsePython', () => {
    it('should return a TestFile node', () => {
      const ir = parsePython('x = 1\n');
      expect(ir.type).toBe('TestFile');
      expect(ir.language).toBe('python');
    });

    it('should parse import statements', () => {
      const ir = parsePython('import pytest\nfrom unittest import TestCase\n');
      expect(ir.imports.length).toBe(2);
      expect(ir.imports[0].type).toBe('ImportStatement');
    });

    it('should parse pytest test functions as TestCase', () => {
      const source = `
def test_addition():
    result = 1 + 1
    assert result == 2
`;
      const ir = parsePython(source);
      const testCases = [];
      walkIR(ir, (n) => { if (n.type === 'TestCase') testCases.push(n); });
      expect(testCases.length).toBe(1);
      expect(testCases[0].name).toBe('test_addition');
    });

    it('should parse assertions inside test functions', () => {
      const source = `
def test_values():
    assert True
    assert 1 == 1
`;
      const ir = parsePython(source);
      const assertions = [];
      walkIR(ir, (n) => { if (n.type === 'Assertion') assertions.push(n); });
      expect(assertions.length).toBe(2);
    });

    it('should parse unittest class as TestSuite', () => {
      const source = `
import unittest

class TestMath(unittest.TestCase):
    def setUp(self):
        self.value = 42

    def test_equality(self):
        self.assertEqual(self.value, 42)

    def tearDown(self):
        pass
`;
      const ir = parsePython(source);
      const suites = [];
      walkIR(ir, (n) => { if (n.type === 'TestSuite') suites.push(n); });
      expect(suites.length).toBe(1);
      expect(suites[0].name).toBe('TestMath');
      expect(suites[0].hooks.length).toBe(2);
      expect(suites[0].tests.length).toBe(1);
    });

    it('should detect setUp/tearDown as hooks', () => {
      const source = `
class TestSetup(unittest.TestCase):
    def setUp(self):
        pass
    def tearDown(self):
        pass
    def setUpClass(cls):
        pass
    def test_something(self):
        pass
`;
      const ir = parsePython(source);
      const suite = ir.body.find((n) => n.type === 'TestSuite');
      expect(suite.hooks.length).toBe(3);
      expect(suite.hooks[0].hookType).toBe('beforeEach');
      expect(suite.hooks[1].hookType).toBe('afterEach');
      expect(suite.hooks[2].hookType).toBe('beforeAll');
    });

    it('should detect assertion types', () => {
      const source = `
class TestAssertions(unittest.TestCase):
    def test_asserts(self):
        self.assertEqual(1, 1)
        self.assertTrue(True)
        self.assertFalse(False)
        self.assertIsNone(None)
        self.assertIn(1, [1, 2])
`;
      const ir = parsePython(source);
      const assertions = [];
      walkIR(ir, (n) => { if (n.type === 'Assertion') assertions.push(n); });
      expect(assertions.length).toBe(5);
      expect(assertions[0].kind).toBe('equal');
      expect(assertions[1].kind).toBe('truthy');
      expect(assertions[2].kind).toBe('falsy');
      expect(assertions[3].kind).toBe('isNull');
      expect(assertions[4].kind).toBe('contains');
    });

    it('should not match patterns inside string literals', () => {
      const source = `
def test_strings():
    msg = "def test_fake(): assert True"
    assert msg is not None
`;
      const ir = parsePython(source);
      const testCases = [];
      walkIR(ir, (n) => { if (n.type === 'TestCase') testCases.push(n); });
      // Only one test — the string content is NOT parsed as a test
      expect(testCases.length).toBe(1);
      expect(testCases[0].name).toBe('test_strings');
    });

    it('should track source locations', () => {
      const source = `def test_located():\n    assert True\n`;
      const ir = parsePython(source);
      const tc = ir.body.find((n) => n.type === 'TestCase');
      expect(tc.sourceLocation).not.toBeNull();
      expect(tc.sourceLocation.line).toBe(1);
    });
  });

  describe('parseJava', () => {
    it('should return a TestFile node', () => {
      const ir = parseJava('public class Test {}');
      expect(ir.type).toBe('TestFile');
      expect(ir.language).toBe('java');
    });

    it('should parse import statements', () => {
      const ir = parseJava(
        'import org.junit.jupiter.api.Test;\nimport org.junit.jupiter.api.Assertions;\n'
      );
      expect(ir.imports.length).toBe(2);
      expect(ir.imports[0].source).toContain('junit');
    });

    it('should parse JUnit test class as TestSuite', () => {
      const source = `
import org.junit.jupiter.api.Test;
import static org.junit.jupiter.api.Assertions.*;

public class CalculatorTest {
    @Test
    public void testAdd() {
        assertEquals(4, 2 + 2);
    }
}
`;
      const ir = parseJava(source);
      const suites = [];
      walkIR(ir, (n) => { if (n.type === 'TestSuite') suites.push(n); });
      expect(suites.length).toBe(1);
      expect(suites[0].name).toBe('CalculatorTest');
    });

    it('should detect assertion types', () => {
      const source = `
public class AssertTest {
    @Test
    void testAssertions() {
        assertEquals(1, 1);
        assertTrue(true);
        assertFalse(false);
        assertNull(null);
        assertThrows(Exception.class, () -> {});
    }
}
`;
      const ir = parseJava(source);
      const assertions = [];
      walkIR(ir, (n) => { if (n.type === 'Assertion') assertions.push(n); });
      expect(assertions.length).toBe(5);
    });

    it('should track source locations', () => {
      const ir = parseJava('public class Located {}');
      expect(ir.body[0].sourceLocation).not.toBeNull();
      expect(ir.body[0].sourceLocation.line).toBe(1);
    });
  });
});
