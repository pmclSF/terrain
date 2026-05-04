package models

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Track 9.11 — Schema migration tests against a real 0.1.x fixture.
//
// MigrateSnapshotInPlace is exercised by migrate_test.go's synthetic
// in-memory cases, but the load-bearing question for adopters is:
// "if I have a snapshot Terrain wrote 6 months ago in 0.1.x, can I
// load it via 0.2's deserializer + migrator without losing data?"
//
// This test uses a hand-crafted JSON fixture whose shape matches
// what 0.1.x actually wrote — schema version field absent, no
// SignalV2 envelope on signals, no UnitID on code units, simpler
// snapshot meta. The contract: load via json.Unmarshal, migrate
// via MigrateSnapshotInPlace, end state has the legacy schema
// version stamped, code unit IDs backfilled, generatedAt
// backfilled from repository.snapshotTimestamp.

const legacyFixturePath = "testdata/snapshot_v0_1_x_legacy.json"

func TestMigrateSnapshot_LoadLegacyFixture(t *testing.T) {
	t.Parallel()
	data := mustReadFixture(t, legacyFixturePath)

	var snap TestSuiteSnapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		t.Fatalf("unmarshal legacy fixture: %v", err)
	}

	// Pre-migration: schema version is empty (legacy snapshots
	// predate the field), generatedAt is empty (not in fixture).
	if snap.SnapshotMeta.SchemaVersion != "" {
		t.Errorf("legacy fixture should have empty SchemaVersion, got %q",
			snap.SnapshotMeta.SchemaVersion)
	}

	notes := MigrateSnapshotInPlace(&snap)
	if len(notes) == 0 {
		t.Error("expected at least one migration note for a legacy snapshot")
	}

	// Post-migration assertions.
	if snap.SnapshotMeta.SchemaVersion != LegacySnapshotSchemaVersion {
		t.Errorf("after migration, SchemaVersion = %q, want %q",
			snap.SnapshotMeta.SchemaVersion, LegacySnapshotSchemaVersion)
	}

	// generatedAt should be backfilled from repository.snapshotTimestamp.
	if snap.GeneratedAt.IsZero() {
		t.Errorf("after migration, GeneratedAt should be backfilled from repository.snapshotTimestamp")
	}
	if !snap.GeneratedAt.Equal(snap.Repository.SnapshotTimestamp) {
		t.Errorf("GeneratedAt (%v) should equal Repository.SnapshotTimestamp (%v)",
			snap.GeneratedAt, snap.Repository.SnapshotTimestamp)
	}

	// Code units should have UnitIDs backfilled. The fixture
	// declares 3 code units, none with UnitIDs.
	if len(snap.CodeUnits) != 3 {
		t.Fatalf("CodeUnits count = %d, want 3", len(snap.CodeUnits))
	}
	for i, cu := range snap.CodeUnits {
		if cu.UnitID == "" {
			t.Errorf("CodeUnits[%d] (%s.%s) UnitID not backfilled", i, cu.Path, cu.Name)
		}
	}

	// The session.ts code unit has a ParentName ("SessionManager")
	// — the legacy ID format includes it via "Path:Parent.Name".
	for _, cu := range snap.CodeUnits {
		if cu.Name == "createSession" {
			want := "src/auth/session.ts:SessionManager.createSession"
			if cu.UnitID != want {
				t.Errorf("createSession UnitID = %q, want %q", cu.UnitID, want)
			}
		}
	}

	// Compatibility notes should be stamped into Metadata so
	// downstream consumers can surface them.
	if snap.Metadata == nil {
		t.Fatal("Metadata should be populated with compatibilityNotes")
	}
	notesAny, ok := snap.Metadata["compatibilityNotes"]
	if !ok {
		t.Fatal("Metadata.compatibilityNotes missing")
	}
	notesSlice, ok := notesAny.([]string)
	if !ok {
		t.Fatalf("compatibilityNotes is %T, want []string", notesAny)
	}
	if len(notesSlice) == 0 {
		t.Error("compatibilityNotes is empty")
	}
}

// TestMigrateSnapshot_LegacyFixtureDataPreserved verifies that the
// migration is purely additive — every field present in the legacy
// fixture (frameworks, test files, signals) survives intact. A
// regression in MigrateSnapshotInPlace that drops or rewrites
// existing data shows up here.
func TestMigrateSnapshot_LegacyFixtureDataPreserved(t *testing.T) {
	t.Parallel()
	data := mustReadFixture(t, legacyFixturePath)

	var snap TestSuiteSnapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	_ = MigrateSnapshotInPlace(&snap)

	if got := len(snap.Frameworks); got != 1 {
		t.Errorf("Frameworks count = %d, want 1 (jest)", got)
	}
	if snap.Frameworks[0].Name != "jest" {
		t.Errorf("Frameworks[0].Name = %q, want jest", snap.Frameworks[0].Name)
	}

	if got := len(snap.TestFiles); got != 2 {
		t.Errorf("TestFiles count = %d, want 2", got)
	}
	if snap.TestFiles[0].TestCount != 5 {
		t.Errorf("TestFiles[0].TestCount = %d, want 5", snap.TestFiles[0].TestCount)
	}

	if got := len(snap.Signals); got != 1 {
		t.Errorf("Signals count = %d, want 1", got)
	}
	if snap.Signals[0].Type != "untestedExport" {
		t.Errorf("Signals[0].Type = %q, want untestedExport", snap.Signals[0].Type)
	}
}

// TestMigrateSnapshot_FixtureRoundTripsViaJSON verifies the migrated
// snapshot can be re-serialized and loaded again without further
// changes (idempotency check). A regression where MigrateSnapshotInPlace
// produces a snapshot that wouldn't survive its own round-trip would
// make `terrain analyze --write-snapshot` followed by a comparison
// run produce different bytes — silently breaking byte-identical
// determinism.
func TestMigrateSnapshot_FixtureRoundTripsViaJSON(t *testing.T) {
	t.Parallel()
	data := mustReadFixture(t, legacyFixturePath)

	var snap TestSuiteSnapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	_ = MigrateSnapshotInPlace(&snap)

	// Serialize, re-deserialize, re-migrate.
	out, err := json.Marshal(&snap)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var snap2 TestSuiteSnapshot
	if err := json.Unmarshal(out, &snap2); err != nil {
		t.Fatalf("re-unmarshal: %v", err)
	}
	notes := MigrateSnapshotInPlace(&snap2)

	// The second migration should produce no new notes (the
	// snapshot is already at the legacy schema version with code
	// unit IDs backfilled). One note allowance: the
	// older-than-current-runtime note may or may not fire
	// depending on whether 0.0.0 < current major.
	tolerableNotes := 1
	if len(notes) > tolerableNotes {
		t.Errorf("re-migration should be near-idempotent, got %d notes: %v",
			len(notes), notes)
	}

	// Code unit IDs should match between the two.
	if len(snap.CodeUnits) != len(snap2.CodeUnits) {
		t.Fatalf("CodeUnits count diverged across round-trip: %d vs %d",
			len(snap.CodeUnits), len(snap2.CodeUnits))
	}
	for i := range snap.CodeUnits {
		if snap.CodeUnits[i].UnitID != snap2.CodeUnits[i].UnitID {
			t.Errorf("CodeUnits[%d].UnitID diverged: %q vs %q",
				i, snap.CodeUnits[i].UnitID, snap2.CodeUnits[i].UnitID)
		}
	}
}

func mustReadFixture(t *testing.T, rel string) []byte {
	t.Helper()
	path := filepath.Clean(rel)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture %s: %v", path, err)
	}
	if !strings.Contains(string(data), "snapshotMeta") {
		t.Fatalf("fixture %s does not look like a snapshot (missing snapshotMeta)", path)
	}
	return data
}
