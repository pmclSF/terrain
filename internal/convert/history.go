package convert

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// HistoryRecord is one entry in the conversion audit trail. Each
// terrain convert run that produces real output appends a record to
// `.terrain/conversion-history/log.jsonl`. Closes the round-4 finding
// "`.terrain/conversion-history/` for audit trail".
type HistoryRecord struct {
	Timestamp      time.Time              `json:"timestamp"`
	Source         string                 `json:"source"`
	Output         string                 `json:"output,omitempty"`
	From           string                 `json:"from"`
	To             string                 `json:"to"`
	Mode           string                 `json:"mode"`
	ValidationMode string                 `json:"validationMode,omitempty"`
	Validated      bool                   `json:"validated"`
	ConvertedCount int                    `json:"convertedCount"`
	UnchangedCount int                    `json:"unchangedCount,omitempty"`
	Files          []HistoryFileRecord    `json:"files,omitempty"`
	Warnings       []string               `json:"warnings,omitempty"`
	TerrainVersion string                 `json:"terrainVersion,omitempty"`
}

// HistoryFileRecord trims the per-file information so the audit log
// stays compact. Carries the confidence metrics so reviewers can spot
// lossy conversions in history without re-running.
type HistoryFileRecord struct {
	SourcePath   string  `json:"sourcePath"`
	OutputPath   string  `json:"outputPath,omitempty"`
	Status       string  `json:"status,omitempty"`
	ItemsCovered int     `json:"itemsCovered,omitempty"`
	ItemsLossy   int     `json:"itemsLossy,omitempty"`
	Confidence   float64 `json:"confidence,omitempty"`
}

// AppendConversionHistory writes one HistoryRecord to
// `<repoRoot>/.terrain/conversion-history/log.jsonl`. The destination
// directory is created if missing. Each line is a single
// JSON-encoded record (JSONL).
//
// Reasonable failures (missing repoRoot, write permission, etc.) are
// returned to the caller; the convert flow logs them as warnings
// rather than aborting — the user's conversion already succeeded by
// the time we get here.
//
// Determining the repo root is the caller's job; we use the source
// path's directory if no explicit root is supplied.
func AppendConversionHistory(repoRoot string, rec HistoryRecord) error {
	if repoRoot == "" {
		return fmt.Errorf("AppendConversionHistory: empty repo root")
	}
	dir := filepath.Join(repoRoot, ".terrain", "conversion-history")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", dir, err)
	}
	path := filepath.Join(dir, "log.jsonl")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(rec); err != nil {
		return fmt.Errorf("encode record: %w", err)
	}
	return nil
}

// HistoryRecordFromExecution distills the full ExecutionResult into the
// trim shape we want to keep in the audit log. The full execution
// result can be many KB on a batch convert; we keep only the auditing
// essentials and drop converter-internal scaffolding (Direction etc.).
func HistoryRecordFromExecution(exec ExecutionResult, terrainVersion string) HistoryRecord {
	rec := HistoryRecord{
		Timestamp:      time.Now().UTC(),
		Source:         exec.Source,
		Output:         exec.Output,
		From:           exec.Direction.From,
		To:             exec.Direction.To,
		Mode:           exec.Mode,
		ValidationMode: exec.ValidationMode,
		Validated:      exec.Validated,
		ConvertedCount: exec.ConvertedCount,
		UnchangedCount: exec.UnchangedCount,
		Warnings:       append([]string(nil), exec.Warnings...),
		TerrainVersion: terrainVersion,
	}
	for _, f := range exec.Files {
		rec.Files = append(rec.Files, HistoryFileRecord{
			SourcePath:   f.SourcePath,
			OutputPath:   f.OutputPath,
			Status:       f.Status,
			ItemsCovered: f.ItemsCovered,
			ItemsLossy:   f.ItemsLossy,
			Confidence:   f.Confidence,
		})
	}
	return rec
}
