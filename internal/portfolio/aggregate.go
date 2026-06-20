package portfolio

import (
	"errors"
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/pmclSF/terrain/internal/models"
)

const (
	// ScopeMultiRepo identifies manifest-backed aggregate portfolio output.
	ScopeMultiRepo = "multi_repo"

	// FindingFrameworkDrift is emitted only for manifest-backed multi-repo
	// portfolio output when observed test frameworks differ from a repo's
	// declared frameworksOfRecord.
	FindingFrameworkDrift = "framework_drift"
)

// RepoPortfolioInput is one analyzed repository ready for multi-repo
// aggregation.
type RepoPortfolioInput struct {
	Entry                RepoEntry
	Snapshot             *models.TestSuiteSnapshot
	ResolvedPath         string
	ResolvedSnapshotPath string
	LoadedFromSnapshot   bool
}

// AggregateManifest combines per-repo portfolio snapshots into one
// manifest-backed portfolio. It preserves per-file assets while prefixing
// paths with the repo name so downstream consumers can distinguish
// identical file names across repositories.
func AggregateManifest(manifest *RepoManifest, inputs []RepoPortfolioInput) (*models.PortfolioSnapshot, error) {
	if manifest == nil {
		return nil, errors.New("portfolio manifest is nil")
	}
	if err := validateRepoManifest(manifest); err != nil {
		return nil, fmt.Errorf("validate repo manifest: %w", err)
	}
	if len(inputs) != len(manifest.Repos) {
		return nil, fmt.Errorf("portfolio input count %d does not match manifest repo count %d", len(inputs), len(manifest.Repos))
	}

	out := &models.PortfolioSnapshot{
		Scope:        ScopeMultiRepo,
		Description:  manifest.Description,
		Repositories: make([]models.PortfolioRepositorySummary, 0, len(inputs)),
	}

	for i, input := range inputs {
		entry := manifest.Repos[i]
		if strings.TrimSpace(input.Entry.Name) != entry.Name {
			return nil, fmt.Errorf("repo input #%d is for %q, expected %q from manifest order", i+1, input.Entry.Name, entry.Name)
		}
		if strings.TrimSpace(entry.Name) == "" {
			return nil, fmt.Errorf("repo #%d: name is required", i+1)
		}
		if input.Snapshot == nil {
			return nil, fmt.Errorf("repo %q: snapshot is nil", entry.Name)
		}

		repoPortfolio := input.Snapshot.Portfolio
		if repoPortfolio == nil {
			repoPortfolio = Analyze(input.Snapshot).ToModel()
		}
		if repoPortfolio == nil {
			repoPortfolio = &models.PortfolioSnapshot{}
		}

		repoSummary, driftFinding := buildRepositorySummary(input, repoPortfolio)
		out.Repositories = append(out.Repositories, repoSummary)

		for _, asset := range repoPortfolio.Assets {
			asset.Repo = entry.Name
			asset.Path = repoPath(entry.Name, asset.Path)
			if isUnknownOwner(asset.Owner) {
				asset.Owner = entry.Owner
			}
			asset.Tags = mergeTags(asset.Tags, entry.Tags)
			out.Assets = append(out.Assets, asset)
		}

		for _, finding := range repoPortfolio.Findings {
			finding.Repo = entry.Name
			finding.Path = repoPath(entry.Name, finding.Path)
			for j, related := range finding.RelatedPaths {
				finding.RelatedPaths[j] = repoPath(entry.Name, related)
			}
			if isUnknownOwner(finding.Owner) {
				finding.Owner = entry.Owner
			}
			finding.Metadata = addManifestMetadata(finding.Metadata, entry)
			out.Findings = append(out.Findings, finding)
		}
		if driftFinding != nil {
			out.Findings = append(out.Findings, *driftFinding)
		}
	}

	sort.Slice(out.Assets, func(i, j int) bool {
		return out.Assets[i].Path < out.Assets[j].Path
	})
	sort.Slice(out.Findings, func(i, j int) bool {
		if out.Findings[i].Type != out.Findings[j].Type {
			return out.Findings[i].Type < out.Findings[j].Type
		}
		if out.Findings[i].Repo != out.Findings[j].Repo {
			return out.Findings[i].Repo < out.Findings[j].Repo
		}
		return out.Findings[i].Path < out.Findings[j].Path
	})

	out.Aggregates = aggregatePortfolioModel(out)
	return out, nil
}

func buildRepositorySummary(input RepoPortfolioInput, p *models.PortfolioSnapshot) (models.PortfolioRepositorySummary, *models.PortfolioFinding) {
	entry := input.Entry
	observed := observedFrameworkCounts(input.Snapshot)
	drift := driftFrameworks(observed, entry.FrameworksOfRecord)
	status := repoStatus(entry, drift)
	assetCount := p.Aggregates.TotalAssets
	if assetCount == 0 && len(p.Assets) > 0 {
		assetCount = len(p.Assets)
	}

	findingCount := len(p.Findings)
	var driftFinding *models.PortfolioFinding
	if len(drift) > 0 {
		findingCount++
		driftFinding = frameworkDriftFinding(entry, observed, drift)
	}

	return models.PortfolioRepositorySummary{
		Name:               entry.Name,
		Path:               input.ResolvedPath,
		SnapshotPath:       input.ResolvedSnapshotPath,
		Owner:              entry.Owner,
		Tags:               append([]string(nil), entry.Tags...),
		FrameworksOfRecord: append([]string(nil), entry.FrameworksOfRecord...),
		ObservedFrameworks: observed,
		DriftFrameworks:    drift,
		Status:             status,
		AssetCount:         assetCount,
		FindingCount:       findingCount,
		TotalRuntimeMs:     p.Aggregates.TotalRuntimeMs,
		HasRuntimeData:     p.Aggregates.HasRuntimeData,
		HasCoverageData:    p.Aggregates.HasCoverageData,
		PostureBand:        p.Aggregates.PortfolioPostureBand,
	}, driftFinding
}

func frameworkDriftFinding(entry RepoEntry, observed, drift []models.PortfolioFrameworkCount) *models.PortfolioFinding {
	driftNames := make([]string, 0, len(drift))
	for _, fw := range drift {
		driftNames = append(driftNames, fmt.Sprintf("%s (%d files)", fw.Name, fw.TestFiles))
	}
	return &models.PortfolioFinding{
		Type:       FindingFrameworkDrift,
		Repo:       entry.Name,
		Path:       entry.Name,
		Owner:      entry.Owner,
		Confidence: string(ConfidenceHigh),
		Explanation: fmt.Sprintf(
			"%s uses test frameworks outside its frameworksOfRecord: %s.",
			entry.Name,
			strings.Join(driftNames, ", "),
		),
		SuggestedAction: "Plan convergence to the declared frameworksOfRecord, or update the manifest if this drift is intentional.",
		Metadata: map[string]any{
			"frameworksOfRecord": entry.FrameworksOfRecord,
			"observedFrameworks": observed,
			"driftFrameworks":    drift,
			"tags":               entry.Tags,
		},
	}
}

func observedFrameworkCounts(snap *models.TestSuiteSnapshot) []models.PortfolioFrameworkCount {
	if snap == nil {
		return nil
	}
	counts := map[string]int{}
	for _, tf := range snap.TestFiles {
		name := normalizeFrameworkName(tf.Framework)
		if name == "" {
			continue
		}
		counts[name]++
	}
	if len(counts) == 0 {
		for _, fw := range snap.Frameworks {
			name := normalizeFrameworkName(fw.Name)
			if name != "" {
				counts[name] = 0
			}
		}
	}
	return sortedFrameworkCounts(counts)
}

func driftFrameworks(observed []models.PortfolioFrameworkCount, frameworksOfRecord []string) []models.PortfolioFrameworkCount {
	if len(frameworksOfRecord) == 0 {
		return nil
	}
	record := map[string]bool{}
	for _, fw := range frameworksOfRecord {
		name := normalizeFrameworkName(fw)
		if name != "" {
			record[name] = true
		}
	}
	var drift []models.PortfolioFrameworkCount
	for _, fw := range observed {
		if fw.Name == "" {
			continue
		}
		if !record[fw.Name] {
			drift = append(drift, fw)
		}
	}
	return drift
}

func sortedFrameworkCounts(counts map[string]int) []models.PortfolioFrameworkCount {
	out := make([]models.PortfolioFrameworkCount, 0, len(counts))
	for name, count := range counts {
		out = append(out, models.PortfolioFrameworkCount{Name: name, TestFiles: count})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Name < out[j].Name
	})
	return out
}

func normalizeFrameworkName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func repoStatus(entry RepoEntry, drift []models.PortfolioFrameworkCount) string {
	if len(entry.FrameworksOfRecord) == 0 {
		return "unconfigured"
	}
	if len(drift) == 0 {
		return "of_record"
	}
	return "drift"
}

func aggregatePortfolioModel(p *models.PortfolioSnapshot) models.PortfolioAggregates {
	if p == nil {
		return models.PortfolioAggregates{}
	}
	agg := models.PortfolioAggregates{
		TotalRepos:      len(p.Repositories),
		TotalAssets:     len(p.Assets),
		ByOwner:         aggregateOwners(p.Assets, p.Findings),
		HasRuntimeData:  false,
		HasCoverageData: false,
	}

	runtimes := make([]float64, 0, len(p.Assets))
	for _, asset := range p.Assets {
		if asset.HasRuntimeData {
			agg.HasRuntimeData = true
			agg.TotalRuntimeMs += asset.RuntimeMs
			runtimes = append(runtimes, asset.RuntimeMs)
		}
		if asset.HasCoverageData {
			agg.HasCoverageData = true
		}
	}
	if len(runtimes) >= 5 && agg.TotalRuntimeMs > 0 {
		sort.Float64s(runtimes)
		topCount := (len(runtimes) + 4) / 5
		var topRuntime float64
		for i := len(runtimes) - topCount; i < len(runtimes); i++ {
			topRuntime += runtimes[i]
		}
		agg.RuntimeConcentration = topRuntime / agg.TotalRuntimeMs
	}

	for _, finding := range p.Findings {
		switch finding.Type {
		case FindingRedundancyCandidate:
			agg.RedundancyCandidateCount++
		case FindingOverbroad:
			agg.OverbroadCount++
		case FindingLowValueHighCost:
			agg.LowValueHighCostCount++
		case FindingHighLeverage:
			agg.HighLeverageCount++
		case FindingFrameworkDrift:
			agg.FrameworkDriftCount++
		}
	}
	agg.PortfolioPostureBand = portfolioModelPosture(agg)
	return agg
}

func aggregateOwners(assets []models.PortfolioAsset, findings []models.PortfolioFinding) []models.PortfolioOwnerSummary {
	type ownerData struct {
		assetCount int
		runtimeMs  float64
		redundancy int
		overbroad  int
		lowValue   int
		highLev    int
		drift      int
	}
	byOwner := map[string]*ownerData{}
	for _, asset := range assets {
		owner := asset.Owner
		if owner == "" {
			owner = "unowned"
		}
		data := byOwner[owner]
		if data == nil {
			data = &ownerData{}
			byOwner[owner] = data
		}
		data.assetCount++
		data.runtimeMs += asset.RuntimeMs
	}
	for _, finding := range findings {
		owner := finding.Owner
		if owner == "" {
			owner = "unowned"
		}
		data := byOwner[owner]
		if data == nil {
			data = &ownerData{}
			byOwner[owner] = data
		}
		switch finding.Type {
		case FindingRedundancyCandidate:
			data.redundancy++
		case FindingOverbroad:
			data.overbroad++
		case FindingLowValueHighCost:
			data.lowValue++
		case FindingHighLeverage:
			data.highLev++
		case FindingFrameworkDrift:
			data.drift++
		}
	}
	out := make([]models.PortfolioOwnerSummary, 0, len(byOwner))
	for owner, data := range byOwner {
		out = append(out, models.PortfolioOwnerSummary{
			Owner:                    owner,
			AssetCount:               data.assetCount,
			TotalRuntimeMs:           data.runtimeMs,
			RedundancyCandidateCount: data.redundancy,
			OverbroadCount:           data.overbroad,
			LowValueHighCostCount:    data.lowValue,
			HighLeverageCount:        data.highLev,
			FrameworkDriftCount:      data.drift,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Owner < out[j].Owner
	})
	return out
}

func portfolioModelPosture(agg models.PortfolioAggregates) string {
	if agg.TotalAssets == 0 {
		if agg.FrameworkDriftCount > 0 {
			return "weak"
		}
		return "unknown"
	}
	problems := agg.RedundancyCandidateCount + agg.OverbroadCount +
		agg.LowValueHighCostCount + agg.FrameworkDriftCount
	ratio := float64(problems) / float64(agg.TotalAssets)
	switch {
	case ratio <= 0.05:
		return "strong"
	case ratio <= 0.15:
		return "moderate"
	case ratio <= 0.30:
		return "weak"
	default:
		return "critical"
	}
}

func repoPath(repoName, rel string) string {
	rel = strings.TrimSpace(rel)
	if rel == "" {
		return repoName
	}
	rel = strings.ReplaceAll(rel, "\\", "/")
	rel = strings.TrimLeft(rel, "/")
	var parts []string
	for _, part := range strings.Split(rel, "/") {
		part = strings.TrimSpace(part)
		switch part {
		case "", ".", "..":
			continue
		default:
			parts = append(parts, part)
		}
	}
	if len(parts) == 0 {
		return repoName
	}
	return path.Join(append([]string{repoName}, parts...)...)
}

func mergeTags(assetTags, repoTags []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, tag := range append(assetTags, repoTags...) {
		tag = strings.TrimSpace(tag)
		if tag == "" || seen[tag] {
			continue
		}
		seen[tag] = true
		out = append(out, tag)
	}
	return out
}

func addManifestMetadata(metadata map[string]any, entry RepoEntry) map[string]any {
	if len(entry.Tags) == 0 && len(entry.FrameworksOfRecord) == 0 {
		return metadata
	}
	out := map[string]any{}
	for k, v := range metadata {
		out[k] = v
	}
	if len(entry.Tags) > 0 {
		out["tags"] = entry.Tags
	}
	if len(entry.FrameworksOfRecord) > 0 {
		out["frameworksOfRecord"] = entry.FrameworksOfRecord
	}
	return out
}

func isUnknownOwner(owner string) bool {
	owner = strings.TrimSpace(strings.ToLower(owner))
	return owner == "" || owner == "unknown" || owner == "unowned"
}
