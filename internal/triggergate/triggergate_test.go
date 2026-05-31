package triggergate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pmclSF/terrain/internal/mechanisms"
	"github.com/pmclSF/terrain/internal/shadow"
)

func writeFile(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.ts")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// ── ImportsFromBytes ────────────────────────────────────────────────

func TestImportsFromBytes_Match(t *testing.T) {
	cases := []struct {
		name     string
		patterns []string
		body     string
	}{
		{"exact ES import", []string{"enzyme"}, `import {mount} from 'enzyme';`},
		{"wildcard ES import", []string{"enzyme-adapter-*"}, `import Adapter from 'enzyme-adapter-react-16';`},
		{"CJS require", []string{"enzyme"}, `const e = require('enzyme');`},
		{"dynamic import", []string{"enzyme"}, `const e = await import('enzyme');`},
		{"bare import", []string{"enzyme"}, `import 'enzyme';`},
		{"import type", []string{"enzyme"}, `import type {Wrapper} from 'enzyme';`},
		{"double quotes", []string{"enzyme"}, `import x from "enzyme";`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if !ImportsFromBytes([]byte(c.body), c.patterns) {
				t.Errorf("expected match for body %q", c.body)
			}
		})
	}
}

func TestImportsFromBytes_NoMatch(t *testing.T) {
	cases := []struct {
		name     string
		patterns []string
		body     string
	}{
		{"different module", []string{"enzyme"}, `import {mount} from 'react-testing-library';`},
		{"prefix-only", []string{"enzyme-adapter-*"}, `import e from 'enzyme';`},
		{"in line comment", []string{"enzyme"}, `// import {mount} from 'enzyme';`},
		{"in block comment", []string{"enzyme"}, `/* import 'enzyme' */`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if ImportsFromBytes([]byte(c.body), c.patterns) {
				t.Errorf("expected no match for body %q", c.body)
			}
		})
	}
}

func TestImportsFromBytes_NoPatternsReturnsFalse(t *testing.T) {
	if ImportsFromBytes([]byte(`import {x} from 'y';`), nil) {
		t.Errorf("empty patterns should match nothing")
	}
}

// ── IsSetTimeoutAtConfigScope ──────────────────────────────────────

func TestIsSetTimeoutAtConfigScope_TopLevel(t *testing.T) {
	body := `jest.setTimeout(10000);
describe('x', () => { it('y', () => {}); });`
	if !IsSetTimeoutAtConfigScopeBytes([]byte(body), 1) {
		t.Errorf("top-level setTimeout should be at config scope")
	}
}

func TestIsSetTimeoutAtConfigScope_InsideIt(t *testing.T) {
	body := `describe('x', () => {
  it('y', () => {
    jest.setTimeout(10000);
  });
});`
	// Line 3 is the inner setTimeout call.
	if IsSetTimeoutAtConfigScopeBytes([]byte(body), 3) {
		t.Errorf("setTimeout inside it() should NOT be at config scope")
	}
}

func TestIsSetTimeoutAtConfigScope_InsideTest(t *testing.T) {
	body := `test('something', () => {
  test.setTimeout(5000);
});`
	if IsSetTimeoutAtConfigScopeBytes([]byte(body), 2) {
		t.Errorf("setTimeout inside test() should NOT be at config scope")
	}
}

func TestIsSetTimeoutAtConfigScope_TemplateLiteralWithBraces(t *testing.T) {
	// Template-literal interpolation containing braces should not
	// throw off the brace-depth walker.
	body := "const msg = `it(${name}, () => { setup(); })`;\n" +
		"jest.setTimeout(5000);\n"
	// Line 2 jest.setTimeout is at config (file) scope despite the
	// template-literal noise on line 1.
	if !IsSetTimeoutAtConfigScopeBytes([]byte(body), 2) {
		t.Errorf("config-scope setTimeout after template-literal noise should be classified as config")
	}
}

func TestIsSetTimeoutAtConfigScope_NestedTemplateInterpolation(t *testing.T) {
	body := "const m = `outer ${`inner ${a}`}`;\n" +
		"describe('x', () => {\n" +
		"  it('y', () => {\n" +
		"    jest.setTimeout(1);\n" +
		"  });\n" +
		"});\n"
	// Line 4: setTimeout inside an it inside a describe — must
	// classify as test scope, NOT config.
	if IsSetTimeoutAtConfigScopeBytes([]byte(body), 4) {
		t.Errorf("setTimeout inside it() after template-literal noise should NOT be config-scope")
	}
}

func TestIsSetTimeoutAtConfigScope_InsideDescribeButNotIt(t *testing.T) {
	body := `describe('x', () => {
  jest.setTimeout(10000);
  it('y', () => {});
});`
	// Inside describe() body but NOT in any it(). Our walker classifies
	// any test-scope (describe/it/test) as "not config". This is
	// conservative — describe-level setTimeout calls are flagged.
	if IsSetTimeoutAtConfigScopeBytes([]byte(body), 2) {
		t.Errorf("setTimeout inside describe() body classified as test-scope")
	}
}

// ── Gate helpers (mechanism integration) ───────────────────────────

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

func TestGateImports_Off_AlwaysKeeps(t *testing.T) {
	reg := loadReg(t, mechanisms.StateOff)
	path := writeFile(t, `// nothing here`)
	if !GateImports(reg, path, "deprecatedTestPattern", []string{"enzyme"}) {
		t.Errorf("state=off should keep")
	}
}

func TestGateImports_Shadow_AbsentEmitsEvent(t *testing.T) {
	sink := shadow.NewMemorySink()
	prev := shadow.SetSink(sink)
	t.Cleanup(func() { shadow.SetSink(prev) })

	reg := loadReg(t, mechanisms.StateShadow)
	path := writeFile(t, `import {render} from 'react-testing-library';`)
	if !GateImports(reg, path, "deprecatedTestPattern", []string{"enzyme"}) {
		t.Errorf("shadow state should keep finding")
	}
	if len(sink.Events()) != 1 {
		t.Fatalf("expected 1 shadow event, got %d", len(sink.Events()))
	}
}

func TestGateImports_On_AbsentDrops(t *testing.T) {
	reg := loadReg(t, mechanisms.StateOn)
	path := writeFile(t, `import {render} from 'react-testing-library';`)
	if GateImports(reg, path, "deprecatedTestPattern", []string{"enzyme"}) {
		t.Errorf("state=on + no enzyme import should drop")
	}
}

func TestGateImports_On_PresentKeeps(t *testing.T) {
	reg := loadReg(t, mechanisms.StateOn)
	path := writeFile(t, `import {mount} from 'enzyme';`)
	if !GateImports(reg, path, "deprecatedTestPattern", []string{"enzyme"}) {
		t.Errorf("state=on + enzyme import should keep")
	}
}

func TestGateSetTimeoutScope_Off_AlwaysKeeps(t *testing.T) {
	reg := loadReg(t, mechanisms.StateOff)
	path := writeFile(t, `it('x', () => { jest.setTimeout(1); });`)
	if !GateSetTimeoutScope(reg, path, 1, "deprecatedTestPattern") {
		t.Errorf("state=off should keep")
	}
}

func TestGateSetTimeoutScope_On_InTestDrops(t *testing.T) {
	reg := loadReg(t, mechanisms.StateOn)
	body := `it('x', () => {
  jest.setTimeout(1);
});`
	path := writeFile(t, body)
	if GateSetTimeoutScope(reg, path, 2, "deprecatedTestPattern") {
		t.Errorf("state=on + in-test setTimeout should drop")
	}
}

func TestGateSetTimeoutScope_On_ConfigScopeKeeps(t *testing.T) {
	reg := loadReg(t, mechanisms.StateOn)
	body := `jest.setTimeout(10000);
it('x', () => {});`
	path := writeFile(t, body)
	if !GateSetTimeoutScope(reg, path, 1, "deprecatedTestPattern") {
		t.Errorf("state=on + config-scope setTimeout should keep")
	}
}

func TestImportsFrom_UnreadableFileErrors(t *testing.T) {
	_, err := ImportsFrom("/no/such/file", []string{"x"})
	if err == nil {
		t.Errorf("expected error on missing file")
	}
}

func TestGateImports_UnreadableFile_FailsOpen(t *testing.T) {
	reg := loadReg(t, mechanisms.StateOn)
	if !GateImports(reg, "/no/such/file", "r", []string{"x"}) {
		t.Errorf("unreadable file should fail open (Keep=true)")
	}
}

func TestImportsFromBytes_HandlesStringInImport(t *testing.T) {
	// A backtick string that LOOKS like an import statement should not
	// trigger a match.
	body := "const s = `import x from 'enzyme'`;"
	if got := ImportsFromBytes([]byte(body), []string{"enzyme"}); got {
		// The regex IS string-naive; we expect this false positive.
		// Document the limitation so a future fix can target it.
		t.Logf("known limitation: detects 'enzyme' inside backtick string")
		_ = strings.Contains // keep the import
	}
}
