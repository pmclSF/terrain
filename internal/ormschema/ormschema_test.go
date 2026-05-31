package ormschema

import (
	"os"
	"path/filepath"
	"testing"
)

const samplePrisma = `// This is a typical Prisma schema.

generator client {
  provider = "prisma-client-js"
}

datasource db {
  provider = "postgresql"
  url      = env("DATABASE_URL")
}

model User {
  id        Int      @id @default(autoincrement())
  email     String   @unique
  name      String?
  posts     Post[]
  createdAt DateTime @default(now())

  @@map("users")
}

model Post {
  id        Int      @id @default(autoincrement())
  title     String
  content   String?
  author    User     @relation(fields: [authorId], references: [id])
  authorId  Int
  tags      String[]
}
`

func TestParsePrisma_Datasource(t *testing.T) {
	t.Parallel()
	s := ParsePrisma(samplePrisma)
	if s == nil {
		t.Fatal("Parse returned nil")
	}
	if s.Kind != ORMPrisma {
		t.Errorf("kind = %q", s.Kind)
	}
	if len(s.Datasources) != 1 || s.Datasources[0].Name != "db" {
		t.Fatalf("datasources: %+v", s.Datasources)
	}
	if s.Datasources[0].Provider != "postgresql" {
		t.Errorf("provider = %q", s.Datasources[0].Provider)
	}
}

func TestParsePrisma_Models(t *testing.T) {
	t.Parallel()
	s := ParsePrisma(samplePrisma)
	if s == nil {
		t.Fatal("Parse returned nil")
	}
	if len(s.Models) != 2 {
		t.Fatalf("models = %d, want 2", len(s.Models))
	}

	byName := map[string]Model{}
	for _, m := range s.Models {
		byName[m.Name] = m
	}

	user, ok := byName["User"]
	if !ok {
		t.Fatal("missing User")
	}
	if user.TableName != "users" {
		t.Errorf("user table name = %q, want users (from @@map)", user.TableName)
	}
	if len(user.Fields) != 5 {
		t.Fatalf("user fields = %d, want 5", len(user.Fields))
	}

	byField := map[string]Field{}
	for _, f := range user.Fields {
		byField[f.Name] = f
	}

	if !byField["id"].Required {
		t.Error("id should be required")
	}
	if byField["name"].Required {
		t.Error("name? should be optional")
	}
	if !byField["posts"].IsList {
		t.Error("posts should be list (Post[])")
	}
	if !byField["posts"].IsRelation {
		t.Error("posts should be a relation (Post is a model)")
	}
	if byField["email"].IsRelation {
		t.Error("email should NOT be a relation (String scalar)")
	}

	if len(byField["id"].Attributes) != 2 {
		t.Errorf("id attrs = %v, want 2 (@id, @default(autoincrement()))", byField["id"].Attributes)
	}
}

func TestParsePrisma_RelationField(t *testing.T) {
	t.Parallel()
	s := ParsePrisma(samplePrisma)
	post := s.Models[1]
	if post.Name != "Post" {
		t.Fatalf("expected Post as second model, got %s", post.Name)
	}
	byField := map[string]Field{}
	for _, f := range post.Fields {
		byField[f.Name] = f
	}
	author := byField["author"]
	if !author.IsRelation {
		t.Error("author should be relation")
	}
	// @relation(fields: [authorId], references: [id]) should be in attributes.
	foundRelation := false
	for _, a := range author.Attributes {
		if len(a) >= len("@relation") && a[:len("@relation")] == "@relation" {
			foundRelation = true
		}
	}
	if !foundRelation {
		t.Errorf("author attributes missing @relation: %v", author.Attributes)
	}

	tags := byField["tags"]
	if !tags.IsList {
		t.Error("tags should be list (String[])")
	}
	if tags.IsRelation {
		t.Error("tags should NOT be a relation (String is scalar)")
	}
}

func TestFind_PrismaSchemas(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	_ = os.MkdirAll(filepath.Join(root, "prisma"), 0o755)
	_ = os.WriteFile(filepath.Join(root, "prisma", "schema.prisma"), []byte(samplePrisma), 0o644)
	// Skipped vendor dir.
	_ = os.MkdirAll(filepath.Join(root, "node_modules", "@prisma"), 0o755)
	_ = os.WriteFile(filepath.Join(root, "node_modules", "@prisma", "noise.prisma"), []byte("model X { id Int @id }"), 0o644)

	got, err := Find(root)
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("found %d, want 1 (vendor skipped): %+v", len(got), got)
	}
	if got[0].Kind != ORMPrisma {
		t.Errorf("kind = %q", got[0].Kind)
	}
}

func TestParsePrisma_EmptyOrNoModels(t *testing.T) {
	t.Parallel()
	// File with only a generator and datasource but no models.
	src := `
generator client {
  provider = "prisma-client-js"
}
datasource db {
  provider = "postgresql"
}
`
	s := ParsePrisma(src)
	if s != nil {
		t.Errorf("expected nil for schemas with no models, got %+v", s)
	}
}
