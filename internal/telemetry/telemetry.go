// Package telemetry provides opt-in, privacy-respecting usage tracking.
//
// Data is written to a local JSONL file (~/.terrain/telemetry.jsonl) and
// never sent anywhere without explicit user action. Users can opt in/out
// via `terrain telemetry --on|--off` or the TERRAIN_TELEMETRY env var.
//
// What is recorded (per command invocation):
//   - Command name (e.g., "analyze", "impact")
//   - Repository size band (small/medium/large — never the path)
//   - Languages detected
//   - Signal count
//   - Execution duration
//   - Terrain version
//   - Timestamp
//
// What is NOT recorded:
//   - File paths, file contents, repo URLs
//   - User names, emails, or any PII
//   - Signal details or finding text
package telemetry

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Event is a single telemetry record.
type Event struct {
	Timestamp  time.Time `json:"ts"`
	Version    string    `json:"version"`
	Command    string    `json:"command"`
	SizeBand   string    `json:"sizeBand"`
	Languages  []string  `json:"languages,omitempty"`
	Signals    int       `json:"signals"`
	DurationMs int64     `json:"durationMs"`
}

// Config holds telemetry preferences.
type Config struct {
	Enabled bool `json:"enabled"`
}

// configDir can be overridden for testing. When empty, terrainDir()
// computes it from the user's home directory.
var configDir string

func terrainDir() string {
	if configDir != "" {
		return configDir
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".terrain")
}

// LoadConfig reads the telemetry preference from ~/.terrain/telemetry.json.
// Returns disabled config if the file doesn't exist or can't be read.
func LoadConfig() Config {
	// Environment variable overrides file config.
	if env := os.Getenv("TERRAIN_TELEMETRY"); env != "" {
		return Config{Enabled: env == "1" || env == "on" || env == "true"}
	}

	dir := terrainDir()
	if dir == "" {
		return Config{}
	}

	data, err := os.ReadFile(filepath.Join(dir, "telemetry.json"))
	if err != nil {
		return Config{}
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}
	}
	return cfg
}

// SaveConfig writes the telemetry preference to ~/.terrain/telemetry.json.
func SaveConfig(cfg Config) error {
	dir := terrainDir()
	if dir == "" {
		return nil
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	// 0o600 keeps telemetry config (and the existence of telemetry, which
	// is itself a privacy signal on shared dev hosts) readable only by the
	// owning user. The directory is also locked down to 0o700 above.
	return os.WriteFile(filepath.Join(dir, "telemetry.json"), data, 0o600)
}

// Record appends an event to the local telemetry log.
// Does nothing if telemetry is disabled or the directory is unavailable.
// Never returns an error — telemetry failures are silently ignored.
func Record(evt Event) {
	if !LoadConfig().Enabled {
		return
	}
	dir := terrainDir()
	if dir == "" {
		return
	}
	_ = os.MkdirAll(dir, 0o700)

	// 0o600 mirrors the config file; the event log can leak repo-size
	// bands and command-name patterns to other users on a shared host
	// otherwise. See SECURITY.md for the full privacy threat model.
	f, err := os.OpenFile(filepath.Join(dir, "telemetry.jsonl"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	defer f.Close()

	data, err := json.Marshal(evt)
	if err != nil {
		return
	}
	data = append(data, '\n')
	_, _ = f.Write(data)
}

// SizeBand categorizes a test file count into a privacy-safe band.
func SizeBand(testFiles int) string {
	switch {
	case testFiles == 0:
		return "empty"
	case testFiles <= 20:
		return "small"
	case testFiles <= 100:
		return "medium"
	case testFiles <= 500:
		return "large"
	default:
		return "xlarge"
	}
}

// Status returns a human-readable description of the current telemetry state.
func Status() string {
	cfg := LoadConfig()
	if cfg.Enabled {
		dir := terrainDir()
		logPath := filepath.Join(dir, "telemetry.jsonl")
		info, err := os.Stat(logPath)
		if err == nil {
			return "enabled (" + logPath + ", " + formatSize(info.Size()) + ")"
		}
		return "enabled (no events recorded yet)"
	}
	return "disabled"
}

func formatSize(bytes int64) string {
	switch {
	case bytes < 1024:
		return "< 1 KB"
	case bytes < 1024*1024:
		return fmt.Sprintf("%d KB", bytes/1024)
	default:
		return fmt.Sprintf("%d MB", bytes/(1024*1024))
	}
}
