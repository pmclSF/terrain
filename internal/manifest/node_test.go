package manifest

import (
	"os"
	"path/filepath"
	"testing"
)

func TestClassifyNodeSpec(t *testing.T) {
	t.Parallel()
	tests := []struct {
		spec string
		want Pinning
	}{
		{"1.2.3", PinningExact},
		{"^1.2.3", PinningRange},
		{"~1.2.3", PinningRange},
		{">=1.2.3", PinningRange},
		{">=1.2.3 <2.0.0", PinningRange},
		{"1.x", PinningRange},
		{"*", PinningUnpinned},
		{"latest", PinningUnpinned},
		{"", PinningUnpinned},
		{"git+https://github.com/foo/bar.git", PinningGit},
		{"github:foo/bar", PinningGit},
		{"git+ssh://git@github.com:foo/bar.git", PinningGit},
		{"https://example.com/pkg.tgz", PinningURL},
		{"file:../local-pkg", PinningPath},
		{"./local-pkg", PinningPath},
		{"workspace:^1.0.0", PinningPath},
		{"npm:other-pkg@^1.2.3", PinningRange},
		{"npm:other-pkg@1.2.3", PinningExact},
	}
	for _, tt := range tests {
		t.Run(tt.spec, func(t *testing.T) {
			got := classifyNodeSpec(tt.spec)
			if got != tt.want {
				t.Errorf("classifyNodeSpec(%q) = %q, want %q", tt.spec, got, tt.want)
			}
		})
	}
}

func TestParsePackageJSON(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	content := `{
  "name": "demo",
  "version": "1.0.0",
  "dependencies": {
    "express": "^4.18.0",
    "lodash": "4.17.21",
    "axios": "*"
  },
  "devDependencies": {
    "jest": "~29.0.0",
    "@types/node": ">=18.0.0"
  },
  "peerDependencies": {
    "react": ">=17.0.0"
  },
  "optionalDependencies": {
    "fsevents": "^2.3.0"
  }
}`
	path := filepath.Join(dir, "package.json")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	m, err := ParsePackageJSON(path)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if m.Ecosystem != EcosystemNode {
		t.Errorf("ecosystem = %q, want node", m.Ecosystem)
	}

	byName := map[string]Dependency{}
	for _, d := range m.Dependencies {
		byName[d.Name] = d
	}

	checks := []struct {
		name    string
		pinning Pinning
		section Section
	}{
		{"express", PinningRange, SectionRuntime},
		{"lodash", PinningExact, SectionRuntime},
		{"axios", PinningUnpinned, SectionRuntime},
		{"jest", PinningRange, SectionDev},
		{"@types/node", PinningRange, SectionDev},
		{"react", PinningRange, SectionOptional},
		{"fsevents", PinningRange, SectionOptional},
	}
	for _, c := range checks {
		d, ok := byName[c.name]
		if !ok {
			t.Errorf("dep %q missing", c.name)
			continue
		}
		if d.Pinning != c.pinning {
			t.Errorf("%s.pinning = %q, want %q", c.name, d.Pinning, c.pinning)
		}
		if d.Section != c.section {
			t.Errorf("%s.section = %q, want %q", c.name, d.Section, c.section)
		}
	}
}
