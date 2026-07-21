// Package dbtmanifest parses dbt's target/manifest.json artifact —
// the canonical inventory of a dbt project's models, tests, sources,
// and the dependency edges between them. Terrain consumes the
// manifest to surface dbt tests in the unified test graph alongside
// language-level tests (pytest, jest, go test) and to wire dbt
// models as SQL surfaces that data-pipeline rules can reason about.
//
// dbt regenerates the manifest on every `dbt compile` / `dbt build`,
// so the file may not exist in a fresh checkout — adapters should
// degrade gracefully (return an empty manifest, not an error).
package dbtmanifest

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/pmclSF/terrain/internal/saferead"
)

// Manifest is the normalized shape of dbt's target/manifest.json.
// Schema unstable across dbt versions; this projection keeps only
// the fields Terrain consumes today (path, depends_on, resource_type,
// test_metadata, columns names). Adding fields here means adopting
// new dbt versions or extending Terrain's dbt-aware rules.
type Manifest struct {
	// ProjectName is the dbt project's root name from dbt_project.yml.
	ProjectName string

	// DBTVersion is the version of dbt that produced this manifest.
	DBTVersion string

	// Models lists every model node in the manifest.
	Models []Model

	// Tests lists every test node in the manifest. dbt tests fall
	// into two families:
	//   - Generic tests (not_null, unique, accepted_values, relationships,
	//     plus custom generic tests) parameterized via schema.yml.
	//   - Singular tests, one-off SQL files under tests/.
	Tests []Test

	// Sources lists raw-data sources declared in schema files.
	Sources []Source
}

// Model is a dbt model — a SQL transformation that produces a table
// or view. Each model corresponds to one .sql file under models/.
type Model struct {
	UniqueID    string   // "model.project.name"
	Name        string   // bare name
	Path        string   // models/<dir>/<name>.sql (repo-relative)
	DependsOn   []string // unique IDs of upstream nodes
	Columns     []string // declared column names
	Description string
}

// Test is a dbt test node. CoveredModels lists the unique IDs of the
// models this test exercises (from depends_on).
type Test struct {
	UniqueID      string
	Name          string
	Path          string
	Kind          TestKind
	TestMetadata  TestMetadata // populated for generic tests only
	CoveredModels []string     // unique IDs from depends_on (model.* entries)
}

// TestKind classifies dbt tests.
type TestKind string

const (
	// TestKindGeneric is a parameterized test instance (not_null,
	// unique, accepted_values, relationships, or a custom generic
	// test) declared in a schema.yml's `tests:` block.
	TestKindGeneric TestKind = "generic"

	// TestKindSingular is a one-off SQL test under tests/, identified
	// by `test_metadata` being absent on the node.
	TestKindSingular TestKind = "singular"
)

// TestMetadata describes a generic test's parameterization.
type TestMetadata struct {
	Name   string                 // "not_null", "unique", custom name
	Kwargs map[string]interface{} // {"column_name": "id"} etc.
}

// Source is a raw-data input declared in a schema file (sources block).
type Source struct {
	UniqueID    string
	Name        string
	Identifier  string // database table name
	Schema      string // database schema
	Description string
}

// Load reads a dbt project's manifest.json from the conventional
// target/manifest.json path under root. Returns nil + nil error when
// the manifest doesn't exist (the project hasn't compiled), which is
// not a failure — callers should check the return.
func Load(root string) (*Manifest, error) {
	manifestPath := filepath.Join(root, "target", "manifest.json")
	// DataCap: a real dbt manifest.json is routinely multi-MB. saferead rejects
	// a manifest symlinked to a device or an oversize blob before the read; a
	// missing or unreadable manifest is not a failure (the project may not have
	// compiled), so both collapse to "no usable manifest".
	data, ok := saferead.File(manifestPath, saferead.DataCap)
	if !ok {
		return nil, nil
	}
	return Parse(data)
}

// Parse decodes a manifest.json byte slice into the normalized shape.
// Tolerant of unknown fields and schema evolution across dbt versions.
func Parse(data []byte) (*Manifest, error) {
	var raw struct {
		Metadata struct {
			DBTVersion  string `json:"dbt_version"`
			ProjectName string `json:"project_name"`
		} `json:"metadata"`
		Nodes   map[string]rawNode   `json:"nodes"`
		Sources map[string]rawSource `json:"sources"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("dbtmanifest: decode: %w", err)
	}

	m := &Manifest{
		ProjectName: raw.Metadata.ProjectName,
		DBTVersion:  raw.Metadata.DBTVersion,
	}

	for id, n := range raw.Nodes {
		switch n.ResourceType {
		case "model":
			model := Model{
				UniqueID:    id,
				Name:        n.Name,
				Path:        n.OriginalFilePath,
				Description: n.Description,
				DependsOn:   nodeIDs(n.DependsOn),
			}
			for col := range n.Columns {
				model.Columns = append(model.Columns, col)
			}
			m.Models = append(m.Models, model)
		case "test":
			t := Test{
				UniqueID: id,
				Name:     n.Name,
				Path:     n.OriginalFilePath,
				Kind:     classifyTestKind(n.TestMetadata),
			}
			if n.TestMetadata != nil {
				t.TestMetadata.Name = n.TestMetadata.Name
				t.TestMetadata.Kwargs = n.TestMetadata.Kwargs
			}
			for _, dep := range nodeIDs(n.DependsOn) {
				if strings.HasPrefix(dep, "model.") {
					t.CoveredModels = append(t.CoveredModels, dep)
				}
			}
			m.Tests = append(m.Tests, t)
		}
	}

	for id, s := range raw.Sources {
		m.Sources = append(m.Sources, Source{
			UniqueID:    id,
			Name:        s.Name,
			Identifier:  s.Identifier,
			Schema:      s.Schema,
			Description: s.Description,
		})
	}

	return m, nil
}

// rawNode mirrors the manifest.json node shape; we project only the
// fields Terrain uses.
type rawNode struct {
	Name             string                 `json:"name"`
	ResourceType     string                 `json:"resource_type"`
	OriginalFilePath string                 `json:"original_file_path"`
	Description      string                 `json:"description"`
	Columns          map[string]interface{} `json:"columns"`
	DependsOn        struct {
		Nodes []string `json:"nodes"`
	} `json:"depends_on"`
	TestMetadata *struct {
		Name   string                 `json:"name"`
		Kwargs map[string]interface{} `json:"kwargs"`
	} `json:"test_metadata"`
}

type rawSource struct {
	Name        string `json:"name"`
	Identifier  string `json:"identifier"`
	Schema      string `json:"schema"`
	Description string `json:"description"`
}

func nodeIDs(d struct {
	Nodes []string `json:"nodes"`
}) []string {
	return append([]string(nil), d.Nodes...)
}

func classifyTestKind(meta *struct {
	Name   string                 `json:"name"`
	Kwargs map[string]interface{} `json:"kwargs"`
}) TestKind {
	if meta != nil && meta.Name != "" {
		return TestKindGeneric
	}
	return TestKindSingular
}

// IsDBTProject returns true when root has a dbt_project.yml at its
// top level — the canonical marker for a dbt project root. Callers
// use this to decide whether to attempt manifest parsing at all.
func IsDBTProject(root string) bool {
	info, err := os.Stat(filepath.Join(root, "dbt_project.yml"))
	if err != nil {
		return false
	}
	return !info.IsDir()
}

// FindDBTProjects scans root for nested dbt projects (any directory
// containing a dbt_project.yml file) and returns the project root
// paths, repo-relative. Useful for monorepos with multiple dbt
// projects at different depths.
//
// Walks at most maxDepth directories deep to bound the cost.
func FindDBTProjects(root string, maxDepth int) ([]string, error) {
	var found []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if !d.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		depth := strings.Count(rel, string(os.PathSeparator))
		if rel != "." {
			depth++
		}
		if depth > maxDepth {
			return filepath.SkipDir
		}
		// Skip common vendored / generated dirs.
		base := d.Name()
		if base == "node_modules" || base == "vendor" || base == ".git" ||
			base == ".terrain" || base == "target" || base == "dbt_packages" {
			return filepath.SkipDir
		}
		if IsDBTProject(path) {
			found = append(found, rel)
		}
		return nil
	})
	return found, err
}
