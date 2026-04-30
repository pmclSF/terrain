package signals

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestManifestExport_RoundTripsSelf verifies the generated JSON parses back
// into the export struct without loss. Catches accidental field-tag drift.
func TestManifestExport_RoundTripsSelf(t *testing.T) {
	t.Parallel()

	data, err := MarshalManifestJSON()
	if err != nil {
		t.Fatalf("MarshalManifestJSON: %v", err)
	}

	var decoded ManifestExport
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal own output: %v", err)
	}

	if decoded.SchemaVersion != CurrentManifestSchemaVersion {
		t.Errorf("schemaVersion = %q, want %q", decoded.SchemaVersion, CurrentManifestSchemaVersion)
	}
	if got, want := len(decoded.Entries), len(allSignalManifest); got != want {
		t.Errorf("entry count: got %d, want %d", got, want)
	}
}

// TestManifestExport_StableEntriesHaveRuleURI is the 0.2 tightening of the
// manifest contract: an entry with status=stable must declare where its
// rule documentation lives. Experimental and planned entries may leave
// RuleURI blank while the docs are being written.
func TestManifestExport_StableEntriesHaveRuleURI(t *testing.T) {
	t.Parallel()

	for _, e := range allSignalManifest {
		if e.Status != StatusStable {
			continue
		}
		if strings.TrimSpace(e.RuleURI) == "" {
			t.Errorf("stable entry %q has empty RuleURI", e.Type)
		}
	}
}

// TestManifestExport_TerminatesWithNewline guards against the file we
// commit losing its trailing newline. Editors and JSON formatters disagree
// on this; the export helper appends one explicitly.
func TestManifestExport_TerminatesWithNewline(t *testing.T) {
	t.Parallel()

	data, err := MarshalManifestJSON()
	if err != nil {
		t.Fatalf("MarshalManifestJSON: %v", err)
	}
	if len(data) == 0 || data[len(data)-1] != '\n' {
		t.Error("MarshalManifestJSON output does not end with a newline")
	}
}
