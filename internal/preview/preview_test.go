package preview

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pmclSF/terrain/internal/models"
)

func TestDetectOrphanedEval(t *testing.T) {
	t.Parallel()
	evals := []models.Eval{
		{EvalID: "e1", Name: "covered", CoveredSurfaceIDs: []string{"s1"}},
		{EvalID: "e2", Name: "orphan"},
	}
	sigs := DetectOrphanedEval(evals)
	if len(sigs) != 1 {
		t.Fatalf("expected 1 signal, got %d", len(sigs))
	}
	if sigs[0].Metadata["evalId"] != "e2" {
		t.Errorf("fired on wrong eval: %+v", sigs[0])
	}
}

func TestDetectMissingEvalCategories(t *testing.T) {
	t.Parallel()
	evals := []models.Eval{
		{Category: "happy_path"},
		{Category: "happy_path"},
	}
	sigs := DetectMissingEvalCategories(evals)
	if len(sigs) != 1 {
		t.Fatalf("expected 1 signal, got %d", len(sigs))
	}
	missing := sigs[0].Metadata["missing"].([]string)
	if len(missing) != 3 {
		t.Errorf("expected 3 missing categories, got %v", missing)
	}
}

func TestDetectPromptBloat(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	big := filepath.Join(dir, "big.txt")
	small := filepath.Join(dir, "small.txt")
	_ = os.WriteFile(big, make([]byte, 10000), 0o644)
	_ = os.WriteFile(small, []byte("short"), 0o644)
	sigs := DetectPromptBloat([]string{big, small}, 8000)
	if len(sigs) != 1 {
		t.Fatalf("expected 1 signal, got %d", len(sigs))
	}
	if sigs[0].Location.File != big {
		t.Errorf("fired on wrong file: %+v", sigs[0])
	}
}

func TestDetectPromptWithoutTemperature(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	withTemp := filepath.Join(dir, "with.py")
	withoutTemp := filepath.Join(dir, "without.py")
	_ = os.WriteFile(withTemp, []byte(`client.chat.completions.create(
    model="gpt-4o",
    temperature=0,
    messages=[{}],
)
`), 0o644)
	_ = os.WriteFile(withoutTemp, []byte(`client.chat.completions.create(
    model="gpt-4o",
    messages=[{}],
)
`), 0o644)

	calls := []CallSite{
		{Path: withTemp, Line: 1, SDK: "openai", Method: "client.chat.completions.create"},
		{Path: withoutTemp, Line: 1, SDK: "openai", Method: "client.chat.completions.create"},
	}
	sigs := DetectPromptWithoutTemperature(calls, nil)
	if len(sigs) != 1 {
		t.Fatalf("expected 1 signal, got %d: %+v", len(sigs), sigs)
	}
	if sigs[0].Location.File != withoutTemp {
		t.Errorf("fired on wrong file: %+v", sigs[0])
	}
}

func TestDetectMissingPromptValidator(t *testing.T) {
	t.Parallel()
	cases := map[string][]byte{
		"with_pydantic.py": []byte(`from pydantic import BaseModel
client.chat.completions.create(messages=[...], response_model=Foo)
`),
		"without_validator.py": []byte(`client.chat.completions.create(messages=[...])
`),
	}
	sigs := DetectMissingPromptValidator(cases)
	if len(sigs) != 1 {
		t.Fatalf("expected 1 signal, got %d", len(sigs))
	}
	if sigs[0].Location.File != "without_validator.py" {
		t.Errorf("fired on wrong file: %+v", sigs[0])
	}
}

func TestDetectAgentLoopRisk(t *testing.T) {
	t.Parallel()
	cases := map[string][]byte{
		"bounded.py":   []byte(`AgentExecutor.from_agent_and_tools(agent=a, tools=t, max_iterations=10)`),
		"unbounded.py": []byte(`AgentExecutor.from_agent_and_tools(agent=a, tools=t)`),
	}
	sigs := DetectAgentLoopRisk(cases)
	if len(sigs) != 1 {
		t.Errorf("expected 1 signal, got %d", len(sigs))
	}
}

func TestDetectToolWithoutBudget(t *testing.T) {
	t.Parallel()
	cases := map[string][]byte{
		"budgeted.py":   []byte(`AgentExecutor(tools=[t], max_iterations=10, max_tool_calls=5)`),
		"unbudgeted.py": []byte(`AgentExecutor(tools=[t], max_iterations=10)`),
	}
	sigs := DetectToolWithoutBudget(cases)
	if len(sigs) != 1 {
		t.Errorf("expected 1 signal, got %d", len(sigs))
	}
}

func TestDetectDuplicateEvalRows(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "evals.csv")
	rows := []string{"x,1", "x,1", "x,1", "y,2", "z,3", "y,2", "y,2", "y,2", "w,4", "v,5"}
	body := ""
	for _, r := range rows {
		body += r + "\n"
	}
	_ = os.WriteFile(path, []byte(body), 0o644)
	sigs := DetectDuplicateEvalRows([]string{path}, 0.05)
	if len(sigs) != 1 {
		t.Fatalf("expected 1 signal, got %d", len(sigs))
	}
}

func TestDetectSchemaDrift(t *testing.T) {
	t.Parallel()
	base := map[string][]string{
		"users":  {"id", "email", "created_at"},
		"orders": {"id", "amount"},
	}
	cur := map[string][]string{
		"users": {"id", "email"},
		// orders missing
		"shipments": {"id"},
	}
	sigs := DetectSchemaDrift(base, cur)
	// users drift + orders missing → 2 signals
	if len(sigs) != 2 {
		t.Errorf("expected 2 signals, got %d", len(sigs))
	}
}

func TestDetectColdStartTime(t *testing.T) {
	t.Parallel()
	samples := []LatencyObservation{
		{Surface: "summarizer", IsFirst: true, Millis: 5000},
		{Surface: "summarizer", IsFirst: false, Millis: 100},
		{Surface: "summarizer", IsFirst: false, Millis: 120},
		{Surface: "summarizer", IsFirst: false, Millis: 110},
	}
	sigs := DetectColdStartTime(samples, 2.0)
	if len(sigs) != 1 {
		t.Fatalf("expected 1 signal, got %d", len(sigs))
	}
}

func TestDetectTokenCostBudget(t *testing.T) {
	t.Parallel()
	base := &CostObservation{CostUSD: 1.00}
	cur := &CostObservation{CostUSD: 3.50, RunID: "r2"}
	sigs := DetectTokenCostBudget(base, cur, 0, 1.0) // ratio > 2x triggers
	if len(sigs) != 1 {
		t.Errorf("expected ratio-based signal, got %d", len(sigs))
	}
}
