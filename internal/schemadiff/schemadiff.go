// Package schemadiff reports field-level changes between two schema
// documents. Used downstream to surface "this PR renames a schema
// field your prompt template references" findings.
//
// Today the package supports JSON Schema documents whose top-level
// shape includes a `properties` map.
package schemadiff

import (
	"encoding/json"
	"sort"
	"strings"
)

// ChangeKind classifies one field-level schema change.
type ChangeKind int

const (
	// ChangeUnknown means the change kind could not be determined.
	ChangeUnknown ChangeKind = iota
	// ChangeAdded means the field exists in the new schema but not the old.
	ChangeAdded
	// ChangeRemoved means the field exists in the old schema but not the new.
	ChangeRemoved
	// ChangeTypeChanged means the field exists in both schemas but its
	// declared type differs.
	ChangeTypeChanged
)

// String returns a short label for k, used in diagnostics and test
// output.
func (k ChangeKind) String() string {
	switch k {
	case ChangeAdded:
		return "added"
	case ChangeRemoved:
		return "removed"
	case ChangeTypeChanged:
		return "type-changed"
	default:
		return "unknown"
	}
}

// Change is a single field-level schema difference. OldType / NewType
// are normalized: a single `type: "string"` renders as `string`; a
// `type: ["string", "null"]` array renders as `string|null` (sorted
// for stable comparison).
type Change struct {
	Kind    ChangeKind
	Field   string
	OldType string
	NewType string
}

type jsonSchema struct {
	Properties map[string]jsonProperty `json:"properties"`
}

// jsonProperty captures Type as a RawMessage so we can accept either
// the string form (`"type": "object"`) or the array form
// (`"type": ["string", "null"]`) without rejecting nullable schemas.
type jsonProperty struct {
	Type json.RawMessage `json:"type"`
}

// normalizeType renders the value of `properties.<field>.type` as a
// stable string. Accepts:
//   - a single string ("string")
//   - an array of strings (["string", "null"]) → "string|null" with
//     the array sorted so reorderings don't show as type changes
//   - absent / null / unparseable → "" (the type is unknown and the
//     diff falls back to "different OR not")
func normalizeType(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var single string
	if err := json.Unmarshal(raw, &single); err == nil {
		return single
	}
	var list []string
	if err := json.Unmarshal(raw, &list); err == nil {
		// Sort for stable comparison so ["null","string"] and
		// ["string","null"] are the same type.
		sort.Strings(list)
		return strings.Join(list, "|")
	}
	return ""
}

// NormalizeType renders a JSON-Schema `type` value (the raw bytes of
// `properties.<field>.type`) as a stable string, accepting both the string
// form and the nullable/union array form. Exported so consumers that parse the
// same property shape (e.g. promptflow's value synthesis) reuse one parser
// instead of a divergent second one.
func NormalizeType(raw json.RawMessage) string {
	return normalizeType(raw)
}

// DiffJSONSchema returns the field-level changes between oldDoc and
// newDoc. Both must be JSON Schema documents whose top-level shape
// includes a `properties` map. Results are sorted by Field for stable
// downstream consumers.
func DiffJSONSchema(oldDoc, newDoc []byte) ([]Change, error) {
	var o, n jsonSchema
	if err := json.Unmarshal(oldDoc, &o); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(newDoc, &n); err != nil {
		return nil, err
	}
	var changes []Change
	for name, newProp := range n.Properties {
		newType := normalizeType(newProp.Type)
		oldProp, ok := o.Properties[name]
		if !ok {
			changes = append(changes, Change{
				Kind:    ChangeAdded,
				Field:   name,
				NewType: newType,
			})
			continue
		}
		oldType := normalizeType(oldProp.Type)
		if oldType != newType {
			changes = append(changes, Change{
				Kind:    ChangeTypeChanged,
				Field:   name,
				OldType: oldType,
				NewType: newType,
			})
		}
	}
	for name, oldProp := range o.Properties {
		if _, ok := n.Properties[name]; !ok {
			changes = append(changes, Change{
				Kind:    ChangeRemoved,
				Field:   name,
				OldType: normalizeType(oldProp.Type),
			})
		}
	}
	sort.Slice(changes, func(i, j int) bool {
		return changes[i].Field < changes[j].Field
	})
	return changes, nil
}
