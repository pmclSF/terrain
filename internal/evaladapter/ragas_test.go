package evaladapter

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRagasAdapter_CanIngest(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	good := filepath.Join(dir, "ragas_results.json")
	_ = os.WriteFile(good, []byte(`[
  {"question": "What is X?", "answer": "X is Y", "faithfulness": 0.9, "answer_relevancy": 0.85}
]`), 0o644)

	other := filepath.Join(dir, "plain.json")
	_ = os.WriteFile(other, []byte(`[{"foo": "bar"}]`), 0o644)

	a := RagasAdapter{}
	if !a.CanIngest(good) {
		t.Error("expected CanIngest=true on ragas results")
	}
	if a.CanIngest(other) {
		t.Error("expected CanIngest=false on JSON without ragas metrics")
	}
}

func TestRagasAdapter_CanIngest_WrapperShape(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "wrapper.json")
	_ = os.WriteFile(path, []byte(`{
  "metrics": {"faithfulness": 0.85, "answer_relevancy": 0.9},
  "results": [
    {"question": "Q?", "faithfulness": 0.85}
  ]
}`), 0o644)
	a := RagasAdapter{}
	if !a.CanIngest(path) {
		t.Error("expected CanIngest=true on wrapper-object shape")
	}
}

func TestRagasAdapter_Ingest(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "results.json")
	_ = os.WriteFile(path, []byte(`[
  {
    "question": "What is the capital of France?",
    "answer": "Paris",
    "contexts": ["Paris is the capital of France."],
    "ground_truth": "Paris",
    "faithfulness": 0.95,
    "answer_relevancy": 0.92,
    "context_precision": 0.88,
    "context_recall": 0.9
  },
  {
    "question": "When did WWII end?",
    "answer": "1945",
    "contexts": ["..."],
    "ground_truth": "1945",
    "faithfulness": 1.0,
    "answer_relevancy": 0.95
  }
]`), 0o644)

	a := RagasAdapter{}
	run, err := a.Ingest(path)
	if err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	if run.Framework != FrameworkRagas {
		t.Errorf("framework = %q", run.Framework)
	}
	if len(run.Cases) != 2 {
		t.Fatalf("cases = %d, want 2", len(run.Cases))
	}

	c0 := run.Cases[0]
	if c0.Metrics["faithfulness"] != 0.95 {
		t.Errorf("case 0 faithfulness = %v", c0.Metrics["faithfulness"])
	}
	if c0.Metrics["context_precision"] != 0.88 {
		t.Errorf("case 0 context_precision = %v", c0.Metrics["context_precision"])
	}
	// Primary should be faithfulness when present.
	if c0.Score != 0.95 {
		t.Errorf("case 0 primary score = %v, want 0.95 (faithfulness)", c0.Score)
	}
	// Synthesized ID from question.
	if c0.ID == "" {
		t.Error("case 0 should have synthesized ID from question")
	}

	c1 := run.Cases[1]
	if c1.Score != 1.0 {
		t.Errorf("case 1 score = %v, want 1.0", c1.Score)
	}

	// PrimaryMetric = mean of 0.95 and 1.0 = 0.975.
	if run.Stats.PrimaryMetric < 0.974 || run.Stats.PrimaryMetric > 0.976 {
		t.Errorf("PrimaryMetric = %v, want ~0.975", run.Stats.PrimaryMetric)
	}
}

func TestSlugifyRagasQuestion(t *testing.T) {
	t.Parallel()
	cases := []struct{ in, want string }{
		{"What is the capital?", "what-is-the-capital"},
		{"Hello, World!", "hello-world"},
		{"  Multiple   spaces  ", "multiple-spaces"},
		{"123-numeric-start", "123-numeric-start"},
		{"long " + repeat("x", 100), "long-" + repeat("x", 55)},
	}
	for _, tc := range cases {
		got := slugifyRagasQuestion(tc.in, 60)
		if got != tc.want {
			t.Errorf("slugifyRagasQuestion(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func repeat(s string, n int) string {
	out := ""
	for i := 0; i < n; i++ {
		out += s
	}
	return out
}
