package promptflow

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/pmclSF/terrain/internal/prompttemplate"
)

// TemplateFile is one discovered prompt template, with its repo-relative
// path and the parsed Template.
type TemplateFile struct {
	Path string
	Tpl  prompttemplate.Template
}

// SchemaFile is one discovered JSON Schema document, with its
// repo-relative path and the raw bytes.
type SchemaFile struct {
	Path string
	Body []byte
}

// Discoveries enumerates the prompt-template and schema files found by
// a single Discover pass.
type Discoveries struct {
	Templates []TemplateFile
	Schemas   []SchemaFile
}

// noiseDirs are directories whose contents are never walked. These are
// build outputs, dependency caches, and version-control internals that
// commonly contain template-or-schema-shaped files Terrain should not
// pick up as user content.
var noiseDirs = map[string]struct{}{
	"node_modules": {},
	"vendor":       {},
	".git":         {},
	"dist":         {},
	"build":        {},
	".terrain":     {},
}

// Discover walks the directory tree rooted at root and returns the
// templates + schemas found. Paths in the result are relative to root.
//
// Template detection: any file whose extension prompttemplate.Detect
// recognises (`.md` / `.markdown` → mustache).
//
// Schema detection: any `.json` file whose top-level object EITHER
// carries a `$schema` URI matching the json-schema.org spec OR
// declares `"type": "object"`, AND has a `properties` key. The
// `properties`-only heuristic was too permissive — many config files
// (webpack, vite, tsconfig overrides) use the key.
//
// Noise directories (`node_modules`, `vendor`, `.git`, `dist`,
// `build`, `.terrain`) are skipped entirely. Malformed JSON is
// silently skipped.
func Discover(root string) (Discoveries, error) {
	var out Discoveries
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if _, skip := noiseDirs[d.Name()]; skip && path != root {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if kind := prompttemplate.Detect(path, body); kind != prompttemplate.KindUnknown {
			out.Templates = append(out.Templates, TemplateFile{
				Path: rel,
				Tpl:  prompttemplate.Template{Kind: kind, Body: string(body), Path: rel},
			})
			return nil
		}
		if strings.EqualFold(filepath.Ext(path), ".json") && looksLikeJSONSchema(body) {
			out.Schemas = append(out.Schemas, SchemaFile{Path: rel, Body: body})
		}
		return nil
	})
	if err != nil {
		return Discoveries{}, err
	}
	return out, nil
}

// jsonSchemaURIPattern is conservative — anchored to the json-schema.org
// host with any path. Covers all draft revisions
// (draft-04 through draft/2020-12).
var jsonSchemaURIPattern = regexp.MustCompile(`^https?://json-schema\.org/`)

// looksLikeJSONSchema returns true when body parses as a JSON object,
// declares a `properties` key, AND carries at least one positive
// schema signal — either a `$schema` URI under json-schema.org or
// `"type": "object"` at the top level. The two-signal rule eliminates
// the dominant false-positive class (config files that use
// "properties" as an organizing key).
func looksLikeJSONSchema(body []byte) bool {
	var probe struct {
		Schema     string          `json:"$schema"`
		Type       string          `json:"type"`
		Properties json.RawMessage `json:"properties"`
	}
	if err := json.Unmarshal(body, &probe); err != nil {
		return false
	}
	if len(probe.Properties) == 0 {
		return false
	}
	if jsonSchemaURIPattern.MatchString(probe.Schema) {
		return true
	}
	return probe.Type == "object"
}
