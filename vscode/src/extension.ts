// Terrain VS Code extension.
//
// Reads .terrain/findings.json from the active workspace and surfaces
// every Finding in the Problems pane with the appropriate severity.
// Click-to-navigate uses the Diagnostic's range; hover renders the
// long_message, cause-path summary, reproduction command, and docs
// link.
//
// Per PRODUCT.md §16 minimum capability set for 0.2.0:
//   - artifacts → Problems pane
//   - click-to-navigate
//   - hover diagnostics

import * as vscode from "vscode";
import * as fs from "fs";
import * as path from "path";

interface Location {
  path: string;
  line?: number;
  column?: number;
  end_line?: number;
  end_column?: number;
  node_kind?: string;
  node_id?: string;
  symbol?: string;
}

interface Finding {
  version: number;
  rule_id: string;
  severity: "error" | "warning" | "notice";
  tier?: string;
  primary_loc: Location;
  cause_loc?: Location;
  cause_path?: Location[];
  short_message: string;
  long_message?: string;
  docs_url?: string;
  reproduction?: string;
  metadata?: Record<string, unknown>;
}

interface Artifact {
  version: number;
  findings: Finding[];
}

const COLLECTION_NAME = "terrain";

let diagnostics: vscode.DiagnosticCollection | undefined;
let activeArtifact: Artifact | undefined;

export function activate(context: vscode.ExtensionContext): void {
  diagnostics = vscode.languages.createDiagnosticCollection(COLLECTION_NAME);
  context.subscriptions.push(diagnostics);

  context.subscriptions.push(
    vscode.commands.registerCommand("terrain.refresh", refreshDiagnostics),
    vscode.commands.registerCommand("terrain.openRuleDocs", openRuleDocs),
  );

  // Initial load.
  refreshDiagnostics();

  // Watch the findings file for changes.
  const watcher = vscode.workspace.createFileSystemWatcher(
    findingsGlobPattern(),
  );
  watcher.onDidCreate(refreshDiagnostics);
  watcher.onDidChange(refreshDiagnostics);
  watcher.onDidDelete(() => {
    if (diagnostics) {
      diagnostics.clear();
    }
    activeArtifact = undefined;
  });
  context.subscriptions.push(watcher);
}

export function deactivate(): void {
  if (diagnostics) {
    diagnostics.dispose();
  }
}

function findingsGlobPattern(): vscode.GlobPattern {
  const config = vscode.workspace.getConfiguration("terrain");
  const relPath = config.get<string>("findingsPath", ".terrain/findings.json");
  return new vscode.RelativePattern(
    vscode.workspace.workspaceFolders?.[0] ??
      vscode.Uri.file(process.cwd()),
    relPath,
  );
}

async function refreshDiagnostics(): Promise<void> {
  if (!diagnostics) {
    return;
  }
  diagnostics.clear();

  const workspaceRoot = vscode.workspace.workspaceFolders?.[0]?.uri.fsPath;
  if (!workspaceRoot) {
    return;
  }

  const config = vscode.workspace.getConfiguration("terrain");
  const relPath = config.get<string>("findingsPath", ".terrain/findings.json");
  const absPath = path.join(workspaceRoot, relPath);

  let artifact: Artifact;
  try {
    const raw = await fs.promises.readFile(absPath, "utf8");
    artifact = JSON.parse(raw);
  } catch {
    activeArtifact = undefined;
    return;
  }
  activeArtifact = artifact;

  // Group findings by file path.
  const byFile = new Map<string, vscode.Diagnostic[]>();
  for (const finding of artifact.findings ?? []) {
    const filePath = path.join(workspaceRoot, finding.primary_loc.path);
    const diag = findingToDiagnostic(finding);
    if (!diag) {
      continue;
    }
    const list = byFile.get(filePath) ?? [];
    list.push(diag);
    byFile.set(filePath, list);
  }

  for (const [filePath, diags] of byFile.entries()) {
    diagnostics.set(vscode.Uri.file(filePath), diags);
  }
}

function findingToDiagnostic(f: Finding): vscode.Diagnostic | undefined {
  const line = (f.primary_loc.line ?? 1) - 1;
  const col = (f.primary_loc.column ?? 1) - 1;
  const endLine = (f.primary_loc.end_line ?? f.primary_loc.line ?? 1) - 1;
  const endCol = (f.primary_loc.end_column ?? (f.primary_loc.column ?? 1) + 1) - 1;
  const range = new vscode.Range(line, col, endLine, Math.max(endCol, col + 1));

  const diag = new vscode.Diagnostic(
    range,
    f.short_message,
    severityToCode(f.severity),
  );
  diag.source = "terrain";
  diag.code = {
    value: f.rule_id,
    target: vscode.Uri.parse(f.docs_url ?? "https://terrain.dev/"),
  };

  const tags: vscode.DiagnosticTag[] = [];
  if (f.tier === "preview") {
    tags.push(vscode.DiagnosticTag.Unnecessary);
  }
  diag.tags = tags;

  const related: vscode.DiagnosticRelatedInformation[] = [];
  if (f.cause_path && f.cause_path.length > 0) {
    const workspaceRoot =
      vscode.workspace.workspaceFolders?.[0]?.uri.fsPath ?? "";
    for (const loc of f.cause_path) {
      const lineN = (loc.line ?? 1) - 1;
      related.push(
        new vscode.DiagnosticRelatedInformation(
          new vscode.Location(
            vscode.Uri.file(path.join(workspaceRoot, loc.path)),
            new vscode.Range(lineN, 0, lineN, 0),
          ),
          `cause-path: ${loc.path}${loc.line ? `:${loc.line}` : ""}${
            loc.node_kind ? ` (${loc.node_kind})` : ""
          }`,
        ),
      );
    }
  }
  if (f.long_message) {
    // Add the long message as related info pointed at the primary loc.
    const workspaceRoot =
      vscode.workspace.workspaceFolders?.[0]?.uri.fsPath ?? "";
    related.push(
      new vscode.DiagnosticRelatedInformation(
        new vscode.Location(
          vscode.Uri.file(path.join(workspaceRoot, f.primary_loc.path)),
          range,
        ),
        f.long_message,
      ),
    );
  }
  if (f.reproduction) {
    const workspaceRoot =
      vscode.workspace.workspaceFolders?.[0]?.uri.fsPath ?? "";
    related.push(
      new vscode.DiagnosticRelatedInformation(
        new vscode.Location(
          vscode.Uri.file(path.join(workspaceRoot, f.primary_loc.path)),
          range,
        ),
        `Reproduce: ${f.reproduction}`,
      ),
    );
  }
  diag.relatedInformation = related;

  return diag;
}

function severityToCode(s: Finding["severity"]): vscode.DiagnosticSeverity {
  switch (s) {
    case "error":
      return vscode.DiagnosticSeverity.Error;
    case "warning":
      return vscode.DiagnosticSeverity.Warning;
    default:
      return vscode.DiagnosticSeverity.Information;
  }
}

async function openRuleDocs(): Promise<void> {
  if (!activeArtifact || activeArtifact.findings.length === 0) {
    vscode.window.showInformationMessage(
      "Terrain: no findings loaded. Run `terrain analyze` first.",
    );
    return;
  }
  const items = activeArtifact.findings.map((f) => ({
    label: f.rule_id,
    description: f.short_message,
    finding: f,
  }));
  const picked = await vscode.window.showQuickPick(items, {
    placeHolder: "Pick a finding to open its rule docs",
  });
  if (!picked) {
    return;
  }
  const url = picked.finding.docs_url ?? "https://terrain.dev/";
  await vscode.env.openExternal(vscode.Uri.parse(url));
}
