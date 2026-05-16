package preview

import (
	"strings"

	"github.com/pmclSF/terrain/internal/models"
	"github.com/pmclSF/terrain/internal/signals"
)

// DetectRetrievalWithoutRerank fires when a Python source file imports
// a retriever / vector store, calls a retrieve method with k > 5, and
// has no reranker in the same file. Implements
// terrain/retrieval-quality/no-rerank.
func DetectRetrievalWithoutRerank(sourceFiles map[string][]byte) []models.Signal {
	var out []models.Signal
	for path, content := range sourceFiles {
		s := string(content)
		if !looksLikeRetrievalCallSite(s) {
			continue
		}
		if hasRerankerMarker(s) {
			continue
		}
		out = append(out, signal(
			signals.SignalRetrievalWithoutRerank, models.SeverityLow,
			"terrain/retrieval-quality/no-rerank",
			"docs/rules/retrieval-quality/no-rerank.md",
			models.SignalLocation{File: path},
			"Retrieval pipeline returns multiple results with no reranking step.",
			"Add a reranker (BgeReranker, CohereRerank, CrossEncoderRerank, or MMR) between retrieval and generation to improve precision.",
			map[string]any{},
		))
	}
	return out
}

func looksLikeRetrievalCallSite(s string) bool {
	markers := []string{
		".as_retriever(",
		".invoke(",
		"VectorStoreRetriever",
		"similarity_search(",
		".retrieve(",
	}
	for _, m := range markers {
		if strings.Contains(s, m) {
			return true
		}
	}
	return false
}

func hasRerankerMarker(s string) bool {
	markers := []string{
		"CrossEncoder", "BgeReranker", "CohereRerank",
		"MaximalMarginalRelevance", "MMR",
		"reranker", "Reranker", "rerank=", "rerank(",
	}
	for _, m := range markers {
		if strings.Contains(s, m) {
			return true
		}
	}
	return false
}

// DetectColdVectorStore fires when a Python file constructs a vector
// store but doesn't appear to populate it in the same module.
// Implements terrain/retrieval-quality/cold-store.
func DetectColdVectorStore(sourceFiles map[string][]byte) []models.Signal {
	var out []models.Signal
	for path, content := range sourceFiles {
		s := string(content)
		if !looksLikeVectorStoreInit(s) {
			continue
		}
		if hasPopulationCall(s) {
			continue
		}
		out = append(out, signal(
			signals.SignalColdVectorStore, models.SeverityLow,
			"terrain/retrieval-quality/cold-store",
			"docs/rules/retrieval-quality/cold-store.md",
			models.SignalLocation{File: path},
			"Vector store initialized but no population call (add_documents / upsert / write_index / INSERT) in the same module.",
			"Confirm the store is populated elsewhere, or add the population call. Querying an empty index returns nothing.",
			map[string]any{},
		))
	}
	return out
}

func looksLikeVectorStoreInit(s string) bool {
	markers := []string{
		"Chroma(", "Chroma.from_",
		"Pinecone(", "Pinecone.from_existing",
		"Weaviate(", "Weaviate.from_",
		"Qdrant(", "Qdrant.from_",
		"Milvus(",
		"FAISS(", "FAISS.from_",
		"PGVector(", "pgvector",
	}
	for _, m := range markers {
		if strings.Contains(s, m) {
			return true
		}
	}
	return false
}

func hasPopulationCall(s string) bool {
	markers := []string{
		".add_documents(",
		".add_texts(",
		".upsert(",
		".add(",
		".write_index(",
		"INSERT INTO",
		".index(",
	}
	for _, m := range markers {
		if strings.Contains(s, m) {
			return true
		}
	}
	return false
}
