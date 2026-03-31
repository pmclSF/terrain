const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const test = require("node:test");
const Module = require("node:module");

const extensionPath = path.join(__dirname, "..", "out", "extension.js");
const viewsPath = path.join(__dirname, "..", "out", "views.js");

function readFixture(name) {
  return JSON.parse(
    fs.readFileSync(path.join(__dirname, "fixtures", name), "utf8")
  );
}

function sampleBundle() {
  return {
    analyze: readFixture("analyze-report.sample.json"),
    insights: readFixture("insights-report.sample.json"),
    migration: readFixture("migration-readiness.sample.json"),
  };
}

function createHarness(options = {}) {
  const {
    autoRefresh = false,
    binaryPath = "terrain",
    workspaceRoot = path.join(path.sep, "workspace"),
    bundle = sampleBundle(),
  } = options;

  const execCalls = [];
  const listeners = {};

  class EventEmitter {
    constructor() {
      this.event = () => ({ dispose() {} });
    }

    fire() {}

    dispose() {}
  }

  class TreeItem {
    constructor(label, collapsibleState) {
      this.label = label;
      this.collapsibleState = collapsibleState;
    }
  }

  class ThemeIcon {
    constructor(id) {
      this.id = id;
    }
  }

  const disposable = () => ({ dispose() {} });

  const commandOutputs = new Map([
    [["analyze", "--json", "--root", workspaceRoot].join("\u0000"), bundle.analyze],
    [["insights", "--json", "--root", workspaceRoot].join("\u0000"), bundle.insights],
    [
      ["migration", "readiness", "--json", "--root", workspaceRoot].join("\u0000"),
      bundle.migration,
    ],
  ]);

  const vscodeMock = {
    EventEmitter,
    ThemeIcon,
    TreeItem,
    TreeItemCollapsibleState: {
      None: 0,
      Collapsed: 1,
      Expanded: 2,
    },
    Uri: {
      joinPath(base, relPath) {
        return { fsPath: path.join(base.fsPath, relPath) };
      },
    },
    commands: {
      registerCommand() {
        return disposable();
      },
    },
    window: {
      registerTreeDataProvider() {
        return disposable();
      },
      createTerminal() {
        return {
          sendText() {},
          show() {},
        };
      },
      showTextDocument() {},
    },
    workspace: {
      workspaceFolders: [{ uri: { fsPath: workspaceRoot } }],
      getConfiguration(section) {
        assert.equal(section, "terrain");
        return {
          get(key) {
            if (key === "autoRefresh") {
              return autoRefresh;
            }
            if (key === "binaryPath") {
              return binaryPath;
            }
            return undefined;
          },
        };
      },
      onDidSaveTextDocument(handler) {
        listeners.save = handler;
        return disposable();
      },
      onDidCreateFiles(handler) {
        listeners.create = handler;
        return disposable();
      },
      onDidDeleteFiles(handler) {
        listeners.delete = handler;
        return disposable();
      },
      onDidRenameFiles(handler) {
        listeners.rename = handler;
        return disposable();
      },
      onDidChangeConfiguration(handler) {
        listeners.config = handler;
        return disposable();
      },
    },
  };

  const childProcessMock = {
    execFile(binary, args, callback) {
      execCalls.push({ binary, args });
      const key = args.join("\u0000");
      const output = commandOutputs.get(key);
      process.nextTick(() => {
        if (!output) {
          callback(new Error(`unexpected command: ${args.join(" ")}`), "", "");
          return;
        }
        callback(null, JSON.stringify(output), "");
      });
    },
  };

  const originalLoad = Module._load;
  Module._load = function patchedLoad(request, parent, isMain) {
    if (request === "vscode") {
      return vscodeMock;
    }
    if (request === "child_process") {
      return childProcessMock;
    }
    return originalLoad.call(this, request, parent, isMain);
  };

  delete require.cache[require.resolve(extensionPath)];
  const extension = require(extensionPath);

  return {
    execCalls,
    extension,
    listeners,
    restore() {
      Module._load = originalLoad;
      delete require.cache[require.resolve(extensionPath)];
    },
    workspaceRoot,
  };
}

function wait(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

test("view builders consume the current CLI report contracts", () => {
  const {
    buildOverview,
    buildHealth,
    buildQuality,
    buildMigration,
    buildReview,
  } = require(viewsPath);

  const bundle = sampleBundle();

  const overview = buildOverview(bundle);
  assert.equal(overview.repoName, "sample-repo");
  assert.equal(overview.frameworkCount, 1);
  assert.equal(overview.testFileCount, 18);
  assert.equal(overview.signalCount, 20);
  assert.equal(overview.topFindings.length, 3);

  const health = buildHealth(bundle);
  assert.equal(health.healthGrade, "D");
  assert.equal(health.runtimeAvailable, false);
  assert.equal(health.skippedTests, 3);
  assert.equal(health.findings.length, 1);

  const quality = buildQuality(bundle);
  assert.equal(quality.findings.length, 2);

  const migration = buildMigration(bundle);
  assert.equal(migration.totalBlockers, 0);
  assert.equal(migration.areaAssessments.length, 4);
  assert.equal(migration.coverageGuidance.length, 2);

  const review = buildReview(bundle);
  assert.equal(review.totalCount, 3);
  assert.equal(review.byCategory.length, 3);
  assert.ok(review.byDirectory.length > 0);
});

test("activate runs analyze, insights, and migration readiness for the workspace root", async () => {
  const harness = createHarness();
  try {
    harness.extension.activate({ subscriptions: [] });
    await wait(10);

    assert.equal(harness.execCalls.length, 3);
    assert.deepEqual(harness.execCalls, [
      {
        binary: "terrain",
        args: ["analyze", "--json", "--root", harness.workspaceRoot],
      },
      {
        binary: "terrain",
        args: ["insights", "--json", "--root", harness.workspaceRoot],
      },
      {
        binary: "terrain",
        args: ["migration", "readiness", "--json", "--root", harness.workspaceRoot],
      },
    ]);
  } finally {
    harness.restore();
  }
});

test("auto refresh reruns the report bundle for supported file saves", async () => {
  const harness = createHarness({ autoRefresh: true });
  try {
    harness.extension.activate({ subscriptions: [] });
    await wait(10);

    assert.equal(harness.execCalls.length, 3);
    assert.equal(typeof harness.listeners.save, "function");

    harness.listeners.save({
      uri: {
        scheme: "file",
        fsPath: path.join(harness.workspaceRoot, "tests", "user.spec.ts"),
      },
    });
    await wait(450);

    assert.equal(harness.execCalls.length, 6);
    assert.deepEqual(harness.execCalls.slice(3), [
      {
        binary: "terrain",
        args: ["analyze", "--json", "--root", harness.workspaceRoot],
      },
      {
        binary: "terrain",
        args: ["insights", "--json", "--root", harness.workspaceRoot],
      },
      {
        binary: "terrain",
        args: ["migration", "readiness", "--json", "--root", harness.workspaceRoot],
      },
    ]);
  } finally {
    harness.restore();
  }
});

test("auto refresh ignores excluded paths", async () => {
  const harness = createHarness({ autoRefresh: true });
  try {
    harness.extension.activate({ subscriptions: [] });
    await wait(10);

    harness.listeners.save({
      uri: {
        scheme: "file",
        fsPath: path.join(
          harness.workspaceRoot,
          "node_modules",
          "dep",
          "index.spec.ts"
        ),
      },
    });
    await wait(450);

    assert.equal(harness.execCalls.length, 3);
  } finally {
    harness.restore();
  }
});
