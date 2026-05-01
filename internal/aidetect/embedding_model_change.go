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

	candidatePaths := d.gatherSourcePaths(snap)
	var out []models.Signal
	emitted := map[string]bool{}
	for _, rel := range candidatePaths {
		hits := scanFileForEmbeddingModels(filepath.Join(d.Root, rel))
		if len(hits) == 0 {
			continue
		}
		if emitted[rel] {
			continue
		}
		emitted[rel] = true

		out = append(out, models.Signal{
			Type:        signals.SignalAIEmbeddingModelChange,
			Category:    models.CategoryAI,
			Severity:    models.SeverityMedium,
			Confidence:  0.8,
			Location:    models.SignalLocation{File: rel, Line: hits[0].Line},
			Explanation: "File references embedding model `" + hits[0].Identifier + "` but the project has no retrieval-shaped eval scenario. A future model swap will silently change retrieval quality.",
			SuggestedAction: "Add a retrieval eval scenario (Ragas, Promptfoo, or DeepEval) that exercises this surface so future embedding swaps surface as a quality regression instead of going unnoticed.",

			SeverityClauses: []string{"sev-medium-005"},
			Actionability:   models.ActionabilityScheduled,
			LifecycleStages: []models.LifecycleStage{models.StageDesign, models.StageMaintenance},
			AIRelevance:     models.AIRelevanceHigh,
			RuleID:          "TER-AI-110",
			RuleURI:         "docs/rules/ai/embedding-model-change.md",
			DetectorVersion: "0.2.0",
			ConfidenceDetail: &models.ConfidenceDetail{
				Value:        0.8,
				IntervalLow:  0.7,
				IntervalHigh: 0.88,
				Quality:      "heuristic",
				Sources:      []models.EvidenceSource{models.SourceStructuralPattern},
			},
			EvidenceSource:   models.SourceStructuralPattern,
			EvidenceStrength: models.EvidenceModerate,
			Metadata: map[string]any{
				"embeddingModel": hits[0].Identifier,
				"matches":        len(hits),
			},
		})
	}
	return out
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
