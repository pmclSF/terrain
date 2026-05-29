// Package schemadiff reports field-level changes between two schema
// documents. Slice 2 of the S2 surface — used downstream to surface
// "this PR renames a schema field your prompt template references"
// findings.
//
// JSON Schema only at this slice. Pydantic models, TypeScript
// interfaces, protobuf, GraphQL SDL are follow-up slices.
package schemadiff

import (
	"encoding/json"
	"sort"
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

// Change is a single field-level schema difference.
type Change struct {
	Kind    ChangeKind
	Field   string
	OldType string
	NewType string
}

type jsonSchema struct {
	Properties map[string]jsonProperty `json:"properties"`
}

type jsonProperty struct {
	Type string `json:"type"`
}

// DiffJSONSchema returns the field-level changes between oldDoc and
// newDoc. Both must be JSON Schema documents whose top-level shape
// includes a `properties` map.
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
		oldProp, ok := o.Properties[name]
		if !ok {
			changes = append(changes, Change{
				Kind:    ChangeAdded,
				Field:   name,
				NewType: newProp.Type,
			})
			continue
		}
		if oldProp.Type != newProp.Type {
			changes = append(changes, Change{
				Kind:    ChangeTypeChanged,
				Field:   name,
				OldType: oldProp.Type,
				NewType: newProp.Type,
			})
		}
	}
	for name, oldProp := range o.Properties {
		if _, ok := n.Properties[name]; !ok {
			changes = append(changes, Change{
				Kind:    ChangeRemoved,
				Field:   name,
				OldType: oldProp.Type,
			})
		}
	}
	sort.Slice(changes, func(i, j int) bool {
		return changes[i].Field < changes[j].Field
	})
	return changes, nil
}
