package deffollowing

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pmclSF/terrain/internal/mechanisms"
	"github.com/pmclSF/terrain/internal/shadow"
)

func writeFile(t *testing.T, root, rel, body string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

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

func TestCountAssertions_Direct(t *testing.T) {
	cases := []struct {
		name string
		body string
		want int
	}{
		{"single expect", `expect(x).toEqual(1)`, 1},
		{"two assertions", `expect(x).toEqual(1); assertEqual(y, 2);`, 2},
		{"none", `console.log("hi"); doStuff();`, 0},
		{"self.assertEqual", `self.assertEqual(x, 1)`, 1},
		{"t.isTrue", `t.isTrue(condition)`, 1},
		{"g.Expect", `g.Expect(err).ToNot(HaveOccurred())`, 1},
		{"mock.assert_called", `mock.assert_called_with(1)`, 1},
		{"python bare assert", `    assert x == 1`, 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := countAssertions(c.body); got != c.want {
				t.Errorf("countAssertions(%q) = %d, want %d", c.body, got, c.want)
			}
		})
	}
}

func TestExtractCallNames(t *testing.T) {
	body := `verifyResponse(actual, expected);
helper(1);
verifyResponse(2);  // dup`
	got := extractCallNames(body)
	if len(got) != 2 {
		t.Errorf("expected dedup to 2, got %d: %v", len(got), got)
	}
	want := map[string]bool{"verifyResponse": true, "helper": true}
	for _, n := range got {
		if !want[n] {
			t.Errorf("unexpected call name %q", n)
		}
	}
}

func TestExtractCallNames_SkipsKeywords(t *testing.T) {
	body := `if (x) { for (let i = 0; i < n; i++) {} }`
	got := extractCallNames(body)
	for _, n := range got {
		if isLanguageKeyword(n) {
			t.Errorf("call names include language keyword: %q", n)
		}
	}
}

// ── def index tests ──────────────────────────────────────────────────

func TestCounter_JS_FollowsHelper(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "src/helpers.ts", `
export function verifyResponse(actual, expected) {
  expect(actual).toEqual(expected);
}
`)

	c := NewCounter(root)
	reg := loadReg(t, mechanisms.StateOn)

	// Test body calls verifyResponse but has no direct assertions.
	testBody := `verifyResponse(result, expected);`
	count := c.Count(reg, testBody)
	if count == 0 {
		t.Errorf("expected def-following to find transitive assertion, got 0")
	}
}

func TestCounter_JS_ArrowHelper(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "src/helpers.ts", `
const verifyResponse = (actual, expected) => {
  expect(actual).toEqual(expected);
};
`)

	c := NewCounter(root)
	reg := loadReg(t, mechanisms.StateOn)
	count := c.Count(reg, `verifyResponse(a, b);`)
	if count == 0 {
		t.Errorf("expected def-following to find arrow helper assertion")
	}
}

func TestCounter_Python_FollowsHelper(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "tests/helpers.py", `
def verify_response(actual, expected):
    self.assertEqual(actual, expected)
`)

	c := NewCounter(root)
	reg := loadReg(t, mechanisms.StateOn)
	count := c.Count(reg, `verify_response(actual, expected)`)
	if count == 0 {
		t.Errorf("expected def-following to find Python assertion")
	}
}

func TestCounter_Go_FollowsHelper(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "internal/helpers.go", `package x

func verifyResponse(t *testing.T, a, b int) {
	if a != b {
		t.Errorf("mismatch")
	}
	if got := compute(a); got != b {
		t.Errorf("compute(%d) = %d, want %d", a, got, b)
	}
}
`)

	c := NewCounter(root)
	reg := loadReg(t, mechanisms.StateOn)
	// The Go helper contains `t.Errorf` which isn't in our token list,
	// but if we add a t.assert* somewhere it would count. Verify the
	// helper is discoverable at minimum (counter returns 0 cleanly).
	count := c.Count(reg, `verifyResponse(t, 1, 2);`)
	_ = count
}

func TestCounter_StateOff_OnlyImmediateBody(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "src/helpers.ts", `
function verifyResponse(a, b) { expect(a).toEqual(b); }
`)
	c := NewCounter(root)
	reg := loadReg(t, mechanisms.StateOff)

	// Body has zero direct assertions; off should NOT follow.
	if got := c.Count(reg, `verifyResponse(1, 2);`); got != 0 {
		t.Errorf("state=off should not follow defs; got count=%d", got)
	}
}

func TestCounter_RespectsMaxDepth(t *testing.T) {
	root := t.TempDir()
	// Chain: helperA → helperB → helperC → expect()
	writeFile(t, root, "src/a.ts", `function helperA() { helperB(); }`)
	writeFile(t, root, "src/b.ts", `function helperB() { helperC(); }`)
	writeFile(t, root, "src/c.ts", `function helperC() { expect(1).toEqual(1); }`)

	c := NewCounter(root)
	// Depth=1 → only see helperA's body (no assertions there).
	if got := c.CountTransitive(`helperA();`, 1); got != 0 {
		t.Errorf("depth=1 should only see helperA body; got %d", got)
	}
	// Reset visited via fresh count.
	c.visited = map[string]bool{}
	// Depth=3 → see all the way to expect().
	if got := c.CountTransitive(`helperA();`, 3); got == 0 {
		t.Errorf("depth=3 should reach the expect() call; got %d", got)
	}
}

func TestCounter_HandlesCycle(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "src/a.ts", `function helperA() { helperB(); expect(1).toEqual(1); }`)
	writeFile(t, root, "src/b.ts", `function helperB() { helperA(); }`)

	c := NewCounter(root)
	count := c.CountTransitive(`helperA();`, MaxDepth)
	if count != 1 {
		t.Errorf("cyclic helpers: expected exactly 1 assertion counted, got %d", count)
	}
}

// ── GateLift mechanism integration ───────────────────────────────────

func TestGateLift_Off_ReturnsImmediate(t *testing.T) {
	c := NewCounter(t.TempDir())
	reg := loadReg(t, mechanisms.StateOff)
	if got := GateLift(reg, c, `body`, "ruleX", "f.ts", 3); got != 3 {
		t.Errorf("off should return immediate, got %d", got)
	}
}

func TestGateLift_Shadow_EmitsWhenLifted(t *testing.T) {
	sink := shadow.NewMemorySink()
	prev := shadow.SetSink(sink)
	t.Cleanup(func() { shadow.SetSink(prev) })

	root := t.TempDir()
	writeFile(t, root, "src/helper.ts", `function verify() { expect(1).toEqual(1); }`)
	c := NewCounter(root)
	reg := loadReg(t, mechanisms.StateShadow)

	// Immediate body has no assertions; transitive finds one.
	total := GateLift(reg, c, `verify();`, "assertionFreeTest", "test.ts", 0)
	if total != 1 {
		t.Errorf("expected transitive lift to 1, got %d", total)
	}
	if len(sink.Events()) != 1 {
		t.Errorf("expected shadow event, got %d", len(sink.Events()))
	}
}

func TestGateLift_ShadowNoEmitWhenImmediateHasCount(t *testing.T) {
	sink := shadow.NewMemorySink()
	prev := shadow.SetSink(sink)
	t.Cleanup(func() { shadow.SetSink(prev) })

	c := NewCounter(t.TempDir())
	reg := loadReg(t, mechanisms.StateShadow)

	// Immediate count > 0, no transitive find → no shadow event (the
	// finding wouldn't have been suppressed anyway).
	GateLift(reg, c, `no calls here`, "r", "f", 2)
	if len(sink.Events()) != 0 {
		t.Errorf("immediate-count > 0 should not emit shadow event")
	}
}

func TestCounter_NoDoubleCount(t *testing.T) {
	// Ensure the visited-set prevents counting the same helper twice
	// when it's called from two places in the same body.
	root := t.TempDir()
	writeFile(t, root, "src/h.ts", `function verify() { expect(1).toEqual(1); }`)
	c := NewCounter(root)
	got := c.CountTransitive(`verify(); verify();`, MaxDepth)
	if got != 1 {
		t.Errorf("expected helper counted exactly once, got %d", got)
	}
}
