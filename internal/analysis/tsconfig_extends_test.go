package analysis

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTSConfig_FollowsExtends(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	// Base config in a shared/ directory carries the path mapping.
	if err := os.MkdirAll(filepath.Join(root, "shared"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "shared", "tsconfig.base.json"), []byte(`{
  "compilerOptions": {
    "baseUrl": ".",
    "paths": {
      "@app/*": ["src/*"]
    }
  }
}`), 0o644); err != nil {
		t.Fatal(err)
	}

	// Leaf config extends the base. No paths of its own.
	if err := os.WriteFile(filepath.Join(root, "tsconfig.json"), []byte(`{
  "extends": "./shared/tsconfig.base.json",
  "compilerOptions": {}
}`), 0o644); err != nil {
		t.Fatal(err)
	}

	aliases := loadTSPathAliases(root)
	if len(aliases) == 0 {
		t.Fatalf("expected aliases from extended base config, got 0")
	}
	found := false
	for _, a := range aliases {
		if a.keyPrefix == "@app" && a.hasWildcard {
			found = true
		}
	}
	if !found {
		t.Errorf("expected @app/* alias, got %+v", aliases)
	}
}

func TestTSConfig_MultipleTargets(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "tsconfig.json"), []byte(`{
  "compilerOptions": {
    "baseUrl": ".",
    "paths": {
      "@util/*": ["src/util/*", "vendor/util/*"]
    }
  }
}`), 0o644); err != nil {
		t.Fatal(err)
	}

	aliases := loadTSPathAliases(root)

	// Should emit one alias per target.
	count := 0
	prefixes := []string{}
	for _, a := range aliases {
		if a.keyPrefix == "@util" {
			count++
			prefixes = append(prefixes, a.targetPrefix)
		}
	}
	if count != 2 {
		t.Errorf("expected 2 aliases for @util/*, got %d (prefixes=%v)", count, prefixes)
	}
}

func TestTSConfig_JSConfigFallback(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	// Only jsconfig.json present.
	if err := os.WriteFile(filepath.Join(root, "jsconfig.json"), []byte(`{
  "compilerOptions": {
    "baseUrl": ".",
    "paths": {
      "@feat/*": ["features/*"]
    }
  }
}`), 0o644); err != nil {
		t.Fatal(err)
	}

	aliases := loadTSPathAliases(root)
	if len(aliases) == 0 {
		t.Fatalf("expected aliases from jsconfig.json fallback, got 0")
	}
	found := false
	for _, a := range aliases {
		if a.keyPrefix == "@feat" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected @feat/* from jsconfig, got %+v", aliases)
	}
}

func TestTSConfig_ExtendsCycleTerminates(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	// Two configs that point at each other. Loader must not loop.
	if err := os.WriteFile(filepath.Join(root, "a.json"), []byte(`{
  "extends": "./b.json"
}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "b.json"), []byte(`{
  "extends": "./a.json"
}`), 0o644); err != nil {
		t.Fatal(err)
	}

	seen := map[string]bool{}
	// Pass through the file directly so we can call into the cycle.
	aliases := loadTSPathAliasesFromFile(root, filepath.Join(root, "a.json"), seen)
	// We expect no aliases, but more importantly no infinite loop.
	if len(aliases) != 0 {
		t.Errorf("expected 0 aliases from circular extends, got %d", len(aliases))
	}
	if len(seen) != 2 {
		t.Errorf("expected both files visited once, seen=%v", keysOf(seen))
	}
}

func keysOf(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, filepath.Base(k))
	}
	return out
}

func TestTSConfig_BaseURLRelativeToConfigDir(t *testing.T) {
	t.Parallel()

	// Base config in shared/ uses baseUrl "." — that should mean
	// "the shared/ directory", not the project root.
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "shared"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "shared", "tsconfig.base.json"), []byte(`{
  "compilerOptions": {
    "baseUrl": ".",
    "paths": {
      "@common/*": ["src/*"]
    }
  }
}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "tsconfig.json"), []byte(`{
  "extends": "./shared/tsconfig.base.json"
}`), 0o644); err != nil {
		t.Fatal(err)
	}

	aliases := loadTSPathAliases(root)
	for _, a := range aliases {
		if a.keyPrefix == "@common" {
			// Target prefix should resolve to shared/src, not just src.
			if !strings.HasPrefix(a.targetPrefix, "shared/") {
				t.Errorf("expected target relative to root with shared/ prefix, got %q", a.targetPrefix)
			}
		}
	}
}
