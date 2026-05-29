package aidetect

import (
	"os"
	"path/filepath"

	"github.com/pmclSF/terrain/internal/models"
	"github.com/pmclSF/terrain/internal/preview"
)

// preview_adapters.go wires the preview-tier detectors in
// internal/preview into the Detector interface that the engine
// registry consumes. The wrapper here owns the filesystem dance
// (walk + slurp); the preview package owns the rule semantics.

// previewSourceExts are the source extensions the preview detectors
// inspect for code-shape rules (prompt-without-temperature,
// agent-loop-risk, etc.). Pinned narrow to keep adapter walks cheap.
var previewSourceExts = map[string]bool{
	".py": true,
}

// previewPromptExts are the file extensions whose contents we treat
// as candidate prompt-text files for promptBloat / promptVersionSkew.
var previewPromptExts = map[string]bool{
	".md":     true,
	".txt":    true,
	".prompt": true,
	".tmpl":   true,
	".jinja":  true,
	".j2":     true,
}

// previewEvalDataExts are the extensions whose contents we treat as
// row-shaped eval datasets for duplicateEvalRows.
var previewEvalDataExts = map[string]bool{
	".csv":   true,
	".jsonl": true,
	".tsv":   true,
}

// ----- adapters: 15 preview detectors -----

type OrphanedEvalDetector struct{}

func (d *OrphanedEvalDetector) Detect(snap *models.TestSuiteSnapshot) []models.Signal {
	if snap == nil {
		return nil
	}
	return preview.DetectOrphanedEval(snap.Evals)
}

type MissingEvalCategoriesDetector struct{}

func (d *MissingEvalCategoriesDetector) Detect(snap *models.TestSuiteSnapshot) []models.Signal {
	if snap == nil {
		return nil
	}
	return preview.DetectMissingEvalCategories(snap.Evals)
}

type PromptBloatDetector struct {
	Root      string
	Threshold int
}

func (d *PromptBloatDetector) Detect(snap *models.TestSuiteSnapshot) []models.Signal {
	if d.Root == "" {
		return nil
	}
	paths := d.gatherPromptPaths(snap)
	return preview.DetectPromptBloat(paths, d.Threshold)
}

func (d *PromptBloatDetector) gatherPromptPaths(snap *models.TestSuiteSnapshot) []string {
	rels := walkRepoForConfigs(d.Root, scanOpts{extensions: previewPromptExts, markers: []string{"prompt", "templates/"}})
	abs := make([]string, 0, len(rels))
	for _, r := range rels {
		abs = append(abs, filepath.Join(d.Root, r))
	}
	return abs
}

type PromptWithoutTemperatureDetector struct {
	Root string
}

func (d *PromptWithoutTemperatureDetector) Detect(snap *models.TestSuiteSnapshot) []models.Signal {
	if d.Root == "" {
		return nil
	}
	ctx := DetectContext(nil, d.Root)
	if ctx == nil || len(ctx.CallSites) == 0 {
		return nil
	}
	mapping := map[string]string{}
	calls := make([]preview.CallSite, 0, len(ctx.CallSites))
	for _, cs := range ctx.CallSites {
		mapping[cs.Path] = filepath.Join(d.Root, cs.Path)
		calls = append(calls, preview.CallSite{Path: cs.Path, Line: cs.Line, SDK: cs.SDK, Method: cs.Method})
	}
	return preview.DetectPromptWithoutTemperature(calls, mapping)
}

type MissingPromptValidatorDetector struct {
	Root string
}

func (d *MissingPromptValidatorDetector) Detect(snap *models.TestSuiteSnapshot) []models.Signal {
	return preview.DetectMissingPromptValidator(d.slurpSourceFiles())
}

func (d *MissingPromptValidatorDetector) slurpSourceFiles() map[string][]byte {
	return slurpSourceFiles(d.Root, previewSourceExts)
}

type PromptVersionSkewDetector struct {
	Root string
}

func (d *PromptVersionSkewDetector) Detect(snap *models.TestSuiteSnapshot) []models.Signal {
	if d.Root == "" {
		return nil
	}
	rels := walkRepoForConfigs(d.Root, scanOpts{extensions: previewPromptExts, markers: []string{"prompt", "templates/"}})
	abs := make([]string, 0, len(rels))
	for _, r := range rels {
		abs = append(abs, filepath.Join(d.Root, r))
	}
	return preview.DetectPromptVersionSkew(abs)
}

type RetrievalWithoutRerankDetector struct {
	Root string
}

func (d *RetrievalWithoutRerankDetector) Detect(snap *models.TestSuiteSnapshot) []models.Signal {
	return preview.DetectRetrievalWithoutRerank(slurpSourceFiles(d.Root, previewSourceExts))
}

type ColdVectorStoreDetector struct {
	Root string
}

func (d *ColdVectorStoreDetector) Detect(snap *models.TestSuiteSnapshot) []models.Signal {
	return preview.DetectColdVectorStore(slurpSourceFiles(d.Root, previewSourceExts))
}

type AgentLoopRiskDetector struct {
	Root string
}

func (d *AgentLoopRiskDetector) Detect(snap *models.TestSuiteSnapshot) []models.Signal {
	return preview.DetectAgentLoopRisk(slurpSourceFiles(d.Root, previewSourceExts))
}

type ToolWithoutBudgetDetector struct {
	Root string
}

func (d *ToolWithoutBudgetDetector) Detect(snap *models.TestSuiteSnapshot) []models.Signal {
	return preview.DetectToolWithoutBudget(slurpSourceFiles(d.Root, previewSourceExts))
}

type TargetLeakageDetector struct {
	Root string
}

func (d *TargetLeakageDetector) Detect(snap *models.TestSuiteSnapshot) []models.Signal {
	return preview.DetectTargetLeakage(slurpSourceFiles(d.Root, previewSourceExts))
}

type DuplicateEvalRowsDetector struct {
	Root      string
	Threshold float64
}

func (d *DuplicateEvalRowsDetector) Detect(snap *models.TestSuiteSnapshot) []models.Signal {
	if d.Root == "" {
		return nil
	}
	rels := walkRepoForConfigs(d.Root, scanOpts{extensions: previewEvalDataExts, markers: []string{"eval", "dataset", "evals/"}})
	abs := make([]string, 0, len(rels))
	for _, r := range rels {
		abs = append(abs, filepath.Join(d.Root, r))
	}
	return preview.DetectDuplicateEvalRows(abs, d.Threshold)
}

// SchemaDriftDetector wires the structural pre-computed-column variant
// of schemaDrift. Until the analyze pipeline computes column sets, this
// adapter has no inputs and returns no signals — it's pre-registered
// so the manifest entry is reachable and a future revision can light
// it up without a registry change.
type SchemaDriftDetector struct{}

func (d *SchemaDriftDetector) Detect(snap *models.TestSuiteSnapshot) []models.Signal {
	return nil
}

// ColdStartTimeDetector consumes pre-computed latency observations.
// Same lifecycle as SchemaDrift — registered, dormant until runtime
// telemetry inputs land.
type ColdStartTimeDetector struct{}

func (d *ColdStartTimeDetector) Detect(snap *models.TestSuiteSnapshot) []models.Signal {
	return nil
}

// TokenCostBudgetDetector consumes pre-computed cost observations.
// Same lifecycle — registered, dormant until eval-run cost telemetry
// is wired through.
type TokenCostBudgetDetector struct{}

func (d *TokenCostBudgetDetector) Detect(snap *models.TestSuiteSnapshot) []models.Signal {
	return nil
}

// ----- shared helpers -----

func slurpSourceFiles(root string, exts map[string]bool) map[string][]byte {
	if root == "" {
		return nil
	}
	rels := walkRepoForConfigs(root, scanOpts{extensions: exts})
	if len(rels) == 0 {
		return nil
	}
	out := make(map[string][]byte, len(rels))
	for _, r := range rels {
		data, err := os.ReadFile(filepath.Join(root, r))
		if err != nil {
			continue
		}
		// Cap per-file bytes so a stray giant fixture doesn't wedge the walk.
		if len(data) > 1<<20 {
			data = data[:1<<20]
		}
		out[r] = data
	}
	return out
}
