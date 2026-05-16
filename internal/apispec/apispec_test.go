package apispec

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseOpenAPI_V3(t *testing.T) {
	t.Parallel()
	spec := []byte(`
openapi: 3.0.3
info:
  title: User API
  version: 1.0.0
paths:
  /users/{id}:
    get:
      operationId: getUserById
      summary: Get a user by ID
      responses:
        "200":
          description: ok
          content:
            application/json:
              schema:
                type: object
                properties:
                  id: {type: integer}
                  name: {type: string}
                  email: {type: string}
  /users:
    post:
      operationId: createUser
      summary: Create user
      requestBody:
        content:
          application/json:
            schema:
              type: object
              properties:
                name: {type: string}
                email: {type: string}
      responses:
        "201":
          description: created
          content:
            application/json:
              schema:
                type: object
                properties:
                  id: {type: integer}
                  name: {type: string}
                  email: {type: string}
`)
	c, err := ParseOpenAPI(spec)
	if err != nil {
		t.Fatalf("ParseOpenAPI: %v", err)
	}
	if c.Version != "3.0.3" {
		t.Errorf("version = %q", c.Version)
	}
	if len(c.Operations) != 2 {
		t.Fatalf("ops = %d, want 2", len(c.Operations))
	}

	byID := map[string]Operation{}
	for _, op := range c.Operations {
		byID[op.OperationID] = op
	}

	get, ok := byID["getUserById"]
	if !ok {
		t.Fatal("missing getUserById")
	}
	if get.Method != "GET" || get.Path != "/users/{id}" {
		t.Errorf("get: %+v", get)
	}
	if len(get.FieldsRead) != 3 {
		t.Errorf("get fields_read = %v, want 3", get.FieldsRead)
	}

	post, ok := byID["createUser"]
	if !ok {
		t.Fatal("missing createUser")
	}
	if post.Method != "POST" {
		t.Errorf("post.method = %q", post.Method)
	}
	if len(post.FieldsWrite) != 2 {
		t.Errorf("post fields_write = %v, want 2 (name, email)", post.FieldsWrite)
	}
}

func TestParseOpenAPI_V2_Swagger(t *testing.T) {
	t.Parallel()
	spec := []byte(`{
  "swagger": "2.0",
  "info": {"title": "Legacy", "version": "1.0"},
  "paths": {
    "/orders": {
      "get": {
        "operationId": "listOrders",
        "responses": {
          "200": {
            "description": "ok",
            "schema": {
              "type": "object",
              "properties": {
                "items": {"type": "array"},
                "total": {"type": "integer"}
              }
            }
          }
        }
      }
    }
  }
}`)
	c, err := ParseOpenAPI(spec)
	if err != nil {
		t.Fatalf("ParseOpenAPI: %v", err)
	}
	if c.Version != "2.0" {
		t.Errorf("version = %q", c.Version)
	}
	if len(c.Operations) != 1 {
		t.Fatalf("ops = %d, want 1", len(c.Operations))
	}
	if len(c.Operations[0].FieldsRead) != 2 {
		t.Errorf("fields_read = %v, want 2", c.Operations[0].FieldsRead)
	}
}

func TestParseGraphQL(t *testing.T) {
	t.Parallel()
	src := `
# Comments are stripped.
type User {
  id: ID!
  name: String
  email: String!
}

type Query {
  user(id: ID!): User
  users: [User!]!
}

type Mutation {
  createUser(name: String!, email: String!): User
}
`
	c := ParseGraphQL(src)
	if c.Kind != ContractGraphQL {
		t.Errorf("kind = %q", c.Kind)
	}
	if len(c.Operations) != 3 {
		t.Fatalf("ops = %d, want 3 (user, users, createUser), got %+v", len(c.Operations), c.Operations)
	}

	byName := map[string]Operation{}
	for _, op := range c.Operations {
		byName[op.OperationID] = op
	}

	user, ok := byName["user"]
	if !ok {
		t.Fatal("missing user query")
	}
	if user.Method != "Query" {
		t.Errorf("user method = %q", user.Method)
	}
	if len(user.FieldsRead) != 3 {
		t.Errorf("user FieldsRead = %v, want 3 (id, name, email)", user.FieldsRead)
	}
	if len(user.FieldsWrite) != 1 || user.FieldsWrite[0] != "id" {
		t.Errorf("user FieldsWrite = %v", user.FieldsWrite)
	}

	create, ok := byName["createUser"]
	if !ok {
		t.Fatal("missing createUser mutation")
	}
	if create.Method != "Mutation" {
		t.Errorf("createUser method = %q", create.Method)
	}
	if len(create.FieldsWrite) != 2 {
		t.Errorf("createUser args = %v, want 2", create.FieldsWrite)
	}
}

func TestFind_Mixed(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	_ = os.MkdirAll(filepath.Join(root, "api"), 0o755)
	_ = os.WriteFile(filepath.Join(root, "api", "users.yaml"), []byte(`openapi: 3.0.3
info: {title: t, version: "1"}
paths:
  /users:
    get:
      operationId: list
      responses:
        "200":
          description: ok
`), 0o644)
	_ = os.WriteFile(filepath.Join(root, "api", "schema.graphql"), []byte(`
type Query {
  hello: String
}
`), 0o644)
	// Non-API yaml should be skipped.
	_ = os.WriteFile(filepath.Join(root, "config.yaml"), []byte("name: not openapi"), 0o644)

	got, err := Find(root)
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("found %d contracts, want 2 (1 openapi + 1 graphql, config.yaml excluded): %+v", len(got), got)
	}

	kinds := map[ContractKind]int{}
	for _, c := range got {
		kinds[c.Kind]++
	}
	if kinds[ContractOpenAPI] != 1 || kinds[ContractGraphQL] != 1 {
		t.Errorf("kinds: %+v", kinds)
	}
}

func TestNormalizeGraphQLType(t *testing.T) {
	t.Parallel()
	cases := []struct{ in, want string }{
		{"User", "User"},
		{"User!", "User"},
		{"[User]", "User"},
		{"[User!]!", "User"},
		{"  [ User! ] !  ", "User"},
	}
	for _, tc := range cases {
		if got := normalizeGraphQLType(tc.in); got != tc.want {
			t.Errorf("normalizeGraphQLType(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
