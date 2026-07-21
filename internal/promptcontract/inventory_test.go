package promptcontract

import (
	"os"
	"path/filepath"
	"testing"
)

// TestAnalyzeRepo_Inventory locks the surface counts AnalyzeRepo returns — the
// "comprehension proof" the first-run report leads with. Two prompts live in one
// file so the dedup (PromptFiles < Prompts) is exercised, and a second file
// contributes a third prompt + a second schema.
func TestAnalyzeRepo_Inventory(t *testing.T) {
	root := t.TempDir()
	write := func(rel, body string) {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// models.py: two schemas.
	write("models.py", "from pydantic import BaseModel\n\n"+
		"class UserProfile(BaseModel):\n    user_id: str\n    name: str\n\n"+
		"class Order(BaseModel):\n    order_id: str\n")
	// prompts.py: TWO prompt surfaces in ONE file (both consistent, no drift).
	write("prompts.py", "import openai\nfrom models import UserProfile, Order\n\n"+
		"def greet(user: UserProfile) -> str:\n    return f\"\"\"Hi {user.name} ({user.user_id}).\"\"\"\n\n"+
		"def order_line(order: Order) -> str:\n    return f\"\"\"Order {order.order_id}.\"\"\"\n")
	// more.py: a THIRD prompt surface in a second file.
	write("more.py", "import openai\nfrom models import UserProfile\n\n"+
		"def bye(user: UserProfile) -> str:\n    return f\"\"\"Bye {user.name}.\"\"\"\n")

	inv, drift, err := AnalyzeRepo(root)
	if err != nil {
		t.Fatalf("AnalyzeRepo: %v", err)
	}
	if len(drift) != 0 {
		t.Fatalf("fixture is consistent; expected no drift, got %d: %+v", len(drift), drift)
	}
	if inv.Schemas != 2 {
		t.Errorf("Schemas = %d, want 2 (UserProfile, Order)", inv.Schemas)
	}
	if inv.Prompts != 3 {
		t.Errorf("Prompts = %d, want 3 (two in prompts.py, one in more.py)", inv.Prompts)
	}
	if inv.PromptFiles != 2 {
		t.Errorf("PromptFiles = %d, want 2 (prompts.py, more.py) — dedup of the two-prompt file", inv.PromptFiles)
	}
	if inv.PromptFiles >= inv.Prompts {
		t.Errorf("PromptFiles (%d) must be < Prompts (%d) when a file holds >1 prompt", inv.PromptFiles, inv.Prompts)
	}
}

// TestAnalyzeRepo_NonAIRepoEmptyInventory confirms a repo with no AI import
// yields a zero inventory (the AI gate short-circuits before parsing).
func TestAnalyzeRepo_NonAIRepoEmptyInventory(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "plain.py"),
		[]byte("def add(a, b):\n    return a + b\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	inv, drift, err := AnalyzeRepo(root)
	if err != nil {
		t.Fatalf("AnalyzeRepo: %v", err)
	}
	if inv.Prompts != 0 || inv.Schemas != 0 || inv.PromptFiles != 0 {
		t.Errorf("non-AI repo should yield an empty inventory, got %+v", inv)
	}
	if len(drift) != 0 {
		t.Errorf("non-AI repo should yield no drift, got %+v", drift)
	}
}
