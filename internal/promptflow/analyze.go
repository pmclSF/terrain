package promptflow

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/pmclSF/terrain/internal/schemadiff"
)

// Finding is one (template, schema-change) risk surfaced during a
// PR-time analysis, together with the before/after rendered prompts
// that show what changes for callers.
type Finding struct {
	TemplatePath   string
	SchemaPath     string
	Risk           Risk
	RenderedBefore string
	RenderedAfter  string
}

// Analyze runs the full pipeline against an after-state Discoveries
// and a map of before-state schema bodies keyed by repo-relative path.
//
// Returns one Finding per (template, schema-change) pair. Findings are
// sorted by TemplatePath, then by Risk.Variable.
//
// Templates whose body or vars don't intersect any schema change
// produce no findings. Schemas absent from before produce no findings
// (a brand-new schema can't break a pre-existing template
// reference — its variables haven't existed before).
func Analyze(after Discoveries, before map[string][]byte) ([]Finding, error) {
	var findings []Finding
	// Precompute each template's Vars() once, not once per schema in
	// the inner loop. Vars() walks the body; without hoisting, the
	// cost is O(schemas × templates × body_length).
	templateVars := make([][]string, len(after.Templates))
	for i, tf := range after.Templates {
		templateVars[i] = tf.Tpl.Vars()
	}
	for _, schema := range after.Schemas {
		// The before map is keyed by forward-slash path so lookups
		// work cross-platform. schema.Path may have OS separators
		// (backslashes on Windows) — normalize before the lookup.
		beforeBody, ok := before[filepath.ToSlash(schema.Path)]
		if !ok {
			continue
		}
		changes, err := schemadiff.DiffJSONSchema(beforeBody, schema.Body)
		if err != nil {
			return nil, fmt.Errorf("diff %s: %w", schema.Path, err)
		}
		if len(changes) == 0 {
			continue
		}
		var beforeProps, afterProps map[string]string
		for i, tf := range after.Templates {
			vars := templateVars[i]
			risks := CorrelateVars(vars, changes)
			if len(risks) == 0 {
				continue
			}
			// Lazy-parse property types only when we have a finding to
			// emit — most (schema, template) pairs intersect empty.
			if beforeProps == nil {
				beforeProps = parsePropertyTypes(beforeBody)
				afterProps = parsePropertyTypes(schema.Body)
			}
			beforeVars := makeVars(vars, beforeProps)
			afterVars := makeVars(vars, afterProps)
			renderedBefore, _ := tf.Tpl.Render(beforeVars)
			renderedAfter, _ := tf.Tpl.Render(afterVars)
			for _, r := range risks {
				findings = append(findings, Finding{
					TemplatePath:   tf.Path,
					SchemaPath:     schema.Path,
					Risk:           r,
					RenderedBefore: renderedBefore,
					RenderedAfter:  renderedAfter,
				})
			}
		}
	}
	sort.Slice(findings, func(i, j int) bool {
		if findings[i].TemplatePath != findings[j].TemplatePath {
			return findings[i].TemplatePath < findings[j].TemplatePath
		}
		return findings[i].Risk.Variable < findings[j].Risk.Variable
	})
	return findings, nil
}

func parsePropertyTypes(body []byte) map[string]string {
	out := map[string]string{}
	var doc struct {
		Properties map[string]struct {
			Type string `json:"type"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return out
	}
	for name, prop := range doc.Properties {
		out[name] = prop.Type
	}
	return out
}

// makeVars produces a vars map for the renderer. Variables that exist
// as schema properties get a synthesized value matching the declared
// type; variables that don't exist get a `MISSING(<name>)` marker so
// the renderer never errors and the missing-ness is visible in the
// rendered output.
func makeVars(vars []string, props map[string]string) map[string]string {
	out := make(map[string]string, len(vars))
	for _, v := range vars {
		if typ, ok := props[v]; ok {
			out[v] = synthesize(typ)
		} else {
			out[v] = fmt.Sprintf("MISSING(%s)", v)
		}
	}
	return out
}

func synthesize(typ string) string {
	switch typ {
	case "string":
		return "example_string"
	case "integer":
		return "42"
	case "number":
		return "3.14"
	case "boolean":
		return "true"
	default:
		return "example"
	}
}

// RenderFinding produces a markdown block describing one finding for
// inclusion in a PR comment. The format is intentionally simple and
// self-contained; callers that prefer the curated label / slash-hint
// vocabulary route through the prtemplates registry instead.
func RenderFinding(f Finding) string {
	var b strings.Builder
	fmt.Fprintf(&b, "### %s — prompt template at risk\n\n", labelFor(f.Risk.Change.Kind))
	fmt.Fprintf(&b, "- Template: `%s`\n", f.TemplatePath)
	fmt.Fprintf(&b, "- Schema: `%s`\n", f.SchemaPath)
	fmt.Fprintf(&b, "- Variable: `%s` (%s)\n\n", f.Risk.Variable, changeDetail(f.Risk.Change))
	b.WriteString("**Rendered before:**\n```\n")
	b.WriteString(f.RenderedBefore)
	b.WriteString("\n```\n\n")
	b.WriteString("**Rendered after:**\n```\n")
	b.WriteString(f.RenderedAfter)
	b.WriteString("\n```\n\n")
	fmt.Fprintf(&b, "Suggested actions: `/dismiss reason:<text>` · `/terrain explain s2PromptSchemaDrift`\n")
	return b.String()
}

func labelFor(kind schemadiff.ChangeKind) string {
	switch kind {
	case schemadiff.ChangeRemoved:
		return "Schema field removed"
	case schemadiff.ChangeTypeChanged:
		return "Schema field type changed"
	case schemadiff.ChangeAdded:
		return "Schema field added"
	default:
		return "Schema change"
	}
}

func changeDetail(c schemadiff.Change) string {
	switch c.Kind {
	case schemadiff.ChangeRemoved:
		return fmt.Sprintf("was %s", c.OldType)
	case schemadiff.ChangeTypeChanged:
		return fmt.Sprintf("%s → %s", c.OldType, c.NewType)
	case schemadiff.ChangeAdded:
		return fmt.Sprintf("now %s", c.NewType)
	default:
		return "changed"
	}
}
