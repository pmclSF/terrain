/**
 * AST-based Python and Java parser using tree-sitter (WASM).
 *
 * Produces IR nodes from a real AST instead of regex line-scanning.
 * Uses web-tree-sitter with pre-built WASM grammars from tree-sitter-wasms.
 *
 * Usage:
 *   import { parsePython, parseJava, initTreeSitter } from './parsers/TreeSitterParser.js';
 *   await initTreeSitter();
 *   const ir = parsePython(source);
 */

import { createRequire } from 'module';
import {
  TestFile,
  TestSuite,
  TestCase,
  Hook,
  Assertion,
  ImportStatement,
  RawCode,
  Comment,
  Modifier,
} from '../ir.js';

const __require = createRequire(import.meta.url);

let Parser = null;
let pythonLang = null;
let javaLang = null;
let initialized = false;
let initPromise = null;

/**
 * Initialize tree-sitter WASM runtime and language grammars.
 * Must be called once before parsePython/parseJava.
 *
 * @returns {Promise<void>}
 */
export async function initTreeSitter() {
  if (initialized) return;

  const TreeSitter = __require('web-tree-sitter');
  await TreeSitter.init();
  Parser = new TreeSitter();

  const pythonWasmPath = __require.resolve(
    'tree-sitter-wasms/out/tree-sitter-python.wasm'
  );
  const javaWasmPath = __require.resolve(
    'tree-sitter-wasms/out/tree-sitter-java.wasm'
  );

  pythonLang = await TreeSitter.Language.load(pythonWasmPath);
  javaLang = await TreeSitter.Language.load(javaWasmPath);
  initialized = true;
}

/**
 * Check if tree-sitter has been initialized.
 * @returns {boolean}
 */
export function isInitialized() {
  return initialized;
}

// ── Python Parser ───────────────────────────────────────────────────

/**
 * Parse Python source into IR using tree-sitter.
 *
 * @param {string} source - Python source code
 * @returns {TestFile}
 */
/**
 * Trigger lazy initialization. Non-blocking — first call returns fallback IR,
 * subsequent calls (after init completes) return full AST-based IR.
 */
export function ensureInitialized() {
  if (!initialized && !initPromise) {
    initPromise = initTreeSitter().catch(() => {
      // Silently fall back to regex if tree-sitter can't load
      initPromise = null;
    });
  }
}

export function parsePython(source) {
  ensureInitialized();
  if (!initialized) {
    return new TestFile({
      language: 'python',
      imports: [],
      body: [new RawCode({ code: source, originalSource: source })],
    });
  }

  Parser.setLanguage(pythonLang);
  const tree = Parser.parse(source);
  const root = tree.rootNode;

  const imports = [];
  const body = [];

  for (let i = 0; i < root.childCount; i++) {
    const child = root.child(i);
    const result = visitPythonNode(child, source);
    if (!result) continue;
    if (result.type === 'ImportStatement') {
      imports.push(result);
    } else {
      body.push(result);
    }
  }

  tree.delete();
  return new TestFile({ language: 'python', imports, body });
}

function visitPythonNode(node, source) {
  const type = node.type;
  const text = node.text;
  const location = {
    line: node.startPosition.row + 1,
    column: node.startPosition.column,
    endLine: node.endPosition.row + 1,
    endColumn: node.endPosition.column,
  };

  // Import statements
  if (type === 'import_statement' || type === 'import_from_statement') {
    const moduleNode =
      node.childForFieldName('module_name') || node.childForFieldName('name');
    const moduleName = moduleNode ? moduleNode.text : '';
    const specifiers = [];
    for (let i = 0; i < node.namedChildCount; i++) {
      const c = node.namedChild(i);
      if (c.type === 'dotted_name' && c !== moduleNode) {
        specifiers.push(c.text);
      }
      if (c.type === 'aliased_import') {
        specifiers.push(c.childForFieldName('name')?.text || c.text);
      }
    }

    return new ImportStatement({
      kind: 'framework',
      source:
        moduleName || text.replace(/^(?:from\s+|import\s+)/, '').split(/\s/)[0],
      specifiers,
      sourceLocation: location,
      originalSource: text,
    });
  }

  // Comments
  if (type === 'comment') {
    return new Comment({
      text: text.replace(/^#\s*/, ''),
      commentKind: 'inline',
      sourceLocation: location,
      originalSource: text,
    });
  }

  // Class definitions (test classes for unittest/pytest)
  if (type === 'class_definition') {
    return parsePythonClass(node, source);
  }

  // Function definitions (test functions for pytest)
  if (type === 'function_definition') {
    return parsePythonFunction(node, source);
  }

  // Decorated definitions
  if (type === 'decorated_definition') {
    const definition = node.namedChildren.find(
      (c) => c.type === 'function_definition' || c.type === 'class_definition'
    );
    if (definition) {
      const result = visitPythonNode(definition, source);
      if (result) {
        // Check for skip/skipIf decorators
        for (let i = 0; i < node.namedChildCount; i++) {
          const dec = node.namedChild(i);
          if (dec.type === 'decorator') {
            const decText = dec.text;
            if (/skip|skipIf|skipUnless/.test(decText)) {
              result.modifiers = result.modifiers || [];
              result.modifiers.push(
                new Modifier({
                  modifierType: 'skip',
                  condition: decText,
                  sourceLocation: {
                    line: dec.startPosition.row + 1,
                    column: dec.startPosition.column,
                  },
                })
              );
            }
            if (/parametrize|parameterized/.test(decText)) {
              result.modifiers = result.modifiers || [];
              result.modifiers.push(
                new Modifier({ modifierType: 'tag', value: decText })
              );
            }
          }
        }
        result.originalSource = text;
      }
      return result;
    }
  }

  return new RawCode({
    code: text,
    sourceLocation: location,
    originalSource: text,
  });
}

function parsePythonClass(node, source) {
  const nameNode = node.childForFieldName('name');
  const name = nameNode ? nameNode.text : '';
  const bodyNode = node.childForFieldName('body');

  const hooks = [];
  const tests = [];

  if (bodyNode) {
    for (let i = 0; i < bodyNode.namedChildCount; i++) {
      const child = bodyNode.namedChild(i);
      if (
        child.type === 'function_definition' ||
        child.type === 'decorated_definition'
      ) {
        const fn =
          child.type === 'decorated_definition'
            ? child.namedChildren.find((c) => c.type === 'function_definition')
            : child;
        if (!fn) continue;

        const fnName = fn.childForFieldName('name')?.text || '';

        if (
          /^(setUp|tearDown|setUpClass|tearDownClass|setup_method|teardown_method)$/.test(
            fnName
          )
        ) {
          const hookMap = {
            setUp: 'beforeEach',
            tearDown: 'afterEach',
            setUpClass: 'beforeAll',
            tearDownClass: 'afterAll',
            setup_method: 'beforeEach',
            teardown_method: 'afterEach',
          };
          hooks.push(
            new Hook({
              hookType: hookMap[fnName],
              scope: 'suite',
              sourceLocation: {
                line: fn.startPosition.row + 1,
                column: fn.startPosition.column,
              },
              originalSource: child.text,
            })
          );
        } else if (fnName.startsWith('test')) {
          const result = parsePythonFunction(fn, source);
          if (result) {
            result.originalSource = child.text;
            tests.push(result);
          }
        }
      }
    }
  }

  return new TestSuite({
    name,
    hooks,
    tests,
    sourceLocation: {
      line: node.startPosition.row + 1,
      column: node.startPosition.column,
    },
    originalSource: node.text,
  });
}

function parsePythonFunction(node, source) {
  const nameNode = node.childForFieldName('name');
  const name = nameNode ? nameNode.text : '';
  const isAsync =
    node.type === 'function_definition' &&
    source.slice(node.startIndex, node.startIndex + 6) === 'async ';

  // Only treat test_* functions as test cases
  if (!name.startsWith('test')) {
    return new RawCode({
      code: node.text,
      sourceLocation: {
        line: node.startPosition.row + 1,
        column: node.startPosition.column,
      },
      originalSource: node.text,
    });
  }

  const bodyNode = node.childForFieldName('body');
  const body = [];

  if (bodyNode) {
    for (let i = 0; i < bodyNode.namedChildCount; i++) {
      const stmt = bodyNode.namedChild(i);
      const stmtText = stmt.text;

      // Detect assertions
      if (
        /\b(assert|self\.assert|assertEqual|assertTrue|assertFalse|assertRaises|assertIn|assertNotIn|assertIs|assertIsNone|assertIsNotNone)\b/.test(
          stmtText
        )
      ) {
        body.push(
          new Assertion({
            kind: detectPythonAssertionKind(stmtText),
            subject: stmtText,
            confidence: 'converted',
            sourceLocation: {
              line: stmt.startPosition.row + 1,
              column: stmt.startPosition.column,
            },
            originalSource: stmtText,
          })
        );
      } else {
        body.push(
          new RawCode({
            code: stmtText,
            sourceLocation: {
              line: stmt.startPosition.row + 1,
              column: stmt.startPosition.column,
            },
            originalSource: stmtText,
          })
        );
      }
    }
  }

  return new TestCase({
    name,
    body,
    isAsync,
    modifiers: [],
    sourceLocation: {
      line: node.startPosition.row + 1,
      column: node.startPosition.column,
    },
    originalSource: node.text,
  });
}

function detectPythonAssertionKind(text) {
  if (/assertEqual/.test(text)) return 'equal';
  if (/assertTrue/.test(text)) return 'truthy';
  if (/assertFalse/.test(text)) return 'falsy';
  if (/assertIsNone/.test(text)) return 'isNull';
  if (/assertIsNotNone/.test(text)) return 'isDefined';
  if (/assertIn/.test(text)) return 'contains';
  if (/assertRaises/.test(text)) return 'throws';
  if (/assertIs\b/.test(text)) return 'strictEqual';
  if (/assert\s/.test(text)) return 'truthy';
  return 'equal';
}

// ── Java Parser ─────────────────────────────────────────────────────

/**
 * Parse Java source into IR using tree-sitter.
 *
 * @param {string} source - Java source code
 * @returns {TestFile}
 */
export function parseJava(source) {
  ensureInitialized();
  if (!initialized) {
    return new TestFile({
      language: 'java',
      imports: [],
      body: [new RawCode({ code: source, originalSource: source })],
    });
  }

  Parser.setLanguage(javaLang);
  const tree = Parser.parse(source);
  const root = tree.rootNode;

  const imports = [];
  const body = [];

  for (let i = 0; i < root.childCount; i++) {
    const child = root.child(i);
    const result = visitJavaNode(child, source);
    if (!result) continue;
    if (result.type === 'ImportStatement') {
      imports.push(result);
    } else {
      body.push(result);
    }
  }

  tree.delete();
  return new TestFile({ language: 'java', imports, body });
}

function visitJavaNode(node, _source) {
  const type = node.type;
  const text = node.text;
  const location = {
    line: node.startPosition.row + 1,
    column: node.startPosition.column,
    endLine: node.endPosition.row + 1,
    endColumn: node.endPosition.column,
  };

  // Import declarations
  if (type === 'import_declaration') {
    const path = text
      .replace(/^import\s+/, '')
      .replace(/;$/, '')
      .trim();
    const isStatic = path.startsWith('static ');
    const cleanPath = isStatic ? path.replace(/^static\s+/, '') : path;
    return new ImportStatement({
      kind: 'framework',
      source: cleanPath,
      specifiers: [cleanPath.split('.').pop()],
      sourceLocation: location,
      originalSource: text,
    });
  }

  // Package declaration
  if (type === 'package_declaration') {
    return new Comment({
      text: text,
      commentKind: 'directive',
      sourceLocation: location,
      originalSource: text,
    });
  }

  // Comments
  if (type === 'line_comment' || type === 'block_comment') {
    return new Comment({
      text: text.replace(/^\/\/\s*/, '').replace(/^\/\*|\*\/$/g, ''),
      commentKind: 'inline',
      sourceLocation: location,
      originalSource: text,
    });
  }

  // Class declarations (test classes)
  if (type === 'class_declaration') {
    return parseJavaClass(node);
  }

  return new RawCode({
    code: text,
    sourceLocation: location,
    originalSource: text,
  });
}

function parseJavaClass(node) {
  const nameNode = node.childForFieldName('name');
  const name = nameNode ? nameNode.text : '';
  const bodyNode = node.childForFieldName('body');

  const hooks = [];
  const tests = [];

  if (bodyNode) {
    for (let i = 0; i < bodyNode.namedChildCount; i++) {
      const member = bodyNode.namedChild(i);
      if (member.type === 'method_declaration') {
        const result = parseJavaMethod(member);
        if (result) {
          if (result.type === 'Hook') hooks.push(result);
          else if (result.type === 'TestCase') tests.push(result);
        }
      }
    }
  }

  return new TestSuite({
    name,
    hooks,
    tests,
    sourceLocation: {
      line: node.startPosition.row + 1,
      column: node.startPosition.column,
    },
    originalSource: node.text,
  });
}

function parseJavaMethod(node) {
  const nameNode = node.childForFieldName('name');
  const name = nameNode ? nameNode.text : '';
  const text = node.text;

  // Check annotations
  const annotations = [];
  let prev = node.previousNamedSibling;
  while (
    (prev && prev.type === 'marker_annotation') ||
    prev?.type === 'annotation'
  ) {
    annotations.push(prev.text);
    prev = prev.previousNamedSibling;
  }

  const allAnnotations = annotations.join(' ');

  // JUnit hooks
  if (
    /@Before\b|@BeforeEach\b|@BeforeClass\b|@BeforeAll\b/.test(allAnnotations)
  ) {
    const hookType = /@BeforeClass|@BeforeAll/.test(allAnnotations)
      ? 'beforeAll'
      : 'beforeEach';
    return new Hook({
      hookType,
      scope: 'suite',
      sourceLocation: {
        line: node.startPosition.row + 1,
        column: node.startPosition.column,
      },
      originalSource: text,
    });
  }

  if (/@After\b|@AfterEach\b|@AfterClass\b|@AfterAll\b/.test(allAnnotations)) {
    const hookType = /@AfterClass|@AfterAll/.test(allAnnotations)
      ? 'afterAll'
      : 'afterEach';
    return new Hook({
      hookType,
      scope: 'suite',
      sourceLocation: {
        line: node.startPosition.row + 1,
        column: node.startPosition.column,
      },
      originalSource: text,
    });
  }

  // JUnit test methods
  if (/@Test\b/.test(allAnnotations) || name.startsWith('test')) {
    const bodyNode = node.childForFieldName('body');
    const body = [];
    const modifiers = [];

    // Check for @Disabled / @Ignore
    if (/@Disabled|@Ignore/.test(allAnnotations)) {
      modifiers.push(new Modifier({ modifierType: 'skip' }));
    }

    if (bodyNode) {
      for (let i = 0; i < bodyNode.namedChildCount; i++) {
        const stmt = bodyNode.namedChild(i);
        const stmtText = stmt.text;

        if (
          /\b(assert|assertEquals|assertTrue|assertFalse|assertNull|assertNotNull|assertThrows|assertThat)\b/.test(
            stmtText
          )
        ) {
          body.push(
            new Assertion({
              kind: detectJavaAssertionKind(stmtText),
              subject: stmtText,
              confidence: 'converted',
              sourceLocation: {
                line: stmt.startPosition.row + 1,
                column: stmt.startPosition.column,
              },
              originalSource: stmtText,
            })
          );
        } else {
          body.push(
            new RawCode({
              code: stmtText,
              sourceLocation: {
                line: stmt.startPosition.row + 1,
                column: stmt.startPosition.column,
              },
              originalSource: stmtText,
            })
          );
        }
      }
    }

    return new TestCase({
      name,
      body,
      modifiers,
      sourceLocation: {
        line: node.startPosition.row + 1,
        column: node.startPosition.column,
      },
      originalSource: text,
    });
  }

  return null;
}

function detectJavaAssertionKind(text) {
  if (/assertEquals/.test(text)) return 'equal';
  if (/assertTrue/.test(text)) return 'truthy';
  if (/assertFalse/.test(text)) return 'falsy';
  if (/assertNull/.test(text)) return 'isNull';
  if (/assertNotNull/.test(text)) return 'isDefined';
  if (/assertThrows/.test(text)) return 'throws';
  if (/assertThat/.test(text)) return 'equal';
  return 'equal';
}
