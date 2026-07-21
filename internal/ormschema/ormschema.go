// Package ormschema parses ORM schema declarations across the major
// Go / TypeScript / Python ORMs. ORMs are the language-side anchor
// for database tables — the model class names, field names, and
// constraints declared in code parallel the SQL schema declared in
// migrations. Pairing the two unlocks column-to-surface propagation
// for field-level narrowing.
//
// Currently parses Prisma (schema.prisma — a single declarative file) into
// the Schema / Model / Field shape.
package ormschema

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/pmclSF/terrain/internal/saferead"
)

// ORMKind identifies the schema source.
type ORMKind string

const (
	ORMPrisma     ORMKind = "prisma"
	ORMGorm       ORMKind = "gorm"
	ORMSQLC       ORMKind = "sqlc"
	ORMSqlAlchemy ORMKind = "sqlalchemy"
)

// Schema is the parsed contents of one ORM source.
type Schema struct {
	// Path is the repo-relative file path.
	Path string

	// Kind identifies the ORM source.
	Kind ORMKind

	// Datasources lists the database connections declared (Prisma-
	// specific concept; populated only for ORMPrisma).
	Datasources []Datasource

	// Models lists every declared model.
	Models []Model
}

// Datasource is one declared database connection.
type Datasource struct {
	Name     string
	Provider string // "postgresql" / "mysql" / "sqlite" / etc.
}

// Model is one ORM model (database table).
type Model struct {
	// Name is the model's declared name (typically PascalCase).
	Name string

	// TableName is the explicit table name when declared (Prisma's
	// @@map, gorm's TableName method, etc.). Empty when the ORM uses
	// implicit pluralization or the model name directly.
	TableName string

	// Fields lists every declared field on the model.
	Fields []Field
}

// Field is one model field (database column).
type Field struct {
	// Name is the field's declared name.
	Name string

	// Type is the raw type text as written.
	Type string

	// Required is true when the field is non-null. Prisma's `Type?`
	// makes it optional; gorm's *T makes it optional.
	Required bool

	// IsRelation is true when the field references another model
	// rather than a scalar database type.
	IsRelation bool

	// IsList is true when the field is a list/array of values.
	IsList bool

	// Attributes lists declared modifiers (@id, @unique, @default(...),
	// @relation(...), etc. for Prisma; struct tags for gorm).
	Attributes []string
}

// Find walks root for schema sources and parses each. Returns one
// Schema per file. Vendor / generated / etc. directories skipped.
func Find(root string) ([]*Schema, error) {
	var schemas []*Schema

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
		// Prisma: any *.prisma file (typically schema.prisma).
		if strings.HasSuffix(strings.ToLower(path), ".prisma") {
			s, err := ParsePrismaFile(path)
			if err != nil {
				return nil
			}
			if s != nil {
				rel, _ := filepath.Rel(root, path)
				s.Path = rel
				schemas = append(schemas, s)
			}
		}
		return nil
	})
	return schemas, err
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

// ParsePrismaFile reads a schema.prisma file.
func ParsePrismaFile(path string) (*Schema, error) {
	data, ok := saferead.File(path, saferead.SourceCap)
	if !ok {
		return nil, fmt.Errorf("ormschema: %s is not a readable regular file within the size limit", path)
	}
	s := ParsePrisma(string(data))
	if s == nil {
		return nil, nil
	}
	s.Path = path
	return s, nil
}

// ParsePrisma parses a schema.prisma source.
//
// Prisma's DSL is line-oriented within block scopes. The parser
// recognizes top-level `model X { ... }` and `datasource X { ... }`
// blocks; everything else (`generator`, `enum`, comments) is ignored.
// Fields are extracted from model bodies; the body parser handles
// trailing `@`-attributes and `@@`-model-attributes (the latter
// surface as TableName when `@@map` is present).
func ParsePrisma(src string) *Schema {
	s := &Schema{Kind: ORMPrisma}

	clean := stripPrismaComments(src)

	for _, blk := range findPrismaBlocks(clean, "datasource") {
		ds := Datasource{Name: blk.name}
		for _, line := range strings.Split(blk.body, "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "provider") {
				if v := extractPrismaAssignment(line); v != "" {
					ds.Provider = v
				}
			}
		}
		s.Datasources = append(s.Datasources, ds)
	}

	for _, blk := range findPrismaBlocks(clean, "model") {
		m := Model{Name: blk.name}
		for _, line := range strings.Split(blk.body, "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "//") {
				continue
			}
			// Model-level attributes start with `@@`.
			if strings.HasPrefix(line, "@@") {
				if strings.HasPrefix(line, "@@map(") {
					m.TableName = extractPrismaArg(line)
				}
				continue
			}
			if f, ok := parsePrismaField(line); ok {
				m.Fields = append(m.Fields, f)
			}
		}
		s.Models = append(s.Models, m)
	}

	if len(s.Models) == 0 {
		return nil
	}
	return s
}

// findPrismaBlocks finds `keyword Name { ... }` blocks and returns
// name + body for each, respecting brace nesting.
type prismaBlock struct {
	name string
	body string
}

func findPrismaBlocks(src, keyword string) []prismaBlock {
	pattern := regexp.MustCompile(`\b` + keyword + `\s+(\w+)\s*\{`)
	var out []prismaBlock
	for _, m := range pattern.FindAllStringSubmatchIndex(src, -1) {
		name := src[m[2]:m[3]]
		bodyStart := m[1]
		depth := 1
		i := bodyStart
		for i < len(src) && depth > 0 {
			switch src[i] {
			case '{':
				depth++
			case '}':
				depth--
			}
			i++
		}
		if depth == 0 {
			out = append(out, prismaBlock{name: name, body: src[bodyStart : i-1]})
		}
	}
	return out
}

// parsePrismaField parses one line of a model body.
//
// Line shapes:
//
//	id        Int      @id @default(autoincrement())
//	email     String   @unique
//	name      String?
//	posts     Post[]
//	author    User     @relation(fields: [authorId], references: [id])
func parsePrismaField(line string) (Field, bool) {
	parts := strings.Fields(line)
	if len(parts) < 2 {
		return Field{}, false
	}
	name := parts[0]
	rawType := parts[1]

	f := Field{Name: name}
	// Optional? trailing question mark.
	if strings.HasSuffix(rawType, "?") {
		f.Type = strings.TrimSuffix(rawType, "?")
		f.Required = false
	} else if strings.HasSuffix(rawType, "[]") {
		f.Type = strings.TrimSuffix(rawType, "[]")
		f.IsList = true
		f.Required = true
	} else {
		f.Type = rawType
		f.Required = true
	}

	// Heuristic: relation when type isn't one of the Prisma scalar
	// builtins. Falls back to flagging anything starting with an
	// uppercase letter that isn't in the scalar set.
	if isPrismaRelationType(f.Type) {
		f.IsRelation = true
	}

	// Collect any @-attributes from the remaining tokens.
	if len(parts) > 2 {
		rest := strings.Join(parts[2:], " ")
		f.Attributes = extractPrismaAttributes(rest)
	}

	return f, true
}

// isPrismaRelationType is true when the type isn't a known Prisma
// scalar / native type. Empty types fall through as scalar.
func isPrismaRelationType(t string) bool {
	scalar := map[string]bool{
		"String":   true,
		"Boolean":  true,
		"Int":      true,
		"BigInt":   true,
		"Float":    true,
		"Decimal":  true,
		"DateTime": true,
		"Json":     true,
		"Bytes":    true,
	}
	if scalar[t] {
		return false
	}
	if t == "" {
		return false
	}
	// Heuristic: relation types are uppercase (model names follow
	// PascalCase). Lowercase types we don't recognize are probably
	// custom scalars or enum names — treat as scalar to avoid false
	// relation flags.
	return t[0] >= 'A' && t[0] <= 'Z'
}

// extractPrismaAttributes pulls @-prefixed attributes from the post-
// type portion of a field line. Handles parenthesized arguments.
func extractPrismaAttributes(s string) []string {
	var attrs []string
	i := 0
	for i < len(s) {
		if s[i] != '@' {
			i++
			continue
		}
		// Walk to whitespace or end, but include matched parens.
		j := i + 1
		for j < len(s) && s[j] != ' ' && s[j] != '\t' && s[j] != '(' {
			j++
		}
		if j < len(s) && s[j] == '(' {
			// Walk to matching close paren.
			depth := 1
			j++
			for j < len(s) && depth > 0 {
				switch s[j] {
				case '(':
					depth++
				case ')':
					depth--
				}
				j++
			}
		}
		attrs = append(attrs, s[i:j])
		i = j
	}
	return attrs
}

// extractPrismaAssignment pulls the right-hand-side string from a line
// like `provider = "postgresql"`. Returns "" when not a simple
// quoted-string assignment.
func extractPrismaAssignment(line string) string {
	eq := strings.Index(line, "=")
	if eq < 0 {
		return ""
	}
	rhs := strings.TrimSpace(line[eq+1:])
	if len(rhs) >= 2 && rhs[0] == '"' && rhs[len(rhs)-1] == '"' {
		return rhs[1 : len(rhs)-1]
	}
	return rhs
}

// extractPrismaArg pulls the first string argument from `@@map("foo")`
// or `@map("foo")`.
func extractPrismaArg(line string) string {
	open := strings.Index(line, "(")
	close := strings.LastIndex(line, ")")
	if open < 0 || close <= open {
		return ""
	}
	inner := strings.TrimSpace(line[open+1 : close])
	if len(inner) >= 2 && inner[0] == '"' && inner[len(inner)-1] == '"' {
		return inner[1 : len(inner)-1]
	}
	return inner
}

func stripPrismaComments(src string) string {
	var b strings.Builder
	for _, line := range strings.Split(src, "\n") {
		// Prisma supports // (line) and /// (doc) comments.
		if idx := strings.Index(line, "//"); idx >= 0 {
			line = line[:idx]
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return b.String()
}
