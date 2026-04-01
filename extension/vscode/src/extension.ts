// Terrain VS Code Extension
//
// Thin client over the CLI. The extension renders the same report contracts
// users can run manually from the terminal.

import { execFile } from "child_process";
import * as path from "path";
import * as vscode from "vscode";
import { GroupedFinding, riskBandIcon, severityIcon } from "./signal_renderer";
import { InsightFinding, KeyFinding, MigrationReadiness, ReportBundle } from "./types";
import {
  buildHealth,
  buildMigration,
  buildOverview,
  buildQuality,
  buildReview,
  HealthData,
  MigrationData,
  OverviewData,
  QualityData,
  ReviewData,
} from "./views";

// ── State ──────────────────────────────────────────────────

type ExtensionState =
  | { kind: "empty" }
  | { kind: "loading" }
  | { kind: "error"; message: string }
  | { kind: "loaded"; bundle: ReportBundle };

let state: ExtensionState = { kind: "empty" };
let autoRefreshHandle: ReturnType<typeof setTimeout> | undefined;

// ── Activation ─────────────────────────────────────────────

export function activate(context: vscode.ExtensionContext) {
  const overviewProvider = new OverviewTreeProvider();
  const healthProvider = new HealthTreeProvider();
  const qualityProvider = new QualityTreeProvider();
  const migrationProvider = new MigrationTreeProvider();
  const reviewProvider = new ReviewTreeProvider();

  const providers = [
    overviewProvider,
    healthProvider,
    qualityProvider,
    migrationProvider,
    reviewProvider,
  ];

  context.subscriptions.push(
    vscode.window.registerTreeDataProvider("terrain.overview", overviewProvider),
    vscode.window.registerTreeDataProvider("terrain.health", healthProvider),
    vscode.window.registerTreeDataProvider("terrain.quality", qualityProvider),
    vscode.window.registerTreeDataProvider("terrain.migration", migrationProvider),
    vscode.window.registerTreeDataProvider("terrain.review", reviewProvider)
  );

  context.subscriptions.push(
    vscode.commands.registerCommand("terrain.refresh", () => {
      runAnalysis(providers);
    }),
    vscode.commands.registerCommand("terrain.openSummary", () => {
      runCliInTerminal("summary");
    }),
    vscode.commands.registerCommand("terrain.openMigrationBlockers", () => {
      runCliInTerminal("migration blockers");
    }),
    vscode.commands.registerCommand("terrain.revealFile", (filePath: string) => {
      if (filePath && vscode.workspace.workspaceFolders?.[0]) {
        const uri = vscode.Uri.joinPath(
          vscode.workspace.workspaceFolders[0].uri,
          filePath
        );
        void vscode.window.showTextDocument(uri);
      }
    })
  );

  context.subscriptions.push(
    vscode.workspace.onDidSaveTextDocument((document) => {
      if (shouldAutoRefreshUri(document.uri)) {
        scheduleAutoRefresh(providers);
      }
    }),
    vscode.workspace.onDidCreateFiles((event) => {
      if (event.files.some(shouldAutoRefreshUri)) {
        scheduleAutoRefresh(providers);
      }
    }),
    vscode.workspace.onDidDeleteFiles((event) => {
      if (event.files.some(shouldAutoRefreshUri)) {
        scheduleAutoRefresh(providers);
      }
    }),
    vscode.workspace.onDidRenameFiles((event) => {
      if (
        event.files.some(
          ({ oldUri, newUri }) =>
            shouldAutoRefreshUri(oldUri) || shouldAutoRefreshUri(newUri)
        )
      ) {
        scheduleAutoRefresh(providers);
      }
    }),
    vscode.workspace.onDidChangeConfiguration((event) => {
      if (
        event.affectsConfiguration("terrain.autoRefresh") ||
        event.affectsConfiguration("terrain.binaryPath")
      ) {
        scheduleAutoRefresh(providers);
      }
    })
  );

  runAnalysis(providers);
}

export function deactivate() {}

// ── CLI Integration ────────────────────────────────────────

function getTerrainBinary(): string {
  return (
    vscode.workspace.getConfiguration("terrain").get<string>("binaryPath") ||
    "terrain"
  );
}

function getWorkspaceRoot(): string | undefined {
  return vscode.workspace.workspaceFolders?.[0]?.uri.fsPath;
}

function isAutoRefreshEnabled(): boolean {
  return (
    vscode.workspace.getConfiguration("terrain").get<boolean>("autoRefresh") ||
    false
  );
}

function shouldAutoRefreshUri(uri: vscode.Uri): boolean {
  const root = getWorkspaceRoot();
  if (!root || uri.scheme !== "file") {
    return false;
  }

  const rel = path.relative(root, uri.fsPath);
  if (!rel || rel.startsWith("..") || path.isAbsolute(rel)) {
    return false;
  }

  const ignored = new Set([
    ".git",
    ".terrain",
    "node_modules",
    "dist",
    "build",
    "out",
  ]);
  return !rel.split(path.sep).some((segment) => ignored.has(segment));
}

function scheduleAutoRefresh(providers: TerrainTreeProvider[]) {
  if (!isAutoRefreshEnabled()) {
    return;
  }
  if (autoRefreshHandle) {
    clearTimeout(autoRefreshHandle);
  }
  autoRefreshHandle = setTimeout(() => {
    autoRefreshHandle = undefined;
    runAnalysis(providers);
  }, 400);
}

function execTerrainJSON<T>(binary: string, args: string[]): Promise<T> {
  return new Promise((resolve, reject) => {
    execFile(binary, args, (err, stdout, stderr) => {
      if (err) {
        const detail = stderr?.trim() || err.message;
        reject(new Error(`${args.join(" ")} failed: ${detail}`));
        return;
      }

      try {
        resolve(JSON.parse(stdout) as T);
      } catch (parseErr) {
        const message =
          parseErr instanceof Error ? parseErr.message : String(parseErr);
        reject(new Error(`Failed to parse ${args.join(" ")} JSON: ${message}`));
      }
    });
  });
}

async function loadReportBundle(binary: string, root: string): Promise<ReportBundle> {
  const [analyze, insights, migration] = await Promise.all([
    execTerrainJSON<ReportBundle["analyze"]>(binary, [
      "analyze",
      "--json",
      "--root",
      root,
    ]),
    execTerrainJSON<ReportBundle["insights"]>(binary, [
      "insights",
      "--json",
      "--root",
      root,
    ]),
    execTerrainJSON<MigrationReadiness>(binary, [
      "migration",
      "readiness",
      "--json",
      "--root",
      root,
    ]),
  ]);

  return { analyze, insights, migration };
}

function runAnalysis(providers: TerrainTreeProvider[]) {
  const root = getWorkspaceRoot();
  if (!root) {
    state = { kind: "error", message: "No workspace folder open" };
    providers.forEach((provider) => provider.refresh());
    return;
  }

  state = { kind: "loading" };
  providers.forEach((provider) => provider.refresh());

  const binary = getTerrainBinary();
  void loadReportBundle(binary, root)
    .then((bundle) => {
      state = { kind: "loaded", bundle };
      providers.forEach((provider) => provider.refresh());
    })
    .catch((error) => {
      const message = error instanceof Error ? error.message : String(error);
      state = { kind: "error", message };
      providers.forEach((provider) => provider.refresh());
    });
}

function runCliInTerminal(subcommand: string) {
  const root = getWorkspaceRoot();
  const binary = getTerrainBinary();
  const terminal = vscode.window.createTerminal("Terrain");
  const rootFlag = root ? ` --root "${root}"` : "";
  terminal.sendText(`${binary} ${subcommand}${rootFlag}`);
  terminal.show();
}

// ── Base TreeDataProvider ──────────────────────────────────

interface TerrainTreeProvider extends vscode.TreeDataProvider<TerrainTreeItem> {
  refresh(): void;
}

class TerrainTreeItem extends vscode.TreeItem {
  constructor(
    label: string,
    collapsibleState: vscode.TreeItemCollapsibleState = vscode
      .TreeItemCollapsibleState.None,
    public children: TerrainTreeItem[] = []
  ) {
    super(label, collapsibleState);
  }
}

function stateItems(): TerrainTreeItem[] | null {
  switch (state.kind) {
    case "empty":
      return [
        makeItem(
          "Run terrain.refresh to analyze",
          "$(play)",
          "No analysis data yet. Click refresh or run 'Terrain: Refresh Analysis'."
        ),
      ];
    case "loading":
      return [makeItem("Analyzing...", "$(sync~spin)")];
    case "error":
      return [
        makeItem("Analysis failed", "$(error)"),
        makeItem(state.message, "$(info)"),
        makeItem(
          "Is terrain installed?",
          "$(question)",
          "Install: go install github.com/pmclSF/terrain/cmd/terrain@latest"
        ),
      ];
    case "loaded":
      return null;
  }
}

function loadedBundle(): ReportBundle {
  return (state as { kind: "loaded"; bundle: ReportBundle }).bundle;
}

function makeItem(
  label: string,
  icon?: string,
  tooltip?: string
): TerrainTreeItem {
  const item = new TerrainTreeItem(label);
  if (icon) {
    item.iconPath = new vscode.ThemeIcon(icon.replace("$(", "").replace(")", ""));
  }
  if (tooltip) {
    item.tooltip = tooltip;
  }
  return item;
}

function attachScopeCommand(item: TerrainTreeItem, scope?: string) {
  if (!scope || !scope.startsWith("file:")) {
    return;
  }

  item.command = {
    command: "terrain.revealFile",
    title: "Open File",
    arguments: [scope.slice("file:".length)],
  };
}

function findingItem(finding: InsightFinding): TerrainTreeItem {
  const label = finding.scope ? `${finding.title} — ${finding.scope}` : finding.title;
  const item = makeItem(label, `$(${severityIcon(finding.severity)})`, finding.description);
  item.description = [finding.severity, finding.metric].filter(Boolean).join(" • ");
  attachScopeCommand(item, finding.scope);
  return item;
}

function keyFindingItem(finding: KeyFinding): TerrainTreeItem {
  const item = makeItem(
    finding.title,
    `$(${severityIcon(finding.severity)})`,
    finding.metric || finding.category
  );
  item.description = [finding.severity, finding.metric].filter(Boolean).join(" • ");
  return item;
}

function groupedFindingItems(groups: GroupedFinding[]): TerrainTreeItem[] {
  return groups.map((group) => {
    const children = group.findings.map(findingItem);
    return new TerrainTreeItem(
      `${group.key} (${group.count})`,
      vscode.TreeItemCollapsibleState.Collapsed,
      children
    );
  });
}

// ── Overview Provider ──────────────────────────────────────

class OverviewTreeProvider implements TerrainTreeProvider {
  private _onDidChangeTreeData = new vscode.EventEmitter<void>();
  readonly onDidChangeTreeData = this._onDidChangeTreeData.event;

  refresh() {
    this._onDidChangeTreeData.fire();
  }

  getTreeItem(element: TerrainTreeItem): TerrainTreeItem {
    return element;
  }

  getChildren(element?: TerrainTreeItem): TerrainTreeItem[] {
    if (element) return element.children;
    const items = stateItems();
    if (items) return items;
    const data = buildOverview(loadedBundle());
    return this.buildItems(data);
  }

  private buildItems(data: OverviewData): TerrainTreeItem[] {
    const items: TerrainTreeItem[] = [
      makeItem(`Repository: ${data.repoName}`, "$(repo)"),
      makeItem(`Frameworks: ${data.frameworkCount}`, "$(package)"),
      makeItem(`Test files: ${data.testFileCount}`, "$(file-code)"),
      makeItem(`Signals: ${data.signalCount}`, "$(warning)"),
    ];

    if (data.branch) {
      items.push(makeItem(`Branch: ${data.branch}`, "$(git-branch)"));
    }

    items.push(makeItem(data.headline, "$(info)"));

    if (data.riskPosture.length > 0) {
      const riskChildren = data.riskPosture.map((risk) => {
        const item = makeItem(
          `${risk.dimension}: ${risk.band.toUpperCase()}`,
          `$(${riskBandIcon(risk.band)})`
        );
        item.description = risk.band;
        return item;
      });
      items.push(
        new TerrainTreeItem(
          "Risk Posture",
          vscode.TreeItemCollapsibleState.Expanded,
          riskChildren
        )
      );
    }

    if (data.topFindings.length > 0) {
      items.push(
        new TerrainTreeItem(
          "Top Findings",
          vscode.TreeItemCollapsibleState.Collapsed,
          data.topFindings.map(keyFindingItem)
        )
      );
    }

    return items;
  }
}

// ── Health Provider ────────────────────────────────────────

class HealthTreeProvider implements TerrainTreeProvider {
  private _onDidChangeTreeData = new vscode.EventEmitter<void>();
  readonly onDidChangeTreeData = this._onDidChangeTreeData.event;

  refresh() {
    this._onDidChangeTreeData.fire();
  }

  getTreeItem(element: TerrainTreeItem): TerrainTreeItem {
    return element;
  }

  getChildren(element?: TerrainTreeItem): TerrainTreeItem[] {
    if (element) return element.children;
    const items = stateItems();
    if (items) return items;
    const data = buildHealth(loadedBundle());
    return this.buildItems(data);
  }

  private buildItems(data: HealthData): TerrainTreeItem[] {
    const items: TerrainTreeItem[] = [
      makeItem(`Health grade: ${data.healthGrade}`, "$(heart)"),
    ];

    if (data.skippedTests > 0) {
      items.push(
        makeItem(
          `Skipped tests detected: ${data.skippedTests}`,
          "$(debug-pause)",
          "Static skip detection works without runtime artifacts."
        )
      );
    }

    if (data.findings.length === 0) {
      const tooltip = data.runtimeAvailable
        ? "No reliability findings surfaced in the current insights report."
        : "No runtime data detected. Static skip detection still works, but flaky/slow/dead findings require runtime artifacts.";
      items.push(makeItem("No reliability findings detected", "$(pass)", tooltip));
      return items;
    }

    items.push(
      ...groupedFindingItems(data.bySeverity)
    );
    return items;
  }
}

// ── Quality Provider ───────────────────────────────────────

class QualityTreeProvider implements TerrainTreeProvider {
  private _onDidChangeTreeData = new vscode.EventEmitter<void>();
  readonly onDidChangeTreeData = this._onDidChangeTreeData.event;

  refresh() {
    this._onDidChangeTreeData.fire();
  }

  getTreeItem(element: TerrainTreeItem): TerrainTreeItem {
    return element;
  }

  getChildren(element?: TerrainTreeItem): TerrainTreeItem[] {
    if (element) return element.children;
    const items = stateItems();
    if (items) return items;
    const data = buildQuality(loadedBundle());
    return this.buildItems(data);
  }

  private buildItems(data: QualityData): TerrainTreeItem[] {
    if (data.findings.length === 0) {
      return [makeItem("No coverage or architecture findings", "$(pass)")];
    }
    return groupedFindingItems(data.byCategory);
  }
}

// ── Migration Provider ─────────────────────────────────────

class MigrationTreeProvider implements TerrainTreeProvider {
  private _onDidChangeTreeData = new vscode.EventEmitter<void>();
  readonly onDidChangeTreeData = this._onDidChangeTreeData.event;

  refresh() {
    this._onDidChangeTreeData.fire();
  }

  getTreeItem(element: TerrainTreeItem): TerrainTreeItem {
    return element;
  }

  getChildren(element?: TerrainTreeItem): TerrainTreeItem[] {
    if (element) return element.children;
    const items = stateItems();
    if (items) return items;
    const data = buildMigration(loadedBundle());
    return this.buildItems(data);
  }

  private buildItems(data: MigrationData): TerrainTreeItem[] {
    const readinessIcon = data.totalBlockers > 0 ? "$(warning)" : "$(pass)";
    const items: TerrainTreeItem[] = [
      makeItem(
        `Readiness: ${data.readinessLevel.toUpperCase()}`,
        readinessIcon
      ),
      makeItem(data.explanation, "$(info)"),
    ];

    if (data.frameworkSummary.length > 0) {
      items.push(
        new TerrainTreeItem(
          "Frameworks",
          vscode.TreeItemCollapsibleState.Expanded,
          data.frameworkSummary.map((framework) => makeItem(framework, "$(package)"))
        )
      );
    }

    if (data.totalBlockers === 0) {
      items.push(makeItem("No migration blockers found", "$(pass)"));
    } else {
      items.push(makeItem(`${data.totalBlockers} migration blocker(s)`, "$(warning)"));
    }

    if (data.blockerGroups.length > 0) {
      items.push(
        new TerrainTreeItem(
          "Blockers by Type",
          vscode.TreeItemCollapsibleState.Collapsed,
          data.blockerGroups.map((group) =>
            makeItem(`${group.key} (${group.count})`, "$(warning)")
          )
        )
      );
    }

    if (data.areaAssessments.length > 0) {
      const areaChildren = data.areaAssessments.map((assessment) => {
        const icon =
          assessment.classification === "risky"
            ? "error"
            : assessment.classification === "caution"
            ? "warning"
            : "pass";
        const item = makeItem(
          `${assessment.directory} [${assessment.classification.toUpperCase()}]`,
          `$(${icon})`,
          assessment.explanation
        );
        item.description = `${assessment.migrationBlockers} blockers, ${assessment.qualityIssues} quality issues`;
        return item;
      });
      items.push(
        new TerrainTreeItem(
          "Area Assessments",
          vscode.TreeItemCollapsibleState.Collapsed,
          areaChildren
        )
      );
    }

    if (data.coverageGuidance.length > 0) {
      const coverageChildren = data.coverageGuidance.map((guidance) => {
        const item = makeItem(
          `${guidance.directory} (${guidance.priority})`,
          "$(lightbulb)",
          guidance.reason
        );
        item.description = guidance.reason;
        return item;
      });
      items.push(
        new TerrainTreeItem(
          "Coverage Guidance",
          vscode.TreeItemCollapsibleState.Collapsed,
          coverageChildren
        )
      );
    }

    return items;
  }
}

// ── Review Provider ────────────────────────────────────────

class ReviewTreeProvider implements TerrainTreeProvider {
  private _onDidChangeTreeData = new vscode.EventEmitter<void>();
  readonly onDidChangeTreeData = this._onDidChangeTreeData.event;

  refresh() {
    this._onDidChangeTreeData.fire();
  }

  getTreeItem(element: TerrainTreeItem): TerrainTreeItem {
    return element;
  }

  getChildren(element?: TerrainTreeItem): TerrainTreeItem[] {
    if (element) return element.children;
    const items = stateItems();
    if (items) return items;
    const data = buildReview(loadedBundle());
    return this.buildItems(data);
  }

  private buildItems(data: ReviewData): TerrainTreeItem[] {
    if (data.totalCount === 0) {
      return [makeItem("No review-worthy findings", "$(pass)")];
    }

    const items: TerrainTreeItem[] = [
      makeItem(`${data.totalCount} findings need attention`, "$(warning)"),
    ];

    if (data.byCategory.length > 0) {
      items.push(
        new TerrainTreeItem(
          "By Category",
          vscode.TreeItemCollapsibleState.Collapsed,
          groupedFindingItems(data.byCategory)
        )
      );
    }

    if (data.bySeverity.length > 0) {
      items.push(
        new TerrainTreeItem(
          "By Severity",
          vscode.TreeItemCollapsibleState.Collapsed,
          groupedFindingItems(data.bySeverity)
        )
      );
    }

    if (data.byDirectory.length > 0) {
      items.push(
        new TerrainTreeItem(
          "By Directory",
          vscode.TreeItemCollapsibleState.Collapsed,
          groupedFindingItems(data.byDirectory)
        )
      );
    }

    return items;
  }
}
