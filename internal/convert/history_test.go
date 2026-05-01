package convert

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestAppendConversionHistory_AppendsJSONL(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	first := HistoryRecord{
		Source: "src/a.test.js", From: "jest", To: "vitest",
		Mode: "file", Validated: true, ConvertedCount: 1,
	}
	if err := AppendConversionHistory(root, first); err != nil {
		t.Fatalf("first append: %v", err)
	}

	second := HistoryRecord{
		Source: "src/b.test.js", From: "jest", To: "vitest",
		Mode: "file", Validated: false, ConvertedCount: 1,
	}
	if err := AppendConversionHistory(root, second); err != nil {
		t.Fatalf("second append: %v", err)
	}

	logPath := filepath.Join(root, ".terrain", "conversion-history", "log.jsonl")
	f, err := os.Open(logPath)
	if err != nil {
		t.Fatalf("open log: %v", err)
	}
	defer f.Close()

	var records []HistoryRecord
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var rec HistoryRecord
		if err := json.Unmarshal(sc.Bytes(), &rec); err != nil {
			t.Fatalf("decode line: %v", err)
		}
		records = append(records, rec)
	}
	if len(records) != 2 {
		t.Fatalf("got %d records, want 2", len(records))
	}
	if records[0].Source != "src/a.test.js" || records[1].Source != "src/b.test.js" {
		t.Errorf("source mismatch: %+v", records)
	}
	if !records[0].Validated || records[1].Validated {
		t.Errorf("validated mismatch: %+v", records)
	}
}

func TestAppendConversionHistory_RejectsEmptyRoot(t *testing.T) {
	t.Parallel()
	if err := AppendConversionHistory("", HistoryRecord{}); err == nil {
		t.Error("expected error on empty root")
	}
}

func TestHistoryRecordFromExecution(t *testing.T) {
	t.Parallel()

	exec := ExecutionResult{
		Source: "src/x.test.js",
		Output: "out/x.test.js",
		Mode:   "file",
		Direction: Direction{
			From: "jest",
			To:   "vitest",
		},
		ValidationMode: "strict",
		Validated:      true,
		ConvertedCount: 1,
		Files: []FileResult{
			{SourcePath: "src/x.test.js", OutputPath: "out/x.test.js",
				Status: "converted", ItemsCovered: 4, ItemsLossy: 0, Confidence: 1.0},
		},
		Warnings: []string{"deprecated assertion replaced"},
	}
	rec := HistoryRecordFromExecution(exec, "0.2.0")
	if rec.From != "jest" || rec.To != "vitest" {
		t.Errorf("direction wrong: %+v", rec)
	}
	if !rec.Validated || rec.ConvertedCount != 1 {
		t.Errorf("metadata wrong: %+v", rec)
	}
	if len(rec.Files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(rec.Files))
	}
	if rec.Files[0].Confidence != 1.0 || rec.Files[0].ItemsCovered != 4 {
		t.Errorf("confidence not propagated: %+v", rec.Files[0])
	}
	if len(rec.Warnings) != 1 {
		t.Errorf("warnings not propagated")
	}
	if rec.TerrainVersion != "0.2.0" {
		t.Errorf("version not stamped")
	}
	if rec.Timestamp.IsZero() {
		t.Errorf("timestamp not set")
	}
}
