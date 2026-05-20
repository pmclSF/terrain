package barrelresolver

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pmclSF/terrain/internal/mechanisms"
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

func TestResolve_StateOff_ReturnsNil(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "jest.config.json", `{"moduleNameMapper":{"^@/(.*)$":"<rootDir>/src/$1"}}`)
	writeFile(t, root, "src/foo.ts", "")
	r, _ := New(root)
	reg := loadReg(t, mechanisms.StateOff)
	if got := r.Resolve(reg, ".", "@/foo"); got != nil {
		t.Errorf("state=off should return nil, got %v", got)
	}
}

func TestResolve_JestJSON(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "jest.config.json", `{"moduleNameMapper":{"^@/(.*)$":"<rootDir>/src/$1"}}`)
	writeFile(t, root, "src/foo.ts", "export const x = 1;")

	r, _ := New(root)
	reg := loadReg(t, mechanisms.StateShadow)
	results := r.Resolve(reg, ".", "@/foo")
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d: %v", len(results), results)
	}
	if results[0].File != "src/foo.ts" {
		t.Errorf("File = %q, want src/foo.ts", results[0].File)
	}
	if results[0].SubClass != "jest-module-name-mapper" {
		t.Errorf("SubClass = %q", results[0].SubClass)
	}
}

func TestResolve_JestJS(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "jest.config.js", `module.exports = {
  moduleNameMapper: {
    '^@/(.*)$': '<rootDir>/src/$1',
  },
};`)
	writeFile(t, root, "src/util.ts", "export {};")

	r, _ := New(root)
	reg := loadReg(t, mechanisms.StateShadow)
	results := r.Resolve(reg, ".", "@/util")
	if len(results) != 1 || results[0].File != "src/util.ts" {
		t.Errorf("expected src/util.ts, got %v", results)
	}
}

func TestResolve_JestPackageJSON(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "package.json", `{
  "name": "thing",
  "jest": {
    "moduleNameMapper": {
      "^~/(.*)$": "<rootDir>/lib/$1"
    }
  }
}`)
	writeFile(t, root, "lib/helper.ts", "")

	r, _ := New(root)
	reg := loadReg(t, mechanisms.StateShadow)
	results := r.Resolve(reg, ".", "~/helper")
	if len(results) != 1 || results[0].File != "lib/helper.ts" {
		t.Errorf("expected lib/helper.ts, got %v", results)
	}
}

func TestResolve_DistPathIndirection(t *testing.T) {
	root := t.TempDir()
	// A package with main: dist/index.js, source: src/index.ts
	writeFile(t, root, "package.json", `{
  "name": "lib",
  "main": "dist/index.js",
  "source": "src/index.ts"
}`)
	writeFile(t, root, "src/index.ts", "export {};")

	r, _ := New(root)
	reg := loadReg(t, mechanisms.StateShadow)
	results := r.Resolve(reg, ".", "dist/index.js")
	if len(results) != 1 || results[0].File != "src/index.ts" {
		t.Errorf("dist→source mapping not applied; got %v", results)
	}
	if results[0].SubClass != "dist-path-indirection" {
		t.Errorf("SubClass = %q", results[0].SubClass)
	}
}

func TestResolve_PythonNamespaceReexport(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "pkg/__init__.py", "from .impl import Helper, Other as Aliased\n")
	writeFile(t, root, "pkg/impl.py", "class Helper:\n    pass\n")

	r, _ := New(root)
	reg := loadReg(t, mechanisms.StateShadow)
	results := r.Resolve(reg, ".", "pkg.Helper")
	if len(results) != 1 || results[0].File != "pkg/impl.py" {
		t.Errorf("python reexport not resolved; got %v", results)
	}
	if results[0].SubClass != "python-namespace-reexport" {
		t.Errorf("SubClass = %q", results[0].SubClass)
	}

	// Aliased import: pkg.Aliased should also resolve to pkg/impl.py
	aliased := r.Resolve(reg, ".", "pkg.Aliased")
	if len(aliased) != 1 || aliased[0].File != "pkg/impl.py" {
		t.Errorf("aliased reexport not resolved; got %v", aliased)
	}
}

func TestResolve_PythonNamespaceMissReturnsNil(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "pkg/__init__.py", "from .impl import Helper\n")
	writeFile(t, root, "pkg/impl.py", "class Helper:\n    pass\n")

	r, _ := New(root)
	reg := loadReg(t, mechanisms.StateShadow)
	// A name that isn't re-exported should not resolve.
	if got := r.Resolve(reg, ".", "pkg.NotAName"); got != nil {
		t.Errorf("unexported name should not resolve; got %v", got)
	}
}

func TestResolve_NoConfigsReturnsEmpty(t *testing.T) {
	root := t.TempDir()
	r, _ := New(root)
	reg := loadReg(t, mechanisms.StateShadow)
	if got := r.Resolve(reg, ".", "@/foo"); len(got) != 0 {
		t.Errorf("no configs → no results, got %v", got)
	}
}

func TestResolve_MalformedJestJSONIgnored(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "jest.config.json", `{ not valid json`)
	r, _ := New(root)
	reg := loadReg(t, mechanisms.StateShadow)
	if got := r.Resolve(reg, ".", "@/anything"); got != nil {
		t.Errorf("malformed config should be ignored, got %v", got)
	}
}

func TestNew_LoadsInputsLazily(t *testing.T) {
	root := t.TempDir()
	r, err := New(root)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	// Should have zero mappers + zero dist mappings, but not error out.
	if got := r.String(); got == "" {
		t.Errorf("expected non-empty String() output")
	}
}
