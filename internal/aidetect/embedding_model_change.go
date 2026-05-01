package aidetect

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/pmclSF/terrain/internal/models"
	"github.com/pmclSF/terrain/internal/signals"
)

// EmbeddingModelChangeDetector flags repos that reference an embedding
// model in source code without a retrieval-shaped eval scenario to
// catch regressions when the model swaps. The round-4 plan named the
// signal "embedding model change without RAG re-evaluation"; the
// 0.2 detector ships the static precondition (embedding referenced
// at all + no retrieval coverage) so the warning fires before a
// silent swap. The literal cross-snapshot diff variant lands once
// content hashes are on the snapshot.
//
// Detection model:
//
//   1. Walk source files (Python / JS / TS / Go / Java / Ruby / Rust)
//      whose path is referenced as a CodeSurface OR appears in the
//      configured surface universe.
//   2. Look for a known embedding-model identifier (regex list).
//   3. If at least one match in the repo AND the snapshot has no
//      retrieval-shaped scenario coverage, emit one signal per file.
//
// Retrieval-shaped scenario means: scenario.Category contains
// "retriev" / "rag" / "embedding" / "vector" / "knn", OR the scenario
// covers a Surface with Kind in {retrieval, agent}.
type EmbeddingModelChangeDetector struct {
	// Root is the absolute path of the repo. Snapshot paths are
	// repo-relative.
	Root string
}

// embeddingModelPatterns matches the most common embedding model
// identifiers across providers. Conservative — we'd rather miss a
// niche provider than fire on a random string. Calibration corpus
// expansions in 0.3 broaden the list.
var embeddingModelPatterns = []*regexp.Regexp{
	// OpenAI.
	regexp.MustCompile(`\btext-embedding-(?:ada-002|3-small|3-large)\b`),
	// Voyage AI.
	regexp.MustCompile(`\bvoyage-(?:large-2|code-2|2)\b`),
	// Cohere.
	regexp.MustCompile(`\bembed-english-(?:v2\.0|v3\.0|light-v3\.0)\b`),
	regexp.MustCompile(`\bembed-multilingual-(?:v2\.0|v3\.0|light-v3\.0)\b`),
	// HuggingFace BAAI / sentence-transformers.
	regexp.MustCompile(`\bBAAI/bge-(?:small|base|large)-en(?:-v1\.5)?\b`),
	regexp.MustCompile(`\bsentence-transformers/all-MiniLM-L6-v2\b`),
	regexp.MustCompile(`\bsentence-transformers/all-mpnet-base-v2\b`),
	// Google.
	regexp.MustCompile(`\btextembedding-gecko(?:@\d+)?\b`),
}

// embeddingScanExtensions is the source-file extension allowlist.
var embeddingScanExtensions = map[string]bool{
	".py": true, ".js": true, ".ts": true, ".tsx": true, ".jsx": true,
	".go": true, ".java": true, ".rb": true, ".rs": true,
	".yaml": true, ".yml": true, ".json": true,
}

// retrievalCategoryMarkers identifies scenarios that exercise
// retrieval. Case-insensitive substring match.
var retrievalCategoryMarkers = []string{
	"retriev", "rag", "embedding", "vector", "knn",
}

// Detect emits SignalAIEmbeddingModelChange per file with an
// embedding identifier when no retrieval-shaped scenario covers it.
func (d *EmbeddingModelChangeDetector) Detect(snap *models.TestSuiteSnapshot) []models.Signal {
	if d == nil || snap == nil {
		return nil
	}

	if hasRetrievalCoverage(snap) {
		// User has retrieval evals already; an embedding swap will
		// surface as aiRetrievalRegression on the next run. No
		// signal here — would be noisy.
		return nil
	}

	var out []models.Signal
	emitted := map[string]bool{}

	// Prefer structured RAG surfaces. ParseRAGStructured already
	// extracted the embedding model name and line into Config.ModelName,
	// so we avoid a redundant file scan and surface a higher-confidence
	// signal when this path fires.
	for _, comp := range snap.RAGPipelineSurfaces {
		if comp.Kind != models.RAGEmbedding || comp.Config.ModelName == "" {
			continue
		}
		if emitted[comp.Path] {
			continue
		}
		emitted[comp.Path] = true
		out = append(out, buildEmbeddingChangeSignal(comp.Path, comp.Line, comp.Config.ModelName, 1, models.EvidenceStrong, 0.85))
	}

	candidatePaths := d.gatherSourcePaths(snap)
	for _, rel := range candidatePaths {
		if emitted[rel] {
			continue
		}
		hits := scanFileForEmbeddingModels(filepath.Join(d.Root, rel))
		if len(hits) == 0 {
			continue
		}
		emitted[rel] = true
		out = append(out, buildEmbeddingChangeSignal(rel, hits[0].Line, hits[0].Identifier, len(hits), models.EvidenceModerate, 0.8))
	}
	return out
}

// buildEmbeddingChangeSignal constructs the canonical
// SignalAIEmbeddingModelChange signal. Confidence and evidence
// strength vary by detection path: structured RAG surfaces (ModelName
// extracted from a known framework constructor) carry stronger
// evidence than a regex match in arbitrary source.
func buildEmbeddingChangeSignal(path string, line int, identifier string, matches int, strength models.EvidenceStrength, confidence float64) models.Signal {
	intervalLow := confidence - 0.1
	if intervalLow < 0 {
		intervalLow = 0
	}
	intervalHigh := confidence + 0.08
	if intervalHigh > 1 {
		intervalHigh = 1
	}
	return models.Signal{
		Type:            signals.SignalAIEmbeddingModelChange,
		Category:        models.CategoryAI,
		Severity:        models.SeverityMedium,
		Confidence:      confidence,
		Location:        models.SignalLocation{File: path, Line: line},
		Explanation:     "File references embedding model `" + identifier + "` but the project has no retrieval-shaped eval scenario. A future model swap will silently change retrieval quality.",
		SuggestedAction: "Add a retrieval eval scenario (Ragas, Promptfoo, or DeepEval) that exercises this surface so future embedding swaps surface as a quality regression instead of going unnoticed.",

		SeverityClauses: []string{"sev-medium-008"},
		Actionability:   models.ActionabilityScheduled,
		LifecycleStages: []models.LifecycleStage{models.StageDesign, models.StageMaintenance},
		AIRelevance:     models.AIRelevanceHigh,
		RuleID:          "TER-AI-110",
		RuleURI:         "docs/rules/ai/embedding-model-change.md",
		DetectorVersion: "0.2.0",
		ConfidenceDetail: &models.ConfidenceDetail{
			Value:        confidence,
			IntervalLow:  intervalLow,
			IntervalHigh: intervalHigh,
			Quality:      "heuristic",
			Sources:      []models.EvidenceSource{models.SourceStructuralPattern},
		},
		EvidenceSource:   models.SourceStructuralPattern,
		EvidenceStrength: strength,
		Metadata: map[string]any{
			"embeddingModel": identifier,
			"matches":        matches,
		},
	}
}

// gatherSourcePaths returns repo-relative paths the detector should
// scan: union of TestFiles + Scenarios + CodeSurface paths,
// extension-filtered.
func (d *EmbeddingModelChangeDetector) gatherSourcePaths(snap *models.TestSuiteSnapshot) []string {
	seen := map[string]bool{}
	var out []string
	add := func(p string) {
		if p == "" {
			return
		}
		if !embeddingScanExtensions[strings.ToLower(filepath.Ext(p))] {
			return
		}
		if seen[p] {
			return
		}
		seen[p] = true
		out = append(out, p)
	}
	for _, tf := range snap.TestFiles {
		add(tf.Path)
	}
	for _, sc := range snap.Scenarios {
		add(sc.Path)
	}
	for _, surface := range snap.CodeSurfaces {
		add(surface.Path)
	}
	return out
}

// hasRetrievalCoverage returns true when the snapshot has at least
// one scenario whose category / name / description references
// retrieval, OR a scenario covers a SurfaceRetrieval / SurfaceAgent
// surface.
func hasRetrievalCoverage(snap *models.TestSuiteSnapshot) bool {
	retrievalSurfaces := map[string]bool{}
	for _, surface := range snap.CodeSurfaces {
		if surface.Kind == models.SurfaceRetrieval || surface.Kind == models.SurfaceAgent {
			retrievalSurfaces[surface.SurfaceID] = true
		}
	}
	for _, sc := range snap.Scenarios {
		hay := strings.ToLower(sc.Category + " " + sc.Name + " " + sc.Description)
		for _, marker := range retrievalCategoryMarkers {
			if strings.Contains(hay, marker) {
				return true
			}
		}
		for _, sid := range sc.CoveredSurfaceIDs {
			if retrievalSurfaces[sid] {
				return true
			}
		}
	}
	return false
}

// embeddingHit is one match in one file.
type embeddingHit struct {
	Identifier string
	Line       int
}

// scanFileForEmbeddingModels reads the file and returns each unique
// embedding model identifier that appears, with first-occurrence line.
// Files that fail to open return nil.
func scanFileForEmbeddingModels(path string) []embeddingHit {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	const maxLine = 1 << 20
	buf := make([]byte, 64*1024)
	sc.Buffer(buf, maxLine)

	seen := map[string]bool{}
	var hits []embeddingHit
	line := 0
	for sc.Scan() {
		line++
		text := sc.Text()
		for _, rx := range embeddingModelPatterns {
			match := rx.FindString(text)
			if match == "" {
				continue
			}
			if seen[match] {
				continue
			}
			seen[match] = true
			hits = append(hits, embeddingHit{Identifier: match, Line: line})
		}
	}
	return hits
}
