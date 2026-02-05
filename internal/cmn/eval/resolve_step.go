package eval

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/dagu-org/dagu/internal/cmn/logger"
	"github.com/dagu-org/dagu/internal/cmn/logger/tag"
)

// StepInfo contains metadata about a step that can be accessed via property syntax.
type StepInfo struct {
	Stdout   string
	Stderr   string
	ExitCode string
}

// resolveStepProperty extracts a step's property value with optional slicing.
func resolveStepProperty(ctx context.Context, stepName, path string, stepMap map[string]StepInfo) (string, bool) {
	stepInfo, ok := stepMap[stepName]
	if !ok {
		logger.Debug(ctx, "Step not found in stepMap", tag.Step(stepName))
		return "", false
	}

	property, sliceSpec, err := parseStepReference(path)
	if err != nil {
		logger.Warn(ctx, "Invalid step reference slice",
			tag.Step(stepName),
			tag.Path(path),
			tag.Error(err))
		return "", false
	}

	var value string
	switch property {
	case ".stdout":
		if stepInfo.Stdout == "" {
			logger.Debug(ctx, "Step stdout is empty", tag.Step(stepName))
			return "", false
		}
		value = stepInfo.Stdout
	case ".stderr":
		if stepInfo.Stderr == "" {
			logger.Debug(ctx, "Step stderr is empty", tag.Step(stepName))
			return "", false
		}
		value = stepInfo.Stderr
	case ".exitCode", ".exit_code":
		value = stepInfo.ExitCode
	default:
		return "", false
	}

	if sliceSpec.hasStart || sliceSpec.hasLength {
		value = applyStepSlice(value, sliceSpec)
	}

	return value, true
}

// stepSliceSpec describes a substring slice operation.
type stepSliceSpec struct {
	hasStart  bool
	start     int
	hasLength bool
	length    int
}

// parseStepReference parses a step reference path like ".stdout:0:5" into property and slice spec.
// Returns the property name (e.g., ".stdout") and slice specification (start, length).
func parseStepReference(path string) (string, stepSliceSpec, error) {
	spec := stepSliceSpec{}

	property, sliceNotation, hasSlice := strings.Cut(path, ":")
	if !hasSlice {
		return path, spec, nil
	}

	if sliceNotation == "" {
		return "", spec, fmt.Errorf("slice specification missing values")
	}

	parts := strings.Split(sliceNotation, ":")
	if len(parts) > 2 {
		return "", spec, fmt.Errorf("too many slice sections")
	}

	// Parse start offset (required)
	if parts[0] == "" {
		return "", spec, fmt.Errorf("slice offset is required")
	}

	start, err := strconv.Atoi(parts[0])
	if err != nil {
		return "", spec, fmt.Errorf("invalid slice offset: %w", err)
	}
	if start < 0 {
		return "", spec, fmt.Errorf("slice offset must be non-negative")
	}
	spec.hasStart = true
	spec.start = start

	// Parse length (optional)
	if len(parts) == 2 && parts[1] != "" {
		length, err := strconv.Atoi(parts[1])
		if err != nil {
			return "", spec, fmt.Errorf("invalid slice length: %w", err)
		}
		if length < 0 {
			return "", spec, fmt.Errorf("slice length must be non-negative")
		}
		spec.hasLength = true
		spec.length = length
	}

	return property, spec, nil
}

// applyStepSlice applies substring slicing to a string value based on the slice specification.
// Similar to Python/shell string slicing: value[start:start+length]
func applyStepSlice(value string, spec stepSliceSpec) string {
	if !spec.hasStart {
		return value
	}

	runes := []rune(value)
	if spec.start >= len(runes) {
		return ""
	}

	end := len(runes)
	if spec.hasLength {
		end = min(spec.start+spec.length, len(runes))
	}

	return string(runes[spec.start:end])
}
