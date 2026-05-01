package aidetect

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pmclSF/terrain/internal/models"
	"github.com/pmclSF/terrain/internal/signals"
)

func writeEmbeddingProbeFile(t *testing.T, root, rel, content string) string {
	t.Helper()
	full := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return rel
}

func TestEmbeddingModelChange_FiresOnOpenAIIdentifier(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	rel := writeEmbeddingProbeFile(t, root, "rag/embed.py", `
from openai import OpenAI

client = OpenAI()

def embed(text: str):
    return client.embeddings.create(
        model="text-embedding-3-large",
        input=text,
    )
`)
	snap := &models.TestSuiteSnapshot{
		CodeSurfaces: []models.CodeSurface{
			{SurfaceID: "s1", Path: rel, Name: "embed", Kind: models.SurfacePrompt},
		},
	}
	got := (&EmbeddingModelChangeDetector{Root: root}).Detect(snap)
	if len(got) != 1 {
		t.Fatalf("got %d signals, want 1", len(got))
	}
	if got[0].Type != signals.SignalAIEmbeddingModelChange {
		t.Errorf("type = %q", got[0].Type)
	}
	if got[0].RuleID != "TER-AI-110" {
		t.Errorf("ruleID = %q, want TER-AI-110", got[0].RuleID)
	}
	if got[0].Metadata["embeddingModel"] != "text-embedding-3-large" {
		t.Errorf("metadata embeddingModel = %v, want text-embedding-3-large", got[0].Metadata["embeddingModel"])
	}
}

func TestEmbeddingModelChange_FiresOnVoyageAndBAAI(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	relVoyage := writeEmbeddingProbeFile(t, root, "rag/voyage.ts", `
import { VoyageAIClient } from "voyageai";
const client = new VoyageAIClient();
const result = await client.embed({ model: "voyage-code-2", input: "..." });
`)
	relBAAI := writeEmbeddingProbeFile(t, root, "rag/bge.py", `
from sentence_transformers import SentenceTransformer
model = SentenceTransformer("BAAI/bge-large-en-v1.5")
`)
	snap := &models.TestSuiteSnapshot{
		CodeSurfaces: []models.CodeSurface{
			{SurfaceID: "s1", Path: relVoyage, Name: "voyage", Kind: models.SurfacePrompt},
			{SurfaceID: "s2", Path: relBAAI, Name: "bge", Kind: models.SurfacePrompt},
		},
	}
	got := (&EmbeddingModelChangeDetector{Root: root}).Detect(snap)
	if len(got) != 2 {
		t.Fatalf("got %d signals, want 2", len(got))
	}
}

func TestEmbeddingModelChange_QuietWhenRetrievalScenarioCovers(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	rel := writeEmbeddingProbeFile(t, root, "rag/embed.py", `
client.embeddings.create(model="text-embedding-3-small", input=text)
`)
	snap := &models.TestSuiteSnapshot{
		CodeSurfaces: []models.CodeSurface{
			{SurfaceID: "s1", Path: rel, Name: "embed", Kind: models.SurfacePrompt},
		},
		Scenarios: []models.Scenario{
			{
				ScenarioID: "scenario:1",
				Name:       "rag baseline",
				Category:   "retrieval",
			},
		},
	}
	if got := (&EmbeddingModelChangeDetector{Root: root}).Detect(snap); len(got) != 0 {
		t.Errorf("retrieval-shaped scenario should suppress, got %d", len(got))
	}
}

func TestEmbeddingModelChange_QuietWhenSurfaceKindIsRetrievalAndCovered(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	rel := writeEmbeddingProbeFile(t, root, "rag/embed.py", `
client.embeddings.create(model="text-embedding-ada-002", input=text)
`)
	snap := &models.TestSuiteSnapshot{
		CodeSurfaces: []models.CodeSurface{
			{SurfaceID: "s1", Path: rel, Name: "embed", Kind: models.SurfaceRetrieval},
		},
		Scenarios: []models.Scenario{
			{
				ScenarioID:        "scenario:1",
				Name:              "happy path",
				Category:          "smoke",
				CoveredSurfaceIDs: []string{"s1"},
			},
		},
	}
	if got := (&EmbeddingModelChangeDetector{Root: root}).Detect(snap); len(got) != 0 {
		t.Errorf("retrieval surface coverage should suppress, got %d", len(got))
	}
}

func TestEmbeddingModelChange_QuietWhenNoEmbeddingReference(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	rel := writeEmbeddingProbeFile(t, root, "rag/handler.py", `
def handler(request):
    return {"status": "ok"}
`)
	snap := &models.TestSuiteSnapshot{
		CodeSurfaces: []models.CodeSurface{
			{SurfaceID: "s1", Path: rel, Name: "handler", Kind: models.SurfacePrompt},
		},
	}
	if got := (&EmbeddingModelChangeDetector{Root: root}).Detect(snap); len(got) != 0 {
		t.Errorf("plain handler file should not fire, got %d", len(got))
	}
}

func TestEmbeddingModelChange_OneSignalPerFile(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	rel := writeEmbeddingProbeFile(t, root, "rag/embed.py", `
PRIMARY = "text-embedding-3-large"
FALLBACK = "text-embedding-3-small"
LEGACY = "text-embedding-ada-002"
`)
	snap := &models.TestSuiteSnapshot{
		CodeSurfaces: []models.CodeSurface{
			{SurfaceID: "s1", Path: rel, Name: "embed", Kind: models.SurfacePrompt},
		},
	}
	got := (&EmbeddingModelChangeDetector{Root: root}).Detect(snap)
	if len(got) != 1 {
		t.Fatalf("got %d signals, want 1 per file regardless of match count", len(got))
	}
	if matches, _ := got[0].Metadata["matches"].(int); matches != 3 {
		t.Errorf("metadata matches = %v, want 3", got[0].Metadata["matches"])
	}
}

func TestEmbeddingModelChange_PrefersStructuredRAGSurface(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	rel := writeEmbeddingProbeFile(t, root, "rag/embed.py", `
embeddings = OpenAIEmbeddings(model="text-embedding-3-large")
`)
	snap := &models.TestSuiteSnapshot{
		CodeSurfaces: []models.CodeSurface{
			{SurfaceID: "s1", Path: rel, Name: "embed", Kind: models.SurfacePrompt},
		},
		RAGPipelineSurfaces: []models.RAGPipelineSurface{
			{
				ComponentID: "rag:" + rel + ":embedding:openai_embeddings",
				Name:        "openai_embeddings",
				Path:        rel,
				Kind:        models.RAGEmbedding,
				Line:        2,
				Config:      models.RAGComponentConfig{ModelName: "text-embedding-3-large"},
			},
		},
	}
	got := (&EmbeddingModelChangeDetector{Root: root}).Detect(snap)
	if len(got) != 1 {
		t.Fatalf("got %d signals, want 1", len(got))
	}
	if got[0].EvidenceStrength != models.EvidenceStrong {
		t.Errorf("structured RAG path should yield EvidenceStrong, got %v", got[0].EvidenceStrength)
	}
	if got[0].Confidence != 0.85 {
		t.Errorf("structured path confidence = %v, want 0.85", got[0].Confidence)
	}
	if got[0].Metadata["embeddingModel"] != "text-embedding-3-large" {
		t.Errorf("metadata embeddingModel = %v", got[0].Metadata["embeddingModel"])
	}
	if got[0].Location.Line != 2 {
		t.Errorf("location.Line = %v, want 2", got[0].Location.Line)
	}
}

func TestEmbeddingModelChange_NilInputs(t *testing.T) {
	t.Parallel()

	var d *EmbeddingModelChangeDetector
	if got := d.Detect(nil); got != nil {
		t.Errorf("nil detector should return nil, got %v", got)
	}
	if got := (&EmbeddingModelChangeDetector{}).Detect(nil); got != nil {
		t.Errorf("nil snapshot should return nil, got %v", got)
	}
}
