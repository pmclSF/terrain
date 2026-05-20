package looppredicate

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pmclSF/terrain/internal/mechanisms"
	"github.com/pmclSF/terrain/internal/shadow"
)

func writeFile(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "f.ts")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// ── IsTestBuilderInLoop ─────────────────────────────────────────────

func TestIsTestBuilderInLoop_ForLoop(t *testing.T) {
	body := `for (const c of cases) {
  it(c.name, () => {});
}`
	if !IsTestBuilderInLoopBytes([]byte(body), 2) {
		t.Errorf("for-of generated tests should be classified as in-loop")
	}
}

func TestIsTestBuilderInLoop_ForEach(t *testing.T) {
	body := `cases.forEach(c => {
  it(c.name, () => {});
});`
	if !IsTestBuilderInLoopBytes([]byte(body), 2) {
		t.Errorf("forEach-generated tests should be classified as in-loop")
	}
}

func TestIsTestBuilderInLoop_ItEach(t *testing.T) {
	body := `it.each([
  ['a', 1],
  ['b', 2],
])('case %s', (n, v) => {
  expect(v).toEqual(v);
});`
	// Even with .each on the outside, the inner expect line should be
	// inside the loop scope opened by `it.each(...)`.
	if !IsTestBuilderInLoopBytes([]byte(body), 5) {
		t.Errorf("it.each body should be classified as in-loop")
	}
}

func TestIsTestBuilderInLoop_WhileLoop(t *testing.T) {
	body := `let i = 0;
while (i < cases.length) {
  it(cases[i].name, () => {});
  i++;
}`
	if !IsTestBuilderInLoopBytes([]byte(body), 3) {
		t.Errorf("while-loop test should be classified as in-loop")
	}
}

func TestIsTestBuilderInLoop_NotInLoop(t *testing.T) {
	body := `describe('a', () => {
  it('case 1', () => {});
});`
	if IsTestBuilderInLoopBytes([]byte(body), 2) {
		t.Errorf("plain describe/it should NOT be classified as in-loop")
	}
}

func TestIsTestBuilderInLoop_LoopClosesBeforeLine(t *testing.T) {
	body := `for (const c of cases) {
  prep(c);
}
it('not in loop', () => {});`
	// Line 4 is OUTSIDE the loop body — the loop closed at line 3.
	if IsTestBuilderInLoopBytes([]byte(body), 4) {
		t.Errorf("line after the loop close should NOT be in-loop")
	}
}

func TestIsTestBuilderInLoop_LoopKeywordInString(t *testing.T) {
	body := `const label = "for each thing";
it('plain', () => {});`
	if IsTestBuilderInLoopBytes([]byte(body), 2) {
		t.Errorf("loop keyword inside string should not open a loop scope")
	}
}

func TestIsTestBuilderInLoop_LoopKeywordInComment(t *testing.T) {
	body := `// for each case
it('plain', () => {});`
	if IsTestBuilderInLoopBytes([]byte(body), 2) {
		t.Errorf("loop keyword inside line comment should not open a loop scope")
	}
}

func TestIsTestBuilderInLoop_NestedLoopAndFunction(t *testing.T) {
	body := `function setupCases() {
  return [1, 2, 3];
}
setupCases().forEach(c => {
  it('case ' + c, () => {});
});`
	// Line 5 is inside forEach but not inside setupCases.
	if !IsTestBuilderInLoopBytes([]byte(body), 5) {
		t.Errorf("forEach-wrapped it should be in-loop")
	}
}

func TestIsTestBuilderInLoop_MapWithIt(t *testing.T) {
	body := `cases.map(c => {
  return it(c.name, () => {});
});`
	if !IsTestBuilderInLoopBytes([]byte(body), 2) {
		t.Errorf("map-wrapped it should be in-loop")
	}
}

func TestIsTestBuilderInLoop_TargetLinePastEnd(t *testing.T) {
	body := `it('one', () => {});`
	// Line 99 doesn't exist — should default to false.
	if IsTestBuilderInLoopBytes([]byte(body), 99) {
		t.Errorf("target line past file end should default to false")
	}
}

// ── Gate ────────────────────────────────────────────────────────────

func loadReg(t *testing.T, state mechanisms.State) *mechanisms.Registry {
	t.Helper()
	reg, err := mechanisms.Load()
	if err != nil {
		t.Fatal(err)
	}
	if err := reg.Override(MechanismName, state); err != nil {
		t.Fatal(err)
	}
	return reg
}

func TestGate_Off_AlwaysKeeps(t *testing.T) {
	reg := loadReg(t, mechanisms.StateOff)
	path := writeFile(t, `for (const c of cases) { it(c.name, () => {}); }`)
	if !Gate(reg, path, 1, "dynamicTestGeneration") {
		t.Errorf("state=off should always keep")
	}
}

func TestGate_On_InLoopDrops(t *testing.T) {
	reg := loadReg(t, mechanisms.StateOn)
	path := writeFile(t, `for (const c of cases) {
  it(c.name, () => {});
}`)
	if Gate(reg, path, 2, "dynamicTestGeneration") {
		t.Errorf("state=on + in-loop should drop")
	}
}

func TestGate_On_NotInLoopKeeps(t *testing.T) {
	reg := loadReg(t, mechanisms.StateOn)
	path := writeFile(t, `describe('a', () => {
  it('x', () => {});
});`)
	if !Gate(reg, path, 2, "dynamicTestGeneration") {
		t.Errorf("state=on + not-in-loop should keep")
	}
}

func TestGate_Shadow_InLoopEmitsEvent(t *testing.T) {
	sink := shadow.NewMemorySink()
	prev := shadow.SetSink(sink)
	t.Cleanup(func() { shadow.SetSink(prev) })

	reg := loadReg(t, mechanisms.StateShadow)
	path := writeFile(t, `cases.forEach(c => {
  it(c.name, () => {});
});`)
	if !Gate(reg, path, 2, "dynamicTestGeneration") {
		t.Errorf("shadow should keep")
	}
	if len(sink.Events()) != 1 {
		t.Errorf("expected 1 shadow event, got %d", len(sink.Events()))
	}
}

func TestSourceShapes_NonEmpty(t *testing.T) {
	shapes := SourceShapes()
	if len(shapes) < 5 {
		t.Errorf("expected ≥5 source shapes, got %d", len(shapes))
	}
}

func TestGate_UnreadableFileFailsOpen(t *testing.T) {
	reg := loadReg(t, mechanisms.StateOn)
	if !Gate(reg, "/no/such/file", 1, "r") {
		t.Errorf("unreadable file should fail open (Keep=true)")
	}
}
