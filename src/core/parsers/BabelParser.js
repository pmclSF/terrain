/**
 * AST-based JavaScript parser using @babel/parser.
 *
 * Produces IR nodes from a real AST instead of regex line-scanning.
 * This fixes correctness issues where regex-based parsing matches
 * patterns inside string literals or comments.
 *
 * Usage:
 *   import { parseJavaScript } from './parsers/BabelParser.js';
 *   const ir = parseJavaScript(source, { framework: 'jest' });
 */

import { parse as babelParse } from '@babel/parser';
import {
  TestFile,
  TestSuite,
  TestCase,
  Hook,
  Assertion,
  MockCall,
  ImportStatement,
  RawCode,
  Comment,
  Modifier,
} from '../ir.js';

const HOOK_NAMES = new Set([
  'beforeAll',
  'afterAll',
  'beforeEach',
  'afterEach',
  'before',
  'after',
]);

const TEST_NAMES = new Set(['it', 'test', 'specify']);
const SUITE_NAMES = new Set(['describe', 'context', 'fdescribe', 'xdescribe']);

const ASSERTION_KINDS = {
  toBe: 'equal',
  toEqual: 'deepEqual',
  toStrictEqual: 'strictEqual',
  toBeTruthy: 'truthy',
  toBeFalsy: 'falsy',
  toBeNull: 'isNull',
  toBeUndefined: 'isUndefined',
  toBeDefined: 'isDefined',
  toBeNaN: 'isNaN',
  toBeInstanceOf: 'instanceOf',
  toMatch: 'matches',
  toContain: 'contains',
  toContainEqual: 'containsEqual',
  toHaveLength: 'hasLength',
  toHaveProperty: 'hasProperty',
  toBeGreaterThan: 'greaterThan',
  toBeLessThan: 'lessThan',
  toBeGreaterThanOrEqual: 'greaterOrEqual',
  toBeLessThanOrEqual: 'lessOrEqual',
  toBeCloseTo: 'closeTo',
  toThrow: 'throws',
  toHaveBeenCalled: 'called',
  toHaveBeenCalledWith: 'calledWith',
  toHaveBeenCalledTimes: 'calledTimes',
  toMatchSnapshot: 'snapshot',
  toBeVisible: 'truthy',
  toBeHidden: 'falsy',
  toBeChecked: 'truthy',
  toBeDisabled: 'truthy',
  toBeEnabled: 'truthy',
  toHaveText: 'equal',
  toContainText: 'contains',
  toHaveValue: 'equal',
  toHaveAttribute: 'hasProperty',
  toHaveClass: 'hasClass',
  toHaveCount: 'hasCount',
};

/**
 * Parse JavaScript/TypeScript source into IR using Babel AST.
 *
 * @param {string} source - Source code
 * @param {Object} [options]
 * @param {string} [options.framework] - Framework hint (jest, vitest, cypress, etc.)
 * @returns {TestFile}
 */
export function parseJavaScript(source, options = {}) {
  let ast;
  try {
    ast = babelParse(source, {
      sourceType: 'module',
      plugins: [
        'typescript',
        'jsx',
        'decorators-legacy',
        'classProperties',
        'optionalChaining',
        'nullishCoalescingOperator',
      ],
      errorRecovery: true,
      allowImportExportEverywhere: true,
      allowReturnOutsideFunction: true,
    });
  } catch (_e) {
    // If Babel can't parse, return a minimal IR with the whole file as RawCode
    return new TestFile({
      language: 'javascript',
      imports: [],
      body: [
        new RawCode({
          code: source,
          confidence: 'warning',
          originalSource: source,
        }),
      ],
    });
  }

  const imports = [];
  const body = [];
  const comments = (ast.comments || []).map(
    (c) =>
      new Comment({
        text: c.value.trim(),
        commentKind: c.type === 'CommentLine' ? 'inline' : 'directive',
        sourceLocation: loc(c),
        originalSource:
          c.type === 'CommentLine' ? `// ${c.value}` : `/* ${c.value} */`,
      })
  );

  for (const node of ast.program.body) {
    const result = visitTopLevel(node, source);
    if (result) {
      if (result.type === 'ImportStatement') {
        imports.push(result);
      } else {
        body.push(result);
      }
    }
  }

  // Add comments that aren't attached to nodes
  if (comments.length > 0 && body.length === 0) {
    body.push(...comments);
  }

  return new TestFile({ language: 'javascript', imports, body });
}

function loc(node) {
  if (!node || !node.loc) return null;
  return {
    line: node.loc.start.line,
    column: node.loc.start.column,
    endLine: node.loc.end.line,
    endColumn: node.loc.end.column,
  };
}

function sourceSlice(node, source) {
  if (!node || node.start == null || node.end == null) return '';
  return source.slice(node.start, node.end);
}

function visitTopLevel(node, source) {
  // Import declarations
  if (node.type === 'ImportDeclaration') {
    return new ImportStatement({
      kind: 'framework',
      source: node.source.value,
      specifiers: node.specifiers.map((s) => {
        if (s.type === 'ImportDefaultSpecifier') return 'default';
        if (s.type === 'ImportNamespaceSpecifier')
          return `* as ${s.local.name}`;
        return s.imported ? s.imported.name : s.local.name;
      }),
      isDefault:
        node.specifiers.length === 1 &&
        node.specifiers[0].type === 'ImportDefaultSpecifier',
      isTypeOnly: node.importKind === 'type',
      sourceLocation: loc(node),
      originalSource: sourceSlice(node, source),
    });
  }

  // Variable declarations (const { x } = require(...))
  if (node.type === 'VariableDeclaration') {
    for (const decl of node.declarations) {
      if (
        decl.init &&
        decl.init.type === 'CallExpression' &&
        decl.init.callee.name === 'require'
      ) {
        const arg = decl.init.arguments[0];
        return new ImportStatement({
          kind: 'framework',
          source: arg ? arg.value : '',
          specifiers: extractDestructuredNames(decl.id),
          sourceLocation: loc(node),
          originalSource: sourceSlice(node, source),
        });
      }
    }
  }

  // Expression statements (describe, it, jest.mock, etc.)
  if (node.type === 'ExpressionStatement') {
    return visitExpression(node.expression, source);
  }

  // Anything else → RawCode
  return new RawCode({
    code: sourceSlice(node, source),
    sourceLocation: loc(node),
    originalSource: sourceSlice(node, source),
  });
}

function visitExpression(expr, source) {
  if (!expr) return null;

  // Call expressions: describe(...), it(...), beforeEach(...), jest.mock(...), expect(...)
  if (expr.type === 'CallExpression') {
    const calleeName = getCalleeName(expr.callee);

    // describe / context
    if (SUITE_NAMES.has(calleeName) || calleeName.endsWith('.describe')) {
      return parseSuite(expr, source, calleeName);
    }

    // it / test
    if (TEST_NAMES.has(calleeName)) {
      return parseTestCase(expr, source, calleeName);
    }

    // Modifiers: it.only, it.skip, describe.only, etc.
    if (
      calleeName.endsWith('.only') ||
      calleeName.endsWith('.skip') ||
      calleeName.endsWith('.todo')
    ) {
      const base = calleeName.split('.')[0];
      const mod = calleeName.split('.').pop();
      if (SUITE_NAMES.has(base) || base === 'test') {
        const result = SUITE_NAMES.has(base)
          ? parseSuite(expr, source, calleeName)
          : parseTestCase(expr, source, calleeName);
        if (result) {
          result.modifiers.push(
            new Modifier({ modifierType: mod, sourceLocation: loc(expr) })
          );
        }
        return result;
      }
      if (TEST_NAMES.has(base)) {
        const result = parseTestCase(expr, source, calleeName);
        if (result) {
          result.modifiers.push(
            new Modifier({ modifierType: mod, sourceLocation: loc(expr) })
          );
        }
        return result;
      }
    }

    // Hooks
    if (HOOK_NAMES.has(calleeName)) {
      return parseHook(expr, source, calleeName);
    }

    // jest.mock / jest.spyOn / vi.mock / vi.spyOn
    if (/^(jest|vi)\.(mock|spyOn|fn|useFakeTimers)$/.test(calleeName)) {
      return parseMockCall(expr, source, calleeName);
    }

    // expect(...).toBe(...) and similar assertion chains
    if (calleeName === 'expect' || isExpectChain(expr)) {
      return parseAssertion(expr, source);
    }
  }

  // Await expressions
  if (expr.type === 'AwaitExpression') {
    const inner = visitExpression(expr.argument, source);
    if (inner) {
      inner.requiresAsync = true;
      inner.originalSource = sourceSlice(expr, source);
    }
    return inner;
  }

  return new RawCode({
    code: sourceSlice(expr, source),
    sourceLocation: loc(expr),
    originalSource: sourceSlice(expr, source),
  });
}

function parseSuite(expr, source, _calleeName) {
  const args = expr.arguments;
  const name = args[0] && args[0].type === 'StringLiteral' ? args[0].value : '';
  const callbackArg = args[1];

  const hooks = [];
  const tests = [];
  const modifiers = [];

  if (callbackArg) {
    const bodyNodes = getCallbackBody(callbackArg);
    for (const stmt of bodyNodes) {
      const child =
        stmt.type === 'ExpressionStatement'
          ? visitExpression(stmt.expression, source)
          : visitTopLevel(stmt, source);
      if (!child) continue;
      if (child.type === 'Hook') hooks.push(child);
      else if (child.type === 'TestCase') tests.push(child);
      else if (child.type === 'TestSuite') tests.push(child);
    }
  }

  return new TestSuite({
    name,
    hooks,
    tests,
    modifiers,
    sourceLocation: loc(expr),
    originalSource: sourceSlice(expr, source),
  });
}

function parseTestCase(expr, source, _calleeName) {
  const args = expr.arguments;
  const name = args[0] && args[0].type === 'StringLiteral' ? args[0].value : '';
  const callbackArg = args[1];
  const isAsync = callbackArg && callbackArg.async === true;

  const body = [];
  if (callbackArg) {
    const bodyNodes = getCallbackBody(callbackArg);
    for (const stmt of bodyNodes) {
      const child =
        stmt.type === 'ExpressionStatement'
          ? visitExpression(stmt.expression, source)
          : new RawCode({
              code: sourceSlice(stmt, source),
              sourceLocation: loc(stmt),
              originalSource: sourceSlice(stmt, source),
            });
      if (child) body.push(child);
    }
  }

  return new TestCase({
    name,
    body,
    isAsync,
    modifiers: [],
    sourceLocation: loc(expr),
    originalSource: sourceSlice(expr, source),
  });
}

function parseHook(expr, source, calleeName) {
  const hookMap = {
    before: 'beforeAll',
    after: 'afterAll',
    beforeAll: 'beforeAll',
    afterAll: 'afterAll',
    beforeEach: 'beforeEach',
    afterEach: 'afterEach',
  };

  const callbackArg = expr.arguments[0];
  const isAsync = callbackArg && callbackArg.async === true;

  const body = [];
  if (callbackArg) {
    const bodyNodes = getCallbackBody(callbackArg);
    for (const stmt of bodyNodes) {
      body.push(
        new RawCode({
          code: sourceSlice(stmt, source),
          sourceLocation: loc(stmt),
          originalSource: sourceSlice(stmt, source),
        })
      );
    }
  }

  return new Hook({
    hookType: hookMap[calleeName] || calleeName,
    scope: 'suite',
    body,
    isAsync,
    sourceLocation: loc(expr),
    originalSource: sourceSlice(expr, source),
  });
}

function parseMockCall(expr, source, calleeName) {
  const kindMap = {
    'jest.mock': 'mockModule',
    'vi.mock': 'mockModule',
    'jest.spyOn': 'spyOnMethod',
    'vi.spyOn': 'spyOnMethod',
    'jest.fn': 'createMock',
    'vi.fn': 'createMock',
    'jest.useFakeTimers': 'fakeTimers',
    'vi.useFakeTimers': 'fakeTimers',
  };

  const firstArg = expr.arguments[0];
  const target =
    firstArg && firstArg.type === 'StringLiteral' ? firstArg.value : '';
  const hasVirtual =
    expr.arguments.length >= 3 &&
    sourceSlice(expr.arguments[2], source).includes('virtual');
  const hasRequireActual = sourceSlice(expr, source).includes('requireActual');

  return new MockCall({
    kind: kindMap[calleeName] || 'createMock',
    target,
    confidence: hasVirtual ? 'unconvertible' : 'converted',
    requiresAsync: hasRequireActual,
    frameworkSpecific:
      calleeName.startsWith('jest.') || calleeName.startsWith('vi.'),
    sourceLocation: loc(expr),
    originalSource: sourceSlice(expr, source),
  });
}

function parseAssertion(expr, source) {
  // Walk the member expression chain to find the assertion method
  // e.g., expect(x).toBe(y) or expect(x).not.toBe(y)
  let current = expr;
  let isNegated = false;
  let assertionMethod = '';
  let subject = '';

  // Unwrap call chain: expect(x).not.toBe(y)
  if (
    current.type === 'CallExpression' &&
    current.callee.type === 'MemberExpression'
  ) {
    assertionMethod = current.callee.property.name || '';

    let obj = current.callee.object;
    // Check for .not.
    if (obj.type === 'MemberExpression' && obj.property.name === 'not') {
      isNegated = true;
      obj = obj.object;
    }
    // Check for .resolves. or .rejects.
    if (
      obj.type === 'MemberExpression' &&
      (obj.property.name === 'resolves' || obj.property.name === 'rejects')
    ) {
      if (obj.property.name === 'rejects') assertionMethod = 'rejects';
      obj = obj.object;
    }
    // obj should now be expect(x)
    if (
      obj.type === 'CallExpression' &&
      getCalleeName(obj.callee) === 'expect'
    ) {
      subject = obj.arguments[0] ? sourceSlice(obj.arguments[0], source) : '';
    }
  }

  const kind = ASSERTION_KINDS[assertionMethod] || 'equal';
  const expected =
    expr.arguments && expr.arguments[0]
      ? sourceSlice(expr.arguments[0], source)
      : null;

  return new Assertion({
    kind,
    subject,
    expected,
    isNegated,
    sourceLocation: loc(expr),
    originalSource: sourceSlice(expr, source),
  });
}

// --- Helpers ---

function getCalleeName(callee) {
  if (!callee) return '';
  if (callee.type === 'Identifier') return callee.name;
  if (callee.type === 'MemberExpression') {
    const obj = getCalleeName(callee.object);
    const prop = callee.property.name || callee.property.value || '';
    return obj ? `${obj}.${prop}` : prop;
  }
  // CallExpression as callee (e.g., test.describe.only())
  if (callee.type === 'CallExpression') {
    return getCalleeName(callee.callee);
  }
  return '';
}

function getCallbackBody(callbackArg) {
  if (!callbackArg) return [];
  if (
    callbackArg.type === 'ArrowFunctionExpression' ||
    callbackArg.type === 'FunctionExpression'
  ) {
    if (callbackArg.body.type === 'BlockStatement') {
      return callbackArg.body.body;
    }
    // Arrow with expression body
    return [
      {
        type: 'ExpressionStatement',
        expression: callbackArg.body,
        ...callbackArg.body,
      },
    ];
  }
  return [];
}

/**
 * Check if an expression is an expect(...).matcher(...) chain.
 * Walks the callee chain looking for expect() at the root.
 */
function isExpectChain(expr) {
  if (!expr || expr.type !== 'CallExpression') return false;
  let node = expr.callee;
  while (node) {
    if (node.type === 'MemberExpression') {
      node = node.object;
    } else if (node.type === 'CallExpression') {
      if (
        node.callee &&
        node.callee.type === 'Identifier' &&
        node.callee.name === 'expect'
      ) {
        return true;
      }
      node = node.callee;
    } else if (node.type === 'Identifier') {
      return node.name === 'expect';
    } else {
      return false;
    }
  }
  return false;
}

function extractDestructuredNames(pattern) {
  if (!pattern) return [];
  if (pattern.type === 'Identifier') return [pattern.name];
  if (pattern.type === 'ObjectPattern') {
    return pattern.properties.map((p) =>
      p.value ? p.value.name : p.key ? p.key.name : ''
    );
  }
  return [];
}
