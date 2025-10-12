package core

import (
	"fmt"
	"regexp"
	"strings"
)

// StepValidator is a function type for validating step configurations.
type StepValidator func(step Step) error

// ValidateSteps exposes validateSteps for packages that need to perform validation during DAG construction.
func ValidateSteps(dag *DAG) error {
	// First pass: collect all names and IDs
	stepNames := make(map[string]struct{})
	stepIDs := make(map[string]struct{})

	for _, step := range dag.Steps {
		// Names should always exist at this point (explicit or auto-generated)
		if step.Name == "" {
			// This should not happen if generation works correctly
			return WrapError("steps", step, fmt.Errorf("internal error: step name not generated"))
		}

		if _, exists := stepNames[step.Name]; exists {
			return WrapError("steps", step.Name, ErrStepNameDuplicate)
		}
		stepNames[step.Name] = struct{}{}

		// Collect IDs if present
		if step.ID != "" {
			// Check ID format
			if !isValidStepID(step.ID) {
				return WrapError("steps", step.ID, fmt.Errorf("invalid step ID format: must match pattern ^[a-zA-Z][a-zA-Z0-9_-]*$"))
			}

			// Check for duplicate IDs
			if _, exists := stepIDs[step.ID]; exists {
				return WrapError("steps", step.ID, fmt.Errorf("duplicate step ID: %s", step.ID))
			}
			stepIDs[step.ID] = struct{}{}

			// Check for reserved words
			if isReservedWord(step.ID) {
				return WrapError("steps", step.ID, fmt.Errorf("step ID '%s' is a reserved word", step.ID))
			}
		}
	}

	// Second pass: check for conflicts between names and IDs
	for _, step := range dag.Steps {
		if step.ID != "" {
			// Check that ID doesn't conflict with any step name
			if _, exists := stepNames[step.ID]; exists && step.ID != step.Name {
				return WrapError("steps", step.ID, fmt.Errorf("step ID '%s' conflicts with another step's name", step.ID))
			}
		}

		// Check that name doesn't conflict with any ID (unless it's the same step)
		if _, exists := stepIDs[step.Name]; exists {
			// Find if this is the same step
			sameStep := false
			for _, s := range dag.Steps {
				if s.Name == step.Name && s.ID == step.Name {
					sameStep = true
					break
				}
			}
			if !sameStep {
				return WrapError("steps", step.Name, fmt.Errorf("step name '%s' conflicts with another step's ID", step.Name))
			}
		}
	}

	// Third pass: resolve step IDs to names in depends fields
	if err := resolveStepDependencies(dag); err != nil {
		return err
	}

	// Fourth pass: validate dependencies exist
	for _, step := range dag.Steps {
		for _, dep := range step.Depends {
			if _, exists := stepNames[dep]; !exists {
				return WrapError("depends", dep, fmt.Errorf("step %s depends on non-existent step %s", step.Name, dep))
			}
		}
	}

	// Final pass: validate each step
	for _, step := range dag.Steps {
		// Validate individual step configuration
		if err := validateStep(step); err != nil {
			return err
		}
	}

	return nil
}

func validateStep(step Step) error {
	if step.Name == "" {
		return WrapError("name", step.Name, ErrStepNameRequired)
	}

	if len(step.Name) > maxStepNameLen {
		return WrapError("name", step.Name, ErrStepNameTooLong)
	}

	if step.Parallel != nil {
		// Parallel steps must have a run field (child-DAG only for MVP)
		if step.ChildDAG == nil {
			return WrapError("parallel", step.Parallel, fmt.Errorf("parallel execution is only supported for child-DAGs (must have 'run' field)"))
		}

		// MaxConcurrent must be positive
		if step.Parallel.MaxConcurrent <= 0 {
			return WrapError("parallel.maxConcurrent", step.Parallel.MaxConcurrent, fmt.Errorf("maxConcurrent must be greater than 0"))
		}

		// Must have either items or variable reference
		if len(step.Parallel.Items) == 0 && step.Parallel.Variable == "" {
			return WrapError("parallel", step.Parallel, fmt.Errorf("parallel must have either items array or variable reference"))
		}
	}

	// Validate executor-specific configuration if validator exists
	return validateStepWithValidator(step)
}

func validateStepWithValidator(step Step) error {
	validator, exists := executorValidators[step.ExecutorConfig.Type]
	if !exists || validator == nil {
		// No validator registered for this executor type
		return nil
	}
	if err := validator(step); err != nil {
		return WrapError("executorConfig", step.ExecutorConfig, err)
	}
	return nil
}

// maxStepNameLen is the maximum length of a step name.
const maxStepNameLen = 40

// stepIDPattern defines the valid format for step IDs.
var stepIDPattern = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]*$`)

// isValidStepID checks if the given ID matches the required pattern.
func isValidStepID(id string) bool {
	return stepIDPattern.MatchString(id)
}

// isReservedWord checks if the given ID is a reserved word.
func isReservedWord(id string) bool {
	reservedWords := map[string]bool{
		"env":     true,
		"params":  true,
		"args":    true,
		"stdout":  true,
		"stderr":  true,
		"output":  true,
		"outputs": true,
	}
	return reservedWords[strings.ToLower(id)]
}

// resolveStepDependencies resolves step IDs to step names in the depends field.
func resolveStepDependencies(dag *DAG) error {
	idToName := make(map[string]string)
	for i := range dag.Steps {
		step := &dag.Steps[i]
		if step.ID != "" {
			idToName[step.ID] = step.Name
		}
	}

	for i := range dag.Steps {
		step := &dag.Steps[i]
		for j, dep := range step.Depends {
			if name, exists := idToName[dep]; exists {
				step.Depends[j] = name
			}
		}
	}

	return nil
}
