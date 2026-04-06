package convert

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ConfigMigrationOptions controls native config migration execution.
type ConfigMigrationOptions struct {
	From           string `json:"from,omitempty"`
	To             string `json:"to,omitempty"`
	Output         string `json:"output,omitempty"`
	DryRun         bool   `json:"dryRun,omitempty"`
	ValidateSyntax bool   `json:"validateSyntax,omitempty"`
}

// ConfigMigrationResult is the native Terrain result for a config migration request.
type ConfigMigrationResult struct {
	Command          string    `json:"command"`
	Source           string    `json:"source"`
	Output           string    `json:"output,omitempty"`
	Mode             string    `json:"mode"`
	From             string    `json:"from"`
	To               string    `json:"to"`
	AutoDetected     bool      `json:"autoDetected,omitempty"`
	DryRun           bool      `json:"dryRun,omitempty"`
	Direction        Direction `json:"direction"`
	Status           string    `json:"status"`
	ConvertedContent string    `json:"-"`
}

// RunConfigMigration executes a native config migration request.
func RunConfigMigration(source string, options ConfigMigrationOptions) (ConfigMigrationResult, error) {
	source = strings.TrimSpace(source)
	if source == "" {
		return ConfigMigrationResult{}, inputErrorf("source is required")
	}

	to := NormalizeFramework(options.To)
	if to == "" {
		return ConfigMigrationResult{}, inputErrorf("target framework is required")
	}

	from := NormalizeFramework(options.From)
	autoDetected := false
	if from == "" {
		from = DetectConfigFramework(source)
		autoDetected = from != ""
		if from == "" {
			return ConfigMigrationResult{}, inputErrorf("could not auto-detect source framework from config filename; use --from <framework>")
		}
	}

	direction, err := resolveSupportedDirection(from, to)
	if err != nil {
		return ConfigMigrationResult{}, err
	}
	if !SupportsConfigConversion(direction.From, direction.To) {
		return ConfigMigrationResult{}, fmt.Errorf("go-native config conversion is not implemented for %s -> %s", direction.From, direction.To)
	}

	content, err := os.ReadFile(source)
	if err != nil {
		return ConfigMigrationResult{}, fmt.Errorf("read config: %w", err)
	}
	converted, err := ConvertConfig(string(content), direction.From, direction.To)
	if err != nil {
		return ConfigMigrationResult{}, err
	}

	outputPath, err := resolveConfigOutputPath(source, options.Output)
	if err != nil {
		return ConfigMigrationResult{}, err
	}

	result := ConfigMigrationResult{
		Command:      "convert-config",
		Source:       source,
		Output:       outputPath,
		Mode:         "stdout",
		From:         direction.From,
		To:           direction.To,
		AutoDetected: autoDetected,
		DryRun:       options.DryRun,
		Direction:    direction,
		Status:       "printed",
	}

	if outputPath != "" {
		result.Mode = "file"
		result.Status = "written"
	}
	if options.DryRun {
		result.Mode = "dry-run"
		result.Status = "previewed"
		return result, nil
	}

	validationPath := outputPath
	if validationPath == "" {
		validationPath = TargetConfigFileName(direction.To, filepath.Base(source))
	}
	if err := ValidateConvertedOutput(validationPath, direction, converted); err != nil {
		return ConfigMigrationResult{}, err
	}

	if outputPath == "" {
		result.ConvertedContent = converted
		return result, nil
	}

	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return ConfigMigrationResult{}, fmt.Errorf("create output directory: %w", err)
	}
	if err := os.WriteFile(outputPath, []byte(converted), 0o644); err != nil {
		return ConfigMigrationResult{}, fmt.Errorf("write config output: %w", err)
	}
	return result, nil
}

func resolveConfigOutputPath(source, output string) (string, error) {
	output = strings.TrimSpace(output)
	if output == "" {
		return "", nil
	}
	if info, err := os.Stat(output); err == nil && info.IsDir() {
		return filepath.Join(output, filepath.Base(source)), nil
	}
	if strings.HasSuffix(output, string(os.PathSeparator)) {
		return filepath.Join(output, filepath.Base(source)), nil
	}
	return output, nil
}
