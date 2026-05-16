package manifest

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParsePEP508_Basic(t *testing.T) {
	t.Parallel()
	tests := []struct {
		spec        string
		wantName    string
		wantSpec    string
		wantPinning Pinning
		wantExtras  []string
		wantMarker  string
	}{
		{"requests==2.31.0", "requests", "==2.31.0", PinningExact, nil, ""},
		{"requests>=2.0,<3.0", "requests", ">=2.0,<3.0", PinningRange, nil, ""},
		{"requests~=2.31", "requests", "~=2.31", PinningRange, nil, ""},
		{"requests", "requests", "", PinningUnpinned, nil, ""},
		{"requests===2.31.0", "requests", "===2.31.0", PinningExact, nil, ""},
		{"pkg[extra1,extra2]>=1.0", "pkg", ">=1.0", PinningRange, []string{"extra1", "extra2"}, ""},
		{"pkg; python_version >= '3.10'", "pkg", "", PinningUnpinned, nil, "python_version >= '3.10'"},
		{"pkg==1.0; sys_platform == 'linux'", "pkg", "==1.0", PinningExact, nil, "sys_platform == 'linux'"},
		{"pkg @ git+https://github.com/foo/bar.git@v1.0", "pkg", "@ git+https://github.com/foo/bar.git@v1.0", PinningGit, nil, ""},
		{"pkg @ https://example.com/pkg.tgz", "pkg", "@ https://example.com/pkg.tgz", PinningURL, nil, ""},
		{"# comment", "", "", "", nil, ""}, // not a valid name
	}

	for _, tt := range tests {
		t.Run(tt.spec, func(t *testing.T) {
			d, ok := parsePEP508(tt.spec)
			if tt.wantName == "" {
				if ok {
					t.Errorf("expected parse failure for %q", tt.spec)
				}
				return
			}
			if !ok {
				t.Fatalf("parsePEP508(%q) failed", tt.spec)
			}
			if d.Name != tt.wantName {
				t.Errorf("name = %q, want %q", d.Name, tt.wantName)
			}
			if d.Spec != tt.wantSpec {
				t.Errorf("spec = %q, want %q", d.Spec, tt.wantSpec)
			}
			if d.Pinning != tt.wantPinning {
				t.Errorf("pinning = %q, want %q", d.Pinning, tt.wantPinning)
			}
			if len(d.Extras) != len(tt.wantExtras) {
				t.Errorf("extras = %v, want %v", d.Extras, tt.wantExtras)
			}
			if d.Markers != tt.wantMarker {
				t.Errorf("markers = %q, want %q", d.Markers, tt.wantMarker)
			}
		})
	}
}

func TestParseRequirementsTxt(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	content := `# Production dependencies
requests==2.31.0
flask>=2.0,<3.0  # range
gunicorn

# Pip flags and includes are ignored:
-r dev-requirements.txt
-i https://pypi.org/simple
--extra-index-url https://example.com

# Editable install:
-e git+https://github.com/foo/bar.git@main#egg=bar

# VCS URL:
git+https://github.com/baz/qux.git@v2.0#egg=qux

# Direct URL:
https://example.com/something.tar.gz

# Empty / comment-only lines skipped.
`
	path := filepath.Join(dir, "requirements.txt")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	m, err := ParseRequirementsTxt(path)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if m.Ecosystem != EcosystemPython || m.Format != "requirements.txt" {
		t.Errorf("ecosystem/format wrong: %q/%q", m.Ecosystem, m.Format)
	}
	if len(m.Dependencies) != 6 {
		t.Fatalf("got %d deps, want 6: %+v", len(m.Dependencies), m.Dependencies)
	}

	checks := []struct {
		idx     int
		name    string
		pinning Pinning
	}{
		{0, "requests", PinningExact},
		{1, "flask", PinningRange},
		{2, "gunicorn", PinningUnpinned},
		{3, "bar", PinningGit},
		{4, "qux", PinningGit},
		{5, "something", PinningURL},
	}
	for _, c := range checks {
		got := m.Dependencies[c.idx]
		if got.Name != c.name {
			t.Errorf("dep[%d].name = %q, want %q", c.idx, got.Name, c.name)
		}
		if got.Pinning != c.pinning {
			t.Errorf("dep[%d].pinning = %q, want %q", c.idx, got.Pinning, c.pinning)
		}
	}
}

func TestParsePyProject_PEP621(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	content := `[build-system]
requires = ["setuptools>=68", "wheel"]

[project]
name = "demo"
version = "0.1.0"
dependencies = [
    "requests>=2.0",
    "click==8.1.7",
    "pyyaml",
]

[project.optional-dependencies]
dev = [
    "pytest>=7.0",
    "ruff",
]
docs = [
    "sphinx>=6.0",
]
`
	path := filepath.Join(dir, "pyproject.toml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	m, err := ParsePyProject(path)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	// Build name → dep map for deterministic lookup
	// (PEP-621 dependencies come back ordered, but optional-dependencies
	// keys iterate in arbitrary map order).
	byName := map[string]Dependency{}
	for _, d := range m.Dependencies {
		byName[d.Name] = d
	}

	checks := []struct {
		name    string
		pinning Pinning
		section Section
	}{
		{"setuptools", PinningRange, SectionBuild},
		{"wheel", PinningUnpinned, SectionBuild},
		{"requests", PinningRange, SectionRuntime},
		{"click", PinningExact, SectionRuntime},
		{"pyyaml", PinningUnpinned, SectionRuntime},
		{"pytest", PinningRange, SectionDev},
		{"ruff", PinningUnpinned, SectionDev},
		{"sphinx", PinningRange, SectionOptional},
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

func TestParsePyProject_Poetry(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	content := `[tool.poetry]
name = "demo"
version = "0.1.0"

[tool.poetry.dependencies]
python = "^3.10"
requests = "^2.31.0"
flask = "2.3.0"
pyyaml = "*"

[tool.poetry.dev-dependencies]
pytest = "~7.4"

[tool.poetry.group.test.dependencies]
hypothesis = ">=6.0"
`
	path := filepath.Join(dir, "pyproject.toml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	m, err := ParsePyProject(path)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	byName := map[string]Dependency{}
	for _, d := range m.Dependencies {
		byName[d.Name] = d
	}

	if _, ok := byName["python"]; ok {
		t.Error("python should be excluded from poetry deps")
	}

	checks := []struct {
		name    string
		pinning Pinning
		section Section
	}{
		{"requests", PinningRange, SectionRuntime},
		{"flask", PinningExact, SectionRuntime},
		{"pyyaml", PinningUnpinned, SectionRuntime},
		{"pytest", PinningRange, SectionDev},
		{"hypothesis", PinningRange, SectionDev},
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
			t.Errorf("%s.section = %q, want %q (spec=%q)", c.name, d.Section, c.section, d.Spec)
		}
	}
}
