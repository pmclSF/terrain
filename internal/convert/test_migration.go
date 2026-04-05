package convert

import (
	"fmt"
	"strings"
)

// ConversionInputError reports invalid user-supplied conversion parameters.
type ConversionInputError struct {
	message string
}

func (e ConversionInputError) Error() string {
	return e.message
}

// TestMigrationOptions controls one-off native test migration execution.
type TestMigrationOptions struct {
	Alias             string `json:"alias,omitempty"`
	From              string `json:"from,omitempty"`
	To                string `json:"to,omitempty"`
	Output            string `json:"output,omitempty"`
	PreserveStructure bool   `json:"preserveStructure,omitempty"`
	BatchSize         int    `json:"batchSize,omitempty"`
	Concurrency       int    `json:"concurrency,omitempty"`
	AutoDetect        bool   `json:"autoDetect,omitempty"`
	ValidateSyntax    bool   `json:"validateSyntax,omitempty"`
	Plan              bool   `json:"plan,omitempty"`
	DryRun            bool   `json:"dryRun,omitempty"`
}

// TestMigrationPlan describes a native conversion plan or dry-run preview.
type TestMigrationPlan struct {
	Command         string     `json:"command"`
	Mode            string     `json:"mode"`
	Source          string     `json:"source"`
	Output          string     `json:"output,omitempty"`
	Alias           string     `json:"alias,omitempty"`
	Direction       Direction  `json:"direction"`
	SourceDetection *Detection `json:"sourceDetection,omitempty"`
	ExecutionStatus string     `json:"executionStatus"`
	NextStep        string     `json:"nextStep"`
}

// TestMigrationResult is the native Terrain result for a single test migration request.
type TestMigrationResult struct {
	Source          string             `json:"source"`
	Alias           string             `json:"alias,omitempty"`
	Direction       Direction          `json:"direction"`
	SourceDetection *Detection         `json:"sourceDetection,omitempty"`
	Plan            *TestMigrationPlan `json:"plan,omitempty"`
	Execution       *ExecutionResult   `json:"execution,omitempty"`
}

// RunTestMigration plans or executes a single native test migration request.
func RunTestMigration(source string, options TestMigrationOptions) (TestMigrationResult, error) {
	source = strings.TrimSpace(source)
	if source == "" {
		return TestMigrationResult{}, inputErrorf("source is required")
	}

	direction, detection, alias, err := resolveTestMigrationDirection(source, options)
	if err != nil {
		return TestMigrationResult{}, err
	}

	result := TestMigrationResult{
		Source:          source,
		Alias:           alias,
		Direction:       direction,
		SourceDetection: detection,
	}

	if options.Plan || options.DryRun {
		plan := buildTestMigrationPlan(source, direction, detection, alias, options)
		result.Plan = &plan
		return result, nil
	}

	if direction.GoNativeState != GoNativeStateImplemented {
		return result, fmt.Errorf(
			"go-native conversion execution for %s -> %s is not implemented yet; use plan mode to inspect the migration path",
			direction.From,
			direction.To,
		)
	}

	execution, err := Execute(source, direction, ExecuteOptions{
		Output:            options.Output,
		PreserveStructure: options.PreserveStructure,
		BatchSize:         options.BatchSize,
		Concurrency:       options.Concurrency,
	})
	if err != nil {
		return result, err
	}
	if err := ValidateExecutionResult(execution, direction.Language); err != nil {
		if cleanupErr := CleanupExecutionOutputs(execution); cleanupErr != nil {
			return result, fmt.Errorf("%v (cleanup failed: %w)", err, cleanupErr)
		}
		return result, err
	}

	result.Execution = &execution
	return result, nil
}

func resolveTestMigrationDirection(source string, options TestMigrationOptions) (Direction, *Detection, string, error) {
	from := NormalizeFramework(options.From)
	to := NormalizeFramework(options.To)
	alias := strings.ToLower(strings.TrimSpace(options.Alias))

	if alias != "" {
		aliasDirection, ok := LookupShorthand(alias)
		if !ok {
			return Direction{}, nil, alias, inputErrorf("unknown shorthand: %s", alias)
		}
		if from != "" && from != aliasDirection.From {
			return Direction{}, nil, alias, inputErrorf(
				"shorthand %s maps to %s -> %s, not %s -> %s",
				alias,
				aliasDirection.From,
				aliasDirection.To,
				from,
				emptyOrUnknown(to),
			)
		}
		if to != "" && to != aliasDirection.To {
			return Direction{}, nil, alias, inputErrorf(
				"shorthand %s maps to %s -> %s, not %s -> %s",
				alias,
				aliasDirection.From,
				aliasDirection.To,
				emptyOrUnknown(from),
				to,
			)
		}
		if from == "" {
			from = aliasDirection.From
		}
		if to == "" {
			to = aliasDirection.To
		}
	}

	var detection *Detection
	if from == "" && options.AutoDetect {
		detected, err := DetectSource(source)
		if err != nil {
			return Direction{}, nil, alias, err
		}
		detection = &detected
		if detected.Framework == "" || detected.Framework == "unknown" {
			return Direction{}, nil, alias, fmt.Errorf("could not auto-detect source framework from %s", source)
		}
		from = detected.Framework
	}

	if from == "" {
		return Direction{}, detection, alias, inputErrorf("source framework is required unless auto-detect is enabled")
	}
	if to == "" {
		return Direction{}, detection, alias, inputErrorf("target framework is required")
	}
	direction, err := resolveSupportedDirection(from, to)
	if err != nil {
		return Direction{}, detection, alias, err
	}
	return direction, detection, alias, nil
}

func buildTestMigrationPlan(source string, direction Direction, detection *Detection, alias string, options TestMigrationOptions) TestMigrationPlan {
	mode := "plan"
	if options.DryRun && !options.Plan {
		mode = "dry-run"
	}

	plan := TestMigrationPlan{
		Command:         "convert",
		Mode:            mode,
		Source:          source,
		Output:          options.Output,
		Alias:           alias,
		Direction:       direction,
		SourceDetection: detection,
		ExecutionStatus: "cataloged-not-executable",
		NextStep:        "The Go CLI now owns the conversion catalog, shorthands, and detection contract. Execution for this direction will land in follow-up migration slices.",
	}
	if direction.GoNativeState == GoNativeStateImplemented {
		plan.ExecutionStatus = "executable"
		plan.NextStep = "Run the same request without plan mode to execute the Go-native converter for this direction."
	}
	return plan
}

func inputErrorf(format string, args ...any) error {
	return ConversionInputError{message: fmt.Sprintf(format, args...)}
}

func resolveSupportedDirection(from, to string) (Direction, error) {
	from = NormalizeFramework(from)
	to = NormalizeFramework(to)

	if _, ok := LookupFramework(from); !ok {
		return Direction{}, inputErrorf(
			"invalid source framework: %s. Valid options: %s",
			from,
			strings.Join(FrameworkNames(), ", "),
		)
	}
	if _, ok := LookupFramework(to); !ok {
		return Direction{}, inputErrorf(
			"invalid target framework: %s. Valid options: %s",
			to,
			strings.Join(FrameworkNames(), ", "),
		)
	}
	if from == to {
		return Direction{}, inputErrorf("source and target frameworks must be different")
	}

	direction, ok := LookupDirection(from, to)
	if !ok {
		targets := SupportedTargets(from)
		if len(targets) == 0 {
			return Direction{}, inputErrorf("unsupported source framework: %s", from)
		}
		return Direction{}, inputErrorf(
			"unsupported conversion: %s to %s. Supported targets for %s: %s",
			from,
			to,
			from,
			strings.Join(targets, ", "),
		)
	}
	return direction, nil
}

func emptyOrUnknown(value string) string {
	if strings.TrimSpace(value) == "" {
		return "unknown"
	}
	return value
}
