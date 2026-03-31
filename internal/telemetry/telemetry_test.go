package telemetry

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSizeBand(t *testing.T) {
	tests := []struct {
		files int
		want  string
	}{
		{0, "empty"},
		{1, "small"},
		{20, "small"},
		{21, "medium"},
		{100, "medium"},
		{101, "large"},
		{500, "large"},
		{501, "xlarge"},
	}
	for _, tt := range tests {
		if got := SizeBand(tt.files); got != tt.want {
			t.Errorf("SizeBand(%d) = %q, want %q", tt.files, got, tt.want)
		}
	}
}

func TestSaveAndLoadConfig(t *testing.T) {
	dir := t.TempDir()

	// Override the config directory for testing.
	origDir := configDir
	configDir = dir
	defer func() { configDir = origDir }()

	// Initially disabled.
	cfg := LoadConfig()
	if cfg.Enabled {
		t.Error("expected disabled by default")
	}

	// Enable and save.
	if err := SaveConfig(Config{Enabled: true}); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}

	cfg = LoadConfig()
	if !cfg.Enabled {
		t.Error("expected enabled after save")
	}

	// Disable and save.
	if err := SaveConfig(Config{Enabled: false}); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}

	cfg = LoadConfig()
	if cfg.Enabled {
		t.Error("expected disabled after second save")
	}
}

func TestRecord_DisabledDoesNothing(t *testing.T) {
	dir := t.TempDir()
	origDir := configDir
	configDir = dir
	defer func() { configDir = origDir }()

	// Telemetry disabled — record should do nothing.
	Record(Event{Command: "analyze", Timestamp: time.Now()})

	logPath := filepath.Join(dir, "telemetry.jsonl")
	if _, err := os.Stat(logPath); err == nil {
		t.Error("telemetry log should not exist when disabled")
	}
}

func TestRecord_EnabledWritesEvent(t *testing.T) {
	dir := t.TempDir()
	origDir := configDir
	configDir = dir
	defer func() { configDir = origDir }()

	if err := SaveConfig(Config{Enabled: true}); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}

	evt := Event{
		Command:    "analyze",
		SizeBand:   "medium",
		Languages:  []string{"js"},
		Signals:    5,
		DurationMs: 1234,
		Version:    "test",
		Timestamp:  time.Now(),
	}
	Record(evt)

	logPath := filepath.Join(dir, "telemetry.jsonl")
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read telemetry log: %v", err)
	}

	var decoded Event
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal event: %v", err)
	}
	if decoded.Command != "analyze" {
		t.Errorf("command = %q, want analyze", decoded.Command)
	}
	if decoded.SizeBand != "medium" {
		t.Errorf("sizeBand = %q, want medium", decoded.SizeBand)
	}
	if decoded.Signals != 5 {
		t.Errorf("signals = %d, want 5", decoded.Signals)
	}
}

func TestEnvOverride(t *testing.T) {
	dir := t.TempDir()
	origDir := configDir
	configDir = dir
	defer func() { configDir = origDir }()

	// File says disabled, env says enabled.
	if err := SaveConfig(Config{Enabled: false}); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}

	t.Setenv("TERRAIN_TELEMETRY", "on")
	cfg := LoadConfig()
	if !cfg.Enabled {
		t.Error("env override should enable telemetry")
	}

	t.Setenv("TERRAIN_TELEMETRY", "off")
	cfg = LoadConfig()
	if cfg.Enabled {
		t.Error("env override 'off' should disable telemetry")
	}
}
