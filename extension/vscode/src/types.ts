// Types aligned with the current Terrain CLI JSON contracts.
// The extension consumes report-oriented JSON, not the internal engine snapshot.

export interface DataSource {
  name: string;
  available: boolean;
}

export interface RepositoryInfo {
  name: string;
  branch?: string;
  commitSha?: string;
  languages?: string[];
}

export interface FrameworkSummary {
  name: string;
  type?: string;
  fileCount: number;
  testCount?: number;
}

export interface TestSummary {
  testFileCount: number;
  testCaseCount: number;
  codeUnitCount: number;
  scenarioCount?: number;
  codeSurfaceCount?: number;
  promptCount?: number;
  datasetCount?: number;
  frameworks: FrameworkSummary[];
}

export interface KeyFinding {
  title: string;
  severity: "critical" | "high" | "medium" | "low";
  category: string;
  metric?: string;
}

export interface RiskDimension {
  dimension: string;
  band:
    | "critical"
    | "high"
    | "medium"
    | "low"
    | "strong"
    | "moderate"
    | "weak"
    | "unknown";
}

export interface SignalSummary {
  total: number;
  critical: number;
  high: number;
  medium: number;
  low: number;
  byCategory?: Record<string, number>;
}

export interface SkipSummary {
  skippedCount: number;
  totalTests: number;
  skipRatio: number;
}

export interface AnalyzeReport {
  schemaVersion: string;
  repository: RepositoryInfo;
  dataCompleteness: DataSource[];
  testsDetected: TestSummary;
  skippedTestBurden?: SkipSummary;
  keyFindings?: KeyFinding[];
  totalFindingCount?: number;
  riskPosture?: RiskDimension[];
  signalSummary: SignalSummary;
  headline: string;
  limitations?: string[];
}

export interface InsightFinding {
  title: string;
  description: string;
  category:
    | "optimization"
    | "reliability"
    | "architecture_debt"
    | "coverage_debt";
  severity: "critical" | "high" | "medium" | "low";
  priority: number;
  scope?: string;
  metric?: string;
}

export interface InsightRecommendation {
  action: string;
  rationale: string;
  category:
    | "optimization"
    | "reliability"
    | "architecture_debt"
    | "coverage_debt";
  priority: number;
  impact?: string;
  targetFiles?: string[];
  effortBand?: string;
  command?: string;
}

export interface InsightsReport {
  headline: string;
  healthGrade: string;
  findings: InsightFinding[];
  recommendations: InsightRecommendation[];
  dataCompleteness: DataSource[];
  limitations?: string[];
}

export interface MigrationBlockerExample {
  type: string;
  file: string;
  explanation: string;
}

export interface MigrationAreaAssessment {
  directory: string;
  classification: "safe" | "caution" | "risky";
  migrationBlockers: number;
  qualityIssues: number;
  testFileCount: number;
  explanation: string;
}

export interface CoverageGuidanceItem {
  directory: string;
  reason: string;
  priority: "high" | "medium" | "low";
}

export interface MigrationReadiness {
  frameworks: FrameworkSummary[];
  totalBlockers: number;
  blockersByType: Record<string, number>;
  representativeBlockers?: MigrationBlockerExample[] | null;
  readinessLevel: "low" | "medium" | "high" | "unknown";
  explanation: string;
  areaAssessments?: MigrationAreaAssessment[];
  coverageGuidance?: CoverageGuidanceItem[];
}

export interface ReportBundle {
  analyze: AnalyzeReport;
  insights: InsightsReport;
  migration: MigrationReadiness;
}
