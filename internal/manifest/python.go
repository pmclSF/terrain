package manifest

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/BurntSushi/toml"
)

// ParsePyProject parses a Python pyproject.toml file, surfacing
// PEP-621 [project] dependencies + optional-dependencies and (when
// present) PEP-518 [build-system] requires. Poetry-style
// [tool.poetry.dependencies] tables are also recognized.
func ParsePyProject(path string) (*Manifest, error) {
	var raw struct {
		BuildSystem struct {
			Requires []string `toml:"requires"`
		} `toml:"build-system"`
		Project struct {
			Dependencies         []string            `toml:"dependencies"`
			OptionalDependencies map[string][]string `toml:"optional-dependencies"`
		} `toml:"project"`
		Tool struct {
			Poetry struct {
				Dependencies    map[string]toml.Primitive `toml:"dependencies"`
				DevDependencies map[string]toml.Primitive `toml:"dev-dependencies"`
				Group           map[string]struct {
					Dependencies map[string]toml.Primitive `toml:"dependencies"`
				} `toml:"group"`
			} `toml:"poetry"`
		} `toml:"tool"`
	}

	meta, err := toml.DecodeFile(path, &raw)
	if err != nil {
		return nil, fmt.Errorf("pyproject: decode %s: %w", path, err)
	}

	m := &Manifest{
		Path:      path,
		Ecosystem: EcosystemPython,
		Format:    "pyproject.toml",
	}

	for _, req := range raw.BuildSystem.Requires {
		if d, ok := parsePEP508(req); ok {
			d.Section = SectionBuild
			m.Dependencies = append(m.Dependencies, d)
		}
	}

	for _, req := range raw.Project.Dependencies {
		if d, ok := parsePEP508(req); ok {
			d.Section = SectionRuntime
			m.Dependencies = append(m.Dependencies, d)
		}
	}

	for group, reqs := range raw.Project.OptionalDependencies {
		section := SectionOptional
		if isDevGroupName(group) {
			section = SectionDev
		}
		for _, req := range reqs {
			if d, ok := parsePEP508(req); ok {
				d.Section = section
				m.Dependencies = append(m.Dependencies, d)
			}
		}
	}

	for name, prim := range raw.Tool.Poetry.Dependencies {
		if name == "python" {
			continue
		}
		spec := poetrySpecToString(meta, prim)
		d := Dependency{Name: name, Spec: spec, Pinning: classifyPoetrySpec(spec), Section: SectionRuntime}
		m.Dependencies = append(m.Dependencies, d)
	}
	for name, prim := range raw.Tool.Poetry.DevDependencies {
		spec := poetrySpecToString(meta, prim)
		d := Dependency{Name: name, Spec: spec, Pinning: classifyPoetrySpec(spec), Section: SectionDev}
		m.Dependencies = append(m.Dependencies, d)
	}
	for group, info := range raw.Tool.Poetry.Group {
		section := SectionOptional
		if isDevGroupName(group) {
			section = SectionDev
		}
		for name, prim := range info.Dependencies {
			spec := poetrySpecToString(meta, prim)
			d := Dependency{Name: name, Spec: spec, Pinning: classifyPoetrySpec(spec), Section: section}
			m.Dependencies = append(m.Dependencies, d)
		}
	}

	return m, nil
}

// ParseRequirementsTxt parses a Python requirements.txt or constraints.txt
// file. Recognizes the PEP-508 syntax, editable installs (-e), VCS URLs,
// direct URL references, and -r / -c includes (resolved as separate
// dependencies on the referenced file rather than recursing into it).
func ParseRequirementsTxt(path string) (*Manifest, error) {
	if fi, statErr := os.Lstat(path); statErr != nil || !fi.Mode().IsRegular() {
		return nil, fmt.Errorf("requirements: open %s: not a regular file", path)
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("requirements: open %s: %w", path, err)
	}
	defer f.Close()

	m := &Manifest{
		Path:      path,
		Ecosystem: EcosystemPython,
		Format:    "requirements.txt",
	}

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := scanner.Text()
		// Strip inline comments. Pip comments start with `# ` (hash + space)
		// or `#` at the start of a line; `#egg=`/`#sha256=` fragments inside
		// URLs use `#` without trailing space and must be preserved.
		line = stripInlineComment(line)
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Skip pip flags and includes — they don't declare deps directly.
		if strings.HasPrefix(line, "-r ") || strings.HasPrefix(line, "--requirement") ||
			strings.HasPrefix(line, "-c ") || strings.HasPrefix(line, "--constraint") ||
			strings.HasPrefix(line, "-i ") || strings.HasPrefix(line, "--index-url") ||
			strings.HasPrefix(line, "--extra-index-url") || strings.HasPrefix(line, "--find-links") ||
			strings.HasPrefix(line, "--no-index") || strings.HasPrefix(line, "--trusted-host") {
			continue
		}
		// Editable install or VCS URL.
		if strings.HasPrefix(line, "-e ") || strings.HasPrefix(line, "--editable ") {
			rest := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(line, "-e "), "--editable "))
			d := classifyURLOrPath(rest)
			d.Section = SectionRuntime
			d.Line = lineNo
			m.Dependencies = append(m.Dependencies, d)
			continue
		}
		if vcsSchemeRegexp.MatchString(line) || urlSchemeRegexp.MatchString(line) {
			d := classifyURLOrPath(line)
			d.Section = SectionRuntime
			d.Line = lineNo
			m.Dependencies = append(m.Dependencies, d)
			continue
		}
		// Standard PEP-508 spec.
		if d, ok := parsePEP508(line); ok {
			d.Section = SectionRuntime
			d.Line = lineNo
			m.Dependencies = append(m.Dependencies, d)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("requirements: scan %s: %w", path, err)
	}
	return m, nil
}

// PEP-508 reference (simplified):
//
//	name [extras] [version_specifier] [environment_marker]
//
// version_specifier := one of:
//
//	==1.2.3 | ===1.2.3 | >=1.0,<2.0 | ~=1.2.3 | !=1.2.3 | >1.0 | <2.0 | *
var (
	pep508NameRegexp   = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._\-]*`)
	pep508ExtrasRegexp = regexp.MustCompile(`^\[([^\]]+)\]`)
	pep508MarkerSplit  = regexp.MustCompile(`\s*;\s*`)
	vcsSchemeRegexp    = regexp.MustCompile(`^(git\+|hg\+|bzr\+|svn\+)`)
	urlSchemeRegexp    = regexp.MustCompile(`^https?://`)
)

func parsePEP508(spec string) (Dependency, bool) {
	s := strings.TrimSpace(spec)
	if s == "" {
		return Dependency{}, false
	}

	// Split off environment marker (after `;`).
	var marker string
	if parts := pep508MarkerSplit.Split(s, 2); len(parts) == 2 {
		s = strings.TrimSpace(parts[0])
		marker = strings.TrimSpace(parts[1])
	}

	// Name.
	nameMatch := pep508NameRegexp.FindString(s)
	if nameMatch == "" {
		return Dependency{}, false
	}
	d := Dependency{Name: nameMatch, Markers: marker}
	rest := strings.TrimSpace(s[len(nameMatch):])

	// Extras.
	if em := pep508ExtrasRegexp.FindStringSubmatch(rest); len(em) == 2 {
		for _, part := range strings.Split(em[1], ",") {
			if p := strings.TrimSpace(part); p != "" {
				d.Extras = append(d.Extras, p)
			}
		}
		rest = strings.TrimSpace(rest[len(em[0]):])
	}

	// Spec.
	d.Spec = strings.TrimSpace(rest)
	d.Pinning = classifyPEP508Spec(d.Spec)
	return d, true
}

func classifyPEP508Spec(spec string) Pinning {
	s := strings.TrimSpace(spec)
	if s == "" {
		return PinningUnpinned
	}
	if strings.HasPrefix(s, "@") {
		// PEP-508 direct URL/file reference (e.g., `foo @ https://example.com/foo.tgz`).
		rest := strings.TrimSpace(s[1:])
		return classifyURLOrPathSpec(rest)
	}
	if strings.HasPrefix(s, "==") || strings.HasPrefix(s, "===") {
		return PinningExact
	}
	if strings.ContainsAny(s, ">~<!*") {
		return PinningRange
	}
	return PinningUnknown
}

func classifyURLOrPathSpec(s string) Pinning {
	switch {
	case vcsSchemeRegexp.MatchString(s):
		return PinningGit
	case urlSchemeRegexp.MatchString(s):
		return PinningURL
	case strings.HasPrefix(s, "file:") || strings.HasPrefix(s, "."):
		return PinningPath
	}
	return PinningUnknown
}

func classifyURLOrPath(line string) Dependency {
	name := extractURLName(line)
	return Dependency{Name: name, Spec: line, Pinning: classifyURLOrPathSpec(line)}
}

// extractURLName produces a synthetic name for URL/VCS/path dependencies.
// Pip would resolve the actual package name from the URL contents at install
// time; we surface the URL fragment or basename so adopters can map findings
// back to the manifest line without re-resolving.
func extractURLName(line string) string {
	if i := strings.LastIndex(line, "#egg="); i >= 0 {
		return strings.SplitN(line[i+5:], "&", 2)[0]
	}
	if i := strings.LastIndex(line, "/"); i >= 0 {
		base := line[i+1:]
		base = strings.TrimSuffix(base, ".git")
		base = strings.TrimSuffix(base, ".tar.gz")
		base = strings.TrimSuffix(base, ".whl")
		base = strings.TrimSuffix(base, ".zip")
		if at := strings.Index(base, "@"); at >= 0 {
			base = base[:at]
		}
		return base
	}
	return line
}

// poetrySpecToString renders a Poetry dependency primitive back to its
// version-specifier string. Poetry supports two shapes:
//
//	foo = "^1.2.3"                          → simple string
//	foo = { version = "^1.2.3", optional = true, extras = ["x"] }
//
// The latter is materialized through toml.Primitive deferred decoding.
func poetrySpecToString(meta toml.MetaData, prim toml.Primitive) string {
	var asString string
	if err := meta.PrimitiveDecode(prim, &asString); err == nil {
		return asString
	}
	var asTable struct {
		Version string `toml:"version"`
		Git     string `toml:"git"`
		URL     string `toml:"url"`
		Path    string `toml:"path"`
	}
	if err := meta.PrimitiveDecode(prim, &asTable); err == nil {
		switch {
		case asTable.Version != "":
			return asTable.Version
		case asTable.Git != "":
			return "git+" + asTable.Git
		case asTable.URL != "":
			return asTable.URL
		case asTable.Path != "":
			return asTable.Path
		}
	}
	return ""
}

func classifyPoetrySpec(spec string) Pinning {
	s := strings.TrimSpace(spec)
	if s == "" {
		return PinningUnpinned
	}
	// Poetry caret and tilde are range constraints.
	if strings.HasPrefix(s, "^") || strings.HasPrefix(s, "~") {
		return PinningRange
	}
	if strings.HasPrefix(s, "git+") || vcsSchemeRegexp.MatchString(s) {
		return PinningGit
	}
	if urlSchemeRegexp.MatchString(s) {
		return PinningURL
	}
	if strings.HasPrefix(s, "/") || strings.HasPrefix(s, ".") {
		return PinningPath
	}
	// Plain "*" means latest.
	if s == "*" {
		return PinningUnpinned
	}
	// Bare version like "1.2.3" is exact in Poetry (no caret implied).
	if s != "" && (s[0] >= '0' && s[0] <= '9') {
		// Range if it contains operators.
		if strings.ContainsAny(s, ">~<!=*") {
			return PinningRange
		}
		return PinningExact
	}
	// Otherwise range (e.g., ">=1.0,<2.0").
	if strings.ContainsAny(s, ">~<!=*") {
		return PinningRange
	}
	return PinningUnknown
}

// stripInlineComment removes a trailing `# comment` while preserving
// URL fragments (`#egg=`, `#sha256=`, `#subdirectory=`, etc.) that pip
// uses as locator metadata in VCS / URL specs.
func stripInlineComment(line string) string {
	if i := strings.Index(line, "#"); i >= 0 {
		if i == 0 {
			return ""
		}
		prev := line[i-1]
		if prev == ' ' || prev == '\t' {
			return line[:i]
		}
	}
	return line
}

func isDevGroupName(group string) bool {
	g := strings.ToLower(group)
	return g == "dev" || g == "test" || g == "tests" || g == "testing" || g == "lint" || strings.HasSuffix(g, "-dev")
}
