package analysis

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/pmclSF/terrain/internal/models"
)

// DetectExtraAISurfaces broadens the set of AI surfaces beyond the
// framework-attributed inference in ai_context_infer.go. Catches
// patterns the round-4 review flagged as gaps:
//
//   - Dataset filename detection: .jsonl, .parquet, .csv, .arrow,
//     .tfrecord, .npy, .npz
//   - DB-cursor / vector-search calls: psycopg2.fetch*, pymongo.find,
//     client.search, ES knn_search, pgvector `<->` / `<#>` operators
//   - MCP tool definitions: Python @mcp.tool / @app.list_tools
//   - In-memory FAISS / NumPy ANN: faiss.IndexFlatL2 etc.
//
// Returns CodeSurface entries that don't already exist in `existing`
// (matched by SurfaceID). Pairs with InferAIContextSurfaces.
func DetectExtraAISurfaces(root string, testFiles []models.TestFile, existing []models.CodeSurface, sourceFiles []string) []models.CodeSurface {
	testPaths := map[string]bool{}
	for _, tf := range testFiles {
		testPaths[tf.Path] = true
	}
	existingIDs := map[string]bool{}
	for _, s := range existing {
		existingIDs[s.SurfaceID] = true
	}

	var out []models.CodeSurface

	// Pass 1: dataset filenames (path-based, no content read).
	out = appendNew(out, existingIDs, detectDatasetSurfaces(testPaths, sourceFiles))

	// Pass 2: content-based detection on each source file.
	for _, rel := range sourceFiles {
		if testPaths[rel] {
			continue
		}
		ext := strings.ToLower(relPathExt(rel))
		if _, ok := contentScanLanguages[ext]; !ok {
			continue
		}
		content, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			continue
		}
		src := string(content)
		out = appendNew(out, existingIDs, detectDBCursorSurfaces(rel, src))
		out = appendNew(out, existingIDs, detectVectorSearchSurfaces(rel, src))
		out = appendNew(out, existingIDs, detectMCPToolSurfaces(rel, src))
	}

	return out
}

// contentScanLanguages is the file-extension allowlist for the extra
// detector. We're stingy with the universe so the cost stays linear
// in test+source-relevant files, not "every text file".
var contentScanLanguages = map[string]bool{
	".py":  true,
	".js":  true,
	".ts":  true,
	".tsx": true,
	".jsx": true,
	".go":  true,
	".rb":  true,
	".rs":  true,
	".java": true,
}

// datasetExtensions is the set of extensions whose mere presence makes
// a file a candidate dataset surface. The `npz` / `npy` / `tfrecord`
// entries cover ML-specific formats that the existing inference misses.
var datasetExtensions = map[string]bool{
	".jsonl":    true,
	".parquet":  true,
	".csv":      true,
	".tsv":      true,
	".arrow":    true,
	".tfrecord": true,
	".npy":      true,
	".npz":      true,
	".pickle":   true,
	".pkl":      true,
}

func detectDatasetSurfaces(testPaths map[string]bool, sourceFiles []string) []models.CodeSurface {
	var out []models.CodeSurface
	for _, rel := range sourceFiles {
		if testPaths[rel] {
			continue
		}
		ext := strings.ToLower(relPathExt(rel))
		if !datasetExtensions[ext] {
			continue
		}
		// Filter out obvious noise: deps lockfiles, package metadata,
		// third-party data fixtures live deeper than top-level data/.
		if strings.HasPrefix(rel, "node_modules/") || strings.HasPrefix(rel, "vendor/") {
			continue
		}
		name := strings.TrimSuffix(filepath.Base(rel), ext)
		out = append(out, models.CodeSurface{
			SurfaceID: models.BuildSurfaceID(rel, name, ""),
			Path:      rel,
			Name:      name,
			Kind:      models.SurfaceDataset,
			Reason:    "Dataset file (extension " + ext + ") referenced in repo tree",
			DetectionTier: "content",
		})
	}
	return out
}

// dbCursorPatterns matches database / ORM calls that frequently
// drive AI context (RAG retrieval, agent state lookups). Matching the
// pattern doesn't prove the call is AI-related, but combined with the
// proximity heuristic in detectDBCursorSurfaces (file already imports
// an LLM/embedding library or has a prompt CodeSurface nearby) the
// false-positive rate stays acceptable.
var dbCursorPatterns = []*regexp.Regexp{
	// Cursor / connection methods. We don't require the variable be
	// named "cursor" — Python idiom names it "cur"; Ruby uses "conn".
	// The fileLooksAIRelated gate keeps the false-positive rate down.
	regexp.MustCompile(`\.fetch(?:one|all|many)\(`),
	regexp.MustCompile(`(?i)\.execute\(\s*["'\x60](?:\s*--[^\n]*\n)*\s*SELECT\b`),
	regexp.MustCompile(`(?i)\bpymongo\b.*\.find\(`),
	regexp.MustCompile(`(?i)\bcollection\.find\(`),
	regexp.MustCompile(`(?i)\bsupabase\b.*\.select\(`),
	regexp.MustCompile(`(?i)\bsqlalchemy\b.*\.execute\(`),
}

// aiSignalPatterns are the substrings whose presence in the same file
// raises confidence that a DB cursor call is AI/RAG-related.
var aiSignalPatterns = []string{
	"openai", "anthropic", "langchain", "llamaindex",
	"embedding", "embed_documents", "embed_query",
	"rag", "retriev", "vector_store", "vectorstore",
	"prompt", "system_prompt", "user_prompt",
}

func detectDBCursorSurfaces(rel, src string) []models.CodeSurface {
	if !fileLooksAIRelated(src) {
		return nil
	}
	var out []models.CodeSurface
	for _, rx := range dbCursorPatterns {
		if loc := rx.FindStringIndex(src); loc != nil {
			line := lineNumberAt(src, loc[0])
			name := "db_retrieval_" + filepath.Base(rel)
			out = append(out, models.CodeSurface{
				SurfaceID: models.BuildSurfaceID(rel, name, ""),
				Path:      rel,
				Name:      name,
				Kind:      models.SurfaceRetrieval,
				Line:      line,
				Reason:    "Database cursor / fetch call in a file with AI-related symbols (likely RAG retrieval)",
				DetectionTier: "content",
			})
			break // one per file is enough; the pattern hits multiple ways
		}
	}
	return out
}

// vectorSearchPatterns matches non-framework retrieval shapes. The
// existing ai_context_infer.go covers framework calls (langchain
// retriever, similarity_search etc.); these handle the raw-API path.
var vectorSearchPatterns = []struct {
	rx   *regexp.Regexp
	name string
}{
	// pgvector: SELECT ... ORDER BY embedding <-> '[...]' or <#>, <=>
	{rx: regexp.MustCompile(`embedding\s*<(?:->|#>|=>)\s*`), name: "pgvector_query"},
	// Elasticsearch knn / kNN search.
	{rx: regexp.MustCompile(`(?i)\bknn_search\b`), name: "es_knn_search"},
	{rx: regexp.MustCompile(`"knn"\s*:`), name: "es_knn_query"},
	// Weaviate REST.
	{rx: regexp.MustCompile(`(?i)/v1/objects.*nearVector`), name: "weaviate_rest"},
	// In-memory FAISS index types.
	{rx: regexp.MustCompile(`\bfaiss\.Index(?:FlatL2|IVFFlat|HNSWFlat)\b`), name: "faiss_in_memory_index"},
	// Generic .search( with vector args.
	{rx: regexp.MustCompile(`\.search\(\s*query_vector\b`), name: "generic_vector_search"},
}

func detectVectorSearchSurfaces(rel, src string) []models.CodeSurface {
	var out []models.CodeSurface
	seen := map[string]bool{}
	for _, p := range vectorSearchPatterns {
		loc := p.rx.FindStringIndex(src)
		if loc == nil {
			continue
		}
		if seen[p.name] {
			continue
		}
		seen[p.name] = true
		out = append(out, models.CodeSurface{
			SurfaceID: models.BuildSurfaceID(rel, p.name, ""),
			Path:      rel,
			Name:      p.name,
			Kind:      models.SurfaceRetrieval,
			Line:      lineNumberAt(src, loc[0]),
			Reason:    "Vector search / retrieval pattern (" + p.name + ")",
			DetectionTier: "content",
		})
	}
	return out
}

// mcpToolPatterns recognize MCP tool definitions across language
// flavours. Python uses decorators (@mcp.tool, @app.list_tools); JS/TS
// uses a `server.tool(...)` call shape.
var mcpToolPatterns = []*regexp.Regexp{
	regexp.MustCompile(`@(?:mcp|app)\.(?:tool|list_tools|call_tool)\b`),
	regexp.MustCompile(`@server\.(?:tool|call_tool)\b`),
	regexp.MustCompile(`\bserver\.tool\(`),
	regexp.MustCompile(`\bregister_tool\(`),
}

func detectMCPToolSurfaces(rel, src string) []models.CodeSurface {
	var out []models.CodeSurface
	for _, rx := range mcpToolPatterns {
		loc := rx.FindStringIndex(src)
		if loc == nil {
			continue
		}
		name := "mcp_tool_" + filepath.Base(rel)
		out = append(out, models.CodeSurface{
			SurfaceID: models.BuildSurfaceID(rel, name, ""),
			Path:      rel,
			Name:      name,
			Kind:      models.SurfaceToolDef,
			Line:      lineNumberAt(src, loc[0]),
			Reason:    "MCP tool definition (decorator / server.tool registration)",
			DetectionTier: "content",
		})
		break
	}
	return out
}

// fileLooksAIRelated returns true when the file contains at least one
// AI-signal substring (lowercased). Used to gate DB-cursor detection
// so we don't flag every `cursor.fetchall` in the codebase as RAG.
func fileLooksAIRelated(src string) bool {
	lower := strings.ToLower(src)
	for _, kw := range aiSignalPatterns {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

// lineNumberAt returns the 1-based line number for byte offset off.
func lineNumberAt(src string, off int) int {
	if off <= 0 || off > len(src) {
		return 1
	}
	return strings.Count(src[:off], "\n") + 1
}
