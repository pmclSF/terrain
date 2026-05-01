package analysis

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pmclSF/terrain/internal/models"
)

func writeSrc(t *testing.T, root, rel, content string) string {
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

func TestDetectExtraAISurfaces_DatasetExtensions(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeSrc(t, root, "data/eval.jsonl", `{"prompt": "x"}`)
	writeSrc(t, root, "data/labels.parquet", "binary-content")
	writeSrc(t, root, "data/notes.md", "regular markdown")

	surfaces := DetectExtraAISurfaces(root, nil, nil, []string{
		"data/eval.jsonl", "data/labels.parquet", "data/notes.md",
	})

	kinds := map[string]int{}
	for _, s := range surfaces {
		kinds[string(s.Kind)]++
	}
	if kinds[string(models.SurfaceDataset)] != 2 {
		t.Errorf("dataset surfaces = %d, want 2", kinds[string(models.SurfaceDataset)])
	}
}

func TestDetectExtraAISurfaces_PgvectorQuery(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	rel := writeSrc(t, root, "src/retrieve.py", `
import psycopg2
def search(embedding):
    cur = psycopg2.connect("...").cursor()
    cur.execute("SELECT id FROM docs ORDER BY embedding <-> %s LIMIT 5", (embedding,))
    return cur.fetchall()
`)
	surfaces := DetectExtraAISurfaces(root, nil, nil, []string{rel})
	if !hasSurfaceWithName(surfaces, "pgvector_query") {
		t.Errorf("expected pgvector_query, got %+v", surfaces)
	}
}

func TestDetectExtraAISurfaces_FAISSIndex(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	rel := writeSrc(t, root, "src/index.py", `
import faiss
import numpy as np
index = faiss.IndexFlatL2(768)
embeddings = np.random.rand(100, 768).astype('float32')
index.add(embeddings)
`)
	surfaces := DetectExtraAISurfaces(root, nil, nil, []string{rel})
	if !hasSurfaceWithName(surfaces, "faiss_in_memory_index") {
		t.Errorf("expected faiss_in_memory_index, got %+v", surfaces)
	}
}

func TestDetectExtraAISurfaces_MCPToolDecorator(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	rel := writeSrc(t, root, "agent/tools.py", `
from mcp.server import Server
app = Server("my-agent")

@app.tool()
def get_weather(city: str) -> str:
    return "sunny"
`)
	surfaces := DetectExtraAISurfaces(root, nil, nil, []string{rel})
	found := false
	for _, s := range surfaces {
		if s.Kind == models.SurfaceToolDef {
			found = true
		}
	}
	if !found {
		t.Errorf("expected MCP tool surface, got %+v", surfaces)
	}
}

func TestDetectExtraAISurfaces_DBCursorOnlyWhenAIContext(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	// File 1: cursor.execute but no AI symbols → should NOT fire.
	relPlain := writeSrc(t, root, "src/db.py", `
import psycopg2
def list_users(conn):
    cur = conn.cursor()
    cur.execute("SELECT id, name FROM users")
    return cur.fetchall()
`)
	// File 2: cursor.execute alongside an embedding library → should fire.
	relAI := writeSrc(t, root, "src/rag.py", `
import psycopg2
from openai import OpenAI

def retrieve(query):
    client = OpenAI()
    embedding = client.embeddings.create(input=query, model="text-embedding-3-small")
    cur = psycopg2.connect("...").cursor()
    cur.execute("SELECT * FROM docs LIMIT 5")
    return cur.fetchall()
`)
	surfaces := DetectExtraAISurfaces(root, nil, nil, []string{relPlain, relAI})

	plainCount, aiCount := 0, 0
	for _, s := range surfaces {
		if s.Kind != models.SurfaceRetrieval {
			continue
		}
		if s.Path == relPlain {
			plainCount++
		}
		if s.Path == relAI {
			aiCount++
		}
	}
	if plainCount != 0 {
		t.Errorf("non-AI file fired %d retrieval surfaces, want 0", plainCount)
	}
	if aiCount == 0 {
		t.Errorf("AI-context file fired %d retrieval surfaces, want >=1", aiCount)
	}
}

func TestDetectExtraAISurfaces_SkipsExisting(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	rel := writeSrc(t, root, "data/eval.jsonl", `{}`)
	existingID := models.BuildSurfaceID(rel, "eval", "")
	existing := []models.CodeSurface{
		{SurfaceID: existingID, Path: rel, Name: "eval", Kind: models.SurfaceDataset},
	}
	surfaces := DetectExtraAISurfaces(root, nil, existing, []string{rel})
	if len(surfaces) != 0 {
		t.Errorf("expected no new surfaces (existing covers it), got %d", len(surfaces))
	}
}

func hasSurfaceWithName(surfaces []models.CodeSurface, name string) bool {
	for _, s := range surfaces {
		if s.Name == name {
			return true
		}
	}
	return false
}
