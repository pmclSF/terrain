package manifest

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
)

// ParsePackageJSON parses a Node.js package.json. Surfaces dependencies,
// devDependencies, peerDependencies, and optionalDependencies as
// SectionRuntime / SectionDev / SectionOptional respectively.
//
// peerDependencies are classified as SectionOptional since they declare
// what the consumer must provide rather than what this package installs.
func ParsePackageJSON(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("package.json: open %s: %w", path, err)
	}

	var raw struct {
		Dependencies         map[string]string `json:"dependencies"`
		DevDependencies      map[string]string `json:"devDependencies"`
		PeerDependencies     map[string]string `json:"peerDependencies"`
		OptionalDependencies map[string]string `json:"optionalDependencies"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("package.json: parse %s: %w", path, err)
	}

	m := &Manifest{
		Path:      path,
		Ecosystem: EcosystemNode,
		Format:    "package.json",
	}

	addNodeDeps(m, raw.Dependencies, SectionRuntime)
	addNodeDeps(m, raw.DevDependencies, SectionDev)
	addNodeDeps(m, raw.PeerDependencies, SectionOptional)
	addNodeDeps(m, raw.OptionalDependencies, SectionOptional)

	return m, nil
}

func addNodeDeps(m *Manifest, deps map[string]string, section Section) {
	// Sort names for determinism.
	names := make([]string, 0, len(deps))
	for n := range deps {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, name := range names {
		spec := deps[name]
		m.Dependencies = append(m.Dependencies, Dependency{
			Name:    name,
			Spec:    spec,
			Pinning: classifyNodeSpec(spec),
			Section: section,
		})
	}
}

// classifyNodeSpec interprets npm/yarn version specifiers.
// Reference: https://docs.npmjs.com/cli/v10/configuring-npm/package-json#dependencies
func classifyNodeSpec(spec string) Pinning {
	s := strings.TrimSpace(spec)
	switch {
	case s == "" || s == "*" || s == "latest":
		return PinningUnpinned
	case strings.HasPrefix(s, "^") || strings.HasPrefix(s, "~"):
		return PinningRange
	case strings.HasPrefix(s, ">=") || strings.HasPrefix(s, ">") ||
		strings.HasPrefix(s, "<=") || strings.HasPrefix(s, "<") ||
		strings.Contains(s, " - ") || strings.Contains(s, "||"):
		return PinningRange
	case strings.HasPrefix(s, "git+") || strings.HasPrefix(s, "git://") ||
		strings.HasPrefix(s, "git@") || strings.HasPrefix(s, "github:") ||
		strings.HasPrefix(s, "gitlab:") || strings.HasPrefix(s, "bitbucket:"):
		return PinningGit
	case strings.HasPrefix(s, "https://") || strings.HasPrefix(s, "http://"):
		return PinningURL
	case strings.HasPrefix(s, "file:") || strings.HasPrefix(s, "./") ||
		strings.HasPrefix(s, "../") || strings.HasPrefix(s, "/"):
		return PinningPath
	case strings.HasPrefix(s, "npm:"):
		// Alias to another package; classify the aliased spec.
		// Format: "npm:package@spec" — extract trailing spec after final `@`.
		if i := strings.LastIndex(s, "@"); i > len("npm:") {
			return classifyNodeSpec(s[i+1:])
		}
		return PinningUnknown
	case strings.HasPrefix(s, "workspace:"):
		// Workspace protocol (yarn/pnpm): reproducible within the monorepo.
		return PinningPath
	default:
		// Bare semver like "1.2.3" — exact in package.json semantics.
		// (npm interprets "1.2.3" as ==1.2.3, not ^1.2.3.)
		if len(s) > 0 && (s[0] >= '0' && s[0] <= '9') {
			if strings.ContainsAny(s, "x*") || strings.Contains(s, "||") {
				return PinningRange
			}
			return PinningExact
		}
		return PinningUnknown
	}
}
