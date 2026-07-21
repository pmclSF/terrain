package models

// SignalSource describes how a signal type is produced.
type SignalSource string

const (
	// SignalSourceStatic signals are produced by static code analysis (no external data needed).
	SignalSourceStatic SignalSource = "static"
	// SignalSourceRuntime signals require runtime test results (JUnit XML, Jest JSON).
	SignalSourceRuntime SignalSource = "runtime"
	// SignalSourceGraph signals are produced by dependency graph traversal.
	SignalSourceGraph SignalSource = "graph"
	// SignalSourceGauntlet signals are produced by Gauntlet evaluation artifact ingestion.
	SignalSourceGauntlet SignalSource = "gauntlet"
)

// SignalCatalogEntry describes a signal type's provenance and data requirements.
type SignalCatalogEntry struct {
	Source SignalSource
}

// SignalCatalog maps every known signal type to its source tier.
// This enables tooling to distinguish which signals require external data.
var SignalCatalog = map[SignalType]SignalCatalogEntry{
	// Core static signals (Tier 1) — no external data required.
	"untestedExport":                     {Source: SignalSourceStatic},
	"weakAssertion":                      {Source: SignalSourceStatic},
	"mockHeavyTest":                      {Source: SignalSourceStatic},
	"testsOnlyMocks":                     {Source: SignalSourceStatic},
	"snapshotHeavyTest":                  {Source: SignalSourceStatic},
	"coverageBlindSpot":                  {Source: SignalSourceStatic},
	"coverageThresholdBreak":             {Source: SignalSourceStatic},
	"frameworkMigration":                 {Source: SignalSourceStatic},
	"migrationBlocker":                   {Source: SignalSourceStatic},
	"deprecatedTestPattern":              {Source: SignalSourceStatic},
	"dynamicTestGeneration":              {Source: SignalSourceStatic},
	"customMatcherRisk":                  {Source: SignalSourceStatic},
	"unsupportedSetup":                   {Source: SignalSourceStatic},
	"policyViolation":                    {Source: SignalSourceStatic},
	"legacyFrameworkUsage":               {Source: SignalSourceStatic},
	"skippedTestsInCI":                   {Source: SignalSourceStatic},
	"runtimeBudgetExceeded":              {Source: SignalSourceStatic},
	"staticSkippedTest":                  {Source: SignalSourceStatic},
	"staticSkippedTest-unconditional":    {Source: SignalSourceStatic},
	"staticSkippedTest-conditional-gate": {Source: SignalSourceStatic},
	"assertionFreeTest":                  {Source: SignalSourceStatic},
	"orphanedTestFile":                   {Source: SignalSourceStatic},
	"depsDriftRisk":                      {Source: SignalSourceStatic},
	"depsDriftRisk-strict-pin":           {Source: SignalSourceStatic},
	"depsDriftRisk-caret-policy":         {Source: SignalSourceStatic},
	"aiHardcodedAPIKey-literal-shape":    {Source: SignalSourceStatic},
	"secretScannerCoverageDegraded":      {Source: SignalSourceStatic},
	"configSchemaDrift":                  {Source: SignalSourceStatic},
	"promptFileMissingEval":              {Source: SignalSourceGraph},

	// Runtime health signals (Tier 2) — require runtime artifacts.
	"slowTest":      {Source: SignalSourceRuntime},
	"flakyTest":     {Source: SignalSourceRuntime},
	"skippedTest":   {Source: SignalSourceRuntime},
	"deadTest":      {Source: SignalSourceRuntime},
	"unstableSuite": {Source: SignalSourceRuntime},

	// Graph-powered structural signals (Tier 3) — produced by dependency graph traversal.
	"uncoveredAISurface":      {Source: SignalSourceGraph},
	"phantomEvalScenario":     {Source: SignalSourceGraph},
	"untestedPromptFlow":      {Source: SignalSourceGraph},
	"blastRadiusHotspot":      {Source: SignalSourceGraph},
	"fixtureFragilityHotspot": {Source: SignalSourceGraph},
	"assertionFreeImport":     {Source: SignalSourceGraph},
	"capabilityValidationGap": {Source: SignalSourceGraph},

	// AI/eval signals (Tier 4) — produced by Gauntlet artifact ingestion.
	"evalFailure":            {Source: SignalSourceGauntlet},
	"evalRegression":         {Source: SignalSourceGauntlet},
	"accuracyRegression":     {Source: SignalSourceGauntlet},
	"citationMissing":        {Source: SignalSourceGauntlet},
	"retrievalMiss":          {Source: SignalSourceGauntlet},
	"answerGroundingFailure": {Source: SignalSourceGauntlet},
	"toolSelectionError":     {Source: SignalSourceGauntlet},
	"schemaParseFailure":     {Source: SignalSourceGauntlet},
	"safetyFailure":          {Source: SignalSourceGauntlet},
	"aiPolicyViolation":      {Source: SignalSourceGauntlet},
	"hallucinationDetected":  {Source: SignalSourceGauntlet},
	"latencyRegression":      {Source: SignalSourceGauntlet},
	"costRegression":         {Source: SignalSourceGauntlet},
	"contextOverflowRisk":    {Source: SignalSourceGauntlet},
	"wrongSourceSelected":    {Source: SignalSourceGauntlet},
	"citationMismatch":       {Source: SignalSourceGauntlet},
	"staleSourceRisk":        {Source: SignalSourceGauntlet},
	"chunkingRegression":     {Source: SignalSourceGauntlet},
	"rerankerRegression":     {Source: SignalSourceGauntlet},
	"topKRegression":         {Source: SignalSourceGauntlet},
	"toolRoutingError":       {Source: SignalSourceGauntlet},
	"toolGuardrailViolation": {Source: SignalSourceGauntlet},
	"toolBudgetExceeded":     {Source: SignalSourceGauntlet},
	"agentFallbackTriggered": {Source: SignalSourceGauntlet},

	// AI signals whose Source is declared here so the catalog and the
	// signal manifest stay in sync.
	"aiSafetyEvalMissing":    {Source: SignalSourceStatic},
	"aiPromptVersioning":     {Source: SignalSourceStatic},
	"aiPromptInjectionRisk":  {Source: SignalSourceStatic},
	"aiHardcodedAPIKey":      {Source: SignalSourceStatic},
	"aiToolWithoutSandbox":   {Source: SignalSourceStatic},
	"aiNonDeterministicEval": {Source: SignalSourceStatic},
	"aiModelDeprecationRisk": {Source: SignalSourceStatic},
	"aiCostRegression":       {Source: SignalSourceGauntlet},
	"aiHallucinationRate":    {Source: SignalSourceGauntlet},
	"aiFewShotContamination": {Source: SignalSourceStatic},
	"aiEmbeddingModelChange": {Source: SignalSourceStatic},
	"aiRetrievalRegression":  {Source: SignalSourceGauntlet},
	"aiPromptSchemaDrift":    {Source: SignalSourceStatic},

	// Engine self-diagnostic signals — emitted by the pipeline itself
	// (not by detectors), surfaced in the snapshot so users see when
	// something internal failed mid-run instead of a half-empty result.
	// detectorPanic is emitted by safeDetect when a registered detector
	// panics; without it in the catalog, ValidateSnapshot would reject
	// the entire snapshot the moment any detector panicked, defeating
	// the panic-recovery path.
	"detectorPanic": {Source: SignalSourceStatic},
	// detectorBudgetExceeded is emitted by safeDetectWithBudget when
	// a registered detector exceeds its DetectorMeta.Budget (default
	// DefaultDetectorBudget). Same posture as detectorPanic — without
	// it in the catalog, ValidateSnapshot would reject the entire
	// snapshot whenever a detector hit its budget, defeating the
	// timeout enforcement.
	"detectorBudgetExceeded": {Source: SignalSourceStatic},
	// detectorMissingInput is emitted by safeDetectChecked when a
	// detector's RequiresRuntime / RequiresBaseline /
	// RequiresEvalArtifact metadata is set but the snapshot lacks
	// the corresponding input — surfaces input gaps as a single
	// per-detector marker instead of silent zero-output.
	"detectorMissingInput": {Source: SignalSourceStatic},

	// suppressionExpired is emitted by the suppression-loading pass
	// when a `.terrain/suppressions.yaml` entry has passed its
	// `expires` date. The user-facing finding it covered fires again,
	// AND this signal surfaces so silent rot doesn't accumulate.
	"suppressionExpired": {Source: SignalSourceStatic},

	// Stable-rule signals — sourced from static analysis of
	// manifests, AI surfaces, code units, and graph state.
	"versionFloating":         {Source: SignalSourceStatic},
	"secretsInPrompt":         {Source: SignalSourceStatic},
	"noTestsForCodeUnit":      {Source: SignalSourceGraph},
	"noEvalForAISurface":      {Source: SignalSourceGraph},
	"modelFixtureUnpinned":    {Source: SignalSourceStatic},
	"evalNoAssertion":         {Source: SignalSourceStatic},
	"noSeed":                  {Source: SignalSourceStatic},
	"missingEnvPinning":       {Source: SignalSourceStatic},
	"piiInEval":               {Source: SignalSourceStatic},
	"insecureDeserialization": {Source: SignalSourceStatic},
	"missingPerfTest":         {Source: SignalSourceGraph},
	"dataLeakageSuspected":    {Source: SignalSourceStatic},
	"missingTrainTestSplit":   {Source: SignalSourceStatic},

	// Regression family signals — produced by eval-adapter ingestion.
	"baselineNotSet":        {Source: SignalSourceGauntlet},
	"passRateDrop":          {Source: SignalSourceGauntlet},
	"snapshotMismatch":      {Source: SignalSourceGauntlet},
	"testFailed":            {Source: SignalSourceRuntime},
	"performanceRegression": {Source: SignalSourceGauntlet},

	// Coverage family signals.
	"missingBaseline":   {Source: SignalSourceStatic},
	"noIntegrationTest": {Source: SignalSourceGraph},
	"noDataValidation":  {Source: SignalSourceStatic},

	// Preview-tier rules.
	"promptBloat":              {Source: SignalSourceStatic},
	"promptWithoutTemperature": {Source: SignalSourceStatic},
	"missingPromptValidator":   {Source: SignalSourceStatic},
	"promptVersionSkew":        {Source: SignalSourceGraph},
	"retrievalWithoutRerank":   {Source: SignalSourceStatic},
	"coldVectorStore":          {Source: SignalSourceStatic},
	"agentLoopRisk":            {Source: SignalSourceStatic},
	"toolWithoutBudget":        {Source: SignalSourceStatic},
	"targetLeakage":            {Source: SignalSourceStatic},
	"duplicateEvalRows":        {Source: SignalSourceStatic},
	"schemaDrift":              {Source: SignalSourceStatic},
	"missingEvalCategories":    {Source: SignalSourceStatic},
	"orphanedEval":             {Source: SignalSourceGraph},
	"coldStartTime":            {Source: SignalSourceRuntime},
	"tokenCostBudget":          {Source: SignalSourceRuntime},
}

// KnownSignalTypes is the canonical signal vocabulary accepted by snapshot
// validation. Derived from SignalCatalog.
var KnownSignalTypes = func() map[SignalType]bool {
	m := make(map[SignalType]bool, len(SignalCatalog))
	for k := range SignalCatalog {
		m[k] = true
	}
	return m
}()

// IsKnownSignalType reports whether t is part of Terrain's canonical catalog.
func IsKnownSignalType(t SignalType) bool {
	return KnownSignalTypes[t]
}
