// Extension view descriptions for the VS Code / Cursor sidebar.
//
// The extension consumes the same JSON contracts users do:
// - `terrain analyze --json`
// - `terrain insights --json`
// - `terrain migration readiness --json`

import {
  CoverageGuidanceItem,
  InsightFinding,
  KeyFinding,
  MigrationAreaAssessment,
  ReportBundle,
  RiskDimension,
} from "./types";
import {
  groupFindingsByCategory,
  groupFindingsByDirectory,
  groupFindingsBySeverity,
  GroupedFinding,
  reviewWorthyFindings,
} from "./signal_renderer";

// ---------- Overview View ----------

export interface OverviewData {
  repoName: string;
  branch: string;
  frameworkCount: number;
  testFileCount: number;
  signalCount: number;
  headline: string;
  topFindings: KeyFinding[];
  riskPosture: RiskDimension[];
}

export function buildOverview(bundle: ReportBundle): OverviewData {
  const { analyze } = bundle;
  return {
    repoName: analyze.repository.name,
    branch: analyze.repository.branch || "",
    frameworkCount: analyze.testsDetected.frameworks.length,
    testFileCount: analyze.testsDetected.testFileCount,
    signalCount: analyze.signalSummary.total,
    headline: analyze.headline,
    topFindings: analyze.keyFindings || [],
    riskPosture: analyze.riskPosture || [],
  };
}

// ---------- Health View ----------

export interface HealthData {
  healthGrade: string;
  findings: InsightFinding[];
  bySeverity: GroupedFinding[];
  runtimeAvailable: boolean;
  skippedTests: number;
  limitations: string[];
}

export function buildHealth(bundle: ReportBundle): HealthData {
  const runtimeAvailable = hasDataSource(bundle.analyze.dataCompleteness, "runtime");
  const findings = bundle.insights.findings.filter(
    (finding) => finding.category === "reliability"
  );
  return {
    healthGrade: bundle.insights.healthGrade,
    findings,
    bySeverity: groupFindingsBySeverity(findings),
    runtimeAvailable,
    skippedTests: bundle.analyze.skippedTestBurden?.skippedCount || 0,
    limitations: bundle.insights.limitations || [],
  };
}

// ---------- Quality View ----------

export interface QualityData {
  findings: InsightFinding[];
  byCategory: GroupedFinding[];
  limitations: string[];
}

export function buildQuality(bundle: ReportBundle): QualityData {
  const findings = bundle.insights.findings.filter((finding) =>
    new Set(["coverage_debt", "architecture_debt"]).has(finding.category)
  );
  return {
    findings,
    byCategory: groupFindingsByCategory(findings),
    limitations: bundle.insights.limitations || [],
  };
}

// ---------- Migration View ----------

export interface BlockerGroup {
  key: string;
  count: number;
}

export interface MigrationData {
  frameworkSummary: string[];
  totalBlockers: number;
  blockerGroups: BlockerGroup[];
  areaAssessments: MigrationAreaAssessment[];
  readinessLevel: string;
  explanation: string;
  coverageGuidance: CoverageGuidanceItem[];
}

export function buildMigration(bundle: ReportBundle): MigrationData {
  const blockerGroups = Object.entries(bundle.migration.blockersByType || {})
    .map(([key, count]) => ({ key, count }))
    .sort((left, right) => right.count - left.count || left.key.localeCompare(right.key));

  return {
    frameworkSummary: bundle.migration.frameworks.map((framework) => {
      const typeLabel = framework.type ? ` [${framework.type}]` : "";
      return `${framework.name} (${framework.fileCount} files)${typeLabel}`;
    }),
    totalBlockers: bundle.migration.totalBlockers,
    blockerGroups,
    areaAssessments: bundle.migration.areaAssessments || [],
    readinessLevel: bundle.migration.readinessLevel,
    explanation: bundle.migration.explanation,
    coverageGuidance: bundle.migration.coverageGuidance || [],
  };
}

// ---------- Review View ----------

export interface ReviewData {
  findings: InsightFinding[];
  byCategory: GroupedFinding[];
  bySeverity: GroupedFinding[];
  byDirectory: GroupedFinding[];
  totalCount: number;
}

export function buildReview(bundle: ReportBundle): ReviewData {
  const findings = reviewWorthyFindings(bundle.insights.findings);
  return {
    findings,
    byCategory: groupFindingsByCategory(findings),
    bySeverity: groupFindingsBySeverity(findings),
    byDirectory: groupFindingsByDirectory(findings),
    totalCount: findings.length,
  };
}

function hasDataSource(dataSources: { name: string; available: boolean }[], name: string): boolean {
  return dataSources.some(
    (dataSource) => dataSource.name === name && dataSource.available
  );
}
