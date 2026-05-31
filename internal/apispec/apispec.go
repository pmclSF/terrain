// Package apispec discovers and parses cross-language API contracts —
// OpenAPI (v2/v3) specifications and GraphQL schemas. Both are the
// declarative interface between a client (any language) and a server
// (any language), so they're the natural anchor for the cross-stack
// edges Terrain needs to trace impact through a typed contract.
//
// The package exposes a uniform APIContract / Operation / Field
// shape. Each Operation carries:
//
//   - Method + Path (REST) or Type + Field (GraphQL)
//   - FieldsRead: the response fields a client could consume, used
//     by R3-I5 field-level narrowing — only when those fields are
//     touched by the diff does impact propagate.
//
// gRPC + tRPC parsing are followup work. tRPC needs TypeScript source
// analysis (router definitions are code, not a schema file); gRPC
// needs .proto parsing. Both are valuable but bigger lifts than the
// declarative-spec parsing this package targets.
package apispec

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// ContractKind identifies the contract format.
type ContractKind string

const (
	ContractOpenAPI ContractKind = "openapi"
	ContractGraphQL ContractKind = "graphql"
)

// APIContract is one parsed contract file.
type APIContract struct {
	// Path is the repo-relative path to the contract source.
	Path string

	// Kind identifies the format.
	Kind ContractKind

	// Version is the contract's own version string when declared
	// (OpenAPI 3.0.3, OpenAPI 2.0, GraphQL schemas don't carry one).
	Version string

	// Operations lists every operation in the contract.
	// REST: one per (path, method) pair.
	// GraphQL: one per top-level Query / Mutation / Subscription field.
	Operations []Operation
}

// Operation is one callable operation in a contract.
type Operation struct {
	// Method is the HTTP method (GET / POST / PUT / PATCH / DELETE) for
	// OpenAPI; "Query" / "Mutation" / "Subscription" for GraphQL.
	Method string

	// Path is the OpenAPI path template (/users/{id}); for GraphQL,
	// this is the operation field name (e.g., "userById").
	Path string

	// OperationID is the contract's stable identifier for the operation
	// when declared. Maps to OpenAPI operationId / GraphQL field name.
	OperationID string

	// Summary is the human-readable one-liner.
	Summary string

	// FieldsRead lists the response field paths a client could read
	// (R3-I5). For OpenAPI: the schema's property names at depth=1.
	// For GraphQL: the inner fields of the operation's return type.
	// Empty when the contract doesn't declare a response schema.
	FieldsRead []string

	// FieldsWrite lists request body / mutation argument field names.
	// Used by impact analysis to detect when a producer's field change
	// affects a known consumer.
	FieldsWrite []string
}

// Find walks root for OpenAPI specs and GraphQL schemas.
func Find(root string) ([]*APIContract, error) {
	var contracts []*APIContract

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
		c, err := parseFile(path)
		if err != nil {
			return nil
		}
		if c != nil {
			rel, _ := filepath.Rel(root, path)
			c.Path = rel
			contracts = append(contracts, c)
		}
		return nil
	})
	return contracts, err
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

// parseFile dispatches by extension and content sniff.
func parseFile(path string) (*APIContract, error) {
	lower := strings.ToLower(path)
	switch {
	case strings.HasSuffix(lower, ".graphql"), strings.HasSuffix(lower, ".gql"):
		return ParseGraphQLFile(path)
	case strings.HasSuffix(lower, ".proto"):
		return ParseProtoFile(path)
	case strings.HasSuffix(lower, ".ts"), strings.HasSuffix(lower, ".tsx"),
		strings.HasSuffix(lower, ".js"), strings.HasSuffix(lower, ".mjs"):
		// Could be a tRPC router — only emit a contract when the file
		// actually defines one.
		return ParseTRPCFile(path)
	case strings.HasSuffix(lower, ".yaml"), strings.HasSuffix(lower, ".yml"),
		strings.HasSuffix(lower, ".json"):
		// Could be OpenAPI; sniff content.
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		if looksLikeOpenAPI(data) {
			return ParseOpenAPI(data)
		}
		return nil, nil
	}
	return nil, nil
}

// looksLikeOpenAPI tests for an `openapi:` or `swagger:` top-level
// field. Cheap pre-parse check — full parse only when this matches.
func looksLikeOpenAPI(data []byte) bool {
	s := string(data)
	// Restrict scan to the first 1 KB.
	if len(s) > 1024 {
		s = s[:1024]
	}
	return strings.Contains(s, "openapi:") ||
		strings.Contains(s, `"openapi"`) ||
		strings.Contains(s, "swagger:") ||
		strings.Contains(s, `"swagger"`)
}

// ParseOpenAPI parses an OpenAPI v2 or v3 spec from JSON or YAML bytes.
func ParseOpenAPI(data []byte) (*APIContract, error) {
	var raw map[string]interface{}

	// Try JSON first (cheap on JSON input); fall back to YAML.
	if err := json.Unmarshal(data, &raw); err != nil {
		if err := yaml.Unmarshal(data, &raw); err != nil {
			return nil, fmt.Errorf("apispec: parse openapi: %w", err)
		}
	}

	c := &APIContract{Kind: ContractOpenAPI}

	if v, ok := raw["openapi"].(string); ok {
		c.Version = v
	} else if v, ok := raw["swagger"].(string); ok {
		c.Version = v
	}

	paths, ok := raw["paths"].(map[string]interface{})
	if !ok {
		return c, nil
	}

	for path, ops := range paths {
		opMap, ok := ops.(map[string]interface{})
		if !ok {
			continue
		}
		for method, opVal := range opMap {
			mu := strings.ToUpper(method)
			if !isHTTPMethod(mu) {
				continue
			}
			opDef, ok := opVal.(map[string]interface{})
			if !ok {
				continue
			}
			op := Operation{
				Method: mu,
				Path:   path,
			}
			if id, ok := opDef["operationId"].(string); ok {
				op.OperationID = id
			}
			if s, ok := opDef["summary"].(string); ok {
				op.Summary = s
			}
			op.FieldsRead = extractOpenAPIResponseFields(opDef)
			op.FieldsWrite = extractOpenAPIRequestFields(opDef)
			c.Operations = append(c.Operations, op)
		}
	}
	return c, nil
}

func isHTTPMethod(m string) bool {
	switch m {
	case "GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS":
		return true
	}
	return false
}

// extractOpenAPIResponseFields pulls the response schema's top-level
// property names from the 200 / 201 response when declared inline.
// Skips $ref-only responses (resolving $ref would require following
// component schemas across the document — followup work).
func extractOpenAPIResponseFields(op map[string]interface{}) []string {
	responses, _ := op["responses"].(map[string]interface{})
	for _, code := range []string{"200", "201", "default"} {
		resp, ok := responses[code].(map[string]interface{})
		if !ok {
			continue
		}
		// OpenAPI 3: response.content.<media>.schema
		content, _ := resp["content"].(map[string]interface{})
		for _, media := range content {
			mm, _ := media.(map[string]interface{})
			if schema, ok := mm["schema"].(map[string]interface{}); ok {
				if fields := schemaPropertyNames(schema); len(fields) > 0 {
					return fields
				}
			}
		}
		// OpenAPI 2: response.schema directly
		if schema, ok := resp["schema"].(map[string]interface{}); ok {
			if fields := schemaPropertyNames(schema); len(fields) > 0 {
				return fields
			}
		}
	}
	return nil
}

// extractOpenAPIRequestFields pulls the request body's top-level
// property names (OpenAPI 3) or the body parameter's schema property
// names (OpenAPI 2).
func extractOpenAPIRequestFields(op map[string]interface{}) []string {
	// OpenAPI 3
	if rb, ok := op["requestBody"].(map[string]interface{}); ok {
		content, _ := rb["content"].(map[string]interface{})
		for _, media := range content {
			mm, _ := media.(map[string]interface{})
			if schema, ok := mm["schema"].(map[string]interface{}); ok {
				if fields := schemaPropertyNames(schema); len(fields) > 0 {
					return fields
				}
			}
		}
	}
	// OpenAPI 2
	if params, ok := op["parameters"].([]interface{}); ok {
		for _, p := range params {
			pm, _ := p.(map[string]interface{})
			if pm["in"] == "body" {
				if schema, ok := pm["schema"].(map[string]interface{}); ok {
					return schemaPropertyNames(schema)
				}
			}
		}
	}
	return nil
}

func schemaPropertyNames(schema map[string]interface{}) []string {
	// schema may be inline object schema or wrap an array of objects.
	if items, ok := schema["items"].(map[string]interface{}); ok {
		schema = items
	}
	props, _ := schema["properties"].(map[string]interface{})
	if len(props) == 0 {
		return nil
	}
	names := make([]string, 0, len(props))
	for k := range props {
		names = append(names, k)
	}
	// Stable order so consumers can diff field sets across runs.
	sortStrings(names)
	return names
}

func sortStrings(s []string) {
	// tiny insertion sort — avoids importing sort just for this.
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j-1] > s[j]; j-- {
			s[j-1], s[j] = s[j], s[j-1]
		}
	}
}

// ParseGraphQLFile reads a .graphql / .gql schema file.
func ParseGraphQLFile(path string) (*APIContract, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("apispec: read %s: %w", path, err)
	}
	c := ParseGraphQL(string(data))
	c.Path = path
	return c, nil
}

// ParseGraphQL extracts top-level Query / Mutation / Subscription
// operations and their inner field types from a GraphQL SDL source.
//
// This is a deliberately small parser — it doesn't validate the
// schema, follow type references, or handle directives. The targets
// are the fields directly under type Query / type Mutation /
// type Subscription, plus the property names of those fields' return
// types when they're declared in the same document.
func ParseGraphQL(src string) *APIContract {
	c := &APIContract{Kind: ContractGraphQL}

	types := parseGraphQLTypes(src)

	for _, opType := range []string{"Query", "Mutation", "Subscription"} {
		fields := types[opType]
		for _, f := range fields {
			op := Operation{
				Method:      opType,
				Path:        f.Name,
				OperationID: f.Name,
			}
			// FieldsRead: inner fields of the return type when in the
			// same schema. Strip array brackets, non-null markers.
			returnType := normalizeGraphQLType(f.ReturnType)
			if inner, ok := types[returnType]; ok {
				op.FieldsRead = make([]string, len(inner))
				for i, ff := range inner {
					op.FieldsRead[i] = ff.Name
				}
				sortStrings(op.FieldsRead)
			}
			// FieldsWrite: argument names for mutations primarily.
			op.FieldsWrite = make([]string, len(f.Args))
			for i, a := range f.Args {
				op.FieldsWrite[i] = a
			}
			sortStrings(op.FieldsWrite)
			c.Operations = append(c.Operations, op)
		}
	}
	return c
}

type graphQLField struct {
	Name       string
	ReturnType string
	Args       []string
}

// parseGraphQLTypes is a tiny SDL extractor — finds `type Foo {...}`
// blocks and parses the field lines. Tolerant of trailing whitespace
// and one-line and multi-line declarations. Doesn't understand input
// types, enums, unions, interfaces — these aren't needed for the
// operation-extraction targets.
func parseGraphQLTypes(src string) map[string][]graphQLField {
	types := map[string][]graphQLField{}

	// Strip line comments.
	clean := stripGraphQLComments(src)

	// Locate `type X {` openers and walk until matching `}`.
	pos := 0
	for pos < len(clean) {
		idx := strings.Index(clean[pos:], "type ")
		if idx < 0 {
			break
		}
		start := pos + idx + len("type ")
		// Read type name.
		end := start
		for end < len(clean) && (isLetterOrDigit(clean[end]) || clean[end] == '_') {
			end++
		}
		typeName := clean[start:end]
		// Find `{` and matching `}`.
		brace := strings.Index(clean[end:], "{")
		if brace < 0 {
			break
		}
		bodyStart := end + brace + 1
		bodyEnd := strings.Index(clean[bodyStart:], "}")
		if bodyEnd < 0 {
			break
		}
		body := clean[bodyStart : bodyStart+bodyEnd]
		types[typeName] = parseGraphQLBody(body)
		pos = bodyStart + bodyEnd + 1
	}
	return types
}

func parseGraphQLBody(body string) []graphQLField {
	var fields []graphQLField
	for _, raw := range strings.Split(body, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// fieldName(arg: Type, ...): ReturnType
		colonIdx := strings.LastIndex(line, ":")
		if colonIdx < 0 {
			continue
		}
		head := strings.TrimSpace(line[:colonIdx])
		ret := strings.TrimSpace(line[colonIdx+1:])

		f := graphQLField{ReturnType: ret}

		if paren := strings.Index(head, "("); paren >= 0 {
			f.Name = strings.TrimSpace(head[:paren])
			argsEnd := strings.LastIndex(head, ")")
			if argsEnd > paren {
				args := head[paren+1 : argsEnd]
				for _, arg := range strings.Split(args, ",") {
					if c := strings.Index(arg, ":"); c > 0 {
						f.Args = append(f.Args, strings.TrimSpace(arg[:c]))
					}
				}
			}
		} else {
			f.Name = head
		}
		fields = append(fields, f)
	}
	return fields
}

// normalizeGraphQLType strips `[`, `]`, `!`, and whitespace from a
// return type expression so we can look it up in the types map.
// `[User!]!` → `User`, `[ User! ] !` → `User`.
func normalizeGraphQLType(t string) string {
	var b strings.Builder
	for _, r := range t {
		switch r {
		case '[', ']', '!', ' ', '\t', '\n':
			// drop
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func stripGraphQLComments(s string) string {
	var b strings.Builder
	inComment := false
	for _, r := range s {
		switch {
		case r == '#':
			inComment = true
		case r == '\n':
			inComment = false
			b.WriteRune(r)
		case !inComment:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func isLetterOrDigit(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9')
}
