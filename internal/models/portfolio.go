package models

// PortfolioSnapshot captures portfolio intelligence results for a point in time.
// This is the serializable model stored in TestSuiteSnapshot.Portfolio.
type PortfolioSnapshot struct {
	// Scope identifies whether this portfolio was computed for one
	// repository or aggregated from a multi-repo manifest.
	Scope string `json:"scope,omitempty"`

	// Description carries the manifest description for multi-repo output.
	Description string `json:"description,omitempty"`

	// Repositories contains per-repo rollups for manifest-backed
	// multi-repo portfolio output. Omitted for single-repo reports.
	Repositories []PortfolioRepositorySummary `json:"repositories,omitempty"`

	// Assets is the list of test assets with cost and coverage metadata.
	Assets []PortfolioAsset `json:"assets,omitempty"`

	// Findings is the list of portfolio findings.
	Findings []PortfolioFinding `json:"findings,omitempty"`

	// Aggregates contains summary statistics.
	Aggregates PortfolioAggregates `json:"aggregates"`
}

// PortfolioAsset is the serializable representation of a test asset.
type PortfolioAsset struct {
	Repo                 string   `json:"repo,omitempty"`
	Path                 string   `json:"path"`
	Framework            string   `json:"framework,omitempty"`
	TestType             string   `json:"testType,omitempty"`
	Owner                string   `json:"owner,omitempty"`
	Tags                 []string `json:"tags,omitempty"`
	TestCount            int      `json:"testCount,omitempty"`
	RuntimeMs            float64  `json:"runtimeMs,omitempty"`
	RetryRate            float64  `json:"retryRate,omitempty"`
	PassRate             float64  `json:"passRate,omitempty"`
	CostClass            string   `json:"costClass"`
	InstabilitySignals   int      `json:"instabilitySignals,omitempty"`
	CoveredUnitCount     int      `json:"coveredUnitCount"`
	CoveredModules       []string `json:"coveredModules,omitempty"`
	ExportedUnitsCovered int      `json:"exportedUnitsCovered,omitempty"`
	OwnersCovered        []string `json:"ownersCovered,omitempty"`
	BreadthClass         string   `json:"breadthClass"`
	HasRuntimeData       bool     `json:"hasRuntimeData"`
	HasCoverageData      bool     `json:"hasCoverageData"`
}

// PortfolioFinding is the serializable representation of a portfolio finding.
type PortfolioFinding struct {
	Type            string         `json:"type"`
	Repo            string         `json:"repo,omitempty"`
	Path            string         `json:"path"`
	RelatedPaths    []string       `json:"relatedPaths,omitempty"`
	Owner           string         `json:"owner,omitempty"`
	Confidence      string         `json:"confidence"`
	Explanation     string         `json:"explanation"`
	SuggestedAction string         `json:"suggestedAction,omitempty"`
	Metadata        map[string]any `json:"metadata,omitempty"`
}

// PortfolioAggregates contains summary statistics for the portfolio.
type PortfolioAggregates struct {
	TotalRepos int `json:"totalRepos,omitempty"`

	TotalAssets              int     `json:"totalAssets"`
	TotalRuntimeMs           float64 `json:"totalRuntimeMs,omitempty"`
	RuntimeConcentration     float64 `json:"runtimeConcentration,omitempty"`
	HasRuntimeData           bool    `json:"hasRuntimeData"`
	HasCoverageData          bool    `json:"hasCoverageData"`
	RedundancyCandidateCount int     `json:"redundancyCandidateCount"`
	OverbroadCount           int     `json:"overbroadCount"`
	LowValueHighCostCount    int     `json:"lowValueHighCostCount"`
	HighLeverageCount        int     `json:"highLeverageCount"`
	FrameworkDriftCount      int     `json:"frameworkDriftCount,omitempty"`
	PortfolioPostureBand     string  `json:"portfolioPostureBand,omitempty"`

	// ByOwner contains per-owner aggregations.
	ByOwner []PortfolioOwnerSummary `json:"byOwner,omitempty"`
}

// PortfolioRepositorySummary is a per-repository rollup for multi-repo output.
type PortfolioRepositorySummary struct {
	Name               string                    `json:"name"`
	Path               string                    `json:"path,omitempty"`
	SnapshotPath       string                    `json:"snapshotPath,omitempty"`
	Owner              string                    `json:"owner,omitempty"`
	Tags               []string                  `json:"tags,omitempty"`
	FrameworksOfRecord []string                  `json:"frameworksOfRecord,omitempty"`
	ObservedFrameworks []PortfolioFrameworkCount `json:"observedFrameworks,omitempty"`
	DriftFrameworks    []PortfolioFrameworkCount `json:"driftFrameworks,omitempty"`
	Status             string                    `json:"status"`
	AssetCount         int                       `json:"assetCount"`
	FindingCount       int                       `json:"findingCount"`
	TotalRuntimeMs     float64                   `json:"totalRuntimeMs,omitempty"`
	HasRuntimeData     bool                      `json:"hasRuntimeData"`
	HasCoverageData    bool                      `json:"hasCoverageData"`
	PostureBand        string                    `json:"postureBand,omitempty"`
}

// PortfolioFrameworkCount records how many test files use a framework.
type PortfolioFrameworkCount struct {
	Name      string `json:"name"`
	TestFiles int    `json:"testFiles"`
}

// PortfolioOwnerSummary aggregates portfolio findings per owner.
type PortfolioOwnerSummary struct {
	Owner                    string  `json:"owner"`
	AssetCount               int     `json:"assetCount"`
	TotalRuntimeMs           float64 `json:"totalRuntimeMs,omitempty"`
	RedundancyCandidateCount int     `json:"redundancyCandidateCount,omitempty"`
	OverbroadCount           int     `json:"overbroadCount,omitempty"`
	LowValueHighCostCount    int     `json:"lowValueHighCostCount,omitempty"`
	HighLeverageCount        int     `json:"highLeverageCount,omitempty"`
	FrameworkDriftCount      int     `json:"frameworkDriftCount,omitempty"`
}
