package preview

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pmclSF/terrain/internal/signals"
)

// TestDetectRetrievalWithoutRerank covers the positive case (retrieval
// call site with no reranker marker), the negative case (reranker is
// present), and the unrelated-code case (no retrieval call at all).
func TestDetectRetrievalWithoutRerank(t *testing.T) {
	cases := []struct {
		name    string
		files   map[string][]byte
		wantSig bool
	}{
		{
			name: "vector retriever without reranker fires",
			files: map[string][]byte{
				"rag.py": []byte(`
docs = vectorstore.as_retriever().invoke(query)
`),
			},
			wantSig: true,
		},
		{
			name: "retriever with BgeReranker does NOT fire",
			files: map[string][]byte{
				"rag.py": []byte(`
docs = vectorstore.as_retriever().invoke(query)
docs = BgeReranker().rerank(docs)
`),
			},
			wantSig: false,
		},
		{
			name: "retriever with CohereRerank does NOT fire",
			files: map[string][]byte{
				"rag.py": []byte(`
results = retriever.invoke(q)
ranked = CohereRerank(model="rerank-2").compress_documents(results, q)
`),
			},
			wantSig: false,
		},
		{
			name: "unrelated code does NOT fire",
			files: map[string][]byte{
				"util.py": []byte(`def add(a, b): return a + b`),
			},
			wantSig: false,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := DetectRetrievalWithoutRerank(c.files)
			has := false
			for _, s := range got {
				if s.Type == signals.SignalRetrievalWithoutRerank {
					has = true
				}
			}
			if has != c.wantSig {
				t.Errorf("got fire=%v want=%v (signals=%+v)", has, c.wantSig, got)
			}
		})
	}
}

// TestDetectColdVectorStore covers the positive case (store init with
// no population call), the negative case (population call present),
// and the unrelated-code case.
func TestDetectColdVectorStore(t *testing.T) {
	cases := []struct {
		name    string
		files   map[string][]byte
		wantSig bool
	}{
		{
			name: "Chroma init without population call fires",
			files: map[string][]byte{
				"index.py": []byte(`
store = Chroma(collection_name="docs")
results = store.similarity_search(query)
`),
			},
			wantSig: true,
		},
		{
			name: "Chroma init WITH add_documents does NOT fire",
			files: map[string][]byte{
				"index.py": []byte(`
store = Chroma(collection_name="docs")
store.add_documents(documents)
results = store.similarity_search(query)
`),
			},
			wantSig: false,
		},
		{
			name: "Pinecone init without upsert fires",
			files: map[string][]byte{
				"main.py": []byte(`pc = Pinecone(api_key=key)`),
			},
			wantSig: true,
		},
		{
			name: "unrelated code does NOT fire",
			files: map[string][]byte{
				"hello.py": []byte(`print("hi")`),
			},
			wantSig: false,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := DetectColdVectorStore(c.files)
			has := false
			for _, s := range got {
				if s.Type == signals.SignalColdVectorStore {
					has = true
				}
			}
			if has != c.wantSig {
				t.Errorf("got fire=%v want=%v (signals=%+v)", has, c.wantSig, got)
			}
		})
	}
}

// TestDetectPromptVersionSkew covers the positive case (two similar
// prompt files), the negative case (two unrelated prompt files), and
// the boundary case (one of the files is too small to compare).
func TestDetectPromptVersionSkew(t *testing.T) {
	dir := t.TempDir()
	writePrompt := func(t *testing.T, name, body string) string {
		t.Helper()
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", p, err)
		}
		return p
	}
	t.Run("two near-identical prompts fire", func(t *testing.T) {
		// Prompts must be >= 200 bytes after whitespace normalization
		// for the version-skew detector's prefix-match heuristic to
		// engage. Build the shared head from repeated content so the
		// matching prefix is well above the threshold.
		shared := "You are an expert support agent for our SaaS product. " +
			"Answer the user politely and concisely. " +
			"Use the knowledge base before improvising. " +
			"If you don't know, say so. " +
			"Always include a citation when claiming a fact. " +
			"Never reveal internal pricing terms. " +
			"Escalate to a human when frustrated."
		p1 := writePrompt(t, "v1.md", shared+" Tone: friendly and encouraging.")
		p2 := writePrompt(t, "v2.md", shared+" Tone: professional and direct.")
		got := DetectPromptVersionSkew([]string{p1, p2})
		if len(got) == 0 {
			t.Errorf("expected a version-skew signal between near-identical prompts; got 0")
		}
	})
	t.Run("two unrelated prompts do NOT fire", func(t *testing.T) {
		// Both need to clear the 200-byte threshold to even be
		// compared. The bodies share no prefix so the match should
		// not fire.
		p1 := writePrompt(t, "support.md", "You are a support agent helping users navigate our product. "+
			"Respond in plain English. Use the knowledge base. Acknowledge frustration. "+
			"Suggest workarounds when a feature is missing. Document each conversation in the ticket.")
		p2 := writePrompt(t, "summarizer.md", "Summarize the following document in three bullet points. "+
			"Be concise. Highlight the action item if any. Preserve quantitative facts verbatim. "+
			"Cite the source paragraph for each bullet. Skip rhetorical language.")
		got := DetectPromptVersionSkew([]string{p1, p2})
		// Independent prompts should not match; if they do, the
		// similarity threshold is too loose.
		for _, s := range got {
			t.Errorf("unexpected match between unrelated prompts: %+v", s)
		}
	})
	t.Run("tiny files are skipped (< 200 bytes)", func(t *testing.T) {
		p := writePrompt(t, "tiny.md", "Be helpful.")
		got := DetectPromptVersionSkew([]string{p, p})
		if len(got) > 0 {
			t.Errorf("tiny files should be skipped; got %+v", got)
		}
	})
}

// TestDetectTargetLeakage covers a training-shaped path with a
// target-derived feature pattern, a training path without the
// pattern, and a non-training path.
func TestDetectTargetLeakage(t *testing.T) {
	cases := []struct {
		name    string
		files   map[string][]byte
		wantSig bool
	}{
		{
			name: "training file with y_train-derived feature fires",
			files: map[string][]byte{
				"models/train.py": []byte(`
y_train = df["target"]
X_train["lagged"] = y_train.shift(1)
`),
			},
			wantSig: true,
		},
		{
			name: "training file with clean feature derivation does NOT fire",
			files: map[string][]byte{
				"models/train.py": []byte(`
y_train = df["target"]
X_train["count"] = df["events"].rolling(7).count()
`),
			},
			wantSig: false,
		},
		{
			name: "non-training path does NOT fire even with leakage pattern",
			files: map[string][]byte{
				"util/helpers.py": []byte(`
y_train = df["target"]
X_train["lagged"] = y_train.shift(1)
`),
			},
			wantSig: false,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := DetectTargetLeakage(c.files)
			has := false
			for _, s := range got {
				if s.Type == signals.SignalTargetLeakage {
					has = true
				}
			}
			if has != c.wantSig {
				t.Errorf("got fire=%v want=%v (signals=%+v)", has, c.wantSig, got)
			}
		})
	}
}
