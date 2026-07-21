package policy

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/pmclSF/terrain/internal/saferead"
)

// PolicyFileName is the expected policy file path relative to repo root.
const PolicyFileName = ".terrain/policy.yaml"

// LoadResult contains the outcome of attempting to load a policy file.
type LoadResult struct {
	// Config is the parsed policy config. Nil if no file was found.
	Config *Config

	// Path is the absolute path to the policy file, if found.
	Path string

	// Found indicates whether a policy file existed.
	Found bool
}

// Load reads .terrain/policy.yaml from the given repository root.
//
// Behavior:
//   - If the file does not exist, returns a LoadResult with Found=false and no error.
//   - If the file exists but is malformed, returns an actionable error.
//   - If the file is valid, returns the parsed Config.
func Load(repoRoot string) (*LoadResult, error) {
	path := filepath.Join(repoRoot, PolicyFileName)

	data, err := saferead.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &LoadResult{Found: false}, nil
		}
		return nil, fmt.Errorf("failed to read policy file %s: %w", path, err)
	}

	// NOTE: NOT a strict (KnownFields) decode. policy.yaml accepts rule keys at
	// the ROOT as well as under `rules:` (via Config's custom unmarshaler), so
	// KnownFields would reject valid root-level-key policy files. Catching a
	// typo'd rule key here needs schema-aware unknown-key detection, not
	// struct-field strictness.
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("malformed policy file %s: %w", path, err)
	}

	return &LoadResult{
		Config: &cfg,
		Path:   path,
		Found:  true,
	}, nil
}
