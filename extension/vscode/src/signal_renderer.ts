// Helpers for transforming CLI findings into grouped view items.
// Keep this thin: formatting and grouping only.

import { InsightFinding } from "./types";

export interface GroupedFinding {
  key: string;
  findings: InsightFinding[];
  count: number;
}

export function groupFindingsByCategory(
  findings: InsightFinding[]
): GroupedFinding[] {
  return groupFindings(findings, (finding) => finding.category);
}

export function groupFindingsBySeverity(
  findings: InsightFinding[]
): GroupedFinding[] {
  return groupFindings(findings, (finding) => finding.severity);
}

export function groupFindingsByDirectory(
  findings: InsightFinding[]
): GroupedFinding[] {
  return groupFindings(findings, (finding) => {
    const scope = finding.scope || "";
    if (!scope || scope.startsWith("scenario:")) {
      return "(repo-level)";
    }
    const normalized = scope.replace(/^file:/, "");
    const lastSlash = normalized.lastIndexOf("/");
    return lastSlash > 0 ? normalized.substring(0, lastSlash) : "(repo-level)";
  });
}

export function reviewWorthyFindings(
  findings: InsightFinding[]
): InsightFinding[] {
  const reviewSeverities = new Set(["medium", "high", "critical"]);
  return findings.filter((finding) => reviewSeverities.has(finding.severity));
}

export function severityIcon(severity: string): string {
  switch (severity) {
    case "critical":
      return "error";
    case "high":
      return "warning";
    case "medium":
      return "info";
    case "low":
      return "debug-stackframe-dot";
    default:
      return "circle-outline";
  }
}

export function riskBandIcon(band: string): string {
  switch (band) {
    case "critical":
      return "error";
    case "high":
      return "warning";
    case "medium":
    case "moderate":
      return "info";
    case "weak":
      return "warning";
    default:
      return "pass";
  }
}

function groupFindings(
  findings: InsightFinding[],
  keyFn: (finding: InsightFinding) => string
): GroupedFinding[] {
  const map = new Map<string, InsightFinding[]>();
  for (const finding of findings) {
    const key = keyFn(finding);
    const existing = map.get(key);
    if (existing) {
      existing.push(finding);
    } else {
      map.set(key, [finding]);
    }
  }

  const groups: GroupedFinding[] = [];
  for (const [key, groupedFindings] of map) {
    groups.push({ key, findings: groupedFindings, count: groupedFindings.length });
  }
  groups.sort((left, right) => right.count - left.count || left.key.localeCompare(right.key));
  return groups;
}
