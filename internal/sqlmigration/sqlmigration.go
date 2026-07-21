// Package sqlmigration discovers and parses SQL migration files —
// the canonical mechanism for evolving a relational schema in versioned
// projects. Supports the common conventions:
//
//   - Flyway-style numbered files: V1__create_users.sql, V12__add_index.sql
//   - Liquibase SQL changesets (header-comment-based ordering)
//   - node-pg-migrate / knex.js style: <timestamp>_<name>.sql
//   - Generic versioned migrations in db/migrate/ or migrations/ dirs
//
// Terrain consumes the parsed schema to wire columns to code surfaces
// (field-level narrowing) and to feed the schema/* rules family
// (hygiene/destructive-migration, lifecycle/schema-drift).
//
// Parsing is pattern-based, not a full SQL parser. The extraction
// targets are CREATE TABLE, ALTER TABLE, DROP TABLE, CREATE INDEX,
// DROP INDEX. Postgres-style identifiers (double-quoted, schema-
// qualified) and MySQL-style identifiers (backticked) are both
// recognized. Lower-level SQL features (CTEs, triggers, functions) are
// intentionally ignored — they don't carry column-to-surface coverage.
package sqlmigration

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/pmclSF/terrain/internal/saferead"
)

// Migration represents one migration file.
type Migration struct {
	// Path is the repository-relative file path.
	Path string

	// Version is the parsed version identifier from the filename
	// (Flyway "V12", timestamp "20240315120000", or sequential
	// "0042"). Empty when the convention isn't recognized.
	Version string

	// Name is the human-readable migration name from the filename
	// (e.g., "create_users" from V12__create_users.sql).
	Name string

	// Statements lists the DDL statements parsed from the file.
	Statements []Statement
}

// StatementKind classifies a DDL statement.
type StatementKind string

const (
	StatementCreateTable StatementKind = "create_table"
	StatementAlterTable  StatementKind = "alter_table"
	StatementDropTable   StatementKind = "drop_table"
	StatementCreateIndex StatementKind = "create_index"
	StatementDropIndex   StatementKind = "drop_index"
)

// Statement is one parsed DDL statement. Columns are populated for
// CREATE TABLE and ALTER TABLE … ADD COLUMN; empty otherwise.
type Statement struct {
	Kind    StatementKind
	Table   string
	Columns []Column
}

// Column is a parsed column definition. Type and Nullable carry
// signal for the schema/* rules; full constraint parsing is deferred.
type Column struct {
	Name     string
	Type     string // raw type text as written
	Nullable bool   // true when no NOT NULL constraint declared
}

// FindMigrations walks root for migration directories and returns one
// Migration per parseable file, sorted by Version. Common
// non-migration paths (vendor/, node_modules/, target/, .git/) are
// skipped.
func FindMigrations(root string) ([]Migration, error) {
	var migrations []Migration

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if d.IsDir() {
			base := d.Name()
			if path != root && skipDirs[base] {
				return filepath.SkipDir
			}
			return nil
		}
		if !looksLikeMigration(path) {
			return nil
		}
		m, err := ParseFile(path)
		if err != nil {
			return nil // tolerate per-file parse errors
		}
		if m != nil {
			// Make path repo-relative.
			rel, _ := filepath.Rel(root, path)
			m.Path = rel
			migrations = append(migrations, *m)
		}
		return nil
	})

	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Version < migrations[j].Version
	})
	return migrations, err
}

// looksLikeMigration returns true when path looks like a migration
// file — .sql in a recognized migrations directory, with a filename
// matching one of the supported conventions.
func looksLikeMigration(path string) bool {
	if !strings.HasSuffix(strings.ToLower(path), ".sql") {
		return false
	}
	// Walk the path's directory components looking for a migrations marker.
	dir := filepath.Dir(path)
	for dir != "/" && dir != "." {
		base := filepath.Base(dir)
		if migrationsDirNames[base] {
			return true
		}
		dir = filepath.Dir(dir)
	}
	return false
}

// migrationsDirNames is the set of directory names that mark a
// migrations location.
var migrationsDirNames = map[string]bool{
	"migrations":    true,
	"migrate":       true,
	"db_migrations": true,
	"changelog":     true,
	"changesets":    true,
	"sql":           false, // too generic on its own
}

var skipDirs = map[string]bool{
	"node_modules": true,
	"vendor":       true,
	".git":         true,
	"target":       true,
	"dist":         true,
	"build":        true,
	".terrain":     true,
	".venv":        true,
	"venv":         true,
}

// ParseFile reads and parses a single migration file.
func ParseFile(path string) (*Migration, error) {
	data, ok := saferead.File(path, saferead.DataCap)
	if !ok {
		return nil, fmt.Errorf("sqlmigration: %s is not a readable regular file within the size limit", path)
	}
	m := Parse(string(data))
	if m == nil {
		return nil, nil
	}
	m.Path = path
	m.Version, m.Name = parseFilename(filepath.Base(path))
	return m, nil
}

// Parse extracts DDL statements from a SQL source string.
func Parse(sql string) *Migration {
	stmts := extractStatements(sql)
	if len(stmts) == 0 {
		return nil
	}
	return &Migration{Statements: stmts}
}

// extractStatements is the pattern-based DDL extractor.
func extractStatements(sql string) []Statement {
	// Strip /* ... */ comments and -- line comments.
	clean := stripSQLComments(sql)

	// Split on semicolons. Naive but sufficient — we don't support
	// procedural blocks with internal semicolons (DO $$...$$). When we
	// do, we'll switch to a real SQL tokenizer.
	var stmts []Statement
	for _, raw := range strings.Split(clean, ";") {
		s := strings.TrimSpace(raw)
		if s == "" {
			continue
		}
		if stmt, ok := matchStatement(s); ok {
			stmts = append(stmts, stmt)
		}
	}
	return stmts
}

var (
	createTableRE = regexp.MustCompile(`(?is)^\s*CREATE\s+(?:TEMP\s+|TEMPORARY\s+|UNLOGGED\s+)?TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?` + identPattern + `\s*\(([\s\S]*)\)`)
	alterTableRE  = regexp.MustCompile(`(?is)^\s*ALTER\s+TABLE\s+(?:IF\s+EXISTS\s+)?` + identPattern + `\s+([\s\S]+)`)
	dropTableRE   = regexp.MustCompile(`(?is)^\s*DROP\s+TABLE\s+(?:IF\s+EXISTS\s+)?` + identPattern)
	createIndexRE = regexp.MustCompile(`(?is)^\s*CREATE\s+(?:UNIQUE\s+)?INDEX\s+(?:CONCURRENTLY\s+)?(?:IF\s+NOT\s+EXISTS\s+)?` + identPattern + `\s+ON\s+` + identPattern)
	dropIndexRE   = regexp.MustCompile(`(?is)^\s*DROP\s+INDEX\s+(?:CONCURRENTLY\s+)?(?:IF\s+EXISTS\s+)?` + identPattern)
)

// identPattern matches a SQL identifier: bare, double-quoted, or
// backticked. Supports schema-qualified names where each part may be
// independently quoted (e.g., "public"."Users", `schema`.`table`).
// The helper unquoteIdent normalizes the captured group after match.
const identPart = `(?:"[^"]+"|` + "`[^`]+`" + `|\w+)`
const identPattern = `(` + identPart + `(?:\.` + identPart + `)*)`

func matchStatement(s string) (Statement, bool) {
	switch {
	case createTableRE.MatchString(s):
		m := createTableRE.FindStringSubmatch(s)
		return Statement{
			Kind:    StatementCreateTable,
			Table:   unquoteIdent(m[1]),
			Columns: parseColumnList(m[2]),
		}, true
	case alterTableRE.MatchString(s):
		m := alterTableRE.FindStringSubmatch(s)
		return Statement{
			Kind:    StatementAlterTable,
			Table:   unquoteIdent(m[1]),
			Columns: parseAlterColumns(m[2]),
		}, true
	case dropTableRE.MatchString(s):
		m := dropTableRE.FindStringSubmatch(s)
		return Statement{Kind: StatementDropTable, Table: unquoteIdent(m[1])}, true
	case createIndexRE.MatchString(s):
		m := createIndexRE.FindStringSubmatch(s)
		return Statement{Kind: StatementCreateIndex, Table: unquoteIdent(m[2])}, true
	case dropIndexRE.MatchString(s):
		m := dropIndexRE.FindStringSubmatch(s)
		return Statement{Kind: StatementDropIndex, Table: unquoteIdent(m[1])}, true
	}
	return Statement{}, false
}

// parseColumnList parses the body of a CREATE TABLE — the contents
// between the outer parentheses. Splits on top-level commas (commas
// inside nested parens, e.g., DECIMAL(10,2), don't separate columns).
func parseColumnList(body string) []Column {
	var cols []Column
	depth := 0
	current := strings.Builder{}
	flush := func() {
		s := strings.TrimSpace(current.String())
		current.Reset()
		if s == "" {
			return
		}
		if c, ok := parseColumnDef(s); ok {
			cols = append(cols, c)
		}
	}
	for _, r := range body {
		switch r {
		case '(':
			depth++
			current.WriteRune(r)
		case ')':
			depth--
			current.WriteRune(r)
		case ',':
			if depth == 0 {
				flush()
				continue
			}
			current.WriteRune(r)
		default:
			current.WriteRune(r)
		}
	}
	flush()
	return cols
}

// parseColumnDef parses one column clause. Returns false for
// table-level constraints (PRIMARY KEY (...), FOREIGN KEY, CHECK,
// UNIQUE (col)), which look like columns but aren't.
func parseColumnDef(s string) (Column, bool) {
	tokens := tokenize(s)
	if len(tokens) < 2 {
		return Column{}, false
	}
	first := strings.ToUpper(tokens[0])
	if first == "CONSTRAINT" || first == "PRIMARY" || first == "FOREIGN" ||
		first == "CHECK" || first == "UNIQUE" || first == "EXCLUDE" ||
		first == "LIKE" {
		return Column{}, false
	}
	col := Column{
		Name:     unquoteIdent(tokens[0]),
		Type:     tokens[1],
		Nullable: true,
	}
	upper := strings.ToUpper(s)
	if strings.Contains(upper, "NOT NULL") {
		col.Nullable = false
	}
	return col, true
}

// parseAlterColumns extracts column changes from an ALTER TABLE body.
// Recognizes ADD COLUMN <name> <type>; deeper alterations (DROP
// COLUMN, ALTER COLUMN type, RENAME) are detected as kind=alter_table
// without column-level detail at 0.2.0.
var addColumnRE = regexp.MustCompile(`(?is)ADD\s+(?:COLUMN\s+)?(?:IF\s+NOT\s+EXISTS\s+)?` + identPattern + `\s+(\S+)`)

func parseAlterColumns(body string) []Column {
	var cols []Column
	for _, m := range addColumnRE.FindAllStringSubmatch(body, -1) {
		nullable := !strings.Contains(strings.ToUpper(body), "NOT NULL")
		cols = append(cols, Column{
			Name:     unquoteIdent(m[1]),
			Type:     m[2],
			Nullable: nullable,
		})
	}
	return cols
}

// tokenize splits on whitespace; preserves quoted-identifier wrappers
// so unquoteIdent can strip them after the split.
func tokenize(s string) []string {
	return strings.Fields(s)
}

func unquoteIdent(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 {
		first := s[0]
		last := s[len(s)-1]
		if (first == '"' && last == '"') || (first == '`' && last == '`') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

// stripSQLComments removes -- and /* */ comments.
func stripSQLComments(sql string) string {
	var out strings.Builder
	i := 0
	for i < len(sql) {
		// Line comment
		if i+1 < len(sql) && sql[i] == '-' && sql[i+1] == '-' {
			for i < len(sql) && sql[i] != '\n' {
				i++
			}
			continue
		}
		// Block comment
		if i+1 < len(sql) && sql[i] == '/' && sql[i+1] == '*' {
			i += 2
			for i+1 < len(sql) && !(sql[i] == '*' && sql[i+1] == '/') {
				i++
			}
			if i+1 < len(sql) {
				i += 2
			}
			continue
		}
		out.WriteByte(sql[i])
		i++
	}
	return out.String()
}

// parseFilename extracts (version, name) from common conventions:
//
//	V12__create_users.sql              → ("12", "create_users")
//	20240315120000_add_index.sql       → ("20240315120000", "add_index")
//	0042_users_alter.sql               → ("0042", "users_alter")
//	create_users.sql                   → ("", "create_users")
func parseFilename(name string) (version, label string) {
	stem := strings.TrimSuffix(name, ".sql")
	stem = strings.TrimSuffix(stem, ".SQL")

	// Flyway: V<number>__<name>
	if strings.HasPrefix(stem, "V") || strings.HasPrefix(stem, "v") {
		if idx := strings.Index(stem, "__"); idx > 0 {
			return stem[1:idx], stem[idx+2:]
		}
	}

	// Numeric prefix (timestamp or sequence) separated by _ or -
	for _, sep := range []string{"_", "-"} {
		if idx := strings.Index(stem, sep); idx > 0 {
			prefix := stem[:idx]
			if isNumeric(prefix) {
				return prefix, stem[idx+1:]
			}
		}
	}

	return "", stem
}

func isNumeric(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
