package dbtmanifest

import (
	"os"
	"path/filepath"
	"testing"
)

const sampleManifest = `{
  "metadata": {
    "dbt_version": "1.8.0",
    "project_name": "analytics"
  },
  "nodes": {
    "model.analytics.users": {
      "name": "users",
      "resource_type": "model",
      "original_file_path": "models/marts/users.sql",
      "description": "Cleaned and deduplicated users.",
      "columns": {"id": {}, "email": {}, "created_at": {}},
      "depends_on": {"nodes": ["source.analytics.raw.users"]}
    },
    "model.analytics.user_orders": {
      "name": "user_orders",
      "resource_type": "model",
      "original_file_path": "models/marts/user_orders.sql",
      "depends_on": {"nodes": ["model.analytics.users", "model.analytics.orders"]}
    },
    "test.analytics.not_null_users_id.abc123": {
      "name": "not_null_users_id",
      "resource_type": "test",
      "original_file_path": "models/marts/schema.yml",
      "test_metadata": {"name": "not_null", "kwargs": {"column_name": "id"}},
      "depends_on": {"nodes": ["model.analytics.users"]}
    },
    "test.analytics.users_have_recent_data.def456": {
      "name": "users_have_recent_data",
      "resource_type": "test",
      "original_file_path": "tests/users_have_recent_data.sql",
      "depends_on": {"nodes": ["model.analytics.users"]}
    },
    "macro.analytics.helper": {
      "name": "helper",
      "resource_type": "macro"
    }
  },
  "sources": {
    "source.analytics.raw.users": {
      "name": "users",
      "identifier": "users_raw",
      "schema": "raw",
      "description": "Raw user events from upstream."
    }
  }
}`

func TestParse_Models(t *testing.T) {
	t.Parallel()
	m, err := Parse([]byte(sampleManifest))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if m.ProjectName != "analytics" {
		t.Errorf("ProjectName = %q", m.ProjectName)
	}
	if m.DBTVersion != "1.8.0" {
		t.Errorf("DBTVersion = %q", m.DBTVersion)
	}
	if len(m.Models) != 2 {
		t.Fatalf("Models = %d, want 2", len(m.Models))
	}

	byName := map[string]Model{}
	for _, mod := range m.Models {
		byName[mod.Name] = mod
	}

	users, ok := byName["users"]
	if !ok {
		t.Fatal("missing users model")
	}
	if users.Path != "models/marts/users.sql" {
		t.Errorf("users path = %q", users.Path)
	}
	if len(users.Columns) != 3 {
		t.Errorf("users columns = %d, want 3", len(users.Columns))
	}

	userOrders, ok := byName["user_orders"]
	if !ok {
		t.Fatal("missing user_orders model")
	}
	if len(userOrders.DependsOn) != 2 {
		t.Errorf("user_orders deps = %d, want 2", len(userOrders.DependsOn))
	}
}

func TestParse_Tests(t *testing.T) {
	t.Parallel()
	m, err := Parse([]byte(sampleManifest))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(m.Tests) != 2 {
		t.Fatalf("Tests = %d, want 2", len(m.Tests))
	}

	var generic, singular *Test
	for i := range m.Tests {
		switch m.Tests[i].Kind {
		case TestKindGeneric:
			generic = &m.Tests[i]
		case TestKindSingular:
			singular = &m.Tests[i]
		}
	}

	if generic == nil || singular == nil {
		t.Fatalf("missing generic/singular: %+v", m.Tests)
	}
	if generic.TestMetadata.Name != "not_null" {
		t.Errorf("generic test_metadata.name = %q", generic.TestMetadata.Name)
	}
	if col, _ := generic.TestMetadata.Kwargs["column_name"].(string); col != "id" {
		t.Errorf("generic kwargs.column_name = %q", col)
	}
	if len(generic.CoveredModels) != 1 || generic.CoveredModels[0] != "model.analytics.users" {
		t.Errorf("generic CoveredModels = %v", generic.CoveredModels)
	}
	// Singular test depends on the users model — should be in CoveredModels.
	if len(singular.CoveredModels) != 1 {
		t.Errorf("singular CoveredModels = %v", singular.CoveredModels)
	}
}

func TestParse_Sources(t *testing.T) {
	t.Parallel()
	m, err := Parse([]byte(sampleManifest))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(m.Sources) != 1 {
		t.Fatalf("Sources = %d, want 1", len(m.Sources))
	}
	if m.Sources[0].Name != "users" || m.Sources[0].Schema != "raw" {
		t.Errorf("source: %+v", m.Sources[0])
	}
}

func TestLoad_MissingFile(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	m, err := Load(root)
	if err != nil {
		t.Errorf("Load with no manifest should not error, got %v", err)
	}
	if m != nil {
		t.Errorf("Load with no manifest should return nil, got %+v", m)
	}
}

func TestLoad_HappyPath(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "target"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "target", "manifest.json"), []byte(sampleManifest), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := Load(root)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if m == nil || m.ProjectName != "analytics" {
		t.Errorf("expected analytics project, got %+v", m)
	}
}

func TestIsDBTProject(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if IsDBTProject(root) {
		t.Error("empty dir should not be dbt project")
	}
	if err := os.WriteFile(filepath.Join(root, "dbt_project.yml"), []byte("name: x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !IsDBTProject(root) {
		t.Error("dir with dbt_project.yml should be dbt project")
	}
}

func TestFindDBTProjects(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	// Top-level project
	_ = os.WriteFile(filepath.Join(root, "dbt_project.yml"), []byte("name: top"), 0o644)
	// Nested project
	_ = os.MkdirAll(filepath.Join(root, "warehouse", "analytics"), 0o755)
	_ = os.WriteFile(filepath.Join(root, "warehouse", "analytics", "dbt_project.yml"), []byte("name: analytics"), 0o644)
	// Nested non-project
	_ = os.MkdirAll(filepath.Join(root, "src"), 0o755)
	// Vendored project — should be skipped
	_ = os.MkdirAll(filepath.Join(root, "dbt_packages", "dbt_utils"), 0o755)
	_ = os.WriteFile(filepath.Join(root, "dbt_packages", "dbt_utils", "dbt_project.yml"), []byte("name: utils"), 0o644)

	found, err := FindDBTProjects(root, 5)
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if len(found) != 2 {
		t.Errorf("found %d projects, want 2: %+v", len(found), found)
	}
}
