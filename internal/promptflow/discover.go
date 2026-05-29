package promptflow

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
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

// Discover walks the directory tree rooted at root and returns the
// templates + schemas found. Paths in the result are relative to root.
//
// Template detection today: any file whose extension prompttemplate.Detect
// recognises (`.md` / `.markdown` → mustache).
//
// Schema detection today: any `.json` file whose top-level object has a
// `properties` key. Malformed JSON is silently skipped — schema-shaped
// files often live next to non-schema JSON in a repo.
func Discover(root string) (Discoveries, error) {
	var out Discoveries
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
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
				Tpl:  prompttemplate.Template{Kind: kind, Body: string(body)},
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

// looksLikeJSONSchema returns true when body parses as a JSON object
// that has a `properties` key. The check is intentionally narrow —
// non-schema JSON files (configs, fixtures) tend not to use that key
// at the top level.
func looksLikeJSONSchema(body []byte) bool {
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(body, &probe); err != nil {
		return false
	}
	_, ok := probe["properties"]
	return ok
}
