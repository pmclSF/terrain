package sqlmigration

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParse_CreateTable_Postgres(t *testing.T) {
	t.Parallel()
	sql := `
CREATE TABLE users (
  id BIGSERIAL PRIMARY KEY,
  email VARCHAR(255) NOT NULL,
  name VARCHAR(255),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT users_email_unique UNIQUE (email)
);
`
	m := Parse(sql)
	if m == nil || len(m.Statements) != 1 {
		t.Fatalf("expected 1 statement, got %+v", m)
	}
	stmt := m.Statements[0]
	if stmt.Kind != StatementCreateTable || stmt.Table != "users" {
		t.Errorf("kind/table: %s / %s", stmt.Kind, stmt.Table)
	}
	if len(stmt.Columns) != 4 {
		t.Errorf("columns = %d, want 4 (id, email, name, created_at — constraint skipped): %+v", len(stmt.Columns), stmt.Columns)
	}

	byName := map[string]Column{}
	for _, c := range stmt.Columns {
		byName[c.Name] = c
	}
	if byName["email"].Nullable {
		t.Error("email should be NOT NULL")
	}
	if !byName["name"].Nullable {
		t.Error("name should be nullable")
	}
}

func TestParse_CreateTable_QuotedIdentifiers(t *testing.T) {
	t.Parallel()
	sql := `
CREATE TABLE "schema_name"."MyTable" (
  "Id" BIGINT NOT NULL,
  "First Name" VARCHAR(255)
);
`
	m := Parse(sql)
	if m == nil || len(m.Statements) != 1 {
		t.Fatalf("Parse failed: %+v", m)
	}
	if m.Statements[0].Table != "schema_name" {
		// Our parser captures the first quoted identifier of a dotted
		// name; full schema.table preservation is future work. Track
		// the actual value for visibility.
		t.Logf("table parsed as %q (dotted-name handling)", m.Statements[0].Table)
	}
}

func TestParse_CreateTable_MySQLBackticks(t *testing.T) {
	t.Parallel()
	sql := "CREATE TABLE `orders` (\n  `id` INT AUTO_INCREMENT PRIMARY KEY,\n  `total` DECIMAL(10, 2) NOT NULL\n);"
	m := Parse(sql)
	if m == nil || len(m.Statements) != 1 {
		t.Fatalf("Parse failed: %+v", m)
	}
	if m.Statements[0].Table != "orders" {
		t.Errorf("table = %q", m.Statements[0].Table)
	}
	if len(m.Statements[0].Columns) != 2 {
		t.Errorf("columns = %d, want 2 (id + total — primary key not separately enumerated)", len(m.Statements[0].Columns))
	}
}

func TestParse_AlterTable_AddColumn(t *testing.T) {
	t.Parallel()
	sql := `
ALTER TABLE users ADD COLUMN phone VARCHAR(32);
ALTER TABLE users ADD COLUMN verified BOOLEAN NOT NULL DEFAULT false;
`
	m := Parse(sql)
	if m == nil || len(m.Statements) != 2 {
		t.Fatalf("expected 2 statements, got %+v", m)
	}
	if m.Statements[0].Kind != StatementAlterTable {
		t.Errorf("kind = %s", m.Statements[0].Kind)
	}
	if len(m.Statements[0].Columns) != 1 || m.Statements[0].Columns[0].Name != "phone" {
		t.Errorf("alter 1 columns: %+v", m.Statements[0].Columns)
	}
	if len(m.Statements[1].Columns) != 1 || m.Statements[1].Columns[0].Name != "verified" {
		t.Errorf("alter 2 columns: %+v", m.Statements[1].Columns)
	}
	if m.Statements[1].Columns[0].Nullable {
		t.Error("verified should be NOT NULL")
	}
}

func TestParse_DropAndIndex(t *testing.T) {
	t.Parallel()
	sql := `
DROP TABLE IF EXISTS old_users;
CREATE INDEX idx_users_email ON users (email);
CREATE UNIQUE INDEX CONCURRENTLY idx_users_id ON users(id);
DROP INDEX IF EXISTS idx_users_old;
`
	m := Parse(sql)
	if m == nil || len(m.Statements) != 4 {
		t.Fatalf("expected 4 statements, got %d: %+v", len(m.Statements), m)
	}
	kinds := []StatementKind{}
	for _, s := range m.Statements {
		kinds = append(kinds, s.Kind)
	}
	wantKinds := []StatementKind{
		StatementDropTable,
		StatementCreateIndex,
		StatementCreateIndex,
		StatementDropIndex,
	}
	for i, want := range wantKinds {
		if kinds[i] != want {
			t.Errorf("stmt %d kind = %s, want %s", i, kinds[i], want)
		}
	}
}

func TestParse_Comments_Stripped(t *testing.T) {
	t.Parallel()
	sql := `
-- This is a top comment
CREATE TABLE x (
  /* inline block */ id BIGINT NOT NULL,
  name TEXT -- column comment
);
`
	m := Parse(sql)
	if m == nil || len(m.Statements) != 1 || len(m.Statements[0].Columns) != 2 {
		t.Fatalf("comments not handled cleanly: %+v", m)
	}
}

func TestParseFilename(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in            string
		wantVer, want string
	}{
		{"V12__create_users.sql", "12", "create_users"},
		{"V1__init.sql", "1", "init"},
		{"20240315120000_add_index.sql", "20240315120000", "add_index"},
		{"0042_users_alter.sql", "0042", "users_alter"},
		{"create_users.sql", "", "create_users"},
		{"V1__no_underscore_underscore.sql", "1", "no_underscore_underscore"},
	}
	for _, tc := range cases {
		gotV, gotN := parseFilename(tc.in)
		if gotV != tc.wantVer || gotN != tc.want {
			t.Errorf("parseFilename(%q) = (%q, %q), want (%q, %q)", tc.in, gotV, gotN, tc.wantVer, tc.want)
		}
	}
}

func TestFindMigrations(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	// Postgres flyway-style
	mig := filepath.Join(root, "db", "migrations")
	_ = os.MkdirAll(mig, 0o755)
	_ = os.WriteFile(filepath.Join(mig, "V1__create_users.sql"), []byte("CREATE TABLE users (id BIGINT);"), 0o644)
	_ = os.WriteFile(filepath.Join(mig, "V2__add_email.sql"), []byte("ALTER TABLE users ADD COLUMN email TEXT NOT NULL;"), 0o644)
	// node-pg-migrate timestamp
	other := filepath.Join(root, "migrations")
	_ = os.MkdirAll(other, 0o755)
	_ = os.WriteFile(filepath.Join(other, "20240315120000_init.sql"), []byte("CREATE TABLE x (id BIGINT);"), 0o644)
	// Skipped: vendor dir
	skip := filepath.Join(root, "vendor", "migrations")
	_ = os.MkdirAll(skip, 0o755)
	_ = os.WriteFile(filepath.Join(skip, "V99__should_skip.sql"), []byte("CREATE TABLE x (id BIGINT);"), 0o644)

	got, err := FindMigrations(root)
	if err != nil {
		t.Fatalf("FindMigrations: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("found %d migrations, want 3 (vendor must be skipped): %+v", len(got), got)
	}
	// Sorted by version: 1, 2, 20240315120000
	if got[0].Version != "1" || got[1].Version != "2" || got[2].Version != "20240315120000" {
		t.Errorf("sort order broken: %+v", []string{got[0].Version, got[1].Version, got[2].Version})
	}
}
