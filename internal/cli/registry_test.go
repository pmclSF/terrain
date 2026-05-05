package cli

import (
	"strings"
	"sync"
	"testing"
)

func TestRegister_RequiresName(t *testing.T) {
	t.Parallel()
	r := New()
	err := r.Register(Command{Pillar: PillarUnderstand})
	if err == nil || !strings.Contains(err.Error(), "Name is required") {
		t.Errorf("expected name-required error, got: %v", err)
	}
}

func TestRegister_RequiresPillar(t *testing.T) {
	t.Parallel()
	r := New()
	err := r.Register(Command{Name: "analyze"})
	if err == nil || !strings.Contains(err.Error(), "no Pillar") {
		t.Errorf("expected pillar-required error, got: %v", err)
	}
}

func TestRegister_DuplicateNameFails(t *testing.T) {
	t.Parallel()
	r := New()
	if err := r.Register(Command{Name: "analyze", Pillar: PillarUnderstand}); err != nil {
		t.Fatalf("first register: %v", err)
	}
	err := r.Register(Command{Name: "analyze", Pillar: PillarUnderstand})
	if err == nil || !strings.Contains(err.Error(), "already registered") {
		t.Errorf("expected duplicate-name error, got: %v", err)
	}
}

func TestRegister_AliasCollisionFails(t *testing.T) {
	t.Parallel()
	r := New()
	if err := r.Register(Command{Name: "analyze", Pillar: PillarUnderstand}); err != nil {
		t.Fatal(err)
	}
	err := r.Register(Command{
		Name:    "report",
		Pillar:  PillarUnderstand,
		Aliases: []string{"analyze"},
	})
	if err == nil || !strings.Contains(err.Error(), "collides") {
		t.Errorf("expected alias-collision error, got: %v", err)
	}
}

func TestGet_FindsByNameAndAlias(t *testing.T) {
	t.Parallel()
	r := New()
	cmd := Command{
		Name:    "report pr",
		Pillar:  PillarGate,
		Tier:    Tier1,
		Aliases: []string{"pr"},
	}
	r.MustRegister(cmd)

	if got, ok := r.Get("report pr"); !ok || got.Name != "report pr" {
		t.Errorf("Get(name) = %v, ok=%v; want command, true", got, ok)
	}
	if got, ok := r.Get("pr"); !ok || got.Name != "report pr" {
		t.Errorf("Get(alias) = %v, ok=%v; want canonical command, true", got, ok)
	}
	if _, ok := r.Get("nonexistent"); ok {
		t.Error("Get(unknown) should return ok=false")
	}
}

func TestAll_DedupesAliases(t *testing.T) {
	t.Parallel()
	r := New()
	r.MustRegister(Command{Name: "analyze", Pillar: PillarUnderstand, Tier: Tier1})
	r.MustRegister(Command{
		Name:    "report pr",
		Pillar:  PillarGate,
		Tier:    Tier1,
		Aliases: []string{"pr"},
	})

	all := r.All()
	if len(all) != 2 {
		t.Errorf("All() = %d entries, want 2 (aliases dedup)", len(all))
		for _, c := range all {
			t.Logf("  %q", c.Name)
		}
	}
}

func TestAll_AlphabeticalOrder(t *testing.T) {
	t.Parallel()
	r := New()
	for _, name := range []string{"zoo", "alpha", "mango"} {
		r.MustRegister(Command{Name: name, Pillar: PillarMeta})
	}
	all := r.All()
	want := []string{"alpha", "mango", "zoo"}
	if len(all) != 3 {
		t.Fatalf("got %d, want 3", len(all))
	}
	for i, cmd := range all {
		if cmd.Name != want[i] {
			t.Errorf("All()[%d] = %q, want %q", i, cmd.Name, want[i])
		}
	}
}

func TestByPillar_GroupsCorrectly(t *testing.T) {
	t.Parallel()
	r := New()
	r.MustRegister(Command{Name: "analyze", Pillar: PillarUnderstand})
	r.MustRegister(Command{Name: "report pr", Pillar: PillarGate})
	r.MustRegister(Command{Name: "migrate run", Pillar: PillarAlign})
	r.MustRegister(Command{Name: "report posture", Pillar: PillarUnderstand})

	groups := r.ByPillar()
	if len(groups[PillarUnderstand]) != 2 {
		t.Errorf("understand pillar = %d, want 2", len(groups[PillarUnderstand]))
	}
	if len(groups[PillarGate]) != 1 {
		t.Errorf("gate pillar = %d, want 1", len(groups[PillarGate]))
	}
	if len(groups[PillarAlign]) != 1 {
		t.Errorf("align pillar = %d, want 1", len(groups[PillarAlign]))
	}
	if _, hasMeta := groups[PillarMeta]; hasMeta {
		t.Error("meta pillar should be omitted (no commands)")
	}
}

func TestNames(t *testing.T) {
	t.Parallel()
	r := New()
	r.MustRegister(Command{Name: "report pr", Pillar: PillarGate, Aliases: []string{"pr"}})
	r.MustRegister(Command{Name: "analyze", Pillar: PillarUnderstand})

	names := r.Names()
	if len(names) != 2 {
		t.Errorf("Names() = %d, want 2 (no aliases)", len(names))
	}
	// Check alphabetical order.
	if names[0] != "analyze" || names[1] != "report pr" {
		t.Errorf("Names() = %v, want [analyze, report pr]", names)
	}
}

// TestRegister_ConcurrentSafe exercises the Register / Get path
// from multiple goroutines so the -race detector can flag any
// mutex regression.
func TestRegister_ConcurrentSafe(t *testing.T) {
	t.Parallel()
	r := New()

	// Register from N goroutines.
	const n = 50
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		i := i
		go func() {
			defer wg.Done()
			cmd := Command{
				Name:   "cmd" + string(rune('a'+(i%26))) + string(rune('0'+(i/26))),
				Pillar: PillarMeta,
			}
			_ = r.Register(cmd) // duplicates may error, that's fine
		}()
	}
	// Read concurrently.
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			_ = r.All()
		}()
	}
	wg.Wait()
}

// TestMustRegister_PanicsOnError verifies the must-variant fails
// loudly on duplicate registration. Used in init() blocks where
// a duplicate is a developer-time bug.
func TestMustRegister_PanicsOnError(t *testing.T) {
	t.Parallel()
	r := New()
	r.MustRegister(Command{Name: "x", Pillar: PillarMeta})

	defer func() {
		if r := recover(); r == nil {
			t.Error("MustRegister should panic on duplicate")
		}
	}()
	r.MustRegister(Command{Name: "x", Pillar: PillarMeta})
}
