package portfolio

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// RepoManifest is the shape of `.terrain/repos.yaml` — the multi-repo
// declaration that lets Terrain aggregate over more than one
// repository. The manifest enumerates each repo Terrain should
// aggregate over, plus per-repo metadata that the cross-repo
// aggregator uses to compute portfolio-level posture.
//
// Status: stable in 0.3.0. The schema supports live repo paths and
// saved snapshot paths; aggregate JSON fields may still grow additively
// under the normal Terrain schema compatibility rules.
type RepoManifest struct {
	// Version is the manifest schema version. 0.3 ships v1; later
	// schema changes that aren't strictly additive will bump this.
	// A loader that finds an unrecognized version refuses to load
	// rather than guessing.
	Version int `yaml:"version" json:"version"`

	// Description is a free-form human-readable label for the
	// manifest (e.g. "Acme Corp engineering portfolio"). Optional;
	// surfaced in `terrain portfolio --from <manifest>` output.
	Description string `yaml:"description,omitempty" json:"description,omitempty"`

	// Repos is the list of repositories to aggregate over.
	Repos []RepoEntry `yaml:"repos" json:"repos"`
}

// RepoEntry is one repository's declaration inside the manifest.
//
// The fields fall into three buckets:
//   - Identity: name, path
//   - Pre-computed inputs: snapshotPath (so adopters who run
//     `terrain analyze` per-repo on their own schedule can hand the
//     aggregator a saved snapshot rather than forcing a re-walk)
//   - Optional metadata: owner, frameworksOfRecord, tags
type RepoEntry struct {
	// Name is the repo's canonical short name. Required; used as
	// the primary key in cross-repo aggregation. Should match the
	// directory basename for consistency but isn't required to.
	Name string `yaml:"name" json:"name"`

	// Path is the on-disk repo path relative to the manifest file.
	// Required when SnapshotPath is empty — the aggregator walks
	// the path to produce a fresh snapshot. When SnapshotPath is
	// set, Path is informational (used in messaging only).
	Path string `yaml:"path,omitempty" json:"path,omitempty"`

	// SnapshotPath, when set, points at a previously-written
	// snapshot JSON for this repo. The aggregator loads the
	// snapshot directly and skips the walk. This is the
	// recommended shape for large portfolios where re-walking
	// every repo for every aggregator run is wasteful.
	SnapshotPath string `yaml:"snapshotPath,omitempty" json:"snapshotPath,omitempty"`

	// Owner is the team or individual responsible for the repo.
	// Optional; surfaces in per-team posture aggregation.
	Owner string `yaml:"owner,omitempty" json:"owner,omitempty"`

	// FrameworksOfRecord is the canonical declaration of which
	// frameworks this repo officially uses. When set, the
	// aggregator's framework-drift detector compares actual
	// framework distribution against this declaration to flag
	// drift; when empty, drift detection skips this repo.
	FrameworksOfRecord []string `yaml:"frameworksOfRecord,omitempty" json:"frameworksOfRecord,omitempty"`

	// Tags is a free-form list of labels (e.g. ["tier-1",
	// "customer-facing"]). Surfaces in cross-repo views and
	// can be used as filter criteria.
	Tags []string `yaml:"tags,omitempty" json:"tags,omitempty"`
}

// LoadRepoManifest reads `path` (typically `.terrain/repos.yaml`),
// parses it, validates the result, and returns the manifest. Returns
// a wrapped error with the file path on parse / validation failures so
// `terrain portfolio --from <manifest>` users can see exactly which
// file is bad.
//
// Validation rules (enforced here, not in YAML schema):
//   - Version must be 1 (the only currently-supported version).
//   - Repos cannot be empty.
//   - Each RepoEntry must have a non-empty Name.
//   - Each RepoEntry must have either Path or SnapshotPath set.
//   - Names must be unique within a manifest.
func LoadRepoManifest(path string) (*RepoManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read repo manifest %q: %w", path, err)
	}
	return ParseRepoManifest(data, path)
}

// ParseRepoManifest is LoadRepoManifest's pure-bytes counterpart.
// `sourceLabel` is used in error messages so callers that load from
// non-file sources (test fixtures, embedded defaults) can still
// produce diagnosable errors.
func ParseRepoManifest(data []byte, sourceLabel string) (*RepoManifest, error) {
	var m RepoManifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse repo manifest %q: %w", sourceLabel, err)
	}
	if err := validateRepoManifest(&m); err != nil {
		return nil, fmt.Errorf("validate repo manifest %q: %w", sourceLabel, err)
	}
	return &m, nil
}

// supportedManifestVersion is the current schema version. Bumping
// this is a breaking change; only do it when the YAML shape changes
// in a non-additive way.
const supportedManifestVersion = 1

func validateRepoManifest(m *RepoManifest) error {
	if m == nil {
		return errors.New("manifest is nil")
	}
	m.Description = strings.TrimSpace(m.Description)
	if m.Version == 0 {
		return errors.New("manifest 'version' field is required (use 'version: 1' for 0.3)")
	}
	if m.Version != supportedManifestVersion {
		return fmt.Errorf("unsupported manifest version %d (this build supports version %d)",
			m.Version, supportedManifestVersion)
	}
	if len(m.Repos) == 0 {
		return errors.New("manifest 'repos' is empty — declare at least one repo")
	}

	seenNames := map[string]int{}
	for i, repo := range m.Repos {
		idx := i + 1
		repo.Name = strings.TrimSpace(repo.Name)
		repo.Path = strings.TrimSpace(repo.Path)
		repo.SnapshotPath = strings.TrimSpace(repo.SnapshotPath)
		repo.Owner = strings.TrimSpace(repo.Owner)
		repo.FrameworksOfRecord = normalizeManifestList(repo.FrameworksOfRecord, true)
		repo.Tags = normalizeManifestList(repo.Tags, false)
		m.Repos[i] = repo

		if repo.Name == "" {
			return fmt.Errorf("repo #%d: 'name' is required", idx)
		}
		if !isSafeRepoName(repo.Name) {
			return fmt.Errorf("repo #%d: name %q must be a safe path segment (no slashes, backslashes, '.' or '..')", idx, repo.Name)
		}
		if dup, ok := seenNames[repo.Name]; ok {
			return fmt.Errorf("repo #%d: duplicate name %q (already used at #%d)",
				idx, repo.Name, dup)
		}
		seenNames[repo.Name] = idx

		if repo.Path == "" && repo.SnapshotPath == "" {
			return fmt.Errorf("repo %q: must set 'path' or 'snapshotPath'", repo.Name)
		}
	}
	return nil
}

func isSafeRepoName(name string) bool {
	if name == "" || name == "." || name == ".." {
		return false
	}
	return !strings.ContainsAny(name, `/\`)
}

func normalizeManifestList(values []string, lower bool) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if lower {
			value = strings.ToLower(value)
		}
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

// ResolveRepoPath resolves a RepoEntry's on-disk path or snapshot
// path against the manifest's containing directory. Used by the
// aggregator to convert manifest-relative paths into absolute paths
// before reading. Returns the empty string if neither is set
// (validation would have caught this earlier).
func ResolveRepoPath(manifestDir string, repo RepoEntry) string {
	target := repo.Path
	if target == "" {
		target = repo.SnapshotPath
	}
	if target == "" {
		return ""
	}
	if filepath.IsAbs(target) {
		return target
	}
	return filepath.Clean(filepath.Join(manifestDir, target))
}

// ResolveSnapshotPath resolves a RepoEntry's snapshot path
// specifically, returning the empty string if the entry has only
// Path set (i.e. the aggregator should walk rather than load).
func ResolveSnapshotPath(manifestDir string, repo RepoEntry) string {
	if repo.SnapshotPath == "" {
		return ""
	}
	if filepath.IsAbs(repo.SnapshotPath) {
		return repo.SnapshotPath
	}
	return filepath.Clean(filepath.Join(manifestDir, repo.SnapshotPath))
}
